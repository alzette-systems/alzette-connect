package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMessagesCapabilityIsScopedAndPreservesVersion(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/messages" || r.Header.Get("Anthropic-Version") != "2023-06-01" || r.Header.Get("Authorization") != "Bearer remote" || r.Header.Get("X-Api-Key") != "" {
			t.Error("incorrect Messages authority or version")
		}
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	local, err := Start(Config{Address: "127.0.0.1:0", Provider: &providerStub{gateway: upstream.URL + "/v1", token: "remote"}, AllowedInferencePaths: []string{"/v1/messages"}})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close(context.Background())
	for _, test := range []struct {
		path string
		want int
	}{{"/messages", 200}, {"/messages?beta=true", 200}, {"/messages?beta=true&x=1", 404}, {"/responses", 404}, {"/chat/completions", 404}, {"/messages?x=1", 404}} {
		r, _ := http.NewRequest(http.MethodPost, local.BaseURL()+test.path, strings.NewReader(`{"model":"claude-sonnet-4-6"}`))
		r.Header.Set("Authorization", "Bearer "+local.Capability())
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Anthropic-Version", "2023-06-01")
		r.Header.Set("X-Api-Key", "must-not-forward")
		response, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != test.want {
			t.Fatalf("%s: %d", test.path, response.StatusCode)
		}
	}
	if calls != 2 {
		t.Fatalf("upstream calls=%d", calls)
	}
}
