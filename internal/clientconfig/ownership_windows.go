//go:build windows

package clientconfig

import (
	"fmt"
	"os"
)

func ensurePrivateOwner(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: reparse/symlink path", ErrUnsafePath)
	}
	// Windows ownership and DACL enforcement remains the responsibility of the
	// canonical per-user AppData directory and the native credential manager.
	return nil
}

func ensureTrustedEvidence(path string) error { return ensurePrivateOwner(path) }
