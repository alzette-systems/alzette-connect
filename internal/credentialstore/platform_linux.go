//go:build linux

package credentialstore

func NewPlatform() Store { return NewLinuxSecretService() }
