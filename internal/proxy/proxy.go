package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type CredentialProvider interface {
	EnsureHumanCredential(context.Context) (string, time.Time, error)
	GatewayBaseURL() string
	SelectedModels() []string
}

type Config struct {
	Address       string
	Provider      CredentialProvider
	HTTPTransport http.RoundTripper
	Random        io.Reader
	MaxBodyBytes  int64
	// AllowedInferencePaths scopes a process capability to the exact wire
	// protocol declared by its adapter. /v1/models remains local and read-only.
	AllowedInferencePaths []string
	// ModelAliases is available only for an isolated Anthropic Messages proxy.
	// Values must be real routes in the signed-in user's current selection.
	ModelAliases map[string]string
}

type Server struct {
	listener   net.Listener
	httpServer *http.Server
	baseURL    string
	capability string
}

func Start(config Config) (*Server, error) {
	if config.Provider == nil {
		return nil, errors.New("credential provider is required")
	}
	if config.Address == "" {
		config.Address = "127.0.0.1:43128"
	}
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil || (host != "127.0.0.1" && host != "::1") || port == "" {
		return nil, errors.New("proxy address must be an explicit loopback IP and port")
	}
	if config.Random == nil {
		config.Random = rand.Reader
	}
	if config.MaxBodyBytes == 0 {
		config.MaxBodyBytes = 32 << 20
	}
	if config.MaxBodyBytes < 1024 || config.MaxBodyBytes > 64<<20 {
		return nil, errors.New("proxy body limit is outside supported bounds")
	}
	allowedPaths := make(map[string]bool)
	if len(config.AllowedInferencePaths) == 0 {
		config.AllowedInferencePaths = []string{"/v1/chat/completions"}
	}
	for _, path := range config.AllowedInferencePaths {
		if path != "/v1/chat/completions" && path != "/v1/responses" && path != "/v1/messages" {
			return nil, errors.New("proxy inference path is unsupported")
		}
		allowedPaths[path] = true
	}
	mapping := make(map[string]string, len(config.ModelAliases))
	if len(config.ModelAliases) != 0 {
		if len(allowedPaths) != 1 || !allowedPaths["/v1/messages"] || len(config.ModelAliases) > 128 {
			return nil, errors.New("model aliases require an isolated Messages proxy")
		}
		selected := config.Provider.SelectedModels()
		for alias, actual := range config.ModelAliases {
			if !strings.HasPrefix(alias, "claude-") || len(alias) > 128 || !containsModel(selected, actual) {
				return nil, errors.New("model alias does not resolve to an assigned route")
			}
			mapping[alias] = actual
		}
	}
	target, err := url.Parse(config.Provider.GatewayBaseURL())
	if err != nil || target.Scheme != "https" && target.Scheme != "http" || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || !strings.HasSuffix(target.Path, "/v1") || target.Scheme == "http" && !isLoopbackHost(target.Hostname()) {
		return nil, errors.New("Alzette gateway URL is invalid")
	}
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		return nil, err
	}
	actualHost, _, _ := net.SplitHostPort(listener.Addr().String())
	if actualHost != "127.0.0.1" && actualHost != "::1" {
		_ = listener.Close()
		return nil, errors.New("proxy listener was not loopback")
	}
	capability, err := randomCapability(config.Random)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	transport := config.HTTPTransport
	if transport == nil {
		transport = http.DefaultTransport
	}
	reverse := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			accept := request.Header.Get("Accept")
			contentType := request.Header.Get("Content-Type")
			anthropicVersion := request.Header.Get("Anthropic-Version")
			anthropicBeta := request.Header.Get("Anthropic-Beta")
			request.URL.Scheme = target.Scheme
			request.URL.Host = target.Host
			request.URL.Path = strings.TrimRight(target.Path, "/") + strings.TrimPrefix(request.URL.Path, "/v1")
			request.URL.RawQuery = ""
			request.Host = target.Host
			request.Header = make(http.Header)
			request.Header.Set("Accept", accept)
			request.Header.Set("Content-Type", contentType)
			if request.URL.Path == strings.TrimRight(target.Path, "/")+"/messages" && anthropicVersion == "2023-06-01" {
				request.Header.Set("Anthropic-Version", anthropicVersion)
				if len(anthropicBeta) <= 1024 && !strings.ContainsAny(anthropicBeta, "\r\n") {
					request.Header.Set("Anthropic-Beta", anthropicBeta)
				}
			}
			// A nil slice tells ReverseProxy not to synthesize X-Forwarded-For.
			// The remote gateway needs the employee authority, not local process
			// addressing or a second proxy-derived trust signal.
			request.Header["X-Forwarded-For"] = nil
		},
		Transport:     &credentialTransport{provider: config.Provider, base: transport},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Alzette request could not be completed", http.StatusBadGateway)
		},
	}
	handler := strictHandler(listener.Addr().String(), capability, config.MaxBodyBytes, allowedPaths, config.Provider, reverse, mapping)
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16 << 10}
	server := &Server{listener: listener, httpServer: httpServer, baseURL: "http://" + listener.Addr().String() + "/v1", capability: capability}
	go func() { _ = httpServer.Serve(listener) }()
	return server, nil
}

