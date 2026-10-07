package appstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/credentialstore"
	"github.com/alzette-systems/alzette-connect/internal/session"
)

type reconnectFixture struct {
	runtime       *Runtime
	store         *credentialstore.Memory
	server        *httptest.Server
	now           atomic.Int64
	tokenCount    atomic.Int64
	browserCalls  atomic.Int64
	rejectRefresh atomic.Bool
	rejectCode    atomic.Bool
	tokenStatus   atomic.Int64
	mintStatus    atomic.Int64
}

func TestReconnectAfterCallbackPortConflict(t *testing.T) {
	f := newReconnectFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	f.runtime.config.CallbackURL = "http://" + listener.Addr().String() + "/callback"
	if err := f.runtime.Connect(context.Background(), ""); !errors.Is(err, session.ErrCallbackPortInUse) {
		t.Fatalf("occupied callback port: %v", err)
	}
	if snapshot := f.runtime.State().Current(); snapshot.Phase != Failed || snapshot.ErrorCode != "sign_in_port_in_use" {
		t.Fatalf("occupied port state=%s code=%s", snapshot.Phase, snapshot.ErrorCode)
	}
	if f.browserCalls.Load() != 0 {
		t.Fatal("occupied port opened browser authentication")
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	f.connect(t)
	if f.browserCalls.Load() != 1 {
		t.Fatal("retry did not open browser authentication")
	}
}

func newReconnectFixture(t *testing.T) *reconnectFixture {
	t.Helper()
	f := &reconnectFixture{store: credentialstore.NewMemory()}
	f.now.Store(time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC).Unix())
	clock := func() time.Time { return time.Unix(f.now.Load(), 0).UTC() }
	write := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/alzette-agent-configuration":
			write(w, 200, map[string]any{"schema": "alzette.agent-configuration.v1", "issuer": f.server.URL, "oauth_client_id": "connect-test", "control_origin": f.server.URL, "gateway_base_url": f.server.URL + "/v1", "login_modes": []string{"authorization_code_pkce_s256"}})
		case "/.well-known/openid-configuration":
			write(w, 200, map[string]any{"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/authorize", "token_endpoint": f.server.URL + "/token"})
		case "/authorize":
			callback, _ := url.Parse(r.URL.Query().Get("redirect_uri"))
			q := url.Values{"state": {r.URL.Query().Get("state")}, "code": {"test-code"}}
			callback.RawQuery = q.Encode()
			http.Redirect(w, r, callback.String(), http.StatusFound)
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "refresh_token" {
				if status := int(f.tokenStatus.Load()); status != 0 {
					write(w, status, map[string]any{"error": "temporarily_unavailable"})
					return
				}
				if f.rejectRefresh.Load() {
					write(w, 400, map[string]any{"error": "invalid_grant"})
					return
				}
			}
			if r.Form.Get("grant_type") == "authorization_code" && f.rejectCode.Load() {
				write(w, 400, map[string]any{"error": "invalid_grant"})
				return
			}
			n := f.tokenCount.Add(1)
			write(w, 200, map[string]any{"access_token": fmt.Sprintf("oauth-%d", n), "refresh_token": fmt.Sprintf("refresh-token-%d-long", n), "token_type": "Bearer", "expires_in": 3600})
		case "/api/agent/contexts":
			write(w, 200, map[string]any{"schema": "alzette.agent-contexts.v1", "contexts": []map[string]any{{"membership_id": "mem_test", "organisation": "Example", "project": "Test", "environment": "Test", "relationship": "employee", "model_aliases": []string{"chat"}}}})
		case "/api/agent/credentials":
			if status := int(f.mintStatus.Load()); status != 0 {
				w.WriteHeader(status)
				return
			}
			write(w, 201, map[string]any{"schema": "alzette.agent-credential.v1", "credential": map[string]any{"access_token": "alz_u_01234567890123456789012345678901", "token_type": "Bearer", "expires_at": clock().Add(10 * time.Minute), "scope": []string{"inference:write"}}, "context": map[string]any{"membership_id": "mem_test"}, "gateway_base_url": f.server.URL + "/v1", "model_aliases": []string{"chat"}})
		case "/api/agent/credentials/revoke":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	f.server.Client().Timeout = 3 * time.Second
	var err error
	f.runtime, err = NewRuntime(RuntimeConfig{
		ControlURL: f.server.URL, CallbackURL: "http://127.0.0.1:0/callback", ProxyAddress: "127.0.0.1:0", Profile: "test", AllowInsecure: true,
		CredentialStore: f.store, HTTPClient: f.server.Client(), Clock: clock,
		OpenBrowser: func(target string) error {
			f.browserCalls.Add(1)
			go func() {
				response, err := f.server.Client().Get(target)
				if err == nil {
					response.Body.Close()
				}
			}()
			return nil
		},
	}, New(clock()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = f.runtime.Stop(ctx)
	})
	return f
}

