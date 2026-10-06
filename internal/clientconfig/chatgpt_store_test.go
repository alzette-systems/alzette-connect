package clientconfig

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestWindowsChatGPTPackageRequiresTheUnifiedTrustedStoreIdentity(t *testing.T) {
	root := canonicalTempRoot(t)
	executable := filepath.Join(root, "app", "ChatGPT.exe")
	mustWrite(t, executable, []byte("test executable"), 0o700)
	valid := windowsChatGPTPackage{
		Name: "OpenAI.Codex", Publisher: chatGPTWindowsPublisher, SignatureKind: "Store", Status: "Ok",
		Version: "26.930.4958.0", Architecture: "X64", Location: root, Executable: "app/ChatGPT.exe", EntryPoint: "Windows.FullTrustApplication",
	}
	if path, err := validatedWindowsChatGPTPath(valid); err != nil || path != executable {
		t.Fatalf("trusted workspace discovery: path=%q err=%v", path, err)
	}
	for name, change := range map[string]func(*windowsChatGPTPackage){
		"classic":                   func(p *windowsChatGPTPackage) { p.Name = "OpenAI.ChatGPT-Desktop" },
		"publisher":                 func(p *windowsChatGPTPackage) { p.Publisher = "CN=Unrelated Publisher" },
		"developer signature":       func(p *windowsChatGPTPackage) { p.SignatureKind = "Developer" },
		"broken package":            func(p *windowsChatGPTPackage) { p.Status = "Tampered" },
		"unsupported architecture":  func(p *windowsChatGPTPackage) { p.Architecture = "Arm64" },
		"other entry point":         func(p *windowsChatGPTPackage) { p.EntryPoint = "Unrelated.Application" },
		"outside executable":        func(p *windowsChatGPTPackage) { p.Executable = "../ChatGPT.exe" },
		"relative install location": func(p *windowsChatGPTPackage) { p.Location = "relative" },
	} {
		t.Run(name, func(t *testing.T) {
			packageInfo := valid
			change(&packageInfo)
			if _, err := validatedWindowsChatGPTPath(packageInfo); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("untrusted package accepted: %v", err)
			}
		})
	}
}
