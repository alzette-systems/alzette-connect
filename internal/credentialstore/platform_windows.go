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
// in the current user's Windows Credential Manager. Long tokens are split into
// protected records behind an atomic generation pointer. No secret is written to a
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

type windowsRecords struct{}

func (windowsRecords) read(target string) ([]byte, error) {
	credential, err := wincred.GetGenericCredential(target)
	if errors.Is(err, wincred.ErrElementNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: Windows Credential Manager read failed", ErrUnavailable)
	}
	return credential.CredentialBlob, nil
}
func (windowsRecords) write(target string, value []byte) error {
	if len(value) > 5*512 {
		return ErrUnavailable
	}
	credential := wincred.NewGenericCredential(target)
	credential.Comment = "Alzette Connect rotating refresh credential"
	credential.CredentialBlob = value
	if err := credential.Write(); err != nil {
		return fmt.Errorf("%w: Windows Credential Manager write failed", ErrUnavailable)
	}
	return nil
}
func (windowsRecords) remove(target string) error {
	credential := wincred.NewGenericCredential(target)
	if err := credential.Delete(); err != nil && !errors.Is(err, wincred.ErrElementNotFound) {
		return fmt.Errorf("%w: Windows Credential Manager delete failed", ErrUnavailable)
	}
	return nil
}
func (windowsRecords) names(prefix string) ([]string, error) {
	credentials, err := wincred.FilteredList(prefix + "*")
	if err != nil {
		return nil, ErrUnavailable
	}
	names := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		names = append(names, credential.TargetName)
		clear(credential.CredentialBlob)
	}
	return names, nil
}
func (s *WindowsCredentialStore) Load(ctx context.Context, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	unlock, err := acquireFileLock(ctx, filepath.Join(s.lockDir, "records"), profile)
	if err != nil {
		return "", err
	}
	defer unlock()
	return loadRefreshRecord(windowsRecords{}, windowsTargetPrefix+profile)
}
func (s *WindowsCredentialStore) Save(ctx context.Context, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	unlock, err := acquireFileLock(ctx, filepath.Join(s.lockDir, "records"), profile)
	if err != nil {
		return err
	}
	defer unlock()
	return saveRefreshRecord(windowsRecords{}, windowsTargetPrefix+profile, value)
}
func (s *WindowsCredentialStore) Delete(ctx context.Context, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	unlock, err := acquireFileLock(ctx, filepath.Join(s.lockDir, "records"), profile)
	if err != nil {
		return err
	}
	defer unlock()
	return deleteRefreshRecord(windowsRecords{}, windowsTargetPrefix+profile)
}

func (s *WindowsCredentialStore) Acquire(ctx context.Context, profile string) (func(), error) {
	return acquireFileLock(ctx, s.lockDir, profile)
}
