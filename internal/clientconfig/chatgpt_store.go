package clientconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// This is the publisher in OpenAI's Store-signed unified ChatGPT MSIX.
const chatGPTWindowsPublisher = "CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B"

type windowsChatGPTPackage struct {
	Name          string `json:"name"`
	Publisher     string `json:"publisher"`
	SignatureKind string `json:"signature_kind"`
	Status        string `json:"status"`
	Version       string `json:"version"`
	Architecture  string `json:"architecture"`
	Location      string `json:"location"`
	Executable    string `json:"executable"`
	EntryPoint    string `json:"entry_point"`
}

// DiscoverWindowsChatGPT inspects the current user's registered package rather
// than guessing a versioned WindowsApps directory or accepting a display name.
func DiscoverWindowsChatGPT(ctx context.Context) (string, error) {
	packageInfo, err := inspectWindowsChatGPT(ctx)
	if err != nil {
		return "", err
	}
	return validatedWindowsChatGPTPath(packageInfo)
}

func inspectWindowsChatGPT(ctx context.Context) (windowsChatGPTPackage, error) {
	if runtime.GOOS != "windows" {
		return windowsChatGPTPackage{}, ErrUnsupported
	}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	const script = `$ErrorActionPreference='Stop';
$packages=@(Get-AppxPackage -Name OpenAI.Codex);
if ($packages.Count -ne 1) { exit 1 };
$p=$packages[0]; $m=Get-AppxPackageManifest -Package $p.PackageFullName;
$apps=@($m.Package.Applications.Application | Where-Object { $_.Id -eq 'App' });
if ($apps.Count -ne 1) { exit 1 };
$a=$apps[0];
@{name=$p.Name;publisher=$p.Publisher;signature_kind=[string]$p.SignatureKind;status=[string]$p.Status;version=[string]$p.Version;architecture=[string]$p.Architecture;location=$p.InstallLocation;executable=[string]$a.Executable;entry_point=[string]$a.EntryPoint} | ConvertTo-Json -Compress`
	command := exec.CommandContext(check, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = launchEnvironment(os.Environ())
	output, err := command.Output()
	var packageInfo windowsChatGPTPackage
	if err != nil || len(output) > 16<<10 || json.Unmarshal(output, &packageInfo) != nil {
		return windowsChatGPTPackage{}, fmt.Errorf("%w: registered ChatGPT package could not be inspected", ErrUnsupported)
	}
	return packageInfo, nil
}

func validatedWindowsChatGPTPath(p windowsChatGPTPackage) (string, error) {
	if p.Name != "OpenAI.Codex" || !strings.EqualFold(p.Publisher, chatGPTWindowsPublisher) ||
		p.SignatureKind != "Store" || p.Status != "Ok" || p.Architecture != "X64" || p.EntryPoint != "Windows.FullTrustApplication" ||
		strings.ReplaceAll(p.Executable, `\`, "/") != "app/ChatGPT.exe" ||
		!filepath.IsAbs(p.Location) || p.Version == "" || len(p.Version) > 64 || strings.ContainsAny(p.Version, "\r\n\t") {
		return "", fmt.Errorf("%w: ChatGPT package identity is not the trusted Windows workspace", ErrUnsupported)
	}
	path := filepath.Join(p.Location, "app", "ChatGPT.exe")
	if err := ensureTrustedEvidence(path); err != nil {
		return "", err
	}
	return path, nil
}
