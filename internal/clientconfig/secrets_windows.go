//go:build windows

package clientconfig

import (
	"context"
	"errors"
	"unicode/utf16"

	"github.com/danieljoos/wincred"
)

type windowsSecrets struct{}

func newPlatformSecretStore() SecretStore { return windowsSecrets{} }

func windowsTarget(service, account string) string { return account + "." + service }

func (windowsSecrets) Get(_ context.Context, service, account string) (string, bool, error) {
	credential, err := wincred.GetGenericCredential(windowsTarget(service, account))
	if errors.Is(err, wincred.ErrElementNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, errors.New("Windows Credential Manager read failed")
	}
	defer clear(credential.CredentialBlob)
	if len(credential.CredentialBlob)%2 != 0 {
		return "", false, errors.New("Windows Credential Manager entry is malformed")
	}
	units := make([]uint16, len(credential.CredentialBlob)/2)
	for index := range units {
		units[index] = uint16(credential.CredentialBlob[index*2]) | uint16(credential.CredentialBlob[index*2+1])<<8
	}
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), true, nil
}

func (windowsSecrets) Set(_ context.Context, service, account, value string) error {
	units := append(utf16.Encode([]rune(value)), 0)
	if len(units)*2 > 5*512 {
		return errors.New("credential exceeds Windows protected-storage limit")
	}
	blob := make([]byte, len(units)*2)
	for index, unit := range units {
		blob[index*2], blob[index*2+1] = byte(unit), byte(unit>>8)
	}
	defer clear(blob)
	credential := wincred.NewGenericCredential(windowsTarget(service, account))
	credential.UserName = account
	credential.Comment = "Managed by Alzette Connect"
	credential.CredentialBlob = blob
	if err := credential.Write(); err != nil {
		return errors.New("Windows Credential Manager write failed")
	}
	return nil
}

func (windowsSecrets) Delete(_ context.Context, service, account string) error {
	credential, err := wincred.GetGenericCredential(windowsTarget(service, account))
	if errors.Is(err, wincred.ErrElementNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("Windows Credential Manager read failed")
	}
	defer clear(credential.CredentialBlob)
	return credential.Delete()
}
