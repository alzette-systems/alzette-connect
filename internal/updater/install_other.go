//go:build !darwin && !windows && !linux

package updater

func startInstall(string, string) error                { return ErrUnsupportedOS }
func applyUpdate(string, string, string, string) error { return ErrUnsupportedOS }
func reopenAfterUpdateFailure(string) error            { return nil }
