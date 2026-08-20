package clientconfig

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestQualifyPiRequiresTheNamedRelease(t *testing.T) {
	root := canonicalTempRoot(t)
	pi := filepath.Join(root, "pi")
	mustWrite(t, pi, []byte("#!/bin/sh\nprintf 'pi 0.84.2\\n'\n"), 0o700)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	version, err := QualifyPi(ctx, pi)
	if err != nil || version != PiSupportedVersion {
		t.Fatalf("version=%q err=%v", version, err)
	}
	mustWrite(t, pi, []byte("#!/bin/sh\nprintf 'pi 0.85.0\\n'\n"), 0o700)
	if _, err := QualifyPi(ctx, pi); !errors.Is(err, ErrWrongVersion) {
		t.Fatalf("wrong version error=%v", err)
	}
}

type memorySecrets struct{ values map[string]string }

func (s *memorySecrets) key(service, account string) string { return service + "\x00" + account }
func (s *memorySecrets) Get(_ context.Context, service, account string) (string, bool, error) {
	value, ok := s.values[s.key(service, account)]
	return value, ok, nil
}
func (s *memorySecrets) Set(_ context.Context, service, account, value string) error {
	s.values[s.key(service, account)] = value
	return nil
}
func (s *memorySecrets) Delete(_ context.Context, service, account string) error {
	delete(s.values, s.key(service, account))
	return nil
}

type stopped bool

func (s stopped) Running(context.Context, string) (bool, error) { return bool(s), nil }

func testManager(t *testing.T, root string, secrets *memorySecrets, running RunningChecker) *Manager {
	return testManagerForOS(t, root, "linux", secrets, running)
}

