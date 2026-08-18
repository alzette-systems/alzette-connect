//go:build darwin || linux

package clientconfig

import (
	"fmt"
	"os"
	"syscall"
)

func ensurePrivateOwner(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("%w: %s is not owned by the current user", ErrUnsafePath, path)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%w: %s is group/world writable", ErrUnsafePath, path)
	}
	return nil
}

func ensureTrustedEvidence(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) && stat.Uid != 0 {
		return fmt.Errorf("%w: %s has an untrusted owner", ErrUnsafePath, path)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%w: %s is not immutable to other users", ErrUnsafePath, path)
	}
	return nil
}
