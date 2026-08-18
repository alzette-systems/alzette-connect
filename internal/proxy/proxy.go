package proxy

import (
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
			request.URL.Scheme = target.Scheme
			request.URL.Host = target.Host
			request.URL.Path = strings.TrimRight(target.Path, "/") + strings.TrimPrefix(request.URL.Path, "/v1")
			request.URL.RawQuery = ""
			request.Host = target.Host
			request.Header = make(http.Header)
			request.Header.Set("Accept", accept)
			request.Header.Set("Content-Type", contentType)
		},
		Transport:     &credentialTransport{provider: config.Provider, base: transport},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Alzette request could not be completed", http.StatusBadGateway)
		},
	}
	handler := strictHandler(listener.Addr().String(), capability, config.MaxBodyBytes, config.Provider, reverse)
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16 << 10}
	server := &Server{listener: listener, httpServer: httpServer, baseURL: "http://" + listener.Addr().String() + "/v1", capability: capability}
	go func() { _ = httpServer.Serve(listener) }()
	return server, nil
}

func strictHandler(host, capability string, maxBody int64, provider CredentialProvider, reverse http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if request.Host != host || request.URL.IsAbs() || request.URL.RawPath != "" || request.URL.RawQuery != "" || request.Header.Get("Origin") != "" || request.Header.Get("Cookie") != "" || request.Header.Get("Proxy-Authorization") != "" || hasForwardingHeader(request.Header) || request.ContentLength > maxBody {
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
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" || !isJSON(request.Header.Get("Content-Type")) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		request.Body = http.MaxBytesReader(w, request.Body, maxBody)
		reverse.ServeHTTP(w, request)
	})
}

func isJSON(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func hasForwardingHeader(header http.Header) bool {
	for name := range header {
		lower := strings.ToLower(name)
		if lower == "forwarded" || strings.HasPrefix(lower, "x-forwarded-") || lower == "via" || lower == "connection" || lower == "upgrade" || strings.HasPrefix(lower, "proxy-") {
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
