//go:build darwin

package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacInstallPreflightAcceptsWritableApplicationFolder(t *testing.T) {
	applications := t.TempDir()
	executable := filepath.Join(applications, "Alzette Connect.app", "Contents", "MacOS", "alzette-connect")
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := preflightMacInstall(executable); err != nil {
		t.Fatal(err)
	}
}

func TestMacInstallPreflightRejectsAppTranslocation(t *testing.T) {
	executable := "/private/var/folders/example/AppTranslocation/session/d/Alzette Connect.app/Contents/MacOS/alzette-connect"
	err := preflightMacInstall(executable)
	if err == nil || !strings.Contains(err.Error(), "Move Alzette Connect to Applications") {
		t.Fatalf("err=%v", err)
	}
}
