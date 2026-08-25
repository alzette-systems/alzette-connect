package updater

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const installResultFilename = "update-result.json"

var userConfigDirectory = os.UserConfigDir

type installResult struct {
	State     string    `json:"state"`
	Version   string    `json:"version"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InstallFailure is a credential-free failure left by the detached updater.
// The next app process consumes it so a failed replacement is visible instead
// of looking like a successful quit followed by an old-version relaunch.
type InstallFailure struct {
	Version string
	Message string
}

func installResultPath() (string, error) {
	root, err := userConfigDirectory()
	if err != nil || !filepath.IsAbs(root) {
		return "", errors.New("locate update result folder")
	}
	return filepath.Join(root, "Alzette Connect", installResultFilename), nil
}

func recordInstallFailure(version string, installErr error) error {
	path, err := installResultPath()
	if err != nil {
		return err
	}
	message := "The previous update could not be installed."
	if installErr != nil && strings.TrimSpace(installErr.Error()) != "" {
		message = strings.TrimSpace(installErr.Error())
	}
	result := installResult{State: "error", Version: normalizeVersion(version), Message: message, UpdatedAt: time.Now().UTC()}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".update-result-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

// ConsumeInstallFailure returns at most one bounded updater error. Malformed or
// stale files are discarded rather than being rendered indefinitely.
func ConsumeInstallFailure() (InstallFailure, bool) {
	path, err := installResultPath()
	if err != nil {
		return InstallFailure{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InstallFailure{}, false
	}
	_ = os.Remove(path)
	var result installResult
	if json.Unmarshal(data, &result) != nil || result.State != "error" || time.Since(result.UpdatedAt) > 24*time.Hour {
		return InstallFailure{}, false
	}
	version := normalizeVersion(result.Version)
	if _, ok := parseVersion(version); !ok {
		return InstallFailure{}, false
	}
	message := strings.TrimSpace(result.Message)
	if message == "" || len(message) > 240 {
		return InstallFailure{}, false
	}
	return InstallFailure{Version: version, Message: message}, true
}
