package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type providerStub struct {
	gateway string
	models  []string
	token   string
}

func (p *providerStub) EnsureHumanCredential(context.Context) (string, time.Time, error) {
	return p.token, time.Now().Add(time.Minute), nil
}
func (p *providerStub) GatewayBaseURL() string   { return p.gateway }
func (p *providerStub) SelectedModels() []string { return append([]string(nil), p.models...) }

func TestStrictLoopbackProxyForwardsOnlySupportedRequests(t *testing.T) {
	var authorization, cookie, forwarded string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization, cookie, forwarded = r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("Forwarded")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()
	server, err := Start(Config{Address: "127.0.0.1:0", Provider: &providerStub{gateway: upstream.URL + "/v1", models: []string{"alzette-chat"}, token: "alz_u_remote-token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())

	models, _ := http.NewRequest(http.MethodGet, server.BaseURL()+"/models", nil)
	models.Header.Set("Authorization", "Bearer "+server.Capability())
	response, err := http.DefaultClient.Do(models)
	if err != nil {
		t.Fatal(err)
	}
	var listing struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&listing) != nil || len(listing.Data) != 1 || listing.Data[0].ID != "alzette-chat" {
		t.Fatalf("models status=%d listing=%#v", response.StatusCode, listing)
	}
	response.Body.Close()

	request, _ := http.NewRequest(http.MethodPost, server.BaseURL()+"/chat/completions", strings.NewReader(`{"model":"alzette-chat","messages":[]}`))
	request.Header.Set("Authorization", "Bearer "+server.Capability())
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || authorization != "Bearer alz_u_remote-token" || cookie != "" || forwarded != "" {
		t.Fatalf("status=%d authorization=%q cookie=%q forwarded=%q", response.StatusCode, authorization, cookie, forwarded)
	}
}

func TestStrictLoopbackProxyRejectsAmbientBrowserAndProxyInput(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	defer upstream.Close()
	server, err := Start(Config{Address: "127.0.0.1:0", Provider: &providerStub{gateway: upstream.URL + "/v1", token: "alz_u_remote-token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	tests := []struct {
		name, method, path, key, header, value string
		want                                   int
	}{
		{"wrong key", http.MethodPost, "/chat/completions", "wrong", "Content-Type", "application/json", http.StatusUnauthorized},
		{"origin", http.MethodPost, "/chat/completions", server.Capability(), "Origin", "https://evil.invalid", http.StatusNotFound},
		{"cookie", http.MethodPost, "/chat/completions", server.Capability(), "Cookie", "session=x", http.StatusNotFound},
		{"forwarded", http.MethodPost, "/chat/completions", server.Capability(), "X-Forwarded-For", "127.0.0.1", http.StatusNotFound},
		{"query", http.MethodPost, "/chat/completions?x=1", server.Capability(), "Content-Type", "application/json", http.StatusNotFound},
		{"encoded path", http.MethodPost, "/chat%2Fcompletions", server.Capability(), "Content-Type", "application/json", http.StatusNotFound},
		{"lookalike content type", http.MethodPost, "/chat/completions", server.Capability(), "Content-Type", "application/jsonp", http.StatusNotFound},
		{"method", http.MethodPut, "/chat/completions", server.Capability(), "Content-Type", "application/json", http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(test.method, server.BaseURL()+test.path, strings.NewReader("{}"))
			request.Header.Set("Authorization", "Bearer "+test.key)
			request.Header.Set(test.header, test.value)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status=%d want=%d", response.StatusCode, test.want)
			}
		})
	}
}

func TestStartRejectsNonLoopback(t *testing.T) {
	_, err := Start(Config{Address: "0.0.0.0:43128", Provider: &providerStub{gateway: "https://gateway.example/v1"}})
	if err == nil {
		t.Fatal("non-loopback proxy address was accepted")
	}
}

func TestStartRejectsPlainHTTPRemoteGateway(t *testing.T) {
	_, err := Start(Config{Address: "127.0.0.1:0", Provider: &providerStub{gateway: "http://gateway.example/v1"}})
	if err == nil {
		t.Fatal("plaintext remote gateway was accepted")
	}
}
