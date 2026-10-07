//go:build windows

package main

import (
	"strings"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

func desktopPrerequisitesReady(app *application.App) bool {
	version, _ := app.Env.Info().PlatformInfo["WebView2"].(string)
	if strings.TrimSpace(version) != "" {
		return true
	}
	// This must be a native dialog: the application UI needs the missing runtime.
	message, _ := windows.UTF16PtrFromString("Microsoft Edge WebView2 Runtime is required to open Alzette Connect.\n\nInstall the Microsoft Edge WebView2 Evergreen Runtime, then open Alzette Connect again.")
	title, _ := windows.UTF16PtrFromString("Alzette Connect — WebView2 required")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(
		0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10,
	)
	return false
}
