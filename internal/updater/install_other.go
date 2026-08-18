//go:build !darwin && !windows && !linux

package updater

func startInstall(string) error                { return ErrUnsupportedOS }
func applyUpdate(string, string, string) error { return ErrUnsupportedOS }
