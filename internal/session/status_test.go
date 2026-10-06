package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
