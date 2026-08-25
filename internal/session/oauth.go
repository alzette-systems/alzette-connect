package session

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const callbackHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="referrer" content="no-referrer"><title>Alzette sign-in complete</title><style>:root{color-scheme:light}*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:#f6f4ef;color:#10151a;font:16px/1.5 system-ui,-apple-system,sans-serif}main{width:min(520px,100%);border:1px solid #c9ccc8;background:#fff;padding:48px}b{display:block;margin-bottom:32px;color:#008c62;font:700 13px/1.2 ui-monospace,monospace;letter-spacing:.08em;text-transform:uppercase}h1{margin:0 0 16px;font-size:clamp(32px,7vw,52px);line-height:1}p{margin:0;color:#55616a}</style></head><body><main><b>Alzette Connect</b><h1>You’re signed in.</h1><p>You can close this tab and return to Alzette Connect.</p></main></body></html>`

func (s *Session) browserAuthorization(ctx context.Context) (oauthTokens, error) {
	configured, _ := url.Parse(s.config.CallbackURL)
	listener, err := net.Listen("tcp", configured.Host)
	if err != nil {
		return oauthTokens{}, fmt.Errorf("listen for OAuth callback: %w", err)
	}
	defer listener.Close()
	actual := *configured
	actual.Host = listener.Addr().String()
	redirectURL := actual.String()
	if s.metadata.OAuthRedirect != "" && s.metadata.OAuthRedirect != redirectURL {
		return oauthTokens{}, errors.New("Alzette requires a different registered loopback callback")
	}
	state, err := opaque(s.config.Random, "state", 32)
	if err != nil {
		return oauthTokens{}, err
	}
	verifier, err := opaque(s.config.Random, "pkce", 48)
	if err != nil {
		return oauthTokens{}, err
	}
	nonce, err := opaque(s.config.Random, "nonce", 32)
	if err != nil {
		return oauthTokens{}, err
	}
	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {s.metadata.OAuthClientID},
		"redirect_uri":          {redirectURL},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	authorizationURL := s.discovery.AuthorizationEndpoint + "?" + query.Encode()

	type callbackResult struct{ code, oauthError string }
	result := make(chan callbackResult, 1)
	complete := make(chan struct{}, 1)
	var acceptOnce sync.Once
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if request.Host != listener.Addr().String() || request.Header.Get("Origin") != "" || request.URL.IsAbs() {
			http.Error(w, "Invalid sign-in callback", http.StatusBadRequest)
			return
		}
		if request.Method == http.MethodGet && request.URL.Path == "/complete" && request.URL.RawQuery == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, callbackHTML)
			select {
			case complete <- struct{}{}:
			default:
			}
			return
		}
		if request.Method != http.MethodGet || request.URL.Path != configured.Path || len(request.URL.Query()) > 3 || subtle.ConstantTimeCompare([]byte(request.URL.Query().Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid sign-in callback", http.StatusBadRequest)
			return
		}
		value := callbackResult{code: request.URL.Query().Get("code"), oauthError: request.URL.Query().Get("error")}
		if value.code == "" || value.oauthError != "" {
			http.Error(w, "Alzette sign-in was not completed", http.StatusBadRequest)
		} else {
			http.Redirect(w, request, "/complete", http.StatusSeeOther)
		}
		acceptOnce.Do(func() { result <- value })
	})
	go func() { _ = server.Serve(listener) }()
	if err := s.config.OpenBrowser(authorizationURL); err != nil {
		_ = server.Shutdown(context.Background())
		return oauthTokens{}, errors.New("open the system browser to sign in")
	}

	timer := time.NewTimer(s.config.LoginTimeout)
	defer timer.Stop()
	var callback callbackResult
	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return oauthTokens{}, ErrSignInCancelled
	case <-timer.C:
		_ = server.Shutdown(context.Background())
		return oauthTokens{}, ErrSignInTimeout
	case callback = <-result:
		select {
		case <-complete:
		case <-time.After(500 * time.Millisecond):
		}
		_ = server.Shutdown(context.Background())
	}
	if callback.oauthError != "" || callback.code == "" {
		return oauthTokens{}, ErrSignInCancelled
	}
	return s.exchange(ctx, url.Values{
		"grant_type": {"authorization_code"}, "client_id": {s.metadata.OAuthClientID},
		"redirect_uri": {redirectURL}, "code": {callback.code}, "code_verifier": {verifier},
	})
}

func (s *Session) exchange(ctx context.Context, form url.Values) (oauthTokens, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.discovery.TokenEndpoint, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return oauthTokens{}, fmt.Errorf("exchange OAuth credential: %w", err)
	}
	defer response.Body.Close()
	var tokens oauthTokens
	if response.StatusCode != http.StatusOK || decodeCompatibleJSON(response.Body, &tokens) != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") || tokens.ExpiresIn <= 0 || tokens.ExpiresIn > 3600 {
		return oauthTokens{}, errors.New("identity service returned an invalid token response")
	}
	return tokens, nil
}
