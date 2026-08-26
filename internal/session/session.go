package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ticruz38/alzette-connect/internal/credentialstore"
)

var (
	ErrSignInRequired        = errors.New("Alzette sign-in is required")
	ErrSignInCancelled       = errors.New("Alzette sign-in was not completed")
	ErrSignInTimeout         = errors.New("Alzette sign-in timed out")
	ErrAccessRemoved         = errors.New("Alzette model access is unavailable")
	ErrCredentialUnavailable = errors.New("Alzette could not start a private application session")
)

type Config struct {
	ControlURL    string
	CallbackURL   string
	Profile       string
	AllowInsecure bool
	HTTPClient    *http.Client
	OpenBrowser   func(string) error
	Store         credentialstore.Store
	Clock         func() time.Time
	Random        io.Reader
	LoginTimeout  time.Duration
}

type Session struct {
	config    Config
	metadata  Metadata
	discovery Discovery

	mu             sync.Mutex
	accessToken    string
	accessExpires  time.Time
	contexts       []Context
	selected       Context
	clientInstance string
	grantRevoked   bool
	humanToken     string
	humanExpires   time.Time
}

func New(config Config) (*Session, error) {
	if err := prepareConfig(&config); err != nil {
		return nil, err
	}
	clientInstance, err := opaque(config.Random, "aci", 16)
	if err != nil {
		return nil, fmt.Errorf("create client identity: %w", err)
	}
	return &Session{config: config, clientInstance: clientInstance}, nil
}

// Connect resumes a protected rotating login when present. With no stored
// login it performs one system-browser Authorization Code + PKCE flow.
func (s *Session) Connect(ctx context.Context) error {
	if err := s.discover(ctx); err != nil {
		return err
	}
	release, err := s.config.Store.Acquire(ctx, s.config.Profile)
	if err != nil {
		return err
	}
	defer release()
	refresh, err := s.config.Store.Load(ctx, s.config.Profile)
	switch {
	case err == nil:
		if err := s.rotateRefreshLocked(ctx, refresh); err != nil {
			_ = s.config.Store.Delete(context.Background(), s.config.Profile)
			return ErrSignInRequired
		}
	case errors.Is(err, credentialstore.ErrNotFound):
		tokens, authErr := s.browserAuthorization(ctx)
		if authErr != nil {
			return authErr
		}
		if tokens.RefreshToken == "" {
			return errors.New("identity service did not issue a rotating refresh credential")
		}
		if err := s.config.Store.Save(ctx, s.config.Profile, tokens.RefreshToken); err != nil {
			return err
		}
		s.setOAuth(tokens)
	default:
		return err
	}
	return s.loadContexts(ctx)
}

func (s *Session) Contexts() []Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneContexts(s.contexts)
}

func (s *Session) SelectContext(membershipID string) (Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if membershipID == "" && len(s.contexts) == 1 {
		s.selected = s.contexts[0]
		return cloneContext(s.selected), nil
	}
	for _, value := range s.contexts {
		if value.MembershipID == membershipID {
			s.selected = value
			return cloneContext(value), nil
		}
	}
	return Context{}, ErrAccessRemoved
}

func (s *Session) SelectedContext() Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneContext(s.selected)
}

func (s *Session) GatewayBaseURL() string { return s.metadata.GatewayBaseURL }

func (s *Session) SelectedModels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.selected.ModelAliases...)
}

func (s *Session) SelectedModelCatalog() []Model {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneModels(s.selected.Models)
}

