package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/ticruz38/alzette-connect/internal/appstate"
	"github.com/ticruz38/alzette-connect/internal/clientconfig"
	"github.com/ticruz38/alzette-connect/internal/platform"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// desktopService is the deliberately narrow bridge exposed to the local
// webview. It returns presentation state only; credentials and loopback
// capabilities stay inside the Go runtime.
type desktopService struct {
	app           *application.App
	state         *appstate.Model
	runtime       *appstate.Runtime
	membershipID  string
	portalOrigin  string
	clients       *desktopClients
	clientConfig  *clientconfig.Manager
	actionMu      sync.Mutex
	applicationMu sync.RWMutex
	applications  []appstate.Application
	signInMu      sync.Mutex
	signInCancel  context.CancelFunc
	window        *application.WebviewWindow
}

func (s *desktopService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if s == nil || s.runtime == nil {
		return nil
	}
	go func() {
		_, _ = s.runtime.Resume(ctx, s.membershipID)
	}()
	return nil
}

func (s *desktopService) CurrentState() (appstate.Snapshot, error) {
	if s == nil || s.state == nil {
		return appstate.Snapshot{}, errors.New("Alzette Connect is still starting")
	}
	return s.presentationState(s.state.Current()), nil
}

func (s *desktopService) presentationState(snapshot appstate.Snapshot) appstate.Snapshot {
	s.applicationMu.RLock()
	snapshot.Applications = append([]appstate.Application(nil), s.applications...)
	s.applicationMu.RUnlock()
	return snapshot
}

func (s *desktopService) OpenPortal(target string) error {
	path := "/app/access"
	if target == "models" {
		path = "/app/models"
	}
	return platform.OpenBrowser(strings.TrimRight(s.portalOrigin, "/") + path)
}

func (s *desktopService) BeginSignIn() error {
	if s == nil || s.runtime == nil {
		return errors.New("Alzette Connect is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	s.signInMu.Lock()
	s.signInCancel = cancel
	s.signInMu.Unlock()
	defer func() {
		cancel()
		s.signInMu.Lock()
		s.signInCancel = nil
		s.signInMu.Unlock()
	}()
	return s.runtime.Connect(ctx, s.membershipID)
}

func (s *desktopService) CancelSignIn() {
	s.signInMu.Lock()
	cancel := s.signInCancel
	s.signInMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *desktopService) SignOut() error {
	if s == nil || s.runtime == nil {
		return errors.New("Alzette Connect is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.runtime.Logout(ctx)
}

type clientSetupResult struct {
	Jan   string `json:"jan"`
	Goose string `json:"goose"`
}

func (s *desktopService) ConfigureApps(target string) (clientSetupResult, error) {
	if s == nil || s.runtime == nil || s.clientConfig == nil || s.clients == nil {
		return clientSetupResult{}, errors.New("Application setup is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	baseURL, capability, models, ok := s.runtime.ClientConnection()
	if !ok {
		return clientSetupResult{}, errors.New("Sign in before connecting applications")
	}
	connection := clientconfig.Connection{BaseURL: baseURL, Capability: capability, Models: models}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if target != "" && target != "jan" && target != "jan,goose" {
		return clientSetupResult{}, errors.New("Unknown application setup request")
	}
	jan, err := s.clientConfig.ConfigureJan(ctx, clientconfig.JanRequest{
		Connection: connection, ExecutablePath: s.clients.janExecutable,
	})
	if err != nil {
		return clientSetupResult{}, friendlyClientError("Jan", err)
	}
	if target == "jan" {
		s.setApplicationReady("jan", clientconfig.JanSupportedVersion)
		return clientSetupResult{Jan: string(jan.Status), Goose: string(clientconfig.Unchanged)}, nil
	}
	goose, err := s.clientConfig.ConfigureGoose(ctx, clientconfig.GooseRequest{
		Connection: connection, ExecutablePath: s.clients.gooseExecutable,
		AppASARPath: s.clients.gooseASAR,
	})
	if err != nil {
		if rollbackErr := jan.Rollback(context.Background()); rollbackErr != nil {
			return clientSetupResult{}, errors.New("Goose setup failed and Jan could not be safely restored; use Repair setup")
		}
		return clientSetupResult{}, friendlyClientError("Goose", err)
	}
	s.setApplicationReady("jan", clientconfig.JanSupportedVersion)
	s.setApplicationReady("goose", clientconfig.GooseSupportedVersion)
	return clientSetupResult{Jan: string(jan.Status), Goose: string(goose.Status)}, nil
}

func (s *desktopService) setApplicationReady(id, version string) {
	s.applicationMu.Lock()
	for index := range s.applications {
		if s.applications[index].ID == id {
			s.applications[index].Status = "connected"
			s.applications[index].Version = version
		}
	}
	s.applicationMu.Unlock()
	if s.app != nil && s.state != nil {
		s.app.Event.Emit("connect:state", s.presentationState(s.state.Current()))
	}
}

func (s *desktopService) SetWindowMode(mode string) error {
	if s == nil || s.window == nil {
		return errors.New("The Alzette Connect window is still starting")
	}
	switch mode {
	case "compact":
		s.window.SetSize(420, 640)
	case "onboarding":
		s.window.SetSize(940, 680)
	default:
		return errors.New("Unknown window mode")
	}
	s.window.Center()
	return nil
}

func (s *desktopService) OpenApp(client string) error {
	if s == nil || s.clients == nil {
		return errors.New("Application discovery is still starting")
	}
	executable := ""
	switch client {
	case "jan":
		executable = s.clients.janExecutable
	case "goose":
		executable = s.clients.gooseExecutable
	default:
		return errors.New("Unknown desktop application")
	}
	if executable == "" {
		return errors.New("That application is not installed in a supported location")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := clientconfig.Launch(ctx, executable)
	return friendlyClientError("Application", err)
}

func (s *desktopService) Quit() {
	if s != nil && s.app != nil {
		s.app.Quit()
	}
}