func (f *reconnectFixture) connect(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.runtime.Connect(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if got := f.runtime.State().Current().Phase; got != Ready {
		t.Fatalf("phase=%s, want ready", got)
	}
}

func TestReconnectAfterRejectedSessionRefresh(t *testing.T) {
	f := newReconnectFixture(t)
	f.connect(t)
	f.now.Add(3600)
	f.rejectRefresh.Store(true)
	if err := f.runtime.RefreshEndpointStatus(context.Background()); !errors.Is(err, session.ErrSignInRequired) {
		t.Fatalf("refresh error=%v", err)
	}
	if got := f.runtime.State().Current().Phase; got != SignInRequired {
		t.Fatalf("phase=%s", got)
	}
	f.connect(t)
	if f.browserCalls.Load() != 2 {
		t.Fatal("reconnect did not open fresh browser sign-in")
	}
}

func TestInteractiveConnectFallsBackAfterSavedRefreshIsRejected(t *testing.T) {
	f := newReconnectFixture(t)
	if err := f.store.Save(context.Background(), "test", "rejected-refresh-token"); err != nil {
		t.Fatal(err)
	}
	f.rejectRefresh.Store(true)
	f.connect(t)
	if f.browserCalls.Load() != 1 {
		t.Fatal("one click did not start fresh sign-in")
	}
}

func TestResumeRejectedRefreshWaitsForExplicitSignIn(t *testing.T) {
	f := newReconnectFixture(t)
	if err := f.store.Save(context.Background(), "test", "rejected-refresh-token"); err != nil {
		t.Fatal(err)
	}
	f.rejectRefresh.Store(true)
	if _, err := f.runtime.Resume(context.Background(), ""); !errors.Is(err, session.ErrSignInRequired) {
		t.Fatalf("resume error=%v", err)
	}
	if f.browserCalls.Load() != 0 {
		t.Fatal("startup opened browser")
	}
	f.connect(t)
}

func TestReconnectAfterOfflineApplicationLaunch(t *testing.T) {
	f := newReconnectFixture(t)
	f.connect(t)
	f.mintStatus.Store(503)
	if err := f.runtime.StartLaunch(context.Background()); err == nil {
		t.Fatal("offline launch succeeded")
	}
	if got := f.runtime.State().Current().Phase; got != Offline {
		t.Fatalf("phase=%s", got)
	}
	f.mintStatus.Store(0)
	f.connect(t)
	if f.browserCalls.Load() != 1 {
		t.Fatal("retry discarded a valid saved sign-in")
	}
}

func TestTransientRefreshFailurePreservesLoginAndRecovers(t *testing.T) {
	f := newReconnectFixture(t)
	f.connect(t)
	saved, _ := f.store.Load(context.Background(), "test")
	f.now.Add(3600)
	f.tokenStatus.Store(503)
	err := f.runtime.RefreshEndpointStatus(context.Background())
	if err == nil || errors.Is(err, session.ErrSignInRequired) {
		t.Fatalf("transient refresh error=%v", err)
	}
	if got := f.runtime.State().Current().Phase; got != Offline {
		t.Fatalf("phase=%s, want offline", got)
	}
	if err := f.runtime.Connect(context.Background(), ""); err == nil {
		t.Fatal("offline reconnect succeeded")
	}
	retained, err := f.store.Load(context.Background(), "test")
	if err != nil || retained != saved {
		t.Fatal("temporary outage deleted saved login")
	}
	f.tokenStatus.Store(0)
	f.connect(t)
	if f.browserCalls.Load() != 1 {
		t.Fatal("network recovery required another browser login")
	}
}

func TestReconnectDoesNotReplaceActiveClientSession(t *testing.T) {
	f := newReconnectFixture(t)
	f.connect(t)
	if err := f.runtime.StartLaunch(context.Background()); err != nil {
		t.Fatal(err)
	}
	base, capability, _, ok := f.runtime.ClientConnection()
	if !ok {
		t.Fatal("no active client connection")
	}
	f.now.Add(3600)
	f.rejectRefresh.Store(true)
	_ = f.runtime.RefreshEndpointStatus(context.Background())
	if err := f.runtime.Connect(context.Background(), ""); err == nil {
		t.Fatal("reconnect replaced an active client session")
	}
	afterBase, afterCapability, _, afterOK := f.runtime.ClientConnection()
	if !afterOK || base != afterBase || capability != afterCapability {
		t.Fatal("reconnect changed the supervised connection")
	}
	if f.browserCalls.Load() != 1 {
		t.Fatal("active-session reconnect opened browser")
	}
	if err := f.runtime.StopLaunch(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Disconnect must retain the sign-in-required state, so reauthentication can proceed.
	f.connect(t)
}

func TestBackgroundRefreshRecoversFromTemporaryOutage(t *testing.T) {
	f := newReconnectFixture(t)
	f.connect(t)
	f.now.Add(3600)
	f.tokenStatus.Store(503)
	if err := f.runtime.RefreshEndpointStatus(context.Background()); err == nil {
		t.Fatal("outage was not detected")
	}
	f.tokenStatus.Store(0)
	if err := f.runtime.RefreshEndpointStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.runtime.State().Current().Phase; got != Ready {
		t.Fatalf("recovery phase=%s", got)
	}
	if f.browserCalls.Load() != 1 {
		t.Fatal("status recovery opened browser")
	}
}

func TestReconnectCancellationAndConcurrentActionsRemainRetryable(t *testing.T) {
	f := newReconnectFixture(t)
	f.connect(t)
	f.now.Add(3600)
	f.rejectRefresh.Store(true)
	_ = f.runtime.RefreshEndpointStatus(context.Background())
	originalBrowser := f.runtime.config.OpenBrowser
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	f.runtime.config.OpenBrowser = func(string) error { close(started); return nil }
	result := make(chan error, 1)
	go func() { result <- f.runtime.Connect(ctx, "") }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not reach browser")
	}
	if err := f.runtime.Connect(context.Background(), ""); err == nil {
		t.Fatal("concurrent sign-in was accepted")
	}
	if err := f.runtime.StartLaunch(context.Background()); err == nil {
		t.Fatal("launch during sign-in was accepted")
	}
	refresh := make(chan error, 1)
	go func() { refresh <- f.runtime.RefreshEndpointStatus(context.Background()) }()
	select {
	case err := <-refresh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("status poll blocked behind browser login")
	}
	if got := f.runtime.State().Current().Phase; got != SigningIn {
		t.Fatalf("status poll replaced signing-in state: %s", got)
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, session.ErrSignInCancelled) {
			t.Fatalf("cancel error=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("sign-in cancellation blocked")
	}
	if got := f.runtime.State().Current().Phase; got != SignInRequired {
		t.Fatalf("cancel phase=%s", got)
	}
	f.runtime.config.OpenBrowser = originalBrowser
	f.connect(t)
}

func TestRejectedBrowserCodeDoesNotRestartBrowserLogin(t *testing.T) {
	f := newReconnectFixture(t)
	f.rejectCode.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.runtime.Connect(ctx, ""); !errors.Is(err, session.ErrSignInRequired) {
		t.Fatalf("sign-in error=%v", err)
	}
	if f.browserCalls.Load() != 1 {
		t.Fatal("rejected browser code restarted authentication")
	}
	f.rejectCode.Store(false)
	f.connect(t)
}
