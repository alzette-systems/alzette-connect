package updater

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallFailureIsConsumedOnce(t *testing.T) {
	config := t.TempDir()
	previous := userConfigDirectory
	userConfigDirectory = func() (string, error) { return config, nil }
	t.Cleanup(func() { userConfigDirectory = previous })
	if err := recordInstallFailure("0.3.7", errors.New("Move Alzette Connect to Applications, reopen it there, then update")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config, "Alzette Connect", installResultFilename)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows protects the user configuration directory with ACLs and does not
	// preserve Unix permission bits reported by FileMode.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("result permissions=%o", info.Mode().Perm())
	}
	failure, ok := ConsumeInstallFailure()
	if !ok || failure.Version != "0.3.7" || !strings.Contains(failure.Message, "Applications") {
		t.Fatalf("failure=%#v ok=%v", failure, ok)
	}
	if _, ok := ConsumeInstallFailure(); ok {
		t.Fatal("install failure was returned twice")
	}
}

func TestHandleHelperRejectsMissingExpectedVersion(t *testing.T) {
	handled, err := HandleHelper([]string{"connect", helperFlag, "42", "asset", "executable"})
	if !handled || err == nil || !strings.Contains(err.Error(), "invalid update helper") {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}
