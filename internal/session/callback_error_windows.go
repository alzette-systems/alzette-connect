//go:build windows

package session

import (
	"errors"

	"golang.org/x/sys/windows"
)

func callbackPortInUse(err error) bool {
	return errors.Is(err, windows.WSAEADDRINUSE)
}
