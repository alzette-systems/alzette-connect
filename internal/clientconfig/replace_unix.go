//go:build darwin || linux

package clientconfig

import "os"

func replaceFile(source, destination string) error { return os.Rename(source, destination) }
