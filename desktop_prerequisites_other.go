//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func desktopPrerequisitesReady(*application.App) bool { return true }
