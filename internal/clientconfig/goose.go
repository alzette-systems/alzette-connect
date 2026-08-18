package clientconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	gooseProviderID     = "custom_alzette_connect"
	gooseSecretEnv      = "CUSTOM_ALZETTE_CONNECT_API_KEY"
	gooseKeyringService = "goose"
	gooseKeyringAccount = "secrets"
)

func (m *Manager) ConfigureGoose(ctx context.Context, request GooseRequest) (*Result, error) {
	connection, err := validateConnection(request.Connection)
	if err != nil {
		return nil, err
	}
	if err := m.requireStopped(ctx, request.ExecutablePath); err != nil {
		return nil, err
	}
	if request.AppASARPath == "" {
		return nil, fmt.Errorf("%w: Goose app.asar path is required for exact version proof", ErrUnsupported)
	}
	version, err := electronPackageVersion(request.AppASARPath)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot prove Goose version: %v", ErrUnsupported, err)
	}
	if version != GooseSupportedVersion {
		return nil, fmt.Errorf("%w: Goose %q, need %s", ErrWrongVersion, version, GooseSupportedVersion)
	}
	configDir := request.ConfigDir
	if configDir == "" {
		configDir = m.gooseConfigDir()
	}
	configPath := filepath.Join(configDir, "config.yaml")
	unlock, err := acquireConfigLock(ctx, configPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	configBytes, configMode, configExisted, err := readSafe(configPath, false)
	if err != nil {
		return nil, fmt.Errorf("read Goose config: %w", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(configBytes, &document); err != nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: Goose config YAML is malformed", ErrUnsupported)
	}
	root := document.Content[0]
	setYAMLScalar(root, "active_provider", gooseProviderID)
	providers := ensureYAMLMapping(root, "providers")
	existing := mappingValue(providers, gooseProviderID)
	if existing != nil {
		configured := mappingValue(existing, "configured")
		if configured == nil || configured.Value != "true" {
			return nil, fmt.Errorf("%w: Goose provider %q", ErrConflict, gooseProviderID)
		}
	}
	providerConfig := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setYAMLBool(providerConfig, "enabled", true)
	setYAMLScalar(providerConfig, "model", connection.Models[0])
	setYAMLBool(providerConfig, "configured", true)
	setMappingValue(providers, gooseProviderID, providerConfig)
	updatedConfig, err := yaml.Marshal(&document)
	if err != nil {
		return nil, err
	}
	providerPath := filepath.Join(configDir, "custom_providers", gooseProviderID+".json")
	providerBefore, providerMode, providerExisted, err := readSafe(providerPath, true)
	if err != nil {
		return nil, fmt.Errorf("read Goose provider: %w", err)
	}
	if providerExisted {
		var existingProvider struct {
			Name, Description string
		}
		if json.Unmarshal(providerBefore, &existingProvider) != nil || existingProvider.Name != gooseProviderID || existingProvider.Description != "Managed by Alzette Connect" {
			return nil, fmt.Errorf("%w: Goose custom provider %q", ErrConflict, gooseProviderID)
		}
	}
	providerBytes, err := json.MarshalIndent(gooseProvider(connection), "", "  ")
	if err != nil {
		return nil, err
	}
	providerBytes = append(providerBytes, '\n')
	secretBefore, found, err := m.secrets.Get(ctx, gooseKeyringService, gooseKeyringAccount)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSecretStore, err)
	}
	secrets := make(map[string]json.RawMessage)
	if found && json.Unmarshal([]byte(secretBefore), &secrets) != nil {
		return nil, fmt.Errorf("%w: Goose protected secrets entry is malformed", ErrUnsupported)
	}
	capability, _ := json.Marshal(connection.Capability)
	secrets[gooseSecretEnv] = capability
	secretAfterBytes, _ := json.Marshal(secrets)
	tx := &transaction{}
	tx.addFile(configPath, configBytes, configMode, configExisted, updatedConfig)
	tx.addFile(providerPath, providerBefore, providerMode, providerExisted, providerBytes)
	if !found || secretBefore != string(secretAfterBytes) {
		tx.secrets = append(tx.secrets, secretChange{service: gooseKeyringService, account: gooseKeyringAccount, before: secretBefore, after: string(secretAfterBytes), existed: found})
	}
	if err := tx.apply(ctx, m.secrets); err != nil {
		return nil, err
	}
	return tx.resultWithStore(Goose, GooseSupportedVersion, m.secrets), nil
}

func (m *Manager) gooseConfigDir() string {
	switch m.goos {
	case "linux":
		return filepath.Join(m.userConfigDir, "goose")
	case "darwin":
		return filepath.Join(m.userDataDir, "Block", "goose")
	default:
		return filepath.Join(m.userConfigDir, "Block", "goose", "config")
	}
}

func gooseProvider(connection Connection) map[string]any {
	models := make([]map[string]any, 0, len(connection.Models))
	for _, model := range connection.Models {
		models = append(models, map[string]any{"name": model, "context_limit": 128000})
	}
	return map[string]any{
		"name": gooseProviderID, "engine": "openai", "display_name": "Alzette Connect",
		"description": "Managed by Alzette Connect", "api_key_env": gooseSecretEnv,
		"base_url": strings.TrimSuffix(connection.BaseURL, "/v1"), "models": models,
		"headers": nil, "timeout_seconds": nil, "supports_streaming": true,
		"requires_auth": true, "catalog_provider_id": nil, "base_path": "v1/chat/completions",
		"env_vars": nil, "dynamic_models": false, "skip_canonical_filtering": false,
		"model_doc_link": nil, "setup_steps": []any{}, "fast_model": nil, "preserves_thinking": true,
	}
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content[index+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func ensureYAMLMapping(mapping *yaml.Node, key string) *yaml.Node {
	if value := mappingValue(mapping, key); value != nil && value.Kind == yaml.MappingNode {
		return value
	}
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setMappingValue(mapping, key, value)
	return value
}

func setYAMLScalar(mapping *yaml.Node, key, value string) {
	setMappingValue(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func setYAMLBool(mapping *yaml.Node, key string, value bool) {
	setMappingValue(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprint(value)})
}
