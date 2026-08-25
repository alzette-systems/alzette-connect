//go:build linux

package updater

import (
	"errors"
	"os/exec"
)

func startInstall(assetPath, _ string) error {
	if err := exec.Command("xdg-open", assetPath).Start(); err != nil {
		return errors.New("the system package installer could not be opened")
	}
	return nil
}

func applyUpdate(_, _, _, _ string) error {
	return ErrUnsupportedOS
}

func reopenAfterUpdateFailure(string) error { return nil }
