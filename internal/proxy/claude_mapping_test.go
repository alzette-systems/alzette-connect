package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClaudeAliasesResolveOnlyAssignedRoutes(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid mapped request")
		}
		if string(body["model"]) != `"qa-flash"` || string(body["thinking"]) != `{"type":"adaptive"}` || r.Header.Get("Authorization") != "Bearer remote" || r.URL.RawQuery != "" {
			t.Errorf("mapped request/header = %#v %#v", body, r.Header)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer upstream.Close()
	p := &providerStub{gateway: upstream.URL + "/v1", models: []string{"qa-flash"}, token: "remote"}
	local, err := Start(Config{Address: "127.0.0.1:0", Provider: p, AllowedInferencePaths: []string{"/v1/messages"}, ModelAliases: map[string]string{"claude-sonnet-4-6": "qa-flash"}})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close(t.Context())
	send := func(model string) int {
		r, _ := http.NewRequest("POST", local.BaseURL()+"/messages?beta=true", strings.NewReader(`{"model":"`+model+`","thinking":{"type":"adaptive"},"messages":[]}`))
		r.Header.Set("Authorization", "Bearer "+local.Capability())
		r.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode
	}
	if send("claude-sonnet-4-6") != 200 || calls.Load() != 1 {
		t.Fatal("assigned route did not execute")
	}
	for _, model := range []string{"claude-opus-4-6", "qa-flash", "unassigned"} {
		if send(model) != 403 {
			t.Errorf("unmapped model %s accepted", model)
		}
	}
	p.models = nil
	if send("claude-sonnet-4-6") != 403 || calls.Load() != 1 {
		t.Fatal("revoked model reached upstream")
	}
}

func TestClaudeMappingCannotChangeOpenAIProxyOrGrantRoutes(t *testing.T) {
	p := &providerStub{gateway: "https://inference.example.test/v1", models: []string{"qa-flash"}}
	for _, config := range []Config{
		{Provider: p, ModelAliases: map[string]string{"claude-sonnet-4-6": "qa-flash"}},
		{Provider: p, AllowedInferencePaths: []string{"/v1/responses", "/v1/messages"}, ModelAliases: map[string]string{"claude-sonnet-4-6": "qa-flash"}},
		{Provider: p, AllowedInferencePaths: []string{"/v1/messages"}, ModelAliases: map[string]string{"claude-sonnet-4-6": "unassigned"}},
	} {
		if local, err := Start(config); err == nil {
			local.Close(t.Context())
			t.Fatal("unsafe mapping configuration accepted")
		}
	}
}
