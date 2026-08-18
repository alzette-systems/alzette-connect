package appstate

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ticruz38/alzette-connect/internal/credentialstore"
	connectplatform "github.com/ticruz38/alzette-connect/internal/platform"
	"github.com/ticruz38/alzette-connect/internal/proxy"
	"github.com/ticruz38/alzette-connect/internal/session"
)

type RuntimeConfig struct {
	ControlURL      string
	CallbackURL     string
	ProxyAddress    string
	Profile         string
	AllowInsecure   bool
	CredentialStore credentialstore.Store
	HTTPClient      *http.Client
	OpenBrowser     func(string) error
	Clock           func() time.Time
	Random          io.Reader
}

// Runtime owns all credential-bearing objects. A frontend receives Model
// snapshots and invokes high-level methods, never ClientConnection.
type Runtime struct {
	config RuntimeConfig
	state  *Model

	mu         sync.Mutex
	connecting bool
	session    *session.Session
	proxy      *proxy.Server
}

func NewRuntime(config RuntimeConfig, state *Model) (*Runtime, error) {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.CredentialStore == nil {
		config.CredentialStore = credentialstore.NewPlatform()
	}
	if config.OpenBrowser == nil {
		config.OpenBrowser = connectplatform.OpenBrowser
	}
	if config.ProxyAddress == "" {
		config.ProxyAddress = "127.0.0.1:43128"
	}
	if config.CallbackURL == "" {
		config.CallbackURL = "http://127.0.0.1:43127/callback"
	}
	if state == nil {
		state = New(config.Clock())
	}
	return &Runtime{config: config, state: state}, nil
}

func (r *Runtime) State() *Model { return r.state }

// Resume reconnects only when a protected refresh credential already exists.
// It never opens a browser on application startup for a first-time user.
func (r *Runtime) Resume(ctx context.Context, membershipID string) (bool, error) {
	if _, err := r.config.CredentialStore.Load(ctx, r.config.Profile); err != nil {
		if errors.Is(err, credentialstore.ErrNotFound) {
			r.set(SignInRequired, "Sign in to connect your company models", "", nil)
			return false, nil
		}
		r.fail(err)
		return false, err
	}
	return true, r.Connect(ctx, membershipID)
}

func (r *Runtime) Connect(ctx context.Context, membershipID string) error {
	r.mu.Lock()
	if r.connecting || r.session != nil || r.proxy != nil {
		r.mu.Unlock()
		return errors.New("Alzette Connect is already running")
	}
	r.connecting = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.connecting = false
		r.mu.Unlock()
	}()
	r.set(SigningIn, "Opening your company sign-in", "", nil)
	connected, err := session.New(session.Config{
		ControlURL: r.config.ControlURL, CallbackURL: r.config.CallbackURL,
		Profile: r.config.Profile, AllowInsecure: r.config.AllowInsecure,
		HTTPClient: r.config.HTTPClient, OpenBrowser: r.config.OpenBrowser,
		Store: r.config.CredentialStore, Clock: r.config.Clock, Random: r.config.Random,
	})
	if err != nil {
		r.fail(err)
		return err
	}
	if err := connected.Connect(ctx); err != nil {
		r.fail(err)
		return err
	}
	contexts := connected.Contexts()
	if len(contexts) == 0 {
		r.set(NoAccess, "Your company has not assigned a model yet", "no_model_access", contexts)
		return session.ErrAccessRemoved
	}
	if _, err := connected.SelectContext(membershipID); err != nil {
		r.set(NoAccess, "Choose an available company workspace", "context_selection_required", contexts)
		return err
	}
	if _, _, err := connected.EnsureHumanCredential(ctx); err != nil {
		r.fail(err)
		return err
	}
	local, err := proxy.Start(proxy.Config{Address: r.config.ProxyAddress, Provider: connected, Random: r.config.Random})
	if err != nil {
		revokeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = connected.RevokeGrant(revokeCtx)
		cancel()
		r.set(Failed, "The private connection could not start", "local_proxy_unavailable", contexts)
		return err
	}
	r.mu.Lock()
	r.session, r.proxy = connected, local
	r.mu.Unlock()
	r.set(Ready, "Your private connection is ready", "", contexts)
	return nil
}

// ClientConnection is private runtime material for a trusted native adapter or
// child launcher. It must never be bound to Wails/frontend IPC.
func (r *Runtime) ClientConnection() (baseURL, capability string, models []string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proxy == nil || r.session == nil {
		return "", "", nil, false
	}
	return r.proxy.BaseURL(), r.proxy.Capability(), r.session.SelectedModels(), true
}

func (r *Runtime) Stop(ctx context.Context) error {
	r.set(Stopping, "Stopping Alzette Connect", "", nil)
	r.mu.Lock()
	local, connected := r.proxy, r.session
	r.proxy, r.session = nil, nil
	r.mu.Unlock()
	var closeErr, revokeErr error
	if local != nil {
		closeErr = local.Close(ctx)
	}
	if connected != nil {
		revokeErr = connected.RevokeGrant(ctx)
	}
	r.set(SignInRequired, "Alzette Connect is stopped", "", nil)
	if closeErr != nil {
		return closeErr
	}
	return revokeErr
}

func (r *Runtime) Logout(ctx context.Context) error {
	r.set(Stopping, "Signing out of Alzette Connect", "", nil)
	r.mu.Lock()
	local, connected := r.proxy, r.session
	r.proxy, r.session = nil, nil
	r.mu.Unlock()
	var closeErr, logoutErr error
	if local != nil {
		closeErr = local.Close(ctx)
	}
	if connected != nil {
		logoutErr = connected.Logout(ctx)
	} else {
		logoutErr = r.config.CredentialStore.Delete(context.Background(), r.config.Profile)
	}
	r.set(SignInRequired, "You are signed out", "", nil)
	if logoutErr != nil {
		return logoutErr
	}
	return closeErr
}

func (r *Runtime) set(phase Phase, message, errorCode string, values []session.Context) {
	contexts := make([]Context, 0, len(values))
	for _, value := range values {
		contexts = append(contexts, Context{ID: value.MembershipID, Organisation: value.Organisation, Project: value.Project, Environment: value.Environment, Models: append([]string(nil), value.ModelAliases...)})
	}
	r.state.Set(Snapshot{Phase: phase, Message: message, ErrorCode: errorCode, Contexts: contexts, UpdatedAt: r.config.Clock().UTC()})
}

func (r *Runtime) fail(err error) {
	switch {
	case errors.Is(err, session.ErrSignInCancelled), errors.Is(err, session.ErrSignInTimeout), errors.Is(err, context.Canceled):
		r.set(SignInRequired, "Sign-in was not completed", "sign_in_cancelled", nil)
	case errors.Is(err, session.ErrSignInRequired):
		r.set(SignInRequired, "Sign in to continue", "sign_in_required", nil)
	case errors.Is(err, session.ErrAccessRemoved):
		r.set(AccessRemoved, "Your company access has ended", "access_removed", nil)
	case errors.Is(err, credentialstore.ErrUnavailable):
		r.set(Failed, "Protected sign-in storage is unavailable", "credential_store_unavailable", nil)
	default:
		r.set(Offline, "Alzette Connect could not reach the service", "service_unavailable", nil)
	}
}
