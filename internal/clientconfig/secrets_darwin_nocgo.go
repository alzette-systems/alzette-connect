//go:build darwin && !cgo

package clientconfig

func newPlatformSecretStore() SecretStore {
	return unavailableSecrets{"macOS Keychain requires a CGO-enabled signed build"}
}
