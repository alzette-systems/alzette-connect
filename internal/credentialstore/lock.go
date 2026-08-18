package credentialstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// acquireFileLock serializes rotating refresh-token use across processes. The
// lock file contains no credential material and is released by the OS if the
// process exits unexpectedly.
func acquireFileLock(ctx context.Context, root, profile string) (func(), error) {
	if err := validate(profile, "", false); err != nil {
		return nil, err
	}
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%w: refresh lock directory is invalid", ErrUnavailable)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("%w: create refresh lock directory", ErrUnavailable)
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: refresh lock directory is unsafe", ErrUnavailable)
	}
	// Chmod is effective on Unix and harmless on Windows. The parent directory
	// is the current user's OS-managed runtime/cache directory.
	if err := os.Chmod(root, 0o700); err != nil && !errors.Is(err, os.ErrPermission) {
		return nil, fmt.Errorf("%w: protect refresh lock directory", ErrUnavailable)
	}

	guard := flock.New(filepath.Join(root, profile+".lock"), flock.SetPermissions(0o600))
	locked, err := guard.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("%w: acquire refresh lock", ErrUnavailable)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = guard.Close() }) }, nil
}