func strictHandler(host, capability string, maxBody int64, allowedPaths map[string]bool, provider CredentialProvider, reverse http.Handler, mapping map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Anthropic's beta Messages SDK adds this exact routing marker. The
		// gateway validates the payload's feature fields; no other query is allowed.
		validQuery := request.URL.RawQuery == "" || request.URL.Path == "/v1/messages" && request.URL.RawQuery == "beta=true"
		if request.Host != host || request.URL.IsAbs() || request.URL.RawPath != "" || !validQuery || request.Header.Get("Origin") != "" || request.Header.Get("Cookie") != "" || request.Header.Get("Content-Encoding") != "" || request.Header.Get("Proxy-Authorization") != "" || hasForwardingHeader(request.Header) || request.ContentLength > maxBody {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		authorization := request.Header.Values("Authorization")
		expected := "Bearer " + capability
		if len(authorization) != 1 || subtle.ConstantTimeCompare([]byte(authorization[0]), []byte(expected)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="alzette-loopback"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if request.Method == http.MethodGet && request.URL.Path == "/v1/models" && request.Body == http.NoBody {
			models := provider.SelectedModels()
			data := make([]map[string]interface{}, 0, len(models))
			for _, alias := range models {
				data = append(data, map[string]interface{}{"id": alias, "object": "model", "created": 0, "owned_by": "alzette"})
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "list", "data": data})
			return
		}
		if request.Method != http.MethodPost || !allowedPaths[request.URL.Path] || !isJSON(request.Header.Get("Content-Type")) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		request.Body = http.MaxBytesReader(w, request.Body, maxBody)
		if len(mapping) != 0 {
			body, err := io.ReadAll(request.Body)
			_ = request.Body.Close()
			var payload map[string]json.RawMessage
			var model string
			if err != nil || json.Unmarshal(body, &payload) != nil || json.Unmarshal(payload["model"], &model) != nil {
				writeMessagesError(w, http.StatusBadRequest, "invalid_request_error", "Invalid Messages request")
				return
			}
			actual, ok := mapping[model]
			if !ok || !containsModel(provider.SelectedModels(), actual) {
				writeMessagesError(w, http.StatusForbidden, "permission_error", "Model is not assigned to this application session")
				return
			}
			payload["model"], _ = json.Marshal(actual)
			body, _ = json.Marshal(payload)
			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
		}
		reverse.ServeHTTP(w, request)
	})
}

func containsModel(models []string, actual string) bool {
	for _, model := range models {
		if model == actual {
			return true
		}
	}
	return false
}

func writeMessagesError(w http.ResponseWriter, status int, kind, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"type": kind, "message": message}})
}

func isJSON(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func hasForwardingHeader(header http.Header) bool {
	for name, values := range header {
		lower := strings.ToLower(name)
		if lower == "connection" {
			// Node/Undici explicitly sends this ordinary HTTP/1.1 persistence
			// hint. It does not nominate another hop-by-hop header and is safe
			// to accept; ReverseProxy removes it before the remote request.
			if len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "keep-alive") {
				continue
			}
			return true
		}
		if lower == "forwarded" || strings.HasPrefix(lower, "x-forwarded-") || lower == "via" || lower == "upgrade" || strings.HasPrefix(lower, "proxy-") {
			return true
		}
	}
	return false
}

type credentialTransport struct {
	provider CredentialProvider
	base     http.RoundTripper
}

func (t *credentialTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	token, _, err := t.provider.EnsureHumanCredential(request.Context())
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(request)
}

func randomCapability(source io.Reader) (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return "alp_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Server) BaseURL() string { return s.baseURL }

// Capability is private runtime material. Callers may pass it to an exact
// client adapter, but must never put it in appstate.Snapshot or frontend IPC.
func (s *Server) Capability() string { return s.capability }

func (s *Server) Close(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
