package clientconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func claudeTestSetup(t *testing.T) (*Manager, ClaudeRequest, *memorySecrets) {
	t.Helper()
	root := canonicalTempRoot(t)
	executable := filepath.Join(root, "Claude.exe")
	helper := filepath.Join(root, "Connect.exe")
	mustWrite(t, executable, []byte("application"), 0o700)
	mustWrite(t, helper, []byte("helper"), 0o700)
	secrets := &memorySecrets{values: map[string]string{}}
	m := testManagerForOS(t, root, "windows", secrets, stopped(false))
	return m, ClaudeRequest{Connection: Connection{BaseURL: "http://127.0.0.1:43128/v1", Capability: "alp_abcdefghijklmnopqrstuvwxyz0123456789", Models: []string{"claude-qa-a"}}, ExecutablePath: executable, HelperExecutablePath: helper, LibraryDir: filepath.Join(root, "library"), Version: "2.19675.0.0"}, secrets
}

func TestClaudeWindowsCrashRecoveryPreservesOtherProfiles(t *testing.T) {
	m, request, secrets := claudeTestSetup(t)
	ctx := context.Background()
	metaPath := filepath.Join(request.LibraryDir, "_meta.json")
	before := `{"appliedId":"original","entries":[{"id":"original","name":"Personal"}],"hybridPointer":{"source":"personal"}}`
	mustWrite(t, metaPath, []byte(before), 0o600)
	personal := filepath.Join(request.LibraryDir, "original.json")
	mustWrite(t, personal, []byte(`{"unrelated":true}`), 0o600)
	result, err := m.ConfigureClaudeWindows(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range result.ChangedFiles {
		data, _ := os.ReadFile(path)
		if bytes.Contains(data, []byte(request.Connection.Capability)) {
			t.Fatal("plaintext credential in profile")
		}
	}
	data, _ := os.ReadFile(metaPath)
	var meta map[string]json.RawMessage
	_ = json.Unmarshal(data, &meta)
	var entries []map[string]string
	_ = json.Unmarshal(meta["entries"], &entries)
	entries = append(entries, map[string]string{"id": "later", "name": "New personal profile"})
	meta["entries"], _ = json.Marshal(entries)
	meta["unrelatedNewSetting"] = json.RawMessage(`true`)
	updated, _ := json.Marshal(meta)
	mustWrite(t, metaPath, updated, 0o600)
	// A new Manager has no in-memory Result and must recover from the journal.
	m = testManagerForOS(t, filepath.Dir(request.LibraryDir), "windows", secrets, stopped(false))
	if err = m.RecoverClaudeWindows(ctx, request.ExecutablePath, request.LibraryDir); err != nil {
		t.Fatal(err)
	}
	if len(secrets.values) != 0 {
		t.Fatal("credential survived recovery")
	}
	data, _ = os.ReadFile(metaPath)
	_ = json.Unmarshal(data, &meta)
	if string(meta["appliedId"]) != `"original"` || string(meta["unrelatedNewSetting"]) != "true" || !bytes.Contains(meta["entries"], []byte("later")) || bytes.Contains(meta["entries"], []byte("Alzette Connect")) {
		t.Fatalf("incorrect recovery metadata: %s", data)
	}
	if data, _ = os.ReadFile(personal); string(data) != `{"unrelated":true}` {
		t.Fatal("personal profile changed")
	}
}

func TestClaudeWindowsEditedOwnedProfileIsPreservedAndCredentialRevoked(t *testing.T) {
	m, request, secrets := claudeTestSetup(t)
	ctx := context.Background()
	result, err := m.ConfigureClaudeWindows(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	owned := result.ChangedFiles[0]
	mustWrite(t, owned, []byte(`{"edited":true}`), 0o600)
	if err = result.Rollback(ctx); !errors.Is(err, ErrStaleRollback) {
		t.Fatalf("rollback=%v", err)
	}
	if len(secrets.values) != 0 {
		t.Fatal("edited profile retained credential")
	}
	if data, _ := os.ReadFile(owned); string(data) != `{"edited":true}` {
		t.Fatal("edited profile was overwritten")
	}
}

func TestClaudeHelperChecksLaunchAndNeverFollowsRedirects(t *testing.T) {
	ctx := context.Background()
	store := &memorySecrets{values: map[string]string{}}
	cap := "alp_abcdefghijklmnopqrstuvwxyz0123456789"
	id := "a1"
	redirect := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer "+cap {
			t.Error("incorrect probe")
		}
		if redirect {
			http.Redirect(w, r, "https://example.invalid/credential", 302)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()
	value, _ := json.Marshal(claudeHelperCredential{BaseURL: server.URL + "/v1", Capability: cap})
	_ = store.Set(ctx, claudeSecretService, id, string(value))
	var output bytes.Buffer
	if err := writeClaudeCredential(ctx, store, id, &output); err != nil || strings.TrimSpace(output.String()) != cap {
		t.Fatal("helper failed live launch")
	}
	redirect = true
	output.Reset()
	if err := writeClaudeCredential(ctx, store, id, &output); err == nil || output.Len() != 0 {
		t.Fatal("helper disclosed credential after redirect")
	}
	server.Close()
	if err := writeClaudeCredential(ctx, store, id, &output); err == nil || output.Len() != 0 {
		t.Fatal("helper disclosed stale credential")
	}
}

func TestClaudeConnectionMapsAssignedModelsAndPreservesLabels(t *testing.T) {
	c, err := ClaudeConnection(Connection{Models: []string{"qa-pro", "qa-flash"}, Catalog: []Model{{Alias: "qa-flash", DisplayName: "DeepSeek Flash"}, {Alias: "qa-pro", DisplayName: "DeepSeek Pro"}}})
	if err != nil || len(c.Models) != 2 || c.ModelAliases["claude-sonnet-4-6"] != "qa-flash" || c.ModelAliases["claude-opus-4-6"] != "qa-pro" || c.Catalog[0].DisplayName != "DeepSeek Flash" || c.Catalog[1].DisplayName != "DeepSeek Pro" {
		t.Fatalf("mapping = %#v, error %v", c, err)
	}
	again, err := ClaudeConnection(c)
	if err != nil || !reflect.DeepEqual(c, again) {
		t.Fatal("mapping is not idempotent")
	}
	if _, err := ClaudeConnection(Connection{}); !errors.Is(err, ErrClaudeModels) {
		t.Fatal("empty selection accepted")
	}
}
