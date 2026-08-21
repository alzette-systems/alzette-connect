package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ticruz38/alzette-connect/internal/appstate"
	"github.com/ticruz38/alzette-connect/internal/clientconfig"
)

type desktopClients struct {
	piExecutable      string
	janExecutable     string
	gooseExecutable   string
	gooseASAR         string
	chatGPTExecutable string
}

// Set by release packaging only for explicitly labelled internal candidate
// builds. Production/default builds keep an unaccepted adapter non-launchable.
var chatGPTCandidateEnabled = "false"

func chatGPTCandidateIsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(chatGPTCandidateEnabled), "true")
}

func (c *desktopClients) applicationStates() []appstate.Application {
	status := func(path string) string {
		if path == "" {
			return "not_installed"
		}
		return "verification_required"
	}
	gooseStatus := status(c.gooseExecutable)
	if c.gooseExecutable != "" && c.gooseASAR == "" {
		gooseStatus = "needs_attention"
	}
	values := []appstate.Application{
		{ID: "pi", Name: "Pi", Status: status(c.piExecutable), DeliveryMode: "catalogue", Installed: c.piExecutable != "", Detail: "Qualification runs before launch"},
		{ID: "jan", Name: "Jan Desktop", Status: status(c.janExecutable), DeliveryMode: "catalogue", Installed: c.janExecutable != "", Detail: "Version 0.8.4"},
		{ID: "goose", Name: "Goose Desktop", Status: gooseStatus, DeliveryMode: "catalogue", Installed: c.gooseExecutable != "", Detail: "Version 1.46.0"},
	}
	chatGPTStatus := status(c.chatGPTExecutable)
	chatGPTDetail := "Codex workspace · temporary Alzette Responses profile prepared at launch"
	if !chatGPTCandidateIsEnabled() {
		chatGPTStatus = "not_supported"
		chatGPTDetail = "Internal adapter candidate is disabled in this build"
	} else if runtime.GOOS == "windows" {
		chatGPTStatus = "not_supported"
		chatGPTDetail = "Windows Store application integration is not yet available"
	} else if runtime.GOOS == "linux" {
		chatGPTStatus = "protocol_unavailable"
		chatGPTDetail = "ChatGPT's Codex workspace is macOS-only in the current candidate"
	}
	values = append(values, appstate.Application{ID: "chatgpt", Name: "ChatGPT", Status: chatGPTStatus, DeliveryMode: "primary_plus_catalogue", Installed: c.chatGPTExecutable != "", Detail: chatGPTDetail})
	return values
}

