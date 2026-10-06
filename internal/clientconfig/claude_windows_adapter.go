package clientconfig

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const claudeHelperFlag = "--alzette-claude-credential"
const claudeSecretService = "alzette-connect.claude-launch"

var claudeID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type claudeLaunchManifest struct {
	ID           string          `json:"id"`
	ProfileHash  string          `json:"profile_hash"`
	OriginalMeta json.RawMessage `json:"original_meta,omitempty"`
}
type claudeHelperCredential struct {
	BaseURL    string `json:"base_url"`
	Capability string `json:"capability"`
}

func (m *Manager) claudeLibrary(override string) (string, error) {
	if m.goos != "windows" {
		return "", ErrUnsupported
	}
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", ErrUnsafePath
		}
		return override, nil
	}
	local := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(local) {
		return "", ErrUnsafePath
	}
	return filepath.Join(local, "Claude-3p", "configLibrary"), nil
}

// ConfigureClaudeWindows activates a separate Claude third-party profile. The
// profile and recovery journal contain no credential; the helper retrieves a
// launch capability from the current user's Windows Credential Manager.
func (m *Manager) ConfigureClaudeWindows(ctx context.Context, request ClaudeRequest) (*Result, error) {
	library, err := m.claudeLibrary(request.LibraryDir)
	if err != nil {
		return nil, err
	}
	connection, err := validateConnection(request.Connection)
	if err != nil {
		return nil, err
	}
	connection, err = ClaudeConnection(connection)
	if err != nil {
		return nil, err
	}
	if err = m.requireStopped(ctx, request.ExecutablePath); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(request.HelperExecutablePath) {
		return nil, ErrUnsafePath
	}
	if err = ensureTrustedEvidence(request.HelperExecutablePath); err != nil {
		return nil, err
	}
	if request.LibraryDir == "" {
		if err = checkWindowsClaudePolicy(ctx); err != nil {
			return nil, err
		}
	}
	metaPath := filepath.Join(library, "_meta.json")
	unlock, err := acquireConfigLock(ctx, metaPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	journal := filepath.Join(library, ".alzette-connect-launch.json")
	if _, _, exists, e := readSafe(journal, true); e != nil {
		return nil, e
	} else if exists {
		return nil, ErrConflict
	}
	before, _, existed, err := readSafe(metaPath, true)
	if err != nil {
		return nil, err
	}
	meta := map[string]json.RawMessage{}
	if existed && json.Unmarshal(before, &meta) != nil {
		return nil, ErrConflict
	}
	var entries []map[string]json.RawMessage
	if raw, ok := meta["entries"]; ok && json.Unmarshal(raw, &entries) != nil {
		return nil, ErrConflict
	}
	var bytes [16]byte
	if _, err = rand.Read(bytes[:]); err != nil {
		return nil, err
	}
	id := fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
	models := make([]map[string]any, 0, len(connection.Catalog))
	for i, model := range connection.Catalog {
		entry := map[string]any{"name": model.Alias, "labelOverride": model.DisplayName, "maxEffort": "high"}
		if i == 0 {
			entry["anthropicFamilyTier"] = "sonnet"
			entry["isFamilyDefault"] = true
		}
		for _, tier := range []string{"haiku", "sonnet", "opus", "fable", "mythos"} {
			if strings.Contains(strings.ToLower(model.Alias), "claude-"+tier+"-") {
				entry["anthropicFamilyTier"] = tier
				entry["isFamilyDefault"] = true
				break
			}
		}
		models = append(models, entry)
	}
	profile := map[string]any{"inferenceProvider": "gateway", "inferenceGatewayBaseUrl": strings.TrimSuffix(connection.BaseURL, "/v1"), "inferenceGatewayAuthScheme": "bearer", "inferenceCredentialKind": "helper-script", "inferenceCredentialHelper": request.HelperExecutablePath, "inferenceCredentialHelperArgs": []string{claudeHelperFlag, id}, "inferenceCredentialHelperTtlSec": 300, "inferenceCredentialHelperTimeoutSec": 10, "inferenceCredentialHelperSilentRefreshEnabled": true, "modelDiscoveryEnabled": false, "modelCatalogEnabled": false, "inferenceModels": models, "defaultModelEffort": "low", "alwaysStartWithDefaultModel": true}
	after, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(after)
	manifest := claudeLaunchManifest{ID: id, ProfileHash: hex.EncodeToString(digest[:])}
	if existed {
		manifest.OriginalMeta = before
	}
	journalBytes, _ := json.Marshal(manifest)
	if err = atomicWrite(journal, journalBytes, 0o600); err != nil {
		return nil, err
	}
	cleanup := func() { _ = m.recoverClaudeLocked(context.Background(), library) }
	credential, _ := json.Marshal(claudeHelperCredential{BaseURL: connection.BaseURL, Capability: connection.Capability})
	if err = m.secrets.Set(ctx, claudeSecretService, id, string(credential)); err != nil {
		cleanup()
		return nil, ErrSecretStore
	}
	profilePath := filepath.Join(library, id+".json")
	if err = atomicWrite(profilePath, after, 0o600); err != nil {
		cleanup()
		return nil, err
	}
	entry := map[string]json.RawMessage{"id": json.RawMessage(fmt.Sprintf("%q", id)), "name": json.RawMessage(`"Alzette Connect"`)}
	entries = append(entries, entry)
	meta["entries"], _ = json.Marshal(entries)
	meta["appliedId"], _ = json.Marshal(id)
	delete(meta, "hybridPointer")
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err = atomicWrite(metaPath, metaBytes, 0o600); err != nil {
		cleanup()
		return nil, err
	}
	r := &Result{Client: Claude, Version: request.Version, Status: Configured, ChangedFiles: []string{profilePath, metaPath}}
	r.rollback = func(ctx context.Context) error { return m.RecoverClaudeWindows(ctx, request.ExecutablePath, library) }
	return r, nil
}

// ClaudeConnection supplies Claude-recognised transport IDs while preserving
// real model labels. Only the private Messages proxy resolves these IDs to
// assigned Alzette routes; they are never new gateway grants or provider models.
func ClaudeConnection(connection Connection) (Connection, error) {
	if len(connection.Models) == 0 || len(connection.Models) > 128 {
		return Connection{}, ErrClaudeModels
	}
	if len(connection.ModelAliases) != 0 {
		return connection, nil
	}
	connection.Models = append([]string(nil), connection.Models...)
	sort.Strings(connection.Models)
	byAlias := map[string]Model{}
	for _, model := range connection.Catalog {
		byAlias[model.Alias] = model
	}
	models := make([]string, 0, len(connection.Models))
	catalog := make([]Model, 0, len(connection.Models))
	mapping := map[string]string{}
	for index, actual := range connection.Models {
		if strings.TrimSpace(actual) == "" || len(actual) > 128 {
			return Connection{}, ErrClaudeModels
		}
		wire := "claude-sonnet-4-6"
		switch index {
		case 1:
			wire = "claude-opus-4-6"
		case 2:
			wire = "claude-haiku-4-5"
		default:
			if index > 2 {
				wire = fmt.Sprintf("claude-sonnet-4-6-alzette-%d", index+1)
			}
		}
		model := byAlias[actual]
		model.Alias = wire
		if model.DisplayName == "" {
			model.DisplayName = actual
		}
		models = append(models, wire)
		catalog = append(catalog, model)
		mapping[wire] = actual
	}
	connection.Models = models
	connection.Catalog = catalog
	connection.ModelAliases = mapping
	return connection, nil
}

func (m *Manager) RecoverClaudeWindows(ctx context.Context, executable, override string) error {
	library, err := m.claudeLibrary(override)
	if err != nil {
		return err
	}
	journal := filepath.Join(library, ".alzette-connect-launch.json")
	if _, _, exists, e := readSafe(journal, true); e != nil {
		return e
	} else if !exists {
		return nil
	}
	if err = m.requireStopped(ctx, executable); err != nil {
		return err
	}
	unlock, err := acquireConfigLock(ctx, filepath.Join(library, "_meta.json"))
	if err != nil {
		return err
	}
	defer unlock()
	return m.recoverClaudeLocked(ctx, library)
}

func (m *Manager) recoverClaudeLocked(ctx context.Context, library string) error {
	journal := filepath.Join(library, ".alzette-connect-launch.json")
	data, _, exists, err := readSafe(journal, true)
	if err != nil || !exists {
		return err
	}
	var manifest claudeLaunchManifest
	if json.Unmarshal(data, &manifest) != nil || !claudeID.MatchString(manifest.ID) || len(manifest.ProfileHash) != 64 {
		return ErrConflict
	}
	// Revoke the helper first, even if an edited profile needs manual review.
	if err = m.secrets.Delete(ctx, claudeSecretService, manifest.ID); err != nil {
		return ErrSecretStore
	}
	profilePath := filepath.Join(library, manifest.ID+".json")
	profile, _, present, err := readSafe(profilePath, true)
	if err != nil {
		return err
	}
	if present {
		hash := sha256.Sum256(profile)
		if hex.EncodeToString(hash[:]) != manifest.ProfileHash {
			return ErrStaleRollback
		}
	}
	metaPath := filepath.Join(library, "_meta.json")
	metaData, _, metaPresent, err := readSafe(metaPath, true)
	if err != nil {
		return err
	}
	if metaPresent {
		meta := map[string]json.RawMessage{}
		if json.Unmarshal(metaData, &meta) != nil {
			return ErrStaleRollback
		}
		var entries []map[string]json.RawMessage
		if json.Unmarshal(meta["entries"], &entries) != nil {
			return ErrStaleRollback
		}
		kept := entries[:0]
		for _, entry := range entries {
			var id string
			_ = json.Unmarshal(entry["id"], &id)
			if id != manifest.ID {
				kept = append(kept, entry)
			}
		}
		var applied string
		_ = json.Unmarshal(meta["appliedId"], &applied)
		original := map[string]json.RawMessage{}
		if len(manifest.OriginalMeta) > 0 && json.Unmarshal(manifest.OriginalMeta, &original) != nil {
			return ErrConflict
		}
		if applied == manifest.ID {
			if value, ok := original["appliedId"]; ok {
				meta["appliedId"] = value
			} else {
				delete(meta, "appliedId")
			}
			if value, ok := original["hybridPointer"]; ok {
				meta["hybridPointer"] = value
			}
		}
		meta["entries"], _ = json.Marshal(kept)
		updated, _ := json.MarshalIndent(meta, "", "  ")
		if len(manifest.OriginalMeta) == 0 && len(kept) == 0 {
			err = os.Remove(metaPath)
		} else {
			err = atomicWrite(metaPath, updated, 0o600)
		}
		if err != nil {
			return err
		}
	}
	if present {
		if err = os.Remove(profilePath); err != nil {
			return err
		}
	}
	return os.Remove(journal)
}

// HandleClaudeCredentialHelper must run before opening Connect's UI. Arguments
// contain only a public launch ID; errors deliberately exclude secret values.
func HandleClaudeCredentialHelper(args []string) (bool, error) {
	if len(args) < 2 || args[1] != claudeHelperFlag {
		return false, nil
	}
	if len(args) != 3 || !claudeID.MatchString(args[2]) {
		return true, ErrSecretStore
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return true, writeClaudeCredential(ctx, newPlatformSecretStore(), args[2], os.Stdout)
}

func writeClaudeCredential(ctx context.Context, store SecretStore, id string, out io.Writer) error {
	value, found, err := store.Get(ctx, claudeSecretService, id)
	if err != nil || !found {
		return ErrSecretStore
	}
	var credential claudeHelperCredential
	if json.Unmarshal([]byte(value), &credential) != nil {
		return ErrSecretStore
	}
	if _, err = validateConnection(Connection{BaseURL: credential.BaseURL, Capability: credential.Capability, Models: []string{"helper-probe"}}); err != nil {
		return ErrSecretStore
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, credential.BaseURL+"/models", nil)
	if err != nil {
		return ErrSecretStore
	}
	request.Header.Set("Authorization", "Bearer "+credential.Capability)
	// Never follow a redirect with a credential, and never use environment proxies.
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return ErrSecretStore
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrSecretStore
	}
	_, err = fmt.Fprintln(out, credential.Capability)
	return err
}
