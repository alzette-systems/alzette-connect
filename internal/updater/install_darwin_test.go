//go:build darwin

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareMacHelperPreservesExecutableBundleSignature(t *testing.T) {
	sourceRoot := t.TempDir()
	sourceExecutable := testMacExecutable(t, sourceRoot, "Alzette Connect.app", "systems.alzette.Connect")
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(testExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceExecutable, contents, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceApp := filepath.Join(sourceRoot, "Alzette Connect.app")
	if output, err := exec.Command("/usr/bin/codesign", "--force", "--deep", "--sign", "-", sourceApp).CombinedOutput(); err != nil {
		t.Fatalf("sign test bundle: %v: %s", err, output)
	}

	helper, err := prepareMacHelper(sourceApp, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(helper, "/Alzette Connect.app/Contents/MacOS/alzette-connect") {
		t.Fatalf("helper is not inside the signed application bundle: %s", helper)
	}
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", filepath.Join(helper, "..", "..", "..")).CombinedOutput(); err != nil {
		t.Fatalf("copied helper signature is invalid: %v: %s", err, output)
	}
	if output, err := exec.Command(helper, "-test.run=^$").CombinedOutput(); err != nil {
		t.Fatalf("macOS rejected the copied helper executable: %v: %s", err, output)
	}
}

func TestMacInstallPreflightAcceptsWritableApplicationFolder(t *testing.T) {
	applications := t.TempDir()
	executable := testMacExecutable(t, applications, "Alzette Connect.app", "systems.alzette.Connect")
	if err := preflightMacInstall(executable); err != nil {
		t.Fatal(err)
	}
}

func TestMacInstallPreflightAcceptsFinderRenamedApplicationBundle(t *testing.T) {
	applications := t.TempDir()
	executable := testMacExecutable(t, applications, "Alzette Connect 2.app", "systems.alzette.Connect")
	if err := preflightMacInstall(executable); err != nil {
		t.Fatal(err)
	}
}

func TestMacInstallPreflightRejectsDifferentApplicationIdentifier(t *testing.T) {
	applications := t.TempDir()
	executable := testMacExecutable(t, applications, "Alzette Connect.app", "example.invalid.Connect")
	err := preflightMacInstall(executable)
	if err == nil || !strings.Contains(err.Error(), "unexpected application bundle") {
		t.Fatalf("err=%v", err)
	}
}

func TestMacInstallPreflightRejectsAppTranslocation(t *testing.T) {
	executable := "/private/var/folders/example/AppTranslocation/session/d/Alzette Connect.app/Contents/MacOS/alzette-connect"
	err := preflightMacInstall(executable)
	if err == nil {
		t.Fatalf("err=%v", err)
	}
}

func testMacExecutable(t *testing.T, parent, appName, identifier string) string {
	t.Helper()
	executable := filepath.Join(parent, appName, "Contents", "MacOS", "alzette-connect")
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>` + identifier + `</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(parent, appName, "Contents", "Info.plist"), []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}
	return executable
}
