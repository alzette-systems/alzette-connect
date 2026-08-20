package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ticruz38/alzette-connect/internal/clientconfig"
)

func TestDiscoverDesktopClientsAcceptsOnlyExistingAbsoluteRegularOverrides(t *testing.T) {
	directory := t.TempDir()
	executableSuffix := ""
	if runtime.GOOS == "windows" {
		executableSuffix = ".exe"
	}
	jan := filepath.Join(directory, "jan"+executableSuffix)
	goose := filepath.Join(directory, "goose"+executableSuffix)
	chatGPT := filepath.Join(directory, "ChatGPT"+executableSuffix)
	asar := filepath.Join(directory, "app.asar")
	for _, path := range []string{jan, goose, chatGPT} {
		if err := os.WriteFile(path, []byte("test"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(asar, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALZETTE_CONNECT_JAN_EXECUTABLE", jan)
	t.Setenv("ALZETTE_CONNECT_GOOSE_EXECUTABLE", goose)
	t.Setenv("ALZETTE_CONNECT_GOOSE_ASAR", asar)
	t.Setenv("ALZETTE_CONNECT_CHATGPT_EXECUTABLE", chatGPT)

	got := discoverDesktopClients(directory)
	if got.janExecutable != jan || got.gooseExecutable != goose || got.gooseASAR != asar || got.chatGPTExecutable != chatGPT {
		t.Fatalf("unexpected discovery: %#v", got)
	}
}

func TestDiscoveryRejectsSymlinkOverride(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "jan-real")
	link := filepath.Join(directory, "jan")
	if err := os.WriteFile(target, []byte("test"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got := validExecutableOverride(link); got != "" {
		t.Fatalf("symlink override was accepted: %q", got)
	}
}

func TestFriendlyClientErrorsDoNotExposePathsOrProtocolDetails(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{os.ErrNotExist, "Open Jan once, close it, then try again"},
		{clientconfig.ErrClientRunning, "Close Jan, then try again"},
		{clientconfig.ErrWrongVersion, "This Jan version is not supported by this Connect build"},
		{clientconfig.ErrConflict, "Jan has a conflicting Alzette Connect profile; review its existing profile before trying again"},
		{errors.New("secret path /tmp/example failed"), "Jan setup could not be completed"},
	}
	for _, test := range tests {
		if got := friendlyClientError("Jan", test.err); got == nil || got.Error() != test.want {
			t.Fatalf("friendlyClientError(%v)=%v, want %q", test.err, got, test.want)
		}
	}
}

func TestChatGPTCandidateRequiresAnExplicitBuildGate(t *testing.T) {
	previous := chatGPTCandidateEnabled
	t.Cleanup(func() { chatGPTCandidateEnabled = previous })
	chatGPTCandidateEnabled = "false"
	if chatGPTCandidateIsEnabled() {
		t.Fatal("default candidate gate was enabled")
	}
	chatGPTCandidateEnabled = "true"
	if !chatGPTCandidateIsEnabled() {
		t.Fatal("explicit candidate gate was ignored")
	}
}
