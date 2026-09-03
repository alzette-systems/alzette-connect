//go:build darwin && cgo

package credentialstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alzette-systems/alzette-connect/internal/mackeychain"
)

const darwinService = "com.alzette.connect.refresh"

// DarwinKeychain stores the rotating identity refresh credential through the
// native Security.framework API. No credential is placed in process argv or a
// plaintext fallback.
type DarwinKeychain struct{ lockDir string }

func NewPlatform() Store {
	root, err := os.UserCacheDir()
	if err != nil || root == "" {
		return Unavailable{Reason: "macOS user cache directory is unavailable"}
	}
	return &DarwinKeychain{lockDir: filepath.Join(root, "Alzette Connect", "locks")}
}

func NewDarwinKeychain() Store { return NewPlatform() }

func (s *DarwinKeychain) Kind() string { return "macos-keychain" }

func (s *DarwinKeychain) Load(_ context.Context, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	value, err := mackeychain.Get(darwinService, profile)
	if err != nil {
		return "", mapDarwinError(err)
	}
	if value == nil {
		return "", ErrNotFound
	}
	defer clear(value)
	credential := string(value)
	if err := validate(profile, credential, true); err != nil {
		return "", fmt.Errorf("%w: stored macOS Keychain value is invalid", ErrUnavailable)
	}
	return credential, nil
}

func (s *DarwinKeychain) Save(_ context.Context, profile, credential string) error {
	if err := validate(profile, credential, true); err != nil {
		return err
	}
	data := []byte(credential)
	defer clear(data)
	return mapDarwinError(mackeychain.Set(darwinService, profile, data))
}

func (s *DarwinKeychain) Delete(_ context.Context, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	return mapDarwinError(mackeychain.Delete(darwinService, profile))
}

func (s *DarwinKeychain) Acquire(ctx context.Context, profile string) (func(), error) {
	return acquireFileLock(ctx, s.lockDir, profile)
}

func mapDarwinError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mackeychain.ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
