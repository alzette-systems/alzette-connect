package clientconfig

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

type fileChange struct {
	path       string
	before     []byte
	after      []byte
	existed    bool
	mode       os.FileMode
	backupPath string
	backupMade bool
}

type secretChange struct {
	service, account string
	before, after    string
	existed          bool
}

type transaction struct {
	files   []fileChange
	secrets []secretChange
}

func acquireConfigLock(ctx context.Context, path string) (func(), error) {
	if err := ensureExistingParentPrivate(filepath.Dir(path)); err != nil {
		return nil, err
	}
	lock := flock.New(path + ".alzette-connect.lock")
	locked, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("%w: another Connect process is configuring this client", ErrClientRunning)
	}
	_ = os.Chmod(path+".alzette-connect.lock", 0o600)
	return func() { _ = lock.Unlock(); _ = lock.Close() }, nil
}

func readSafe(path string, allowMissing bool) ([]byte, os.FileMode, bool, error) {
	if !filepath.IsAbs(path) {
		return nil, 0, false, fmt.Errorf("%w: path must be absolute", ErrUnsafePath)
	}
	if err := rejectSymlinks(path); err != nil {
		return nil, 0, false, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		if err := ensureExistingParentPrivate(filepath.Dir(path)); err != nil {
			return nil, 0, false, err
		}
		return nil, 0o600, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, fmt.Errorf("%w: %s is not a regular file", ErrUnsafePath, path)
	}
	if err := ensurePrivateOwner(path); err != nil {
		return nil, 0, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16<<20))
	if err != nil {
		return nil, 0, false, err
	}
	if len(data) == 16<<20 {
		return nil, 0, false, fmt.Errorf("%w: configuration file is too large", ErrUnsupported)
	}
	return data, info.Mode().Perm(), true, nil
}

func ensureExistingParentPrivate(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		_, err := os.Lstat(current)
		if err == nil {
			return ensurePrivateOwner(current)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Dir(current) == current {
			return err
		}
	}
}

func rejectSymlinks(path string) error {
	clean := filepath.Clean(path)
	for current := clean; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in path %s", ErrUnsafePath, current)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func (t *transaction) addFile(path string, before []byte, mode os.FileMode, existed bool, after []byte) {
	if bytes.Equal(before, after) {
		return
	}
	t.files = append(t.files, fileChange{path: path, before: append([]byte(nil), before...), after: append([]byte(nil), after...), mode: mode, existed: existed, backupPath: path + ".alzette-connect.bak"})
}

func (t *transaction) addSecret(ctx context.Context, store SecretStore, service, account, after string) error {
	before, found, err := store.Get(ctx, service, account)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStore, err)
	}
	if found && before == after {
		return nil
	}
	t.secrets = append(t.secrets, secretChange{service: service, account: account, before: before, after: after, existed: found})
	return nil
}

func (t *transaction) apply(ctx context.Context, store SecretStore) error {
	appliedFiles := 0
	appliedSecrets := 0
	defer func() {
		if r := recover(); r != nil {
			_ = t.rollbackApplied(context.Background(), store, appliedFiles, appliedSecrets, false)
			panic(r)
		}
	}()
	for index := range t.files {
		change := &t.files[index]
		if change.existed {
			if err := rejectSymlinks(change.backupPath); err != nil {
				return err
			}
			if _, err := os.Lstat(change.backupPath); errors.Is(err, os.ErrNotExist) {
				if err := atomicWrite(change.backupPath, change.before, 0o600); err != nil {
					_ = t.rollbackApplied(ctx, store, appliedFiles, appliedSecrets, false)
					return fmt.Errorf("create backup: %w", err)
				}
				change.backupMade = true
			} else if err != nil {
				return fmt.Errorf("inspect backup: %w", err)
			}
		}
		if err := atomicWrite(change.path, change.after, change.mode); err != nil {
			_ = t.rollbackApplied(ctx, store, appliedFiles, appliedSecrets, false)
			return err
		}
		appliedFiles++
	}
	for _, change := range t.secrets {
		if err := store.Set(ctx, change.service, change.account, change.after); err != nil {
			_ = t.rollbackApplied(ctx, store, appliedFiles, appliedSecrets, false)
			return fmt.Errorf("%w: %v", ErrSecretStore, err)
		}
		appliedSecrets++
	}
	return nil
}

func (t *transaction) rollbackApplied(ctx context.Context, store SecretStore, fileCount, secretCount int, checkCurrent bool) error {
	for index := secretCount - 1; index >= 0; index-- {
		change := t.secrets[index]
		if checkCurrent {
			current, found, err := store.Get(ctx, change.service, change.account)
			if err != nil || !found || current != change.after {
				return ErrStaleRollback
			}
		}
		var err error
		if change.existed {
			err = store.Set(ctx, change.service, change.account, change.before)
		} else {
			err = store.Delete(ctx, change.service, change.account)
		}
		if err != nil {
			return fmt.Errorf("rollback protected credential: %w", err)
		}
	}
	for index := fileCount - 1; index >= 0; index-- {
		change := t.files[index]
		if checkCurrent {
			current, _, found, err := readSafe(change.path, true)
			if err != nil || !found || sha256.Sum256(current) != sha256.Sum256(change.after) {
				return ErrStaleRollback
			}
		}
		if change.existed {
			if err := atomicWrite(change.path, change.before, change.mode); err != nil {
				return err
			}
		} else if err := os.Remove(change.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (t *transaction) result(client Client, version string) *Result {
	result := &Result{Client: client, Version: version, Status: Configured}
	for _, change := range t.files {
		result.ChangedFiles = append(result.ChangedFiles, change.path)
		if change.backupMade {
			result.BackupFiles = append(result.BackupFiles, change.backupPath)
		}
	}
	if len(t.files) == 0 && len(t.secrets) == 0 {
		result.Status = Unchanged
		return result
	}
	result.rollback = func(ctx context.Context) error {
		return t.rollbackApplied(ctx, nilStoreGuard{SecretStore: nil}, len(t.files), len(t.secrets), true)
	}
	return result
}

// resultWithStore is separate so the Result closure captures the exact native
// store used for the transaction without exposing it publicly.
func (t *transaction) resultWithStore(client Client, version string, store SecretStore) *Result {
	r := t.result(client, version)
	if r.Status == Configured {
		r.rollback = func(ctx context.Context) error {
			return t.rollbackApplied(ctx, store, len(t.files), len(t.secrets), true)
		}
	}
	return r
}

type nilStoreGuard struct{ SecretStore }

func (nilStoreGuard) Get(context.Context, string, string) (string, bool, error) {
	return "", false, ErrSecretStore
}
func (nilStoreGuard) Set(context.Context, string, string, string) error { return ErrSecretStore }
func (nilStoreGuard) Delete(context.Context, string, string) error      { return ErrSecretStore }

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := rejectSymlinks(path); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".alzette-connect-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode & 0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryPath, path)
}
