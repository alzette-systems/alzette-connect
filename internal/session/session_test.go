package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ticruz38/alzette-connect/internal/credentialstore"
)

func TestBrowserLoginResumeContextMintAndRevoke(t *testing.T) {
	callback := unusedCallback(t)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	store := credentialstore.NewMemory()
	var server *httptest.Server
	var mu sync.Mutex
	var challenge string
	browserCalls, refreshCalls, revokeCalls := 0, 0, 0
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/alzette-agent-configuration":
			writeJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.agent-configuration.v1", "issuer": server.URL, "oauth_client_id": "connect-test", "control_origin": server.URL, "gateway_base_url": server.URL + "/v1", "oauth_redirect_uri": callback, "login_modes": []string{"authorization_code_pkce_s256"}})
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token",
				"userinfo_endpoint": server.URL + "/api/userinfo", "jwks_uri": server.URL + "/.well-known/jwks",
				"scopes_supported": []string{"openid", "email", "profile", "offline_access"}, "code_challenge_methods_supported": []string{"S256"},
			})
		case "/authorize":
			if r.URL.Query().Get("code_challenge_method") != "S256" || r.URL.Query().Get("state") == "" || r.URL.Query().Get("nonce") == "" || r.URL.Query().Get("redirect_uri") != callback {
				http.Error(w, "bad authorization", http.StatusBadRequest)
				return
			}
			mu.Lock()
			challenge = r.URL.Query().Get("code_challenge")
			mu.Unlock()
			target, _ := url.Parse(callback)
			query := target.Query()
			query.Set("state", r.URL.Query().Get("state"))
			query.Set("code", "one-use-code")
			target.RawQuery = query.Encode()
			http.Redirect(w, r, target.String(), http.StatusFound)
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "authorization_code" {
				digest := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
				mu.Lock()
				valid := challenge == base64.RawURLEncoding.EncodeToString(digest[:])
				mu.Unlock()
				if !valid || r.Form.Get("code") != "one-use-code" {
					http.Error(w, "bad code", http.StatusBadRequest)
					return
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{"access_token": "oauth-access-one", "refresh_token": "refresh-token-one-long", "token_type": "Bearer", "expires_in": 3600, "scope": "openid email profile offline_access"})
				return
			}
			refreshCalls++
			if r.Form.Get("refresh_token") != "refresh-token-one-long" {
				http.Error(w, "bad refresh", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"access_token": "oauth-access-two", "refresh_token": "refresh-token-two-long", "token_type": "Bearer", "expires_in": 3600, "scope": "openid email profile offline_access"})
		case "/api/agent/contexts":
			if r.Header.Get("Authorization") != "Bearer oauth-access-one" && r.Header.Get("Authorization") != "Bearer oauth-access-two" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.agent-contexts.v1", "contexts": []map[string]interface{}{{"membership_id": "mem_test", "organisation": "Example", "project": "Research", "environment": "Production", "relationship": "employee", "model_aliases": []string{"alzette-chat"}}}})
		case "/api/agent/credentials":
			writeJSON(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.agent-credential.v1", "credential": map[string]interface{}{"access_token": "alz_u_01234567890123456789012345678901", "token_type": "Bearer", "expires_at": now.Add(10 * time.Minute), "scope": []string{"inference:write"}}, "context": map[string]interface{}{"membership_id": "mem_test", "organisation": "Example", "project": "Research", "environment": "Production", "relationship": "employee", "model_aliases": []string{"alzette-chat"}}, "gateway_base_url": server.URL + "/v1", "model_aliases": []string{"alzette-chat"}})
		case "/api/agent/credentials/revoke":
			revokeCalls++
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	newSession := func(open func(string) error) *Session {
		value, err := New(Config{ControlURL: server.URL, CallbackURL: callback, Profile: "work", AllowInsecure: true, HTTPClient: server.Client(), Store: store, Clock: func() time.Time { return now }, OpenBrowser: open})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	browser := func(target string) error {
		browserCalls++
		go func() {
			response, err := http.Get(target)
			if err == nil {
				response.Body.Close()
			}
		}()
		return nil
	}
	first := newSession(browser)
	if err := first.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	contexts := first.Contexts()
	if len(contexts) != 1 || contexts[0].ModelAliases[0] != "alzette-chat" || browserCalls != 1 {
		t.Fatalf("contexts=%#v browserCalls=%d", contexts, browserCalls)
	}
	if _, err := first.SelectContext(""); err != nil {
		t.Fatal(err)
	}
	token, expires, err := first.EnsureHumanCredential(context.Background())
	if err != nil || !strings.HasPrefix(token, "alz_u_") || !expires.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("token=%q expires=%s err=%v", token, expires, err)
	}
	if err := first.RevokeGrant(context.Background()); err != nil || revokeCalls != 1 {
		t.Fatalf("revokeCalls=%d err=%v", revokeCalls, err)
	}

	second := newSession(func(string) error { t.Fatal("resume opened browser"); return nil })
	if err := second.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls=%d", refreshCalls)
	}
	stored, err := store.Load(context.Background(), "work")
	if err != nil || stored != "refresh-token-two-long" {
		t.Fatalf("stored=%q err=%v", stored, err)
	}
}

