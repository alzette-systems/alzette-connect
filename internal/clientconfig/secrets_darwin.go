//go:build darwin && cgo

package clientconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/keybase/go-keychain"
)

type darwinSecrets struct{}

func newPlatformSecretStore() SecretStore { return darwinSecrets{} }

func (darwinSecrets) Get(_ context.Context, service, account string) (string, bool, error) {
	value, err := keychain.GetGenericPassword(service, account, "", "")
	if errors.Is(err, keychain.ErrorItemNotFound) || value == nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("macOS Keychain read failed")
	}
	defer clear(value)
	return string(value), true, nil
}

func (darwinSecrets) Set(_ context.Context, service, account, value string) error {
	data := []byte(value)
	defer clear(data)
	item := keychain.NewGenericPassword(service, account, "Alzette Connect", data, "")
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	if err := keychain.AddItem(item); err == nil {
		return nil
	} else if !errors.Is(err, keychain.ErrorDuplicateItem) {
		return errors.New("macOS Keychain write failed")
	}
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService(service)
	query.SetAccount(account)
	update := keychain.NewItem()
	update.SetData(data)
	update.SetSynchronizable(keychain.SynchronizableNo)
	update.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	if err := keychain.UpdateItem(query, update); err != nil {
		return errors.New("macOS Keychain update failed")
	}
	return nil
}

func (darwinSecrets) Delete(_ context.Context, service, account string) error {
	err := keychain.DeleteGenericPasswordItem(service, account)
	if err != nil && !errors.Is(err, keychain.ErrorItemNotFound) {
		return errors.New("macOS Keychain delete failed")
	}
	return nil
}
