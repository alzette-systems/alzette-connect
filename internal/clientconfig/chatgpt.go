package clientconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	chatGPTCatalogFilename = "alzette-connect-models.json"
	chatGPTRestoreFilename = "chatgpt-restore.json"
	chatGPTCapabilityEnv   = "ALZETTE_CONNECT_SESSION_KEY"
	// The current context API returns aliases, not route capability metadata.
	// Use a conservative adapter ceiling and make no per-model capability
	// claim until that metadata has an evidenced server contract.
	chatGPTContextWindow = 16_384
)

type chatGPTRestoreValue struct {
	Present bool   `json:"present"`
	Value   string `json:"value,omitempty"`
}

type chatGPTRestoreState struct {
	ConfigExisted    bool                `json:"config_existed"`
	Profile          chatGPTRestoreValue `json:"profile"`
	Model            chatGPTRestoreValue `json:"model"`
	ModelProvider    chatGPTRestoreValue `json:"model_provider"`
	ModelCatalogJSON chatGPTRestoreValue `json:"model_catalog_json"`
}

// ConfigureChatGPT adapts Ollama's reversible ChatGPT/Codex workspace provider
// pattern to Alzette. The unified ChatGPT app currently exposes this workspace
// through the com.openai.codex bundle and the documented user-level Codex
// configuration. The config contains only the loopback URL and the name of an
// ephemeral environment variable; the capability itself is injected only
// into the supervised child process.
func (m *Manager) ConfigureChatGPT(ctx context.Context, request ChatGPTRequest) (*Result, error) {
	if m.goos != "darwin" && m.goos != "windows" {
		return nil, fmt.Errorf("%w: ChatGPT is supported on macOS and Windows", ErrUnsupported)
	}
	connection, err := validateConnection(request.Connection)
	if err != nil {
		return nil, err
	}
	if err := m.requireStopped(ctx, request.ExecutablePath); err != nil {
		return nil, err
	}
	configPath := request.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(m.homeDir, ".codex", "config.toml")
	}
	catalogPath := filepath.Join(filepath.Dir(configPath), chatGPTCatalogFilename)
	unlock, err := acquireConfigLock(ctx, configPath)
	if err != nil {
		return nil, err
	}
	defer unlock()

	beforeConfig, configMode, configExists, err := readSafe(configPath, true)
	if err != nil {
		return nil, err
	}
	parsed, err := parseChatGPTConfig(string(beforeConfig))
	if err != nil {
		return nil, err
	}
	if chatGPTConfigIsManaged(parsed) && (chatGPTProviderIsOwned(parsed) || m.hasChatGPTRecoveryEvidence(configPath)) {
		if err := m.restoreChatGPTLocked(ctx, configPath, catalogPath); err != nil {
			return nil, fmt.Errorf("restore previous Alzette ChatGPT profile: %w", err)
		}
		beforeConfig, configMode, configExists, err = readSafe(configPath, true)
		if err != nil {
			return nil, err
		}
		parsed, err = parseChatGPTConfig(string(beforeConfig))
		if err != nil {
			return nil, err
		}
	}
	if err := validateChatGPTOwnership(parsed); err != nil {
		return nil, err
	}
	afterConfig, err := renderChatGPTConfig(string(beforeConfig), connection, catalogPath)
	if err != nil {
		return nil, err
	}
	beforeCatalog, catalogMode, catalogExists, err := readSafe(catalogPath, true)
	if err != nil {
		return nil, err
	}
	afterCatalog, err := renderChatGPTCatalog(connection.Models)
	if err != nil {
		return nil, err
	}

	transaction := &transaction{}
	transaction.addFile(configPath, beforeConfig, configMode, configExists, []byte(afterConfig))
	transaction.addFile(catalogPath, beforeCatalog, catalogMode, catalogExists, afterCatalog)
	if err := m.saveChatGPTRestoreState(beforeConfig, configExists); err != nil {
		return nil, err
	}
	if err := transaction.apply(ctx, m.secrets); err != nil {
		_ = os.Remove(m.chatGPTRestorePath())
		return nil, err
	}
	result := transaction.resultWithStore(ChatGPT, strings.TrimSpace(request.Version), m.secrets)
	if result.Status == Configured {
		result.rollback = func(ctx context.Context) error {
			return m.restoreChatGPT(ctx, configPath, catalogPath)
		}
	}
	return result, nil
}

