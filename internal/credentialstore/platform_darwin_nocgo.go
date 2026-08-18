//go:build darwin && !cgo

package credentialstore

// The native Security.framework adapter requires cgo, as does the supported
// macOS desktop build. Headless/cross builds fail closed instead of storing a
// refresh credential elsewhere.
func NewPlatform() Store {
	return Unavailable{Reason: "macOS Keychain requires a cgo-enabled native build"}
}

func NewDarwinKeychain() Store { return NewPlatform() }
