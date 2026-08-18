//go:build windows

package credentialstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danieljoos/wincred"
)

const windowsTargetPrefix = "Alzette Connect/refresh/"

// WindowsCredentialStore stores refresh credentials through CredRead/CredWrite
// in the current user's Windows Credential Manager. No secret is written to a
// file or passed to a subprocess.
type WindowsCredentialStore struct{ lockDir string }

func NewPlatform() Store {
	root, err := os.UserCacheDir()
	if err != nil || root == "" {
		return Unavailable{Reason: "Windows user cache directory is unavailable"}
	}
	return &WindowsCredentialStore{lockDir: filepath.Join(root, "Alzette Connect", "locks")}
}

func NewWindowsCredentialStore() Store { return NewPlatform() }

func (s *WindowsCredentialStore) Kind() string { return "windows-credential-manager" }

func (s *WindowsCredentialStore) Load(_ context.Context, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	credential, err := wincred.GetGenericCredential(windowsTargetPrefix + profile)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("%w: Windows Credential Manager read failed", ErrUnavailable)
	}
	defer clear(credential.CredentialBlob)
	value := string(credential.CredentialBlob)
	if err := validate(profile, value, true); err != nil {
		return "", fmt.Errorf("%w: stored Windows credential is invalid", ErrUnavailable)
	}
	return value, nil
}

func (s *WindowsCredentialStore) Save(_ context.Context, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	// Windows limits CRED_TYPE_GENERIC blobs to 5*512 bytes.
	if len(value) > 5*512 {
		return errors.New("refresh credential exceeds the Windows protected-storage limit")
	}
	credential := wincred.NewGenericCredential(windowsTargetPrefix + profile)
	credential.UserName = profile
	credential.Comment = "Alzette Connect rotating refresh credential"
	credential.CredentialBlob = []byte(value)
	defer clear(credential.CredentialBlob)
	if err := credential.Write(); err != nil {
		return fmt.Errorf("%w: Windows Credential Manager write failed", ErrUnavailable)
	}
	return nil
}

func (s *WindowsCredentialStore) Delete(_ context.Context, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	credential, err := wincred.GetGenericCredential(windowsTargetPrefix + profile)
	if errors.Is(err, wincred.ErrElementNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: Windows Credential Manager read failed", ErrUnavailable)
	}
	defer clear(credential.CredentialBlob)
	if err := credential.Delete(); err != nil && !errors.Is(err, wincred.ErrElementNotFound) {
		return fmt.Errorf("%w: Windows Credential Manager delete failed", ErrUnavailable)
	}
	return nil
}

func (s *WindowsCredentialStore) Acquire(ctx context.Context, profile string) (func(), error) {
	return acquireFileLock(ctx, s.lockDir, profile)
}
