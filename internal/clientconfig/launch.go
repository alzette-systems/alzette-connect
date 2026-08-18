package clientconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Launch starts an exact client executable without a shell, arguments, or
// credentials. On macOS callers pass Contents/MacOS/<client>, not an .app
// directory. The process is detached from Connect after a successful start.
func Launch(ctx context.Context, executable string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !filepath.IsAbs(executable) {
		return 0, fmt.Errorf("%w: executable path must be absolute", ErrUnsafePath)
	}
	info, err := os.Stat(executable)
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 || runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(executable), ".exe") {
		return 0, fmt.Errorf("%w: client executable is not an executable regular file", ErrUnsafePath)
	}
	command := exec.Command(executable)
	command.Dir = filepath.Dir(executable)
	command.Env = launchEnvironment(os.Environ())
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
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
