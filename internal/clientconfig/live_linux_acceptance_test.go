//go:build linux

package clientconfig

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/credentialstore"
	"github.com/alzette-systems/alzette-connect/internal/proxy"
	"github.com/alzette-systems/alzette-connect/internal/session"
)

const liveLinuxQAGate = "ALZETTE_CONNECT_LIVE_QA"

// TestLiveLinuxConnectAcceptance exercises the real Connect security core on
// Linux. It is deliberately excluded from ordinary test runs because it logs
// a real Casdoor employee in and sends bounded requests to paid model routes.
// The companion scripts/qa-linux-live.sh command supplies an isolated Secret
// Service session so no persistent developer keyring is used.
func TestLiveLinuxConnectAcceptance(t *testing.T) {
	if os.Getenv(liveLinuxQAGate) != "1" {
		t.Skip("requires explicit live Connect QA approval")
	}
	username := strings.TrimSpace(os.Getenv("ALZETTE_CONNECT_LIVE_QA_USERNAME"))
	password := os.Getenv("ALZETTE_CONNECT_LIVE_QA_PASSWORD")
	expectedModels := liveQAExpectedModels(t, os.Getenv("ALZETTE_CONNECT_LIVE_QA_MODELS"))
	expectedOrganisation := strings.TrimSpace(os.Getenv("ALZETTE_CONNECT_LIVE_QA_ORGANISATION"))
	if username == "" || password == "" {
		t.Fatal("live Connect QA requires the invited employee username and password")
	}
	controlURL := strings.TrimRight(strings.TrimSpace(os.Getenv("ALZETTE_CONNECT_LIVE_QA_CONTROL_URL")), "/")
	if controlURL == "" {
		controlURL = "https://app.alzette.systems"
	}

	store := credentialstore.NewPlatform()
	if store.Kind() != "linux-secret-service" {
		t.Fatalf("live Connect QA requires Linux Secret Service, got %q", store.Kind())
	}
	profile := "qa-" + liveQARandomHex(t, 8)
	defer func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := store.Delete(cleanupContext, profile); err != nil {
			t.Errorf("delete QA credential profile: %v", err)
		}
	}()

	driver := &liveQACasdoorDriver{username: username, password: password, client: &http.Client{Timeout: 20 * time.Second}}
	callbackURL := "http://127.0.0.1:43127/callback"
	first, err := session.New(session.Config{
		ControlURL: controlURL, CallbackURL: callbackURL, Profile: profile,
		Store: store, OpenBrowser: driver.Open, LoginTimeout: 45 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	connectContext, connectCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer connectCancel()
	if err := first.Connect(connectContext); err != nil {
		t.Fatalf("real Casdoor Connect login: %v", err)
	}
	if driver.calls.Load() != 1 {
		t.Fatalf("real Casdoor login driver calls=%d, want 1", driver.calls.Load())
	}
	firstRefresh, err := store.Load(connectContext, profile)
	if err != nil {
		t.Fatalf("load first protected refresh credential: %v", err)
	}
	firstDigest := sha256.Sum256([]byte(firstRefresh))
	firstRefresh = ""

	// A fresh Session must rotate the protected refresh credential without
	// reopening the browser. This mirrors application restart/resume.
	second, err := session.New(session.Config{
		ControlURL: controlURL, CallbackURL: callbackURL, Profile: profile, Store: store,
		OpenBrowser: func(string) error { return errors.New("browser reopened during protected resume") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Connect(connectContext); err != nil {
		t.Fatalf("resume protected Connect login: %v", err)
	}
	secondRefresh, err := store.Load(connectContext, profile)
	if err != nil {
		t.Fatalf("load rotated protected refresh credential: %v", err)
	}
	secondDigest := sha256.Sum256([]byte(secondRefresh))
	secondRefresh = ""
	if firstDigest == secondDigest {
		t.Fatal("protected refresh credential did not rotate on resume")
	}

	selected := liveQASelectContext(t, second.Contexts(), expectedModels, expectedOrganisation)
	if _, err := second.SelectContext(selected.MembershipID); err != nil {
		t.Fatalf("select real employee context: %v", err)
	}
	agentToken, _, err := second.EnsureHumanCredential(connectContext)
	if err != nil {
		t.Fatalf("mint real short-lived application credential: %v", err)
	}
	defer func() {
		revokeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := second.RevokeGrant(revokeContext); err != nil {
			t.Errorf("revoke QA application grant: %v", err)
		}
	}()

	remoteStatus := &liveQAStatusTransport{base: http.DefaultTransport}
	local, err := proxy.Start(proxy.Config{
		Address: "127.0.0.1:0", Provider: second, HTTPTransport: remoteStatus,
		AllowedInferencePaths: []string{"/v1/chat/completions"},
	})
	if err != nil {
		t.Fatalf("start Connect loopback proxy: %v", err)
	}
	localClosed := false
	defer func() {
		if localClosed {
			return
		}
		closeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := local.Close(closeContext); err != nil {
			t.Errorf("close Connect loopback proxy: %v", err)
		}
	}()

	capability := local.Capability()
	client := &http.Client{Timeout: 90 * time.Second}
	liveQAAssertModelDiscovery(t, client, local.BaseURL(), capability, expectedModels)
	liveQAAssertHostileBrowserRequestFails(t, client, local.BaseURL(), capability)
	for _, model := range expectedModels {
		liveQAInference(t, client, local.BaseURL()+"/chat/completions", capability, model)
	}
	if executable := strings.TrimSpace(os.Getenv("ALZETTE_CONNECT_LIVE_QA_PI")); executable != "" {
		liveQAPi(t, executable, local.BaseURL(), capability, expectedModels, agentToken, remoteStatus)
	} else {
		t.Log("Pi path not supplied; real named-client subtest skipped")
	}

	closeContext, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := local.Close(closeContext); err != nil {
		closeCancel()
		t.Fatalf("close Connect loopback proxy: %v", err)
	}
	closeCancel()
	localClosed = true
	if request, requestErr := http.NewRequest(http.MethodGet, local.BaseURL()+"/models", nil); requestErr == nil {
		request.Header.Set("Authorization", "Bearer "+capability)
		if response, requestErr := (&http.Client{Timeout: time.Second}).Do(request); requestErr == nil {
			response.Body.Close()
			t.Fatal("loopback proxy remained reachable after disconnect")
		}
	}
	revokeContext, revokeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := second.RevokeGrant(revokeContext); err != nil {
		revokeCancel()
		t.Fatalf("revoke real application grant: %v", err)
	}
	revokeCancel()
	liveQAAssertRevoked(t, client, second.GatewayBaseURL()+"/chat/completions", agentToken, expectedModels[0])
	agentToken = ""
	capability = ""
	if err := store.Delete(connectContext, profile); err != nil {
		t.Fatalf("delete protected refresh credential: %v", err)
	}
	if _, err := store.Load(connectContext, profile); !errors.Is(err, credentialstore.ErrNotFound) {
		t.Fatalf("protected refresh credential survived logout cleanup: %v", err)
	}
	t.Logf("live Linux Connect acceptance passed with %d real catalogue models and %s", len(expectedModels), PiSupportedVersion)
}

type liveQACasdoorDriver struct {
	username string
	password string
	client   *http.Client
	calls    atomic.Int32
}

func (d *liveQACasdoorDriver) Open(authorizeURL string) error {
	d.calls.Add(1)
	target, err := url.Parse(authorizeURL)
	if err != nil || target.Scheme != "https" || target.Host == "" {
		return errors.New("Casdoor authorization URL is unsafe")
	}
	query := target.Query()
	if query.Get("response_type") != "code" || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("state") == "" || query.Get("nonce") == "" {
		return errors.New("Casdoor authorization request omitted PKCE, state, or nonce")
	}
	redirect, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || redirect.Scheme != "http" || redirect.Hostname() != "127.0.0.1" || redirect.Path != "/callback" {
		return errors.New("Casdoor authorization request used an unsafe callback")
	}
	apiQuery := url.Values{
		"clientId":              {query.Get("client_id")},
		"responseType":          {query.Get("response_type")},
		"redirectUri":           {query.Get("redirect_uri")},
		"type":                  {"code"},
		"scope":                 {query.Get("scope")},
		"state":                 {query.Get("state")},
		"nonce":                 {query.Get("nonce")},
		"code_challenge_method": {query.Get("code_challenge_method")},
		"code_challenge":        {query.Get("code_challenge")},
	}
	var application struct {
		Status string `json:"status"`
		Data   struct {
			Name         string `json:"name"`
			Organization string `json:"organization"`
		} `json:"data"`
	}
	if err := liveQAJSONRequest(d.client, http.MethodGet, target.Scheme+"://"+target.Host+"/api/get-app-login?"+apiQuery.Encode(), nil, &application); err != nil {
		return err
	}
	if application.Status != "ok" || application.Data.Name == "" || application.Data.Organization == "" {
		return errors.New("Casdoor application login configuration is unavailable")
	}
	input := map[string]any{
		"application": application.Data.Name, "organization": application.Data.Organization,
		"username": d.username, "password": d.password, "autoSignin": false,
		"signinMethod": "Password", "type": "code", "language": "en",
	}
	var login struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := liveQAJSONRequest(d.client, http.MethodPost, target.Scheme+"://"+target.Host+"/api/login?"+apiQuery.Encode(), input, &login); err != nil {
		return err
	}
	var code string
	if login.Status != "ok" || json.Unmarshal(login.Data, &code) != nil || len(code) < 16 {
		return errors.New("Casdoor employee login was rejected")
	}
	callbackQuery := redirect.Query()
	callbackQuery.Set("code", code)
	callbackQuery.Set("state", query.Get("state"))
	redirect.RawQuery = callbackQuery.Encode()
	request, _ := http.NewRequest(http.MethodGet, redirect.String(), nil)
	response, err := d.client.Do(request)
	if err != nil {
		return fmt.Errorf("complete loopback callback: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("loopback callback status %d", response.StatusCode)
	}
	return nil
}

func liveQAJSONRequest(client *http.Client, method, target string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Casdoor request status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(output); err != nil {
		return errors.New("Casdoor returned invalid JSON")
	}
	return nil
}

func liveQAExpectedModels(t *testing.T, value string) []string {
	t.Helper()
	parts := strings.Split(value, ",")
	models := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		model := strings.TrimSpace(part)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		models = append(models, model)
	}
	if len(models) == 0 {
		t.Fatal("live Connect QA requires an explicit comma-separated model allow-list")
	}
	sort.Strings(models)
	return models
}

func liveQASelectContext(t *testing.T, contexts []session.Context, expected []string, expectedOrganisation string) session.Context {
	t.Helper()
	for _, candidate := range contexts {
		models := append([]string(nil), candidate.ModelAliases...)
		sort.Strings(models)
		organisationMatches := expectedOrganisation == "" || candidate.Organisation == expectedOrganisation
		if candidate.Relationship == "employee" && organisationMatches && strings.Join(models, "\x00") == strings.Join(expected, "\x00") {
			return candidate
		}
	}
	if expectedOrganisation != "" {
		t.Fatalf("no employee context in %q exposed exactly the %d approved models", expectedOrganisation, len(expected))
	}
	t.Fatalf("no employee relationship context exposed exactly the %d approved models", len(expected))
	return session.Context{}
}

func liveQAAssertModelDiscovery(t *testing.T, client *http.Client, baseURL, capability string, expected []string) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	request.Header.Set("Authorization", "Bearer "+capability)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var listing struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&listing) != nil {
		t.Fatalf("loopback model discovery status=%d", response.StatusCode)
	}
	models := make([]string, 0, len(listing.Data))
	for _, model := range listing.Data {
		models = append(models, model.ID)
	}
	sort.Strings(models)
	if strings.Join(models, "\x00") != strings.Join(expected, "\x00") {
		t.Fatalf("loopback model discovery returned %d models, want %d", len(models), len(expected))
	}
}

func liveQAAssertHostileBrowserRequestFails(t *testing.T, client *http.Client, baseURL, capability string) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	request.Header.Set("Authorization", "Bearer "+capability)
	request.Header.Set("Origin", "https://evil.invalid")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("hostile browser-shaped request status=%d", response.StatusCode)
	}
}

