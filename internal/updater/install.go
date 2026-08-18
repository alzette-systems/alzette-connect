package updater

import (
	"errors"
	"os"
)

const helperFlag = "--apply-connect-update"

// StartInstall launches the platform updater only after the package has passed
// the pinned release URL, size, and SHA-256 checks in Download.
func StartInstall(assetPath string) error {
	if assetPath == "" {
		return errors.New("verified update package is missing")
	}
	return startInstall(assetPath)
}

// HandleHelper runs before Wails starts. The helper receives only local paths
// and a process identifier; no OAuth or Alzette credential crosses this seam.
func HandleHelper(arguments []string) (bool, error) {
	if len(arguments) < 2 || arguments[1] != helperFlag {
		return false, nil
	}
	if len(arguments) != 5 {
		return true, errors.New("invalid update helper request")
	}
	return true, applyUpdate(arguments[2], arguments[3], arguments[4])
}

func currentExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil || path == "" {
		return "", errors.New("Alzette Connect could not locate its installed application")
	}
	return path, nil
}
