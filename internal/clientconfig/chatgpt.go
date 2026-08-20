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
	chatGPTCapabilityEnv   = "ALZETTE_CONNECT_SESSION_KEY"
	// The current context API returns aliases, not route capability metadata.
	// Use a conservative adapter ceiling and make no per-model capability
	// claim until that metadata has an evidenced server contract.
	chatGPTContextWindow = 16_384
)

// ConfigureChatGPT adapts Ollama's reversible Codex App provider pattern to
// Alzette. The config contains only the loopback URL and the name of an
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
	if err := transaction.apply(ctx, m.secrets); err != nil {
		return nil, err
	}
	return transaction.resultWithStore(ChatGPT, strings.TrimSpace(request.Version), m.secrets), nil
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
			return "", fmt.Errorf("%w: the selected macOS application is not ChatGPT", ErrUnsupported)
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
