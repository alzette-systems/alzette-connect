package appstate

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/credentialstore"
	"github.com/alzette-systems/alzette-connect/internal/session"
)

func TestSnapshotIsCopiedAndContainsNoCredentialField(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	model := New(now)
	next := Snapshot{Phase: Ready, Message: "Connected", Contexts: []Context{{ID: "mem_a", Models: []string{"z", "a"}}}, Applications: []Application{{ID: "jan", Status: "connected"}}, UpdatedAt: now}
	model.Set(next)
	next.Contexts[0].Models[0] = "mutated"
	next.Applications[0].Status = "mutated"
	current := model.Current()
	if len(current.Contexts) != 1 || strings.Join(current.Contexts[0].Models, ",") != "a,z" {
		t.Fatalf("snapshot was mutable: %#v", current)
	}
	if current.Applications[0].Status != "connected" {
		t.Fatalf("application state was mutable: %#v", current.Applications)
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "api_key", "capability"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("UI state exposed %q: %s", forbidden, encoded)
		}
	}
}

func TestInferenceSessionCannotStartBeforeEmployeeSignIn(t *testing.T) {
	runtime, err := NewRuntime(RuntimeConfig{ControlURL: "https://control.example", CredentialStore: credentialstore.NewMemory()}, New(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.StartLaunch(context.Background()); !errors.Is(err, session.ErrSignInRequired) {
		t.Fatalf("StartLaunch error=%v", err)
	}
	if baseURL, capability, models, ok := runtime.ClientConnection(); ok || baseURL != "" || capability != "" || len(models) != 0 {
		t.Fatalf("unsigned-in runtime exposed a client connection: %q %q %v %t", baseURL, capability, models, ok)
	}
}

func TestResumeWithoutProtectedLoginDoesNotStartBrowserFlow(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	model := New(now)
	runtime, err := NewRuntime(RuntimeConfig{
		ControlURL:      "https://control.example",
		Profile:         "test",
		CredentialStore: credentialstore.NewMemory(),
		Clock:           func() time.Time { return now },
		OpenBrowser: func(string) error {
			t.Fatal("resume opened a browser without a protected login")
			return nil
		},
	}, model)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := runtime.Resume(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if resumed {
		t.Fatal("resume reported a session without a protected login")
	}
	if got := model.Current().Phase; got != SignInRequired {
		t.Fatalf("phase=%q, want %q", got, SignInRequired)
	}
}

func TestSubscribeReceivesCurrentAndUpdatedState(t *testing.T) {
	now := time.Now().UTC()
	model := New(now)
	updates, cancel := model.Subscribe()
	defer cancel()
	if first := <-updates; first.Phase != Starting {
		t.Fatalf("first phase=%q", first.Phase)
	}
	model.Set(Snapshot{Phase: SignInRequired, Message: "Sign in", UpdatedAt: now})
	if second := <-updates; second.Phase != SignInRequired {
		t.Fatalf("second phase=%q", second.Phase)
	}
}
