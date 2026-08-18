//go:build linux

package clientconfig

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type linuxSecrets struct{ path string }

func newPlatformSecretStore() SecretStore {
	path, err := exec.LookPath("secret-tool")
	if err != nil {
		return unavailableSecrets{"secret-tool is not installed"}
	}
	return linuxSecrets{path: path}
}

func (s linuxSecrets) args(service, account string) []string {
	return []string{"service", service, "username", account, "target", "default", "application", "rust-keyring"}
}

func (s linuxSecrets) Get(ctx context.Context, service, account string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, s.path, append([]string{"lookup"}, s.args(service, account)...)...)
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("Linux Secret Service read failed")
	}
	return strings.TrimSuffix(string(output), "\n"), true, nil
}

func (s linuxSecrets) Set(ctx context.Context, service, account, value string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args := append([]string{"store", "--label=Alzette Connect managed " + service}, s.args(service, account)...)
	command := exec.CommandContext(ctx, s.path, args...)
	command.Stdin = bytes.NewBufferString(value + "\n")
	if err := command.Run(); err != nil {
		return errors.New("Linux Secret Service write failed")
	}
	return nil
}

func (s linuxSecrets) Delete(ctx context.Context, service, account string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, s.path, append([]string{"clear"}, s.args(service, account)...)...)
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return errors.New("Linux Secret Service delete failed")
		}
	}
	return nil
}