func discoverDesktopClients(home string) *desktopClients {
	result := &desktopClients{
		piExecutable:      validExecutableOverride(os.Getenv("ALZETTE_CONNECT_PI_EXECUTABLE")),
		janExecutable:     validExecutableOverride(os.Getenv("ALZETTE_CONNECT_JAN_EXECUTABLE")),
		gooseExecutable:   validExecutableOverride(os.Getenv("ALZETTE_CONNECT_GOOSE_EXECUTABLE")),
		gooseASAR:         validRegularOverride(os.Getenv("ALZETTE_CONNECT_GOOSE_ASAR")),
		chatGPTExecutable: validExecutableOverride(os.Getenv("ALZETTE_CONNECT_CHATGPT_EXECUTABLE")),
	}
	var piCandidates, janCandidates, gooseCandidates, asarCandidates, chatGPTCandidates []string
	switch runtime.GOOS {
	case "darwin":
		piCandidates = []string{"/opt/homebrew/bin/pi", "/usr/local/bin/pi", filepath.Join(home, ".local", "bin", "pi")}
		janCandidates = []string{
			"/Applications/Jan.app/Contents/MacOS/Jan",
			filepath.Join(home, "Applications", "Jan.app", "Contents", "MacOS", "Jan"),
		}
		gooseCandidates = []string{
			"/Applications/Goose.app/Contents/MacOS/Goose",
			"/Applications/goose.app/Contents/MacOS/goose",
			filepath.Join(home, "Applications", "Goose.app", "Contents", "MacOS", "Goose"),
			filepath.Join(home, "Applications", "goose.app", "Contents", "MacOS", "goose"),
		}
		asarCandidates = []string{
			"/Applications/Goose.app/Contents/Resources/app.asar",
			"/Applications/goose.app/Contents/Resources/app.asar",
			filepath.Join(home, "Applications", "Goose.app", "Contents", "Resources", "app.asar"),
			filepath.Join(home, "Applications", "goose.app", "Contents", "Resources", "app.asar"),
		}
		chatGPTCandidates = []string{
			"/Applications/ChatGPT.app/Contents/MacOS/ChatGPT",
			filepath.Join(home, "Applications", "ChatGPT.app", "Contents", "MacOS", "ChatGPT"),
		}
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		piCandidates = []string{filepath.Join(local, "Programs", "pi", "pi.exe")}
		janCandidates = []string{filepath.Join(local, "Programs", "Jan", "Jan.exe")}
		gooseCandidates = []string{
			filepath.Join(local, "Programs", "Goose", "Goose.exe"),
			filepath.Join(local, "Programs", "goose", "goose.exe"),
		}
		asarCandidates = []string{
			filepath.Join(local, "Programs", "Goose", "resources", "app.asar"),
			filepath.Join(local, "Programs", "goose", "resources", "app.asar"),
		}
	case "linux":
		piCandidates = []string{filepath.Join(home, ".local", "bin", "pi"), "/usr/local/bin/pi", "/usr/bin/pi"}
		janCandidates = []string{filepath.Join(home, ".local", "bin", "jan"), "/usr/local/bin/jan", "/usr/bin/jan"}
		gooseCandidates = []string{filepath.Join(home, ".local", "bin", "goose"), "/usr/local/bin/goose", "/usr/bin/goose"}
		asarCandidates = []string{
			filepath.Join(home, ".local", "lib", "goose", "resources", "app.asar"),
			"/opt/Goose/resources/app.asar", "/opt/goose/resources/app.asar",
		}
	}
	if result.piExecutable == "" {
		result.piExecutable = firstExecutable(piCandidates)
	}
	if result.janExecutable == "" {
		result.janExecutable = firstRegular(janCandidates, true)
	}
	if result.gooseExecutable == "" {
		result.gooseExecutable = firstRegular(gooseCandidates, true)
	}
	if result.gooseASAR == "" {
		result.gooseASAR = firstRegular(asarCandidates, false)
	}
	if result.chatGPTExecutable == "" {
		result.chatGPTExecutable = firstExecutable(chatGPTCandidates)
	}
	return result
}

// firstExecutable accepts a canonical installation symlink only after it has
// been resolved to an absolute regular executable. The resolved target—not the
// writable link—is retained for launch.
func firstExecutable(candidates []string) string {
	for _, candidate := range candidates {
		if path := validPath(candidate, true); path != "" {
			return path
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil && filepath.IsAbs(resolved) {
			if path := validPath(resolved, true); path != "" {
				return path
			}
		}
	}
	return ""
}

func validExecutableOverride(path string) string { return validPath(path, true) }
func validRegularOverride(path string) string    { return validPath(path, false) }

func validPath(path string, executable bool) string {
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	if executable {
		if runtime.GOOS == "windows" {
			if !strings.EqualFold(filepath.Ext(path), ".exe") {
				return ""
			}
		} else if info.Mode().Perm()&0o111 == 0 {
			return ""
		}
	}
	return path
}

func firstRegular(candidates []string, executable bool) string {
	for _, candidate := range candidates {
		if path := validPath(candidate, executable); path != "" {
			return path
		}
	}
	return ""
}

func friendlyClientError(name string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("Open %s once, close it, then try again", name)
	case errors.Is(err, clientconfig.ErrClientRunning):
		return fmt.Errorf("Close %s, then try again", name)
	case errors.Is(err, clientconfig.ErrWrongVersion):
		return fmt.Errorf("This %s version is not supported by this Connect build", name)
	case errors.Is(err, clientconfig.ErrConflict):
		return fmt.Errorf("%s has a conflicting Alzette Connect profile; review its existing profile before trying again", name)
	case errors.Is(err, clientconfig.ErrSecretStore):
		return fmt.Errorf("Unlock your computer's protected credential store, then try again")
	case errors.Is(err, clientconfig.ErrUnsafePath):
		return fmt.Errorf("%s is installed in a location Connect cannot safely modify", name)
	case errors.Is(err, clientconfig.ErrUnsupported):
		return fmt.Errorf("Install the supported %s desktop version, then try again", name)
	default:
		return fmt.Errorf("%s setup could not be completed", name)
	}
}