func liveQAInference(t *testing.T, client *http.Client, endpoint, capability, model string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"model": model, "messages": []map[string]string{{"role": "user", "content": "Reply with exactly: OK"}},
		"stream": false, "max_tokens": 8,
	})
	request, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+capability)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("real inference for %s: %v", model, err)
	}
	defer response.Body.Close()
	result, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Alzette-Request-ID") == "" || !json.Valid(result) {
		t.Fatalf("real inference for %s status=%d length=%d", model, response.StatusCode, len(result))
	}
}

func liveQAPi(t *testing.T, executable, baseURL, capability string, models []string, agentToken string, remoteStatus *liveQAStatusTransport) {
	t.Helper()
	executable, err := filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	if version, err := QualifyPi(context.Background(), executable); err != nil || version != PiSupportedVersion {
		t.Fatalf("qualify Pi: version=%q err=%v", version, err)
	}
	root := t.TempDir()
	extension := filepath.Join(root, "alzette.ts")
	if err := os.WriteFile(extension, []byte(piExtensionSource), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := append(launchEnvironment(os.Environ()),
		"ALZETTE_PI_PROXY_URL="+baseURL,
		"ALZETTE_PI_PROXY_KEY="+capability,
		"ALZETTE_PI_MODELS="+liveQAModelJSON(t, models),
		"PI_CODING_AGENT_DIR="+filepath.Join(root, "pi"),
	)
	listContext, listCancel := context.WithTimeout(context.Background(), 20*time.Second)
	list := exec.CommandContext(listContext, executable, "--extension", extension, "--list-models", "deepseek-v4")
	list.Env = environment
	listing, listErr := list.CombinedOutput()
	listCancel()
	if listErr != nil {
		t.Fatalf("Pi model discovery failed with %d output bytes", len(listing))
	}
	for _, model := range models {
		if !bytes.Contains(listing, []byte(model)) {
			t.Fatalf("Pi model discovery omitted %s", model)
		}
	}
	runContext, runCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer runCancel()
	command := exec.CommandContext(runContext, executable,
		"--extension", extension, "--provider", "alzette-employee", "--model", models[0],
		"--no-session", "--no-tools", "--print", "Reply with exactly: OK",
	)
	command.Env = environment
	requestsBefore := remoteStatus.count.Load()
	output, err := command.CombinedOutput()
	for _, secret := range []string{capability, agentToken} {
		if secret != "" && (bytes.Contains(output, []byte(secret)) || strings.Contains(strings.Join(command.Args, "\x00"), secret)) {
			t.Fatal("Pi exposed a Connect credential in output or arguments")
		}
	}
	if err != nil || len(bytes.TrimSpace(output)) == 0 {
		exitCode := -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
		t.Fatalf("Pi real inference failed: exit=%d output_bytes=%d gateway_requests=%d last_gateway_status=%d", exitCode, len(output), remoteStatus.count.Load()-requestsBefore, remoteStatus.status.Load())
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return walkErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if bytes.Contains(data, []byte(capability)) || bytes.Contains(data, []byte(agentToken)) {
			return errors.New("Pi persisted a Connect credential")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type liveQAStatusTransport struct {
	base   http.RoundTripper
	count  atomic.Int64
	status atomic.Int64
}

func (t *liveQAStatusTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if response != nil {
		t.count.Add(1)
		t.status.Store(int64(response.StatusCode))
	}
	return response, err
}

func liveQAAssertRevoked(t *testing.T, client *http.Client, endpoint, agentToken, model string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"model": model, "messages": []map[string]string{{"role": "user", "content": "This request must be rejected."}},
		"stream": false, "max_tokens": 1,
	})
	request, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+agentToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusForbidden {
		t.Fatalf("revoked application credential status=%d", response.StatusCode)
	}
}

func liveQAModelJSON(t *testing.T, models []string) string {
	t.Helper()
	encoded, err := json.Marshal(models)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func liveQARandomHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}