func (s *Session) EnsureHumanCredential(ctx context.Context) (string, time.Time, error) {
	s.mu.Lock()
	if s.grantRevoked {
		next, err := opaque(s.config.Random, "aci", 16)
		if err != nil {
			s.mu.Unlock()
			return "", time.Time{}, fmt.Errorf("rotate client identity: %w", err)
		}
		s.clientInstance = next
		s.grantRevoked = false
	}
	selected := cloneContext(s.selected)
	humanToken, humanExpires, accessExpires := s.humanToken, s.humanExpires, s.accessExpires
	s.mu.Unlock()
	if selected.MembershipID == "" {
		return "", time.Time{}, ErrAccessRemoved
	}
	now := s.config.Clock().UTC()
	if humanToken != "" && humanExpires.Sub(now) > 45*time.Second {
		return humanToken, humanExpires, nil
	}
	if accessExpires.Sub(now) <= 45*time.Second {
		if err := s.refreshUnderStoreLock(ctx); err != nil {
			return "", time.Time{}, err
		}
	}
	s.mu.Lock()
	accessToken := s.accessToken
	clientInstance := s.clientInstance
	s.mu.Unlock()
	input := MintInput{ClientInstanceID: clientInstance, MembershipID: selected.MembershipID, ModelAliases: append([]string(nil), selected.ModelAliases...)}
	body, _ := json.Marshal(input)
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.metadata.ControlOrigin+"/api/agent/credentials", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	idempotencyKey, err := opaque(s.config.Random, "agm", 16)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create request identity: %w", err)
	}
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("mint Alzette session credential: %w", err)
	}
	defer response.Body.Close()
	var result mintResponse
	if response.StatusCode == http.StatusUnauthorized {
		s.mu.Lock()
		s.humanToken, s.humanExpires = "", time.Time{}
		s.mu.Unlock()
		return "", time.Time{}, ErrSignInRequired
	}
	if response.StatusCode == http.StatusForbidden {
		s.mu.Lock()
		s.humanToken, s.humanExpires = "", time.Time{}
		s.mu.Unlock()
		// A mint denial is not by itself proof of offboarding. Re-read the
		// employee's current contexts before showing the terminal access-ended
		// state. If the exact membership and aliases remain present, the failure
		// is limited to this application session and can be retried safely.
		if err := s.loadContexts(ctx); err == nil && s.contextStillAvailable(selected) {
			return "", time.Time{}, ErrCredentialUnavailable
		}
		return "", time.Time{}, ErrAccessRemoved
	}
	if response.StatusCode != http.StatusCreated || decodeJSON(response.Body, &result) != nil || result.Schema != "alzette.agent-credential.v1" || !validHumanToken(result.Credential.AccessToken) || !strings.EqualFold(result.Credential.TokenType, "Bearer") || !sameStrings(result.Credential.Scope, []string{"inference:write"}) {
		return "", time.Time{}, errors.New("Alzette could not create a session credential")
	}
	if !sameURL(result.GatewayBaseURL, s.metadata.GatewayBaseURL) || result.Context.MembershipID != selected.MembershipID || !sameStrings(result.ModelAliases, selected.ModelAliases) {
		return "", time.Time{}, errors.New("Alzette returned an unexpected credential scope")
	}
	remaining := result.Credential.ExpiresAt.Sub(now)
	if remaining <= 0 || remaining > 10*time.Minute+30*time.Second {
		return "", time.Time{}, errors.New("Alzette returned an invalid credential lifetime")
	}
	s.mu.Lock()
	s.humanToken, s.humanExpires = result.Credential.AccessToken, result.Credential.ExpiresAt.UTC()
	token, expires := s.humanToken, s.humanExpires
	s.mu.Unlock()
	return token, expires, nil
}

func (s *Session) RevokeGrant(ctx context.Context) error {
	s.mu.Lock()
	selected := s.selected
	accessToken := s.accessToken
	instance := s.clientInstance
	// A disconnected local application must never reuse a prior human token,
	// even when remote revocation cannot be confirmed. The server remains the
	// authority for the outstanding grant; a later launch must mint again.
	s.humanToken, s.humanExpires = "", time.Time{}
	s.mu.Unlock()
	if selected.MembershipID == "" || accessToken == "" {
		return nil
	}
	body, _ := json.Marshal(MintInput{ClientInstanceID: instance, MembershipID: selected.MembershipID})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.metadata.ControlOrigin+"/api/agent/credentials/revoke", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return ErrSignInRequired
	}
	if response.StatusCode == http.StatusForbidden {
		return ErrAccessRemoved
	}
	if response.StatusCode != http.StatusNoContent {
		return errors.New("Alzette session revocation failed")
	}
	s.mu.Lock()
	s.grantRevoked = true
	s.mu.Unlock()
	return nil
}

func (s *Session) contextStillAvailable(expected Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, current := range s.contexts {
		if current.MembershipID == expected.MembershipID && sameStrings(current.ModelAliases, expected.ModelAliases) {
			s.selected = current
			return true
		}
	}
	return false
}

// Logout always clears the protected local refresh credential. Callers must
// report a remote revocation error separately rather than claiming a confirmed
// global logout.
func (s *Session) Logout(ctx context.Context) error {
	revokeErr := s.RevokeGrant(ctx)
	deleteErr := s.config.Store.Delete(context.Background(), s.config.Profile)
	s.mu.Lock()
	s.accessToken, s.humanToken = "", ""
	s.accessExpires, s.humanExpires = time.Time{}, time.Time{}
	s.mu.Unlock()
	if revokeErr != nil {
		return fmt.Errorf("remote grant revocation was not confirmed: %w", revokeErr)
	}
	return deleteErr
}

