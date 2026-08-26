package clientconfig

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func New(options Options) (*Manager, error) {
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOOS != "darwin" && options.GOOS != "windows" && options.GOOS != "linux" {
		return nil, fmt.Errorf("%w: operating system %q", ErrUnsupported, options.GOOS)
	}
	var err error
	if options.HomeDir == "" {
		options.HomeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
	}
	if options.UserConfigDir == "" {
		options.UserConfigDir, err = os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolve config directory: %w", err)
		}
	}
	if options.UserDataDir == "" {
		options.UserDataDir = defaultDataDir(options.GOOS, options.HomeDir)
	}
	if options.SecretStore == nil {
		options.SecretStore = newPlatformSecretStore()
	}
	if options.Running == nil {
		options.Running = processChecker{}
	}
	for _, path := range []string{options.HomeDir, options.UserConfigDir, options.UserDataDir} {
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("%w: base directories must be absolute", ErrUnsafePath)
		}
	}
	return &Manager{goos: options.GOOS, homeDir: options.HomeDir, userConfigDir: options.UserConfigDir, userDataDir: options.UserDataDir, secrets: options.SecretStore, running: options.Running}, nil
}

func defaultDataDir(goos, home string) string {
	switch goos {
	case "linux":
		if path := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(home, ".local", "share")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support")
	default:
		return os.Getenv("APPDATA")
	}
}

func validateConnection(connection Connection) (Connection, error) {
	parsed, err := url.Parse(connection.BaseURL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/v1" {
		return Connection{}, errors.New("Connect URL must be an http loopback URL ending in /v1")
	}
	host := parsed.Hostname()
	port := parsed.Port()
	address := net.ParseIP(host)
	if address == nil || !address.IsLoopback() || port == "" {
		return Connection{}, errors.New("Connect URL must contain an explicit loopback IP and port")
	}
	if len(connection.Capability) < 32 || len(connection.Capability) > 4096 || !strings.HasPrefix(connection.Capability, "alp_") {
		return Connection{}, errors.New("Connect capability is invalid")
	}
	seen := make(map[string]bool)
	models := make([]string, 0, len(connection.Models))
	for _, model := range connection.Models {
		if model == "" || len(model) > 128 || strings.ContainsAny(model, "\r\n\t") {
			return Connection{}, errors.New("model alias is invalid")
		}
		if !seen[model] {
			seen[model] = true
			models = append(models, model)
		}
	}
	if len(models) == 0 || len(models) > 64 {
		return Connection{}, errors.New("at least one and no more than 64 model aliases are required")
	}
	sort.Strings(models)
	connection.Models = models
	catalogByAlias := make(map[string]Model, len(connection.Catalog))
	for _, model := range connection.Catalog {
		if !seen[model.Alias] || len(model.DisplayName) > 128 || strings.ContainsAny(model.DisplayName, "\r\n\t") || len(model.Capabilities) > 64 {
			return Connection{}, errors.New("model capability metadata is invalid")
		}
		if model.DisplayName == "" {
			model.DisplayName = model.Alias
		}
		capabilities := make([]string, 0, len(model.Capabilities))
		capabilitySeen := make(map[string]bool)
		for _, capability := range model.Capabilities {
			if capability == "" || len(capability) > 128 || strings.ContainsAny(capability, "\r\n\t") {
				return Connection{}, errors.New("model capability metadata is invalid")
			}
			if !capabilitySeen[capability] {
				capabilitySeen[capability] = true
				capabilities = append(capabilities, capability)
			}
		}
		sort.Strings(capabilities)
		model.Capabilities = capabilities
		if model.ContextWindowTokens != nil {
			if *model.ContextWindowTokens < 1 || *model.ContextWindowTokens > 100_000_000 {
				return Connection{}, errors.New("model capability metadata is invalid")
			}
			value := *model.ContextWindowTokens
			model.ContextWindowTokens = &value
		}
		catalogByAlias[model.Alias] = model
	}
	connection.Catalog = make([]Model, 0, len(models))
	for _, alias := range models {
		model, ok := catalogByAlias[alias]
		if !ok {
			model = Model{Alias: alias, DisplayName: alias}
		}
		connection.Catalog = append(connection.Catalog, model)
	}
	connection.BaseURL = strings.TrimSuffix(connection.BaseURL, "/")
	return connection, nil
}

func (m *Manager) requireStopped(ctx context.Context, executable string) error {
	if executable == "" {
		return fmt.Errorf("%w: an exact executable path is required to prove the client is closed", ErrUnsupported)
	}
	if !filepath.IsAbs(executable) {
		return fmt.Errorf("%w: executable path must be absolute", ErrUnsafePath)
	}
	running, err := m.running.Running(ctx, executable)
	if err != nil {
		return fmt.Errorf("%w: cannot determine whether the client is running: %v", ErrUnsupported, err)
	}
	if running {
		return ErrClientRunning
	}
	return nil
}
