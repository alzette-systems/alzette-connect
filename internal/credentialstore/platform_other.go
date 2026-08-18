//go:build !linux && !darwin && !windows

package credentialstore

func NewPlatform() Store {
	return Unavailable{Reason: "this operating system has no supported protected credential store"}
}
