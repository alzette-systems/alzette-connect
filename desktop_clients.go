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
	janExecutable   string
	gooseExecutable string
	gooseASAR       string
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
	return []appstate.Application{
		{ID: "jan", Status: status(c.janExecutable)},
		{ID: "goose", Status: gooseStatus},
	}
}

func discoverDesktopClients(home string) *desktopClients {
	result := &desktopClients{
		janExecutable:   validExecutableOverride(os.Getenv("ALZETTE_CONNECT_JAN_EXECUTABLE")),
		gooseExecutable: validExecutableOverride(os.Getenv("ALZETTE_CONNECT_GOOSE_EXECUTABLE")),
		gooseASAR:       validRegularOverride(os.Getenv("ALZETTE_CONNECT_GOOSE_ASAR")),
	}
	var janCandidates, gooseCandidates, asarCandidates []string
	switch runtime.GOOS {
	case "darwin":
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
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
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
		janCandidates = []string{filepath.Join(home, ".local", "bin", "jan"), "/usr/local/bin/jan", "/usr/bin/jan"}
		gooseCandidates = []string{filepath.Join(home, ".local", "bin", "goose"), "/usr/local/bin/goose", "/usr/bin/goose"}
		asarCandidates = []string{
			filepath.Join(home, ".local", "lib", "goose", "resources", "app.asar"),
			"/opt/Goose/resources/app.asar", "/opt/goose/resources/app.asar",
		}
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
	return result
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
		return fmt.Errorf("%s already has an Alzette Connect entry that Connect does not own", name)
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
