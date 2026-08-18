//go:build linux

package credentialstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type commandRunner interface {
	Run(context.Context, []string, []byte) ([]byte, error)
}

type execRunner struct{ path string }

func (r execRunner) Run(ctx context.Context, arguments []string, input []byte) ([]byte, error) {
	command := exec.CommandContext(ctx, r.path, arguments...)
	command.Stdin = bytes.NewReader(input)
	return command.Output()
}

// LinuxSecretService uses libsecret's secret-tool frontend. A missing binary,
// session bus, or unlocked collection is an explicit unavailable error; it
// never falls back to a plaintext file.
type LinuxSecretService struct {
	runner     commandRunner
	runtimeDir string
}

func NewLinuxSecretService() Store {
	path, err := exec.LookPath("secret-tool")
	if err != nil {
		return Unavailable{Reason: "secret-tool is not installed"}
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return Unavailable{Reason: "XDG_RUNTIME_DIR is unavailable"}
	}
	info, err := os.Stat(runtimeDir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return Unavailable{Reason: "XDG_RUNTIME_DIR is not a private directory"}
	}
	return &LinuxSecretService{runner: execRunner{path: path}, runtimeDir: runtimeDir}
}

func (s *LinuxSecretService) Kind() string { return "linux-secret-service" }

func (s *LinuxSecretService) Load(ctx context.Context, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := s.runner.Run(ctx, []string{"lookup", "service", "alzette-connect", "profile", profile}, nil)
	if err != nil {
		return "", mapSecretToolError(err)
	}
	value := strings.TrimSuffix(string(output), "\n")
	if err := validate(profile, value, true); err != nil {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *LinuxSecretService) Save(ctx context.Context, profile, credential string) error {
	if err := validate(profile, credential, true); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.runner.Run(ctx, []string{"store", "--label=Alzette Connect", "service", "alzette-connect", "profile", profile}, []byte(credential+"\n"))
	if err != nil {
		return fmt.Errorf("%w: Linux Secret Service write failed", ErrUnavailable)
	}
	return nil
}

func (s *LinuxSecretService) Delete(ctx context.Context, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.runner.Run(ctx, []string{"clear", "service", "alzette-connect", "profile", profile}, nil)
	mapped := mapSecretToolError(err)
	if errors.Is(mapped, ErrNotFound) {
		return nil
	}
	return mapped
}

func mapSecretToolError(err error) error {
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return ErrNotFound
	}
	return fmt.Errorf("%w: Linux Secret Service request failed", ErrUnavailable)
}

// Acquire serializes refresh rotation across Connect processes. flock state is
// kernel-owned and is released if the process crashes.
func (s *LinuxSecretService) Acquire(ctx context.Context, profile string) (func(), error) {
	return acquireFileLock(ctx, filepath.Join(s.runtimeDir, "alzette-connect"), profile)
}
