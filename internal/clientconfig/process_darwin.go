//go:build darwin

package clientconfig

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

type processChecker struct{}

func (processChecker) Running(ctx context.Context, executable string) (bool, error) {
	target, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return false, err
	}
	output, err := exec.CommandContext(ctx, "/bin/ps", "-axo", "comm=").Output()
	if err != nil {
		return false, err
	}
	for _, candidate := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(candidate) == target {
			return true, nil
		}
	}
	return false, nil
}
