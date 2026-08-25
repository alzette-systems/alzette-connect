//go:build darwin && cgo

package clientconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/ticruz38/alzette-connect/internal/mackeychain"
)

type darwinSecrets struct{}

func newPlatformSecretStore() SecretStore { return darwinSecrets{} }

func (darwinSecrets) Get(_ context.Context, service, account string) (string, bool, error) {
	value, err := mackeychain.Get(service, account)
	if errors.Is(err, mackeychain.ErrNotFound) || value == nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("macOS Keychain read failed: %v", err)
	}
	defer clear(value)
	return string(value), true, nil
}

func (darwinSecrets) Set(_ context.Context, service, account, value string) error {
	data := []byte(value)
	defer clear(data)
	if err := mackeychain.Set(service, account, data); err != nil {
		return fmt.Errorf("macOS Keychain write failed: %v", err)
	}
	return nil
}

func (darwinSecrets) Delete(_ context.Context, service, account string) error {
	if err := mackeychain.Delete(service, account); err != nil {
		return fmt.Errorf("macOS Keychain delete failed: %v", err)
	}
	return nil
}
