package clientconfig

import (
	"context"
	"fmt"
)

type unavailableSecrets struct{ reason string }

func (u unavailableSecrets) Get(context.Context, string, string) (string, bool, error) {
	return "", false, fmt.Errorf("%w: %s", ErrSecretStore, u.reason)
}
func (u unavailableSecrets) Set(context.Context, string, string, string) error {
	return fmt.Errorf("%w: %s", ErrSecretStore, u.reason)
}
func (u unavailableSecrets) Delete(context.Context, string, string) error {
	return fmt.Errorf("%w: %s", ErrSecretStore, u.reason)
}
