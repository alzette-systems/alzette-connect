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

const claudeWindowsPublisher = `CN="Anthropic, PBC", O="Anthropic, PBC", L=San Francisco, S=California, C=US, SERIALNUMBER=4860621, OID.2.5.4.15=Private Organization, OID.1.3.6.1.4.1.311.60.2.1.2=Delaware, OID.1.3.6.1.4.1.311.60.2.1.3=US`

func inspectWindowsClaude(ctx context.Context) (windowsChatGPTPackage, error) {
	if runtime.GOOS != "windows" {
		return windowsChatGPTPackage{}, ErrUnsupported
	}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	const script = `$ErrorActionPreference='Stop'; $ps=@(Get-AppxPackage -Name Claude); if($ps.Count -ne 1){exit 1}; $p=$ps[0]; $m=Get-AppxPackageManifest -Package $p.PackageFullName; $as=@($m.Package.Applications.Application | Where-Object {$_.Id -eq 'Claude'}); if($as.Count -ne 1){exit 1}; $a=$as[0]; @{name=$p.Name;publisher=$p.Publisher;signature_kind=[string]$p.SignatureKind;status=[string]$p.Status;version=[string]$p.Version;architecture=[string]$p.Architecture;location=$p.InstallLocation;executable=[string]$a.Executable;entry_point=[string]$a.EntryPoint} | ConvertTo-Json -Compress`
	command := exec.CommandContext(check, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = launchEnvironment(os.Environ())
	output, err := command.Output()
	var p windowsChatGPTPackage
	if err != nil || len(output) > 16<<10 || json.Unmarshal(output, &p) != nil {
		return p, fmt.Errorf("%w: registered Claude package could not be inspected", ErrUnsupported)
	}
	return p, nil
}
func validatedWindowsClaudePath(p windowsChatGPTPackage) (string, error) {
	if p.Name != "Claude" || p.Publisher != claudeWindowsPublisher || (p.SignatureKind != "Developer" && p.SignatureKind != "Store") || p.Status != "Ok" || p.Architecture != "X64" || p.EntryPoint != "Windows.FullTrustApplication" || strings.ReplaceAll(p.Executable, `\`, "/") != "app/Claude.exe" || !filepath.IsAbs(p.Location) || p.Version == "" || len(p.Version) > 64 || strings.ContainsAny(p.Version, "\r\n\t") {
		return "", ErrUnsupported
	}
	path := filepath.Join(p.Location, "app", "Claude.exe")
	if err := ensureTrustedEvidence(path); err != nil {
		return "", err
	}
	return path, nil
}
func DiscoverWindowsClaude(ctx context.Context) (string, error) {
	p, err := inspectWindowsClaude(ctx)
	if err != nil {
		return "", err
	}
	return validatedWindowsClaudePath(p)
}
func ObserveWindowsClaudeVersion(ctx context.Context, path string) (string, error) {
	p, err := inspectWindowsClaude(ctx)
	if err != nil {
		return "", err
	}
	observed, err := validatedWindowsClaudePath(p)
	if err != nil || !strings.EqualFold(filepath.Clean(path), filepath.Clean(observed)) {
		return "", ErrUnsupported
	}
	return p.Version, nil
}

func checkWindowsClaudePolicy(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return ErrUnsupported
	}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	const script = `$ErrorActionPreference='Stop'; foreach($path in @('HKLM:\SOFTWARE\Policies\Claude','HKCU:\SOFTWARE\Policies\Claude')){if(Test-Path $path){$names=(Get-Item $path).GetValueNames(); if(@($names | Where-Object {$_ -match '^(inference|model|defaultModel|configBootstrap)'}).Count -gt 0){exit 2}}}; exit 0`
	command := exec.CommandContext(check, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = launchEnvironment(os.Environ())
	if command.Run() != nil {
		return fmt.Errorf("%w: Claude inference is managed by device policy or policy could not be inspected", ErrConflict)
	}
	return nil
}
