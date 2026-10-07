//go:build !windows

package session

import (
	"errors"
	"syscall"
)

func callbackPortInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
