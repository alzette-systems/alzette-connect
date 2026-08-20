package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ticruz38/alzette-connect/internal/appstate"
)

func TestPresentationStateUsesSelectedContextAndContainsNoConnectionSecret(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	service := &desktopService{
		applications: []appstate.Application{{ID: "jan", Name: "Jan Desktop", Status: "verification_required", Installed: true, DeliveryMode: "catalogue"}},
		launch:       appstate.Launch{Phase: "preparing", ApplicationID: "jan", Application: "Jan Desktop", ModelCount: 2},
		update:       appstate.Update{State: "idle", CurrentVersion: "test"},
	}
	presented := service.presentationState(appstate.Snapshot{
		Phase: appstate.Ready, SelectedContextID: "membership-two", UpdatedAt: now,
		Contexts: []appstate.Context{
			{ID: "membership-one", Models: []string{"one"}},
			{ID: "membership-two", Models: []string{"chat", "coder"}},
		},
	})
	if got := presented.Applications[0].ModelCount; got != 2 {
		t.Fatalf("model count=%d, want 2", got)
	}
	if presented.Launch.Phase != "preparing" || presented.Launch.ApplicationID != "jan" {
		t.Fatalf("launch state=%+v", presented.Launch)
	}
	if presented.Platform == "" {
		t.Fatal("presentation omitted the current platform")
	}
	encoded, err := json.Marshal(presented)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "capability", "api_key", "alp_", "alz_u_"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("presentation leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestObservedChatGPTVersionDoesNotClaimNamedCompatibility(t *testing.T) {
	service := &desktopService{applications: []appstate.Application{{ID: "chatgpt", Name: "ChatGPT", Status: "verification_required", Installed: true}}}
	service.setApplicationObserved("chatgpt", "9.9.9")
	application := service.applications[0]
	if application.Status != "verification_required" || application.Version != "9.9.9" || application.Configured {
		t.Fatalf("observed ChatGPT version became a compatibility claim: %+v", application)
	}
}
