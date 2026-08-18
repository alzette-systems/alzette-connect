package clientconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

const (
	janProviderID      = "Alzette Connect"
	janKeyringService  = "jan-providers"
	janProviderSetting = "model-provider"
)

type janAppSettings struct {
	DataFolder string `json:"data_folder"`
}

type janProviderState struct {
	State struct {
		Providers        []map[string]any `json:"providers"`
		SelectedProvider string           `json:"selectedProvider"`
		SelectedModel    map[string]any   `json:"selectedModel"`
		DeletedModels    []any            `json:"deletedModels"`
	} `json:"state"`
	Version int `json:"version"`
}

func (m *Manager) ConfigureJan(ctx context.Context, request JanRequest) (*Result, error) {
	connection, err := validateConnection(request.Connection)
	if err != nil {
		return nil, err
	}
	if err := m.requireStopped(ctx, request.ExecutablePath); err != nil {
		return nil, err
	}
	appPath := request.AppSettingsPath
	if appPath == "" {
		appPath = filepath.Join(m.userDataDir, "Jan", "settings.json")
	}
	appBytes, _, _, err := readSafe(appPath, false)
	if err != nil {
		return nil, fmt.Errorf("read Jan app settings: %w", err)
	}
	var app janAppSettings
	if json.Unmarshal(appBytes, &app) != nil || !filepath.IsAbs(app.DataFolder) {
		return nil, fmt.Errorf("%w: Jan data_folder is missing or invalid", ErrUnsupported)
	}
	storePath := filepath.Join(app.DataFolder, "store.json")
	storeBytes, _, _, err := readSafe(storePath, false)
	if err != nil {
		return nil, fmt.Errorf("read Jan version metadata: %w", err)
	}
	var metadata struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(storeBytes, &metadata) != nil || metadata.Version != JanSupportedVersion {
		return nil, fmt.Errorf("%w: Jan %q, need %s", ErrWrongVersion, metadata.Version, JanSupportedVersion)
	}
	settingsPath := filepath.Join(app.DataFolder, "settings.json")
	unlock, err := acquireConfigLock(ctx, settingsPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	settingsBytes, mode, existed, err := readSafe(settingsPath, false)
	if err != nil {
		return nil, fmt.Errorf("read Jan settings: %w", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		return nil, fmt.Errorf("%w: invalid Jan settings JSON", ErrUnsupported)
	}
	var encoded string
	if err := json.Unmarshal(settings[janProviderSetting], &encoded); err != nil {
		return nil, fmt.Errorf("%w: Jan model-provider state is absent or malformed", ErrUnsupported)
	}
	var state janProviderState
	if err := json.Unmarshal([]byte(encoded), &state); err != nil || state.Version != 17 {
		return nil, fmt.Errorf("%w: Jan model-provider schema is not version 17", ErrUnsupported)
	}
	provider := janProvider(connection)
	replaced := false
	for index, existing := range state.State.Providers {
		name, _ := existing["provider"].(string)
		if name != janProviderID {
			continue
		}
		managed, _ := existing["alzette_connect_managed"].(bool)
		if !managed {
			return nil, fmt.Errorf("%w: Jan provider %q", ErrConflict, janProviderID)
		}
		state.State.Providers[index] = provider
		replaced = true
	}
	if !replaced {
		state.State.Providers = append(state.State.Providers, provider)
	}
	state.State.SelectedProvider = janProviderID
	state.State.SelectedModel = map[string]any{"id": connection.Models[0], "model": connection.Models[0], "name": connection.Models[0]}
	providerBytes, _ := json.Marshal(state)
	encodedBytes, _ := json.Marshal(string(providerBytes))
	settings[janProviderSetting] = encodedBytes
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	updated = append(updated, '\n')
	keys, _ := json.Marshal([]string{connection.Capability})
	tx := &transaction{}
	tx.addFile(settingsPath, settingsBytes, mode, existed, updated)
	if err := tx.addSecret(ctx, m.secrets, janKeyringService, janProviderID, string(keys)); err != nil {
		return nil, err
	}
	if err := tx.apply(ctx, m.secrets); err != nil {
		return nil, err
	}
	return tx.resultWithStore(Jan, JanSupportedVersion, m.secrets), nil
}

func janProvider(connection Connection) map[string]any {
	models := make([]map[string]any, 0, len(connection.Models))
	for _, model := range connection.Models {
		models = append(models, map[string]any{"id": model, "model": model, "name": model, "capabilities": []string{"completion"}, "version": "1.0"})
	}
	return map[string]any{
		"provider": janProviderID, "active": true, "persist": true,
		"alzette_connect_managed": true,
		"models":                  models, "base_url": connection.BaseURL,
		"settings": []map[string]any{
			{"key": "api-key", "title": "API key", "description": "Managed securely by Alzette Connect", "controller_type": "input", "controller_props": map[string]any{"value": "", "type": "password", "placeholder": "Managed by Alzette Connect"}},
			{"key": "base-url", "title": "Base URL", "description": "Local Alzette Connect endpoint", "controller_type": "input", "controller_props": map[string]any{"value": connection.BaseURL, "type": "text"}},
		},
	}
}