func chatGPTConfigIsManaged(config chatGPTConfig) bool {
	profile, _ := config.string("profile")
	provider, _ := config.string("model_provider")
	return profile == ChatGPTProviderID || provider == ChatGPTProviderID
}

func chatGPTProviderIsOwned(config chatGPTConfig) bool {
	name, nameOK := config.string("model_providers", ChatGPTProviderID, "name")
	environmentKey, keyOK := config.string("model_providers", ChatGPTProviderID, "env_key")
	wireAPI, wireOK := config.string("model_providers", ChatGPTProviderID, "wire_api")
	baseURL, baseOK := config.string("model_providers", ChatGPTProviderID, "base_url")
	return nameOK && name == "Alzette" && keyOK && environmentKey == chatGPTCapabilityEnv &&
		wireOK && wireAPI == "responses" && baseOK && strings.HasPrefix(baseURL, "http://")
}

func (m *Manager) hasChatGPTRecoveryEvidence(configPath string) bool {
	for _, path := range []string{m.chatGPTRestorePath(), configPath + ".alzette-connect.bak"} {
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func chatGPTRestoreValueFrom(config chatGPTConfig, key string) chatGPTRestoreValue {
	value, present := config.string(key)
	return chatGPTRestoreValue{Present: present, Value: value}
}

func chatGPTRestoreStateFrom(data []byte, existed bool) (chatGPTRestoreState, error) {
	config, err := parseChatGPTConfig(string(data))
	if err != nil {
		return chatGPTRestoreState{}, err
	}
	return chatGPTRestoreState{
		ConfigExisted:    existed,
		Profile:          chatGPTRestoreValueFrom(config, "profile"),
		Model:            chatGPTRestoreValueFrom(config, "model"),
		ModelProvider:    chatGPTRestoreValueFrom(config, "model_provider"),
		ModelCatalogJSON: chatGPTRestoreValueFrom(config, "model_catalog_json"),
	}, nil
}

func (m *Manager) chatGPTRestorePath() string {
	return filepath.Join(m.userDataDir, "Alzette Connect", chatGPTRestoreFilename)
}

func (m *Manager) saveChatGPTRestoreState(config []byte, existed bool) error {
	state, err := chatGPTRestoreStateFrom(config, existed)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(m.chatGPTRestorePath(), append(data, '\n'), 0o600)
}

func (m *Manager) loadChatGPTRestoreState(configPath string) (chatGPTRestoreState, error) {
	data, _, _, err := readSafe(m.chatGPTRestorePath(), false)
	if err == nil {
		var state chatGPTRestoreState
		if json.Unmarshal(data, &state) != nil {
			return chatGPTRestoreState{}, errors.New("invalid Alzette ChatGPT restore state")
		}
		return state, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return chatGPTRestoreState{}, err
	}
	// Versions before 0.3.6 recorded the employee's original profile in the
	// transaction backup only. Read it once so an upgrade can repair an
	// already-preserved profile without overwriting later ChatGPT changes.
	backup, _, existed, backupErr := readSafe(configPath+".alzette-connect.bak", true)
	if backupErr != nil {
		return chatGPTRestoreState{}, backupErr
	}
	return chatGPTRestoreStateFrom(backup, existed)
}

func (m *Manager) restoreChatGPT(ctx context.Context, configPath, catalogPath string) error {
	unlock, err := acquireConfigLock(ctx, configPath)
	if err != nil {
		return err
	}
	defer unlock()
	return m.restoreChatGPTLocked(ctx, configPath, catalogPath)
}

func (m *Manager) restoreChatGPTLocked(ctx context.Context, configPath, catalogPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, mode, exists, err := readSafe(configPath, true)
	if err != nil {
		return err
	}
	parsed, err := parseChatGPTConfig(string(current))
	if err != nil {
		return err
	}
	state, err := m.loadChatGPTRestoreState(configPath)
	if err != nil {
		return err
	}

	restored := string(current)
	if chatGPTConfigIsManaged(parsed) {
		restored = chatGPTRestoreRootString(restored, "profile", state.Profile)
		restored = chatGPTRestoreRootString(restored, "model", state.Model)
		restored = chatGPTRestoreRootString(restored, "model_provider", state.ModelProvider)
		restored = chatGPTRestoreRootString(restored, "model_catalog_json", state.ModelCatalogJSON)
	}
	// This identifier is reserved by validateChatGPTOwnership, so the section
	// belongs to Connect even when ChatGPT has reformatted it or added runtime
	// defaults. Removing that section preserves every unrelated newer setting.
	restored = chatGPTRemoveSection(restored, "[model_providers."+ChatGPTProviderID+"]")
	restored = chatGPTRemoveSection(restored, "[profiles."+ChatGPTProviderID+"]")
	if _, err := parseChatGPTConfig(restored); err != nil {
		return err
	}
	if state.ConfigExisted || strings.TrimSpace(restored) != "" {
		if !exists {
			mode = 0o600
		}
		if err := atomicWrite(configPath, []byte(restored), mode); err != nil {
			return err
		}
	} else if err := os.Remove(configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	finalConfig, err := parseChatGPTConfig(restored)
	if err != nil {
		return err
	}
	activeCatalog, _ := finalConfig.string("model_catalog_json")
	if activeCatalog != catalogPath {
		if err := os.Remove(catalogPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(m.chatGPTRestorePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.Remove(configPath + ".alzette-connect.bak")
	return nil
}

func validateChatGPTOwnership(config chatGPTConfig) error {
	if !config.exists("model_providers", ChatGPTProviderID) {
		if provider, ok := config.string("model_provider"); ok && provider == ChatGPTProviderID {
			return fmt.Errorf("%w: incomplete Alzette ChatGPT provider", ErrConflict)
		}
		return nil
	}
	// A provider left behind by a crashed process is not proof that this
	// launch owns the associated backup. Until Connect has a durable recovery
	// journal, fail closed instead of silently adopting or overwriting it.
	return fmt.Errorf("%w: ChatGPT already contains an %s provider; restore or remove the previous Connect profile before launching", ErrConflict, ChatGPTProviderID)
}

func renderChatGPTConfig(before string, connection Connection, catalogPath string) (string, error) {
	primary := connection.Models[0]
	text := chatGPTRemoveRoot(before, "profile")
	text = chatGPTSetRootString(text, "model", primary)
	text = chatGPTSetRootString(text, "model_provider", ChatGPTProviderID)
	text = chatGPTSetRootString(text, "model_catalog_json", catalogPath)
	header := fmt.Sprintf("[model_providers.%s]", ChatGPTProviderID)
	text = chatGPTUpsertSection(text, header, []string{
		`name = "Alzette"`,
		fmt.Sprintf("base_url = %q", strings.TrimRight(connection.BaseURL, "/")+"/"),
		`env_key = "` + chatGPTCapabilityEnv + `"`,
		`wire_api = "responses"`,
		`requires_openai_auth = false`,
		`request_max_retries = 0`,
		`stream_max_retries = 0`,
	})
	parsed, err := parseChatGPTConfig(text)
	if err != nil {
		return "", err
	}
	checks := []struct {
		path []string
		want string
	}{
		{[]string{"model"}, primary},
		{[]string{"model_provider"}, ChatGPTProviderID},
		{[]string{"model_catalog_json"}, catalogPath},
		{[]string{"model_providers", ChatGPTProviderID, "name"}, "Alzette"},
		{[]string{"model_providers", ChatGPTProviderID, "base_url"}, strings.TrimRight(connection.BaseURL, "/") + "/"},
		{[]string{"model_providers", ChatGPTProviderID, "env_key"}, chatGPTCapabilityEnv},
		{[]string{"model_providers", ChatGPTProviderID, "wire_api"}, "responses"},
	}
	for _, check := range checks {
		if value, ok := parsed.string(check.path...); !ok || value != check.want {
			return "", errors.New("generated ChatGPT provider failed validation")
		}
	}
	return text, nil
}

func renderChatGPTCatalog(models []string) ([]byte, error) {
	entries := make([]map[string]any, 0, len(models))
	for index, model := range models {
		entries = append(entries, map[string]any{
			"slug":                             model,
			"display_name":                     model,
			"description":                      "Company model through Alzette",
			"default_reasoning_level":          nil,
			"supported_reasoning_levels":       []any{},
			"shell_type":                       "default",
			"visibility":                       "list",
			"supported_in_api":                 true,
			"priority":                         index,
			"additional_speed_tiers":           []any{},
			"availability_nux":                 nil,
			"upgrade":                          nil,
			"base_instructions":                "You are a helpful assistant using the employee's company-approved Alzette model access.",
			"model_messages":                   nil,
			"supports_reasoning_summaries":     false,
			"default_reasoning_summary":        "auto",
			"support_verbosity":                false,
			"default_verbosity":                nil,
			"apply_patch_tool_type":            nil,
			"web_search_tool_type":             "text",
			"supports_parallel_tool_calls":     false,
			"supports_image_detail_original":   false,
			"context_window":                   chatGPTContextWindow,
			"max_context_window":               chatGPTContextWindow,
			"auto_compact_token_limit":         nil,
			"effective_context_window_percent": 95,
			"truncation_policy":                map[string]any{"mode": "bytes", "limit": 10_000},
			"experimental_supported_tools":     []any{},
			"input_modalities":                 []string{"text"},
			"supports_search_tool":             false,
		})
	}
	data, err := json.MarshalIndent(map[string]any{"models": entries}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ObserveChatGPTVersion records the native application version after the
// employee explicitly launches it. Observation is not qualification:
// compatibility remains release-gated to a named version/OS acceptance run.
func ObserveChatGPTVersion(ctx context.Context, executable string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", ErrUnsafePath
	}
	if err := ensureTrustedEvidence(executable); err != nil {
		return "", err
	}
	check, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		marker := string(filepath.Separator) + "Contents" + string(filepath.Separator) + "MacOS" + string(filepath.Separator)
		index := strings.LastIndex(filepath.Clean(executable), marker)
		if index < 0 {
			return "", ErrUnsupported
		}
		infoPath := filepath.Join(filepath.Clean(executable)[:index], "Contents", "Info.plist")
		if err := ensureTrustedEvidence(infoPath); err != nil {
			return "", err
		}
		identity := exec.CommandContext(check, "/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", infoPath)
		identity.Env = launchEnvironment(os.Environ())
		bundleID, identityErr := identity.Output()
		if identityErr != nil || strings.TrimSpace(string(bundleID)) != "com.openai.codex" {
			return "", fmt.Errorf("%w: the selected macOS application does not expose ChatGPT's Codex workspace", ErrUnsupported)
		}
		command = exec.CommandContext(check, "/usr/bin/plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-", infoPath)
	case "windows":
		command = exec.CommandContext(check, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `(Get-Item -LiteralPath $args[0]).VersionInfo.ProductVersion`, executable)
	default:
		return "", ErrUnsupported
	}
	command.Env = launchEnvironment(os.Environ())
	output, err := command.Output()
	version := strings.TrimSpace(string(output))
	if err != nil || version == "" || len(version) > 64 || strings.ContainsAny(version, "\r\n\t") {
		return "", ErrWrongVersion
	}
	return version, nil
}

// LaunchChatGPT starts the exact qualified application directly so the local
// capability exists only in that process environment and Connect can observe
// exit for grant revocation and profile rollback.
func LaunchChatGPT(ctx context.Context, executable string, connection Connection) (*Process, error) {
	validated, err := validateConnection(connection)
	if err != nil {
		return nil, err
	}
	return launchObserved(ctx, executable, nil, []string{chatGPTCapabilityEnv + "=" + validated.Capability}, nil)
}
