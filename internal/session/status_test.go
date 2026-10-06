package session

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestContextResponseFailureDoesNotClaimAccessEnded(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"authentication expired", http.StatusUnauthorized, "", ErrSignInRequired},
		{"access forbidden", http.StatusForbidden, "", ErrAccessRemoved},
		{"service unavailable", http.StatusServiceUnavailable, "", nil},
		{"unexpected status", http.StatusNotFound, "", nil},
		{"malformed response", http.StatusOK, `{`, nil},
		{"unknown field", http.StatusOK, `{"schema":"alzette.agent-contexts.v1","contexts":[],"unexpected":true}`, nil},
		{"unsupported schema", http.StatusOK, `{"schema":"future","contexts":[]}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			s := &Session{config: Config{HTTPClient: server.Client()}, metadata: Metadata{ControlOrigin: server.URL}, accessToken: "test-access"}
			err := s.loadContexts(context.Background())
			if err == nil {
				t.Fatal("invalid response accepted")
			}
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v, want %v", err, tc.want)
				}
			} else if errors.Is(err, ErrAccessRemoved) || errors.Is(err, ErrSignInRequired) {
				t.Fatalf("response failure misreported as an authority change: %v", err)
			}
		})
	}
}

func TestInitialContextLoadAcceptsCurrentEndpointStatusContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"schema":"alzette.agent-contexts.v1","contexts":[{"membership_id":"mem_employee","organisation":"Company","project":"AI workspace","environment":"Development","relationship":"employee","model_aliases":["alzette"],"models":[{"alias":"alzette","display_name":"DeepSeek V4 Flash","capabilities":["chat_completions","function_tools"],"context_window_tokens":1000000,"status":"unknown","callable":true,"status_detail":"No recent health evidence","freshness":"unknown"}]}]}`)
	}))
	defer server.Close()
	s := &Session{config: Config{HTTPClient: server.Client()}, metadata: Metadata{ControlOrigin: server.URL}, accessToken: "test-access"}
	if err := s.loadContexts(context.Background()); err != nil {
		t.Fatal(err)
	}
	selected, err := s.SelectContext("")
	if err != nil {
		t.Fatal(err)
	}
	if selected.MembershipID != "mem_employee" || len(selected.ModelAliases) != 1 || selected.ModelAliases[0] != "alzette" {
		t.Fatalf("employee entitlement changed: %#v", selected)
	}
	if len(selected.Models) != 1 || !selected.Models[0].Callable || selected.Models[0].Status != "unknown" {
		t.Fatalf("status metadata was not preserved: %#v", selected.Models)
	}
}

func TestHealthRefreshRetainsAliasAndHumanCredential(t *testing.T) {
	now := time.Now()
	status := "unavailable"
	code := 200
	alias := "Team.Chat"
	access := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/contexts" {
			t.Errorf("unexpected credential request: %s", r.URL.Path)
			w.WriteHeader(500)
			return
		}
		contexts := []Context{{MembershipID: "mem_status", ModelAliases: []string{alias}, Models: []Model{{Alias: alias, Status: status}}}}
		if !access {
			contexts = nil
		}
		writeJSON(w, code, contextsResponse{Schema: "alzette.agent-contexts.v1", Contexts: contexts})
	}))
	defer server.Close()
	selected := Context{MembershipID: "mem_status", ModelAliases: []string{"Team.Chat"}}
	s := &Session{config: Config{HTTPClient: server.Client(), Clock: func() time.Time { return now }}, metadata: Metadata{ControlOrigin: server.URL}, accessExpires: now.Add(time.Hour), accessToken: "test-access", selected: selected, humanToken: "test-human", humanExpires: now.Add(time.Minute)}
	if err := s.RefreshContexts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.SelectedModels()[0] != "Team.Chat" || s.SelectedModelCatalog()[0].Status != "unavailable" || s.humanToken != "test-human" {
		t.Fatal("outage changed entitlement or credential")
	}
	status = "operational"
	if err := s.RefreshContexts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.SelectedModelCatalog()[0].Status != "operational" || s.humanToken != "test-human" {
		t.Fatal("recovery did not refresh in place")
	}
	code = 503
	if err := s.RefreshContexts(context.Background()); err == nil || errors.Is(err, ErrAccessRemoved) {
		t.Fatalf("server outage treated as access removal: %v", err)
	}
	code, alias = 200, "Team.Code"
	if err := s.RefreshContexts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.humanToken != "" || s.SelectedModels()[0] != alias {
		t.Fatal("changed entitlement retained its old credential")
	}
	access = false
	if err := s.RefreshContexts(context.Background()); !errors.Is(err, ErrAccessRemoved) {
		t.Fatalf("missing membership did not remove access: %v", err)
	}
	if s.SelectedContext().MembershipID != "" || s.humanToken != "" {
		t.Fatal("removed access retained a selection or credential")
	}
}
