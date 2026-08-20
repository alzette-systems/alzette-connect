package clientconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Process is a supervised child application. No command line, environment, or
// executable path is exposed through this handle.
type Process struct {
	PID  int
	Done <-chan error
	cmd  *exec.Cmd
}

// Launch starts an exact client executable without a shell, arguments, or
// credentials. On macOS callers pass Contents/MacOS/<client>, not an .app
// directory. The child is reaped in the background even when a legacy caller
// does not retain the supervised handle.
func Launch(ctx context.Context, executable string) (int, error) {
	process, err := launchObserved(ctx, executable, nil, nil, nil)
	if err != nil {
		return 0, err
	}
	return process.PID, nil
}

// LaunchObserved starts an exact qualified desktop executable and keeps a
// wait handle so Connect can reflect crashes/exits and disconnect safely.
func LaunchObserved(ctx context.Context, executable string) (*Process, error) {
	return launchObserved(ctx, executable, nil, nil, nil)
}

func launchObserved(ctx context.Context, executable string, arguments, environment []string, cleanup func()) (*Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(executable) {
		return nil, fmt.Errorf("%w: executable path must be absolute", ErrUnsafePath)
	}
	info, err := os.Stat(executable)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 || runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(executable), ".exe") {
		return nil, fmt.Errorf("%w: client executable is not an executable regular file", ErrUnsafePath)
	}
	command := exec.Command(executable, arguments...)
	command.Dir = filepath.Dir(executable)
	command.Env = append(launchEnvironment(os.Environ()), environment...)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	process := &Process{PID: command.Process.Pid, Done: done, cmd: command}
	go func() {
		err := command.Wait()
		if cleanup != nil {
			cleanup()
		}
		done <- err
		close(done)
	}()
	return process, nil
}

// Stop requests a graceful exit and escalates only after the caller's bounded
// context expires. A process that already exited is treated as stopped.
func (p *Process) Stop(ctx context.Context) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.Done:
		return nil
	case <-ctx.Done():
		if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return ctx.Err()
	}
}

func launchEnvironment(parent []string) []string {
	allowed := map[string]bool{
		"HOME": true, "PATH": true, "TMPDIR": true, "TEMP": true, "TMP": true,
		"USER": true, "USERNAME": true, "LOGNAME": true, "SHELL": true, "LANG": true,
		"DISPLAY": true, "WAYLAND_DISPLAY": true, "XDG_RUNTIME_DIR": true,
		"DBUS_SESSION_BUS_ADDRESS": true, "SYSTEMROOT": true, "WINDIR": true,
		"COMSPEC": true, "APPDATA": true, "LOCALAPPDATA": true, "USERPROFILE": true,
	}
	result := make([]string, 0, len(parent))
	for _, entry := range parent {
		name := entry
		if index := strings.IndexByte(entry, '='); index >= 0 {
			name = entry[:index]
		}
		upper := strings.ToUpper(name)
		if allowed[upper] || strings.HasPrefix(upper, "LC_") {
			result = append(result, entry)
		}
	}
	return result
}
