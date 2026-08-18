package platform

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func OpenBrowser(target string) error {
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://127.0.0.1") && !strings.HasPrefix(target, "http://[::1]") {
		return errors.New("browser target is unsafe")
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Start()
}