func (s *Session) discover(ctx context.Context) error {
	metadata, err := readJSON[Metadata](ctx, s.config.HTTPClient, s.config.ControlURL+"/.well-known/alzette-agent-configuration")
	if err != nil {
		return fmt.Errorf("read Alzette agent configuration: %w", err)
	}
	if metadata.Schema != "alzette.agent-configuration.v1" || metadata.OAuthClientID == "" || metadata.ControlOrigin == "" || metadata.GatewayBaseURL == "" || metadata.Issuer == "" || !contains(metadata.LoginModes, "authorization_code_pkce_s256") {
		return errors.New("Alzette agent configuration is incomplete")
	}
	if !sameURL(metadata.ControlOrigin, s.config.ControlURL) || validateServerURL(metadata.ControlOrigin, s.config.AllowInsecure) != nil || validateServerURL(metadata.GatewayBaseURL, s.config.AllowInsecure) != nil || validateServerURL(metadata.Issuer, s.config.AllowInsecure) != nil {
		return errors.New("Alzette agent configuration is unsafe")
	}
	discovery, err := readCompatibleJSON[Discovery](ctx, s.config.HTTPClient, strings.TrimRight(metadata.Issuer, "/")+"/.well-known/openid-configuration")
	if err != nil {
		return fmt.Errorf("discover Alzette identity service: %w", err)
	}
	if discovery.Issuer != metadata.Issuer || !sameOrigin(metadata.Issuer, discovery.AuthorizationEndpoint) || !sameOrigin(metadata.Issuer, discovery.TokenEndpoint) {
		return errors.New("identity discovery did not match the configured issuer")
	}
	s.metadata, s.discovery = metadata, discovery
	return nil
}

func (s *Session) loadContexts(ctx context.Context) error {
	s.mu.Lock()
	access := s.accessToken
	s.mu.Unlock()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.metadata.ControlOrigin+"/api/agent/contexts", nil)
	request.Header.Set("Authorization", "Bearer "+access)
	request.Header.Set("Accept", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("read Alzette contexts: %w", err)
	}
	defer response.Body.Close()
	var result contextsResponse
	if response.StatusCode != http.StatusOK || decodeJSON(response.Body, &result) != nil || result.Schema != "alzette.agent-contexts.v1" {
		return ErrAccessRemoved
	}
	for index := range result.Contexts {
		normalizeContext(&result.Contexts[index])
	}
	sort.Slice(result.Contexts, func(i, j int) bool {
		left, right := result.Contexts[i], result.Contexts[j]
		return left.Organisation+left.Project+left.Environment+left.MembershipID < right.Organisation+right.Project+right.Environment+right.MembershipID
	})
	s.mu.Lock()
	s.contexts = cloneContexts(result.Contexts)
	s.mu.Unlock()
	return nil
}

func (s *Session) refreshUnderStoreLock(ctx context.Context) error {
	release, err := s.config.Store.Acquire(ctx, s.config.Profile)
	if err != nil {
		return err
	}
	defer release()
	refresh, err := s.config.Store.Load(ctx, s.config.Profile)
	if err != nil {
		if errors.Is(err, credentialstore.ErrNotFound) {
			return ErrSignInRequired
		}
		return err
	}
	return s.rotateRefreshLocked(ctx, refresh)
}

func (s *Session) rotateRefreshLocked(ctx context.Context, refresh string) error {
	tokens, err := s.exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {s.metadata.OAuthClientID}, "refresh_token": {refresh}})
	if err != nil || tokens.RefreshToken == "" || subtle.ConstantTimeCompare([]byte(tokens.RefreshToken), []byte(refresh)) == 1 {
		return ErrSignInRequired
	}
	if err := s.config.Store.Save(ctx, s.config.Profile, tokens.RefreshToken); err != nil {
		_ = s.config.Store.Delete(context.Background(), s.config.Profile)
		return ErrSignInRequired
	}
	s.setOAuth(tokens)
	return nil
}

func (s *Session) setOAuth(tokens oauthTokens) {
	s.mu.Lock()
	s.accessToken = tokens.AccessToken
	s.accessExpires = s.config.Clock().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	s.mu.Unlock()
}

