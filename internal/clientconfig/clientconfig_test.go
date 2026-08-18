package clientconfig

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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
	t.Helper()
	manager, err := New(Options{GOOS: "linux", HomeDir: root, UserConfigDir: filepath.Join(root, ".config"), UserDataDir: filepath.Join(root, ".local", "share"), SecretStore: secrets, Running: running})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func connection(capability string) Connection {
	return Connection{BaseURL: "http://127.0.0.1:41001/v1", Capability: capability, Models: []string{"alzette-chat"}}
}

func TestConfigureJanPreservesSettingsAndRollsBack(t *testing.T) {
	root := t.TempDir()
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
	root := t.TempDir()
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
	root := t.TempDir()
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
