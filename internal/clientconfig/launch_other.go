//go:build !windows

package clientconfig

import "os/exec"

func prepareProcessSupervision(_ *exec.Cmd) (func() error, func() error, error) {
	return func() error { return nil }, nil, nil
}
