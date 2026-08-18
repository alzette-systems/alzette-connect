//go:build linux

package clientconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type processChecker struct{}

func (processChecker) Running(ctx context.Context, executable string) (bool, error) {
	target, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		path, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil && !errors.Is(err, os.ErrPermission) {
			continue
		}
		if path == target {
			return true, nil
		}
	}
	return false, nil
}