func TestRefreshWithoutRotationFailsClosedAndDeletesCredential(t *testing.T) {
	store := credentialstore.NewMemory()
	_ = store.Save(context.Background(), "work", "refresh-token-one-long")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/alzette-agent-configuration":
			writeJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.agent-configuration.v1", "issuer": server.URL, "oauth_client_id": "connect-test", "control_origin": server.URL, "gateway_base_url": server.URL + "/v1", "login_modes": []string{"authorization_code_pkce_s256"}})
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]string{"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token"})
		case "/token":
			writeJSON(w, http.StatusOK, map[string]interface{}{"access_token": "oauth-access", "refresh_token": "refresh-token-one-long", "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	value, err := New(Config{ControlURL: server.URL, CallbackURL: unusedCallback(t), Profile: "work", AllowInsecure: true, HTTPClient: server.Client(), Store: store, OpenBrowser: func(string) error { return errors.New("must not open") }})
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Connect(context.Background()); !errors.Is(err, ErrSignInRequired) {
		t.Fatalf("connect error=%v", err)
	}
	if _, err := store.Load(context.Background(), "work"); !errors.Is(err, credentialstore.ErrNotFound) {
		t.Fatalf("credential survived ambiguous rotation: %v", err)
	}
}

func TestProviderJSONAllowsExtensionsWhileOwnedJSONStaysStrict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metadata":
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"issuer": "https://identity.example", "authorization_endpoint": "https://identity.example/authorize", "token_endpoint": "https://identity.example/token",
				"userinfo_endpoint": "https://identity.example/api/userinfo", "jwks_uri": "https://identity.example/.well-known/jwks",
			})
		case "/unavailable":
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		case "/invalid":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{not-json}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	discovery, err := readCompatibleJSON[Discovery](context.Background(), server.Client(), server.URL+"/metadata")
	if err != nil || discovery.Issuer != "https://identity.example" {
		t.Fatalf("compatible discovery=%#v err=%v", discovery, err)
	}
	if _, err := readJSON[Discovery](context.Background(), server.Client(), server.URL+"/metadata"); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("strict provider decode error=%v", err)
	}
	if _, err := readCompatibleJSON[Discovery](context.Background(), server.Client(), server.URL+"/unavailable"); err == nil || !strings.Contains(err.Error(), "HTTP status 502") {
		t.Fatalf("status error=%v", err)
	}
	if _, err := readCompatibleJSON[Discovery](context.Background(), server.Client(), server.URL+"/invalid"); err == nil || !strings.Contains(err.Error(), "decode JSON response") {
		t.Fatalf("decode error=%v", err)
	}
}

func TestNewReturnsRandomFailureInsteadOfPanicking(t *testing.T) {
	_, err := New(Config{
		ControlURL:  "https://control.example",
		CallbackURL: "http://127.0.0.1:43127/callback",
		Store:       credentialstore.NewMemory(),
		OpenBrowser: func(string) error { return nil },
		Random:      errorReader{},
	})
	if err == nil || !strings.Contains(err.Error(), "secure random source unavailable") {
		t.Fatalf("New error=%v", err)
	}
}

func TestRevokeDoesNotClaimUnauthorizedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	value := &Session{
		config:         Config{HTTPClient: server.Client()},
		metadata:       Metadata{ControlOrigin: server.URL},
		accessToken:    "oauth-access",
		clientInstance: "aci_test",
		selected:       Context{MembershipID: "mem_test"},
	}
	if err := value.RevokeGrant(context.Background()); !errors.Is(err, ErrSignInRequired) {
		t.Fatalf("revoke error=%v", err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func unusedCallback(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	return "http://" + address + "/callback"
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
