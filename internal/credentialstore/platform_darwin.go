//go:build darwin && cgo

package credentialstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keybase/go-keychain"
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
	value, err := keychain.GetGenericPassword(darwinService, profile, "", "")
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
	item := keychain.NewGenericPassword(darwinService, profile, "Alzette Connect", data, "")
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	if err := keychain.AddItem(item); err == nil {
		return nil
	} else if !errors.Is(err, keychain.ErrorDuplicateItem) {
		return mapDarwinError(err)
	}
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService(darwinService)
	query.SetAccount(profile)
	update := keychain.NewItem()
	update.SetData(data)
	update.SetSynchronizable(keychain.SynchronizableNo)
	update.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	return mapDarwinError(keychain.UpdateItem(query, update))
}

func (s *DarwinKeychain) Delete(_ context.Context, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	err := keychain.DeleteGenericPasswordItem(darwinService, profile)
	if errors.Is(err, keychain.ErrorItemNotFound) {
		return nil
	}
	return mapDarwinError(err)
}

func (s *DarwinKeychain) Acquire(ctx context.Context, profile string) (func(), error) {
	return acquireFileLock(ctx, s.lockDir, profile)
}

func mapDarwinError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, keychain.ErrorItemNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%w: macOS Keychain request failed", ErrUnavailable)
}
