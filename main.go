package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ticruz38/alzette-connect/internal/appstate"
	"github.com/ticruz38/alzette-connect/internal/clientconfig"
	"github.com/ticruz38/alzette-connect/internal/updater"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var version = "0.2.0-demo.1"

//go:embed all:frontend/dist
var frontendAssets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	application.RegisterEvent[appstate.Snapshot]("connect:state")
}

func main() {
	if handled, err := updater.HandleHelper(os.Args); handled {
		if err != nil {
			log.Print(err)
		}
		return
	}
	state := appstate.New(time.Now())
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		log.Fatal("Alzette Connect could not resolve the current user's home directory")
	}
	controlURL := envOr("ALZETTE_CONTROL_URL", "https://app.alzette.systems")
	runtime, err := appstate.NewRuntime(appstate.RuntimeConfig{
		ControlURL:    controlURL,
		CallbackURL:   envOr("ALZETTE_CONNECT_CALLBACK_URL", "http://127.0.0.1:43127/callback"),
		ProxyAddress:  envOr("ALZETTE_CONNECT_PROXY_ADDRESS", "127.0.0.1:43128"),
		Profile:       envOr("ALZETTE_CONNECT_PROFILE", "default"),
		AllowInsecure: os.Getenv("ALZETTE_CONNECT_ALLOW_INSECURE") == "1",
	}, state)
	if err != nil {
		log.Fatal(err)
	}
	updateClient, err := updater.New(updater.Options{CurrentVersion: version})
	if err != nil {
		log.Fatal(err)
	}
	clientManager, err := clientconfig.New(clientconfig.Options{})
	if err != nil {
		log.Fatal(err)
	}
	state.Set(appstate.Snapshot{
		Phase:     appstate.SignInRequired,
		Message:   "Sign in to connect your company models",
		UpdatedAt: time.Now().UTC(),
	})
	clients := discoverDesktopClients(home)
	desktop := &desktopService{
		state:        state,
		runtime:      runtime,
		membershipID: strings.TrimSpace(os.Getenv("ALZETTE_CONNECT_MEMBERSHIP_ID")),
		portalOrigin: controlURL,
		clients:      clients,
		clientConfig: clientManager,
		applications: clients.applicationStates(),
		updater:      updateClient,
		update: appstate.Update{
			State:          "idle",
			CurrentVersion: updateClient.CurrentVersion(),
		},
	}
	var primaryWindow *application.WebviewWindow

	app := application.New(application.Options{
		Name:        "Alzette Connect",
		Description: "Secure employee connection to company AI models",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontendAssets),
		},
		Services: []application.Service{
			application.NewService(desktop),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "systems.alzette.Connect",
			ExitCode: 0,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if primaryWindow != nil {
					primaryWindow.Show()
					primaryWindow.Focus()
				}
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})
	desktop.app = app
	app.OnShutdown(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		desktop.shutdown(ctx)
	})

	primaryWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "connect",
		Title:            "Alzette Connect",
		Width:            720,
		Height:           640,
		MinWidth:         380,
		MinHeight:        560,
		InitialPosition:  application.WindowCentered,
		BackgroundColour: application.NewRGB(250, 249, 246),
		URL:              "/",
	})
	desktop.window = primaryWindow

	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("Alzette Connect")
	tray.AttachWindow(primaryWindow).WindowOffset(8)

	menu := app.NewMenu()
	menu.Add("Open Alzette Connect").OnClick(func(*application.Context) {
		tray.ShowWindow()
	})
	menu.Add("Check for Updates…").OnClick(func(*application.Context) {
		tray.ShowWindow()
		go func() { _, _ = desktop.CheckForUpdates() }()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	updates, unsubscribe := state.Subscribe()
	defer unsubscribe()
	go func() {
		for update := range updates {
			app.Event.Emit("connect:state", desktop.presentationState(update))
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