func testManagerForOS(t *testing.T, root, goos string, secrets *memorySecrets, running RunningChecker) *Manager {
	t.Helper()
	manager, err := New(Options{GOOS: goos, HomeDir: root, UserConfigDir: filepath.Join(root, ".config"), UserDataDir: filepath.Join(root, ".local", "share"), SecretStore: secrets, Running: running})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestConfigureChatGPTPreservesConfigUsesEnvironmentKeyAndRollsBack(t *testing.T) {
	root := canonicalTempRoot(t)
	configPath := filepath.Join(root, ".codex", "config.toml")
	before := "# employee setting\nmodel = \"usual-model\"\nmodel_provider = \"openai\"\n\n[features]\napps = true\n"
	mustWrite(t, configPath, []byte(before), 0o600)
	executable := filepath.Join(root, "ChatGPT")
	mustWrite(t, executable, []byte("app"), 0o700)
	secrets := &memorySecrets{values: map[string]string{}}
	requestConnection := connection("alp_abcdefghijklmnopqrstuvwxyz0123456789")
	requestConnection.Models = []string{"document-review", "alzette-chat"}
	result, err := testManagerForOS(t, root, "darwin", secrets, stopped(false)).ConfigureChatGPT(context.Background(), ChatGPTRequest{
		Connection: requestConnection, ConfigPath: configPath, ExecutablePath: executable, Version: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Client != ChatGPT || result.Version != "1.2.3" || result.Status != Configured {
		t.Fatalf("unexpected result: %+v", result)
	}
	updated, _ := os.ReadFile(configPath)
	text := string(updated)
	for _, required := range []string{"# employee setting", "[features]", "apps = true", `model_provider = "alzette-connect"`, `wire_api = "responses"`, `env_key = "ALZETTE_CONNECT_SESSION_KEY"`, "request_max_retries = 0"} {
		if !contains(updated, []byte(required)) {
			t.Fatalf("ChatGPT config missing %q:\n%s", required, text)
		}
	}
	if strings.Contains(text, "alp_") {
		t.Fatal("ChatGPT configuration persisted the loopback capability")
	}
	catalogPath := filepath.Join(filepath.Dir(configPath), chatGPTCatalogFilename)
	catalog, _ := os.ReadFile(catalogPath)
	var listing struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if json.Unmarshal(catalog, &listing) != nil || len(listing.Models) != 2 || listing.Models[0].Slug != "alzette-chat" || listing.Models[1].Slug != "document-review" {
		t.Fatalf("unexpected ChatGPT catalogue: %s", catalog)
	}
	for _, required := range []string{`"additional_speed_tiers": []`, `"supports_parallel_tool_calls": false`, `"supports_search_tool": false`, `"input_modalities": [`} {
		if !strings.Contains(string(catalog), required) {
			t.Fatalf("ChatGPT catalogue missing %s: %s", required, catalog)
		}
	}
	if strings.Contains(string(catalog), requestConnection.Capability) {
		t.Fatal("ChatGPT catalogue persisted the loopback capability")
	}
	if err := result.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(configPath)
	if string(restored) != before {
		t.Fatalf("ChatGPT config was not restored exactly:\n%s", restored)
	}
	if _, err := os.Stat(catalogPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated catalogue survived rollback: %v", err)
	}
}

func TestConfigureChatGPTRejectsUnownedProvider(t *testing.T) {
	root := canonicalTempRoot(t)
	configPath := filepath.Join(root, ".codex", "config.toml")
	mustWrite(t, configPath, []byte("[model_providers.alzette-connect]\nname = \"Someone else\"\n"), 0o600)
	executable := filepath.Join(root, "ChatGPT")
	mustWrite(t, executable, []byte("app"), 0o700)
	_, err := testManagerForOS(t, root, "windows", &memorySecrets{values: map[string]string{}}, stopped(false)).ConfigureChatGPT(context.Background(), ChatGPTRequest{
		Connection: connection("alp_abcdefghijklmnopqrstuvwxyz0123456789"), ConfigPath: configPath, ExecutablePath: executable,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("unowned provider error=%v", err)
	}
}

func TestChatGPTRollbackRefusesToOverwriteAChangedProfile(t *testing.T) {
	root := canonicalTempRoot(t)
	configPath := filepath.Join(root, ".codex", "config.toml")
	mustWrite(t, configPath, []byte("model = \"personal\"\n"), 0o600)
	executable := filepath.Join(root, "ChatGPT")
	mustWrite(t, executable, []byte("app"), 0o700)
	result, err := testManagerForOS(t, root, "darwin", &memorySecrets{values: map[string]string{}}, stopped(false)).ConfigureChatGPT(context.Background(), ChatGPTRequest{
		Connection: connection("alp_abcdefghijklmnopqrstuvwxyz0123456789"), ConfigPath: configPath, ExecutablePath: executable,
	})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, configPath, []byte("model = \"employee-changed-this\"\n"), 0o600)
	if err := result.Rollback(context.Background()); !errors.Is(err, ErrStaleRollback) {
		t.Fatalf("rollback error=%v, want ErrStaleRollback", err)
	}
	changed, _ := os.ReadFile(configPath)
	if string(changed) != "model = \"employee-changed-this\"\n" {
		t.Fatalf("stale rollback overwrote the employee profile: %s", changed)
	}
}

func TestLaunchChatGPTUsesChildEnvironmentNotArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Unix fixture is covered by native Windows launch acceptance")
	}
	root := canonicalTempRoot(t)
	executable := filepath.Join(root, "ChatGPT")
	observed := filepath.Join(root, "observed")
	mustWrite(t, executable, []byte("#!/bin/sh\nprintf '%s|%s' \"$ALZETTE_CONNECT_SESSION_KEY\" \"$#\" > observed\n"), 0o700)
	requestConnection := connection("alp_abcdefghijklmnopqrstuvwxyz0123456789")
	process, err := LaunchChatGPT(context.Background(), executable, requestConnection)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-process.Done; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(observed)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != requestConnection.Capability+"|0" {
		t.Fatalf("observed child boundary=%q", data)
	}
}

func canonicalTempRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func connection(capability string) Connection {
	return Connection{BaseURL: "http://127.0.0.1:41001/v1", Capability: capability, Models: []string{"alzette-chat"}}
}

func TestConfigureJanPreservesSettingsAndRollsBack(t *testing.T) {
	root := canonicalTempRoot(t)
	data := filepath.Join(root, "jan-data")
	mustMkdir(t, data)
	app := filepath.Join(root, "jan-settings.json")
	mustWrite(t, app, []byte(`{"data_folder":`+quoted(data)+`}`), 0o600)
	mustWrite(t, filepath.Join(data, "store.json"), []byte(`{"version":"0.8.4"}`), 0o600)
	state := `{"state":{"providers":[{"provider":"Other","active":true,"models":[],"settings":[]}],"selectedProvider":"Other","selectedModel":{},"deletedModels":[]},"version":17}`
	mustWrite(t, filepath.Join(data, "settings.json"), []byte(`{"theme":"dark","model-provider":`+quoted(state)+`}`), 0o600)
	exe := filepath.Join(root, "jan")
	mustWrite(t, exe, []byte("x"), 0o700)
	secrets := &memorySecrets{values: map[string]string{}}
	result, err := testManager(t, root, secrets, stopped(false)).ConfigureJan(context.Background(), JanRequest{Connection: connection("alp_abcdefghijklmnopqrstuvwxyz0123456789"), AppSettingsPath: app, ExecutablePath: exe})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Configured || len(result.BackupFiles) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	updated, _ := os.ReadFile(filepath.Join(data, "settings.json"))
	var settings map[string]json.RawMessage
	if json.Unmarshal(updated, &settings) != nil || string(settings["theme"]) != `"dark"` {
		t.Fatal("unrelated Jan setting was not preserved")
	}
	if value := secrets.values[secrets.key(janKeyringService, janProviderID)]; value == "" || contains(updated, []byte("alp_")) {
		t.Fatal("capability storage boundary failed")
	}
	if err := result.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := secrets.values[secrets.key(janKeyringService, janProviderID)]; ok {
		t.Fatal("new secret survived rollback")
	}
}

func TestConfigureGoosePreservesConfigAndSecrets(t *testing.T) {
	root := canonicalTempRoot(t)
	configDir := filepath.Join(root, ".config", "goose")
	mustMkdir(t, configDir)
	mustWrite(t, filepath.Join(configDir, "config.yaml"), []byte("extensions:\n  developer: true\nproviders:\n  ollama:\n    enabled: true\n"), 0o600)
	exe := filepath.Join(root, "goose")
	mustWrite(t, exe, []byte("x"), 0o700)
	asar := filepath.Join(root, "app.asar")
	writeASAR(t, asar, GooseSupportedVersion)
	secrets := &memorySecrets{values: map[string]string{gooseKeyringService + "\x00" + gooseKeyringAccount: `{"USER_SECRET":"keep"}`}}
	result, err := testManager(t, root, secrets, stopped(false)).ConfigureGoose(context.Background(), GooseRequest{Connection: connection("alp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"), ConfigDir: configDir, AppASARPath: asar, ExecutablePath: exe})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != GooseSupportedVersion {
		t.Fatalf("wrong result: %+v", result)
	}
	config, _ := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if !contains(config, []byte("developer: true")) || contains(config, []byte("alp_")) {
		t.Fatal("Goose config preservation/secret boundary failed")
	}
	var values map[string]json.RawMessage
	json.Unmarshal([]byte(secrets.values[secrets.key(gooseKeyringService, gooseKeyringAccount)]), &values)
	if string(values["USER_SECRET"]) != `"keep"` || len(values[gooseSecretEnv]) == 0 {
		t.Fatal("Goose secrets were not merged")
	}
}

func TestRefusesRunningClientAndWrongGooseVersion(t *testing.T) {
	root := canonicalTempRoot(t)
	secrets := &memorySecrets{values: map[string]string{}}
	exe := filepath.Join(root, "client")
	mustWrite(t, exe, []byte("x"), 0o700)
	_, err := testManager(t, root, secrets, stopped(true)).ConfigureJan(context.Background(), JanRequest{Connection: connection("alp_abcdefghijklmnopqrstuvwxyz0123456789"), AppSettingsPath: filepath.Join(root, "missing"), ExecutablePath: exe})
	if !errors.Is(err, ErrClientRunning) {
		t.Fatalf("got %v", err)
	}
	configDir := filepath.Join(root, ".config", "goose")
	mustMkdir(t, configDir)
	mustWrite(t, filepath.Join(configDir, "config.yaml"), []byte("{}\n"), 0o600)
	asar := filepath.Join(root, "app.asar")
	writeASAR(t, asar, "1.45.0")
	_, err = testManager(t, root, secrets, stopped(false)).ConfigureGoose(context.Background(), GooseRequest{Connection: connection("alp_abcdefghijklmnopqrstuvwxyz0123456789"), ConfigDir: configDir, AppASARPath: asar, ExecutablePath: exe})
	if !errors.Is(err, ErrWrongVersion) {
		t.Fatalf("got %v", err)
	}
}

func writeASAR(t *testing.T, path, version string) {
	t.Helper()
	manifest := []byte(`{"name":"goose-app","version":` + quoted(version) + `}`)
	index, _ := json.Marshal(map[string]any{"files": map[string]any{"package.json": map[string]any{"size": len(manifest), "offset": "0"}}})
	header := make([]byte, 16)
	binary.LittleEndian.PutUint32(header[0:], 4)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(index)+8))
	binary.LittleEndian.PutUint32(header[8:], uint32(len(index)+4))
	binary.LittleEndian.PutUint32(header[12:], uint32(len(index)))
	mustWrite(t, path, append(append(header, index...), manifest...), 0o600)
}
func quoted(value string) string { data, _ := json.Marshal(value); return string(data) }
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}
func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}
func contains(data, part []byte) bool { return stringIndex(string(data), string(part)) >= 0 }
func stringIndex(value, part string) int {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