func prepareConfig(config *Config) error {
	config.ControlURL = strings.TrimRight(strings.TrimSpace(config.ControlURL), "/")
	config.CallbackURL = strings.TrimSpace(config.CallbackURL)
	if config.Profile == "" {
		config.Profile = "default"
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.Random == nil {
		config.Random = rand.Reader
	}
	if config.LoginTimeout == 0 {
		config.LoginTimeout = 3 * time.Minute
	}
	if config.OpenBrowser == nil || config.Store == nil {
		return errors.New("browser opener and protected credential store are required")
	}
	if config.HTTPClient == nil {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return errors.New("default HTTP transport is unavailable")
		}
		transport := base.Clone()
		config.HTTPClient = &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	}
	if err := validateServerURL(config.ControlURL, config.AllowInsecure); err != nil {
		return fmt.Errorf("control URL: %w", err)
	}
	callback, err := url.Parse(config.CallbackURL)
	if err != nil || callback.Scheme != "http" || callback.User != nil || callback.RawQuery != "" || callback.Fragment != "" || callback.Path == "" || (callback.Hostname() != "127.0.0.1" && callback.Hostname() != "::1") {
		return errors.New("OAuth callback must be an exact loopback HTTP URL")
	}
	if port, err := strconv.Atoi(callback.Port()); err != nil || port < 0 || port > 65535 {
		return errors.New("OAuth callback must include a valid port")
	}
	return nil
}

func validateServerURL(raw string, allowInsecure bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("URL is invalid")
	}
	if parsed.Scheme != "https" {
		if !allowInsecure || parsed.Scheme != "http" || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" && parsed.Hostname() != "localhost") {
			return errors.New("URL must use HTTPS")
		}
	}
	return nil
}

func sameOrigin(origin, target string) bool {
	left, leftErr := url.Parse(origin)
	right, rightErr := url.Parse(target)
	return leftErr == nil && rightErr == nil && left.Scheme == right.Scheme && left.Host == right.Host && right.User == nil
}

func sameURL(left, right string) bool {
	return strings.TrimRight(left, "/") == strings.TrimRight(right, "/")
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func normalizeContext(value *Context) {
	sort.Strings(value.ModelAliases)
	byAlias := make(map[string]Model, len(value.Models))
	for _, model := range value.Models {
		if model.Alias == "" || !contains(value.ModelAliases, model.Alias) {
			continue
		}
		if model.DisplayName == "" {
			model.DisplayName = model.Alias
		}
		sort.Strings(model.Capabilities)
		byAlias[model.Alias] = cloneModel(model)
	}
	value.Models = make([]Model, 0, len(value.ModelAliases))
	for _, alias := range value.ModelAliases {
		model, ok := byAlias[alias]
		if !ok {
			model = Model{Alias: alias, DisplayName: alias}
		}
		value.Models = append(value.Models, model)
	}
}

func cloneModel(value Model) Model {
	value.Capabilities = append([]string(nil), value.Capabilities...)
	if value.ContextWindowTokens != nil {
		contextWindow := *value.ContextWindowTokens
		value.ContextWindowTokens = &contextWindow
	}
	return value
}

func cloneModels(values []Model) []Model {
	result := make([]Model, len(values))
	for index, value := range values {
		result[index] = cloneModel(value)
	}
	return result
}

func cloneContext(value Context) Context {
	value.ModelAliases = append([]string(nil), value.ModelAliases...)
	value.Models = cloneModels(value.Models)
	return value
}
func cloneContexts(values []Context) []Context {
	result := append([]Context(nil), values...)
	for index := range result {
		result[index] = cloneContext(result[index])
	}
	return result
}
func sameStrings(left, right []string) bool {
	a, b := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func validHumanToken(value string) bool {
	return strings.HasPrefix(value, "alz_u_") && len(value) >= 32 && len(value) <= 256
}

func opaque(source io.Reader, prefix string, size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(source, buffer); err != nil {
		return "", errors.New("secure random source unavailable")
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func readJSON[T any](ctx context.Context, client *http.Client, target string) (T, error) {
	return readJSONWith[T](ctx, client, target, decodeJSON)
}

// readCompatibleJSON accepts extension members in provider-owned protocol
// documents. OIDC discovery and OAuth responses are explicitly extensible;
// Alzette-owned schemas continue to use readJSON and reject unknown fields.
func readCompatibleJSON[T any](ctx context.Context, client *http.Client, target string) (T, error) {
	return readJSONWith[T](ctx, client, target, decodeCompatibleJSON)
}

func readJSONWith[T any](ctx context.Context, client *http.Client, target string, decode func(io.Reader, interface{}) error) (T, error) {
	var result T
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	if err := decode(response.Body, &result); err != nil {
		return result, fmt.Errorf("decode JSON response: %w", err)
	}
	return result, nil
}

func decodeJSON(reader io.Reader, target interface{}) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	return decodeJSONDocument(decoder, target)
}

func decodeCompatibleJSON(reader io.Reader, target interface{}) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	return decodeJSONDocument(decoder, target)
}

func decodeJSONDocument(decoder *json.Decoder, target interface{}) error {
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected extra JSON value")
	}
	return nil
}
