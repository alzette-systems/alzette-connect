package main

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ticruz38/alzette-connect/internal/appstate"
	"github.com/ticruz38/alzette-connect/internal/clientconfig"
	"github.com/ticruz38/alzette-connect/internal/platform"
	"github.com/ticruz38/alzette-connect/internal/updater"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// desktopService is the deliberately narrow bridge exposed to the local
// webview. It returns presentation state only; credentials and loopback
// capabilities stay inside the Go runtime.
type desktopService struct {
	app             *application.App
	state           *appstate.Model
	runtime         *appstate.Runtime
	membershipID    string
	portalOrigin    string
	clients         *desktopClients
	clientConfig    *clientconfig.Manager
	actionMu        sync.Mutex
	applicationMu   sync.RWMutex
	applications    []appstate.Application
	launchMu        sync.Mutex
	launch          appstate.Launch
	activeProcess   *clientconfig.Process
	activeRollback  func(context.Context) error
	pendingRollback func(context.Context) error
	pendingRemote   bool
	launchCancelMu  sync.Mutex
	launchCancel    context.CancelFunc
	updateActionMu  sync.Mutex
	updateMu        sync.RWMutex
	update          appstate.Update
	updater         *updater.Client
	updateRelease   updater.Release
	signInMu        sync.Mutex
	signInCancel    context.CancelFunc
	window          *application.WebviewWindow
}

func (s *desktopService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if s == nil || s.runtime == nil {
		return nil
	}
	go func() {
		_, _ = s.runtime.Resume(ctx, s.membershipID)
	}()
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(8 * time.Second):
			_, _ = s.checkForUpdates(false)
		}
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
	snapshot.Platform = runtime.GOOS
	// Linux keeps the documented normal-window fallback until tray support is
	// evidenced across the named desktop environments.
	snapshot.TrayAvailable = runtime.GOOS == "darwin" || runtime.GOOS == "windows"
	s.applicationMu.RLock()
	snapshot.Applications = append([]appstate.Application(nil), s.applications...)
	s.applicationMu.RUnlock()
	modelCount := selectedModelCount(snapshot)
	for index := range snapshot.Applications {
		if snapshot.Applications[index].Status == "ready" || snapshot.Applications[index].Status == "verification_required" || snapshot.Applications[index].Status == "needs_attention" {
			snapshot.Applications[index].ModelCount = modelCount
		}
	}
	s.launchMu.Lock()
	snapshot.Launch = s.launch
	s.launchMu.Unlock()
	s.updateMu.RLock()
	snapshot.Update = s.update
	s.updateMu.RUnlock()
	return snapshot
}

func selectedModelCount(snapshot appstate.Snapshot) int {
	for _, available := range snapshot.Contexts {
		if available.ID == snapshot.SelectedContextID || snapshot.SelectedContextID == "" && len(snapshot.Contexts) == 1 {
			return len(available.Models)
		}
	}
	return 0
}

func (s *desktopService) setLaunch(next appstate.Launch) {
	s.launchMu.Lock()
	s.launch = next
	s.launchMu.Unlock()
	if s.app != nil && s.state != nil {
		s.app.Event.Emit("connect:state", s.presentationState(s.state.Current()))
	}
}

func (s *desktopService) rememberPendingCleanup(rollback func(context.Context) error, retryRemote bool) {
	s.launchMu.Lock()
	s.pendingRollback = rollback
	s.pendingRemote = retryRemote
	s.launchMu.Unlock()
}

func (s *desktopService) clearPendingCleanup() {
	s.launchMu.Lock()
	s.pendingRollback = nil
	s.pendingRemote = false
	s.launchMu.Unlock()
}

func (s *desktopService) setUpdate(next appstate.Update) {
	s.updateMu.Lock()
	s.update = next
	s.updateMu.Unlock()
	if s.app != nil && s.state != nil {
		s.app.Event.Emit("connect:state", s.presentationState(s.state.Current()))
	}
}

func (s *desktopService) CheckForUpdates() (appstate.Update, error) {
	return s.checkForUpdates(true)
}

func (s *desktopService) checkForUpdates(interactive bool) (appstate.Update, error) {
	if s == nil || s.updater == nil {
		return appstate.Update{}, errors.New("Update checks are still starting")
	}
	s.updateActionMu.Lock()
	defer s.updateActionMu.Unlock()
	s.updateMu.RLock()
	current := s.update.CurrentVersion
	s.updateMu.RUnlock()
	if interactive {
		s.setUpdate(appstate.Update{State: "checking", CurrentVersion: current, Message: "Checking the Alzette Connect release channel…"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	release, err := s.updater.Check(ctx)
	if errors.Is(err, updater.ErrNoUpdate) {
		s.updateMu.Lock()
		s.updateRelease = updater.Release{}
		s.updateMu.Unlock()
		next := appstate.Update{State: "current", CurrentVersion: current, Message: "You’re using the latest Alzette Connect release."}
		s.setUpdate(next)
		return next, nil
	}
	if err != nil {
		s.updateMu.Lock()
		s.updateRelease = updater.Release{}
		s.updateMu.Unlock()
		next := appstate.Update{State: "idle", CurrentVersion: current}
		if interactive {
			next = appstate.Update{State: "error", CurrentVersion: current, Message: "Connect couldn’t check the release channel. Try again when you’re online."}
		}
		s.setUpdate(next)
		return next, errors.New(next.Message)
	}
	s.updateMu.Lock()
	s.updateRelease = release
	s.updateMu.Unlock()
	message := "Signed update verified. Connect will close, install, and reopen."
	if release.Prerelease {
		message = "Integrity-checked internal preview. Connect will close, install, and reopen."
	}
	if runtime.GOOS == "linux" {
		message = "Integrity-checked, unsigned internal demo. Connect will open your system package installer."
	}
	next := appstate.Update{State: "available", CurrentVersion: current, AvailableVersion: release.Version, Message: message}
	s.setUpdate(next)
	return next, nil
}

func (s *desktopService) InstallUpdate() error {
	if s == nil || s.updater == nil {
		return errors.New("Updates are still starting")
	}
	s.updateActionMu.Lock()
	defer s.updateActionMu.Unlock()
	s.updateMu.RLock()
	release := s.updateRelease
	updateState := s.update.State
	current := s.update.CurrentVersion
	s.updateMu.RUnlock()
	if release.Version == "" || updateState != "available" {
		return errors.New("Check for an update first")
	}
	s.setUpdate(appstate.Update{State: "downloading", CurrentVersion: current, AvailableVersion: release.Version, Message: "Downloading and verifying the update…"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	assetPath, err := s.updater.Download(ctx, release)
	if err != nil {
		next := appstate.Update{State: "error", CurrentVersion: current, AvailableVersion: release.Version, Message: "The update could not be downloaded and verified."}
		s.setUpdate(next)
		return errors.New(next.Message)
	}
	if err := updater.StartInstall(assetPath, release.Version); err != nil {
		next := appstate.Update{State: "error", CurrentVersion: current, AvailableVersion: release.Version, Message: err.Error()}
		s.setUpdate(next)
		return errors.New(next.Message)
	}
	next := appstate.Update{State: "installing", CurrentVersion: current, AvailableVersion: release.Version, Message: "Update integrity confirmed. Connect will reopen after installation…"}
	if runtime.GOOS == "linux" {
		next = appstate.Update{State: "installer_opened", CurrentVersion: current, AvailableVersion: release.Version, Message: "System installer opened. Finish the update there, then reopen Connect."}
	}
	s.setUpdate(next)
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			s.app.Quit()
		}()
	}
	return nil
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
	if err := s.disconnectLocked(ctx); err != nil {
		return err
	}
	return s.runtime.Logout(ctx)
}

func (s *desktopService) SelectContext(membershipID string) error {
	if s == nil || s.runtime == nil {
		return errors.New("Alzette Connect is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.runtime.SelectContext(ctx, strings.TrimSpace(membershipID))
}

func (s *desktopService) setApplicationReady(id, version string) {
	s.applicationMu.Lock()
	for index := range s.applications {
		if s.applications[index].ID == id {
			s.applications[index].Status = "ready"
			s.applications[index].Version = version
			s.applications[index].Configured = true
			s.applications[index].Detail = "Qualified for this Connect build"
		}
	}
	s.applicationMu.Unlock()
	if s.app != nil && s.state != nil {
		s.app.Event.Emit("connect:state", s.presentationState(s.state.Current()))
	}
}

func (s *desktopService) setApplicationObserved(id, version string) {
	s.applicationMu.Lock()
	for index := range s.applications {
		if s.applications[index].ID == id {
			s.applications[index].Status = "verification_required"
			s.applications[index].Version = version
			s.applications[index].Configured = false
			s.applications[index].Detail = "Compatibility is checked on every launch"
		}
	}
	s.applicationMu.Unlock()
	if s.app != nil && s.state != nil {
		s.app.Event.Emit("connect:state", s.presentationState(s.state.Current()))
	}
}

func (s *desktopService) application(id string) (appstate.Application, bool) {
	s.applicationMu.RLock()
	defer s.applicationMu.RUnlock()
	for _, application := range s.applications {
		if application.ID == id {
			return application, true
		}
	}
	return appstate.Application{}, false
}

// LaunchApplication executes the product lifecycle behind one native action:
// qualify, create a fresh inference session, prepare reversible configuration,
// launch, supervise, and clean up on exit.
func (s *desktopService) LaunchApplication(id string) error {
	if s == nil || s.runtime == nil || s.clients == nil || s.clientConfig == nil {
		return errors.New("Application launch is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	application, ok := s.application(id)
	if !ok || application.Status == "not_supported" {
		return errors.New("That application adapter is not released")
	}
	if application.Status != "ready" && application.Status != "verification_required" && application.Status != "needs_attention" {
		return errors.New("That application is not available on this computer")
	}
	if !application.Installed {
		return errors.New("That application is not installed in a supported location")
	}
	s.launchMu.Lock()
	active := s.activeProcess != nil || s.launch.Phase == "preparing" || s.launch.CleanupPending
	s.launchMu.Unlock()
	if active {
		return errors.New("Disconnect the active application or review pending cleanup before launching another")
	}
	modelCount := selectedModelCount(s.state.Current())
	if modelCount == 0 {
		return errors.New("No compatible company model is available for this application")
	}
	now := time.Now().UTC()
	s.setLaunch(appstate.Launch{Phase: "preparing", ApplicationID: id, Application: application.Name, Message: "Checking the application", ModelCount: modelCount})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	s.launchCancelMu.Lock()
	s.launchCancel = cancel
	s.launchCancelMu.Unlock()
	defer func() {
		cancel()
		s.launchCancelMu.Lock()
		s.launchCancel = nil
		s.launchCancelMu.Unlock()
	}()
	// Pi can be qualified without connection material, so do that before
	// minting a grant or opening the loopback listener. Passive discovery still
	// never executes the binary; this runs only after the employee launches it.
	observedVersion := ""
	if id == "pi" {
		version, err := clientconfig.QualifyPi(ctx, s.clients.piExecutable)
		if err != nil {
			s.setLaunch(appstate.Launch{Phase: "idle"})
			return friendlyClientError("Pi", err)
		}
		s.setApplicationReady("pi", version)
	} else if id == "chatgpt" {
		version, err := clientconfig.ObserveChatGPTVersion(ctx, s.clients.chatGPTExecutable)
		if err != nil {
			s.setLaunch(appstate.Launch{Phase: "idle"})
			return friendlyClientError("ChatGPT", err)
		}
		observedVersion = version
	}
	protocolPath := "/v1/chat/completions"
	if id == "chatgpt" {
		protocolPath = "/v1/responses"
	}
	if err := s.runtime.StartLaunch(ctx, protocolPath); err != nil {
		s.setLaunch(appstate.Launch{Phase: "idle"})
		return err
	}
	baseURL, capability, models, connected := s.runtime.ClientConnection()
	if !connected {
		_ = s.runtime.StopLaunch(context.Background())
		s.setLaunch(appstate.Launch{Phase: "idle"})
		return errors.New("The private application connection did not start")
	}
	connection := clientconfig.Connection{BaseURL: baseURL, Capability: capability, Models: models}
	var process *clientconfig.Process
	var rollback func(context.Context) error
	var err error
	switch id {
	case "pi":
		process, err = clientconfig.LaunchPi(ctx, s.clients.piExecutable, connection)
	case "jan":
		var result *clientconfig.Result
		result, err = s.clientConfig.ConfigureJan(ctx, clientconfig.JanRequest{Connection: connection, ExecutablePath: s.clients.janExecutable})
		if err == nil {
			s.setApplicationReady("jan", clientconfig.JanSupportedVersion)
			rollback = result.Rollback
			process, err = clientconfig.LaunchObserved(ctx, s.clients.janExecutable)
		}
	case "goose":
		var result *clientconfig.Result
		result, err = s.clientConfig.ConfigureGoose(ctx, clientconfig.GooseRequest{Connection: connection, ExecutablePath: s.clients.gooseExecutable, AppASARPath: s.clients.gooseASAR})
		if err == nil {
			s.setApplicationReady("goose", clientconfig.GooseSupportedVersion)
			rollback = result.Rollback
			process, err = clientconfig.LaunchObserved(ctx, s.clients.gooseExecutable)
		}
	case "chatgpt":
		var result *clientconfig.Result
		result, err = s.clientConfig.ConfigureChatGPT(ctx, clientconfig.ChatGPTRequest{
			Connection: connection, ExecutablePath: s.clients.chatGPTExecutable, Version: observedVersion,
		})
		if err == nil {
			rollback = result.Rollback
			process, err = clientconfig.LaunchChatGPT(ctx, s.clients.chatGPTExecutable, connection)
			if err == nil {
				s.setApplicationObserved("chatgpt", observedVersion)
			}
		}
	default:
		err = errors.New("That application adapter is not released")
	}
	if err != nil {
		var restoreErr error
		if rollback != nil {
			restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 8*time.Second)
			restoreErr = rollback(restoreCtx)
			restoreCancel()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 8*time.Second)
		remoteErr := s.runtime.StopLaunch(cleanupCtx)
		cleanupCancel()
		if restoreErr != nil || remoteErr != nil {
			message := application.Name + " did not open; the private connection is closed, but cleanup needs review"
			profileStatus := "restored"
			if restoreErr != nil {
				message = application.Name + " did not open and its profile changed before Connect could restore it"
				profileStatus = "needs_review"
			}
			grantStatus := "confirmed"
			if remoteErr != nil {
				grantStatus = "unconfirmed"
			}
			var retryRollback func(context.Context) error
			if restoreErr != nil {
				retryRollback = rollback
			}
			s.rememberPendingCleanup(retryRollback, remoteErr != nil)
			s.setLaunch(appstate.Launch{Phase: "recovery", ApplicationID: id, Application: application.Name, Message: message, CleanupPending: true, LocalClosed: true, GrantStatus: grantStatus, ProfileStatus: profileStatus})
		} else {
			s.clearPendingCleanup()
			s.setLaunch(appstate.Launch{Phase: "idle"})
		}
		return friendlyClientError(application.Name, err)
	}
	s.launchMu.Lock()
	s.activeProcess = process
	s.activeRollback = rollback
	s.launchMu.Unlock()
	s.setLaunch(appstate.Launch{Phase: "running", ApplicationID: id, Application: application.Name, Message: "Connected through Alzette", StartedAt: now, ModelCount: len(models)})
	go s.observeApplication(process)
	return nil
}

func (s *desktopService) CancelLaunch() {
	s.launchCancelMu.Lock()
	cancel := s.launchCancel
	s.launchCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *desktopService) observeApplication(process *clientconfig.Process) {
	if process == nil {
		return
	}
	<-process.Done
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	s.launchMu.Lock()
	if s.activeProcess != process {
		s.launchMu.Unlock()
		return
	}
	s.activeProcess = nil
	rollback := s.activeRollback
	s.activeRollback = nil
	s.launchMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	remoteErr := s.runtime.StopLaunch(ctx)
	var restoreErr error
	if rollback != nil {
		if err := rollback(ctx); err != nil {
			restoreErr = err
		}
	}
	if remoteErr != nil || restoreErr != nil {
		message := "The local connection is closed, but remote grant revocation could not be confirmed"
		profileStatus := "restored"
		if restoreErr != nil {
			message = "The private connection is closed, but the application profile changed and needs review"
			profileStatus = "needs_review"
		}
		grantStatus := "confirmed"
		if remoteErr != nil {
			grantStatus = "unconfirmed"
		}
		var retryRollback func(context.Context) error
		if restoreErr != nil {
			retryRollback = rollback
		}
		s.rememberPendingCleanup(retryRollback, remoteErr != nil)
		s.setLaunch(appstate.Launch{Phase: "recovery", Message: message, CleanupPending: true, LocalClosed: true, GrantStatus: grantStatus, ProfileStatus: profileStatus})
		return
	}
	s.clearPendingCleanup()
	s.setLaunch(appstate.Launch{Phase: "idle"})
}

func (s *desktopService) Disconnect() error {
	if s == nil || s.runtime == nil {
		return errors.New("Alzette Connect is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.disconnectLocked(ctx)
}

func (s *desktopService) disconnectLocked(ctx context.Context) error {
	s.launchMu.Lock()
	process := s.activeProcess
	rollback := s.activeRollback
	applicationID := s.launch.ApplicationID
	applicationName := s.launch.Application
	s.activeProcess = nil
	s.activeRollback = nil
	s.launchMu.Unlock()
	if process == nil {
		return nil
	}
	s.setLaunch(appstate.Launch{Phase: "disconnecting", ApplicationID: applicationID, Application: applicationName, Message: "Closing the private connection"})
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = process.Stop(stopCtx)
	stopCancel()
	remoteErr := s.runtime.StopLaunch(ctx)
	var restoreErr error
	if rollback != nil {
		restoreErr = rollback(ctx)
	}
	if restoreErr != nil {
		grantStatus := "confirmed"
		if remoteErr != nil {
			grantStatus = "unconfirmed"
		}
		s.rememberPendingCleanup(rollback, remoteErr != nil)
		s.setLaunch(appstate.Launch{Phase: "recovery", Message: "The connection is closed, but the application profile changed and needs review", CleanupPending: true, LocalClosed: true, GrantStatus: grantStatus, ProfileStatus: "needs_review"})
		return restoreErr
	}
	if remoteErr != nil {
		s.rememberPendingCleanup(nil, true)
		s.setLaunch(appstate.Launch{Phase: "recovery", Message: "The local connection is closed and the profile is restored, but remote grant revocation could not be confirmed", CleanupPending: true, LocalClosed: true, GrantStatus: "unconfirmed", ProfileStatus: "restored"})
		return remoteErr
	}
	s.clearPendingCleanup()
	s.setLaunch(appstate.Launch{Phase: "idle"})
	return nil
}

// RetryCleanup repeats only the cleanup work that was not confirmed. It never
// reopens the listener or mints new inference authority.
func (s *desktopService) RetryCleanup() error {
	if s == nil || s.runtime == nil {
		return errors.New("Alzette Connect is still starting")
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	s.launchMu.Lock()
	rollback := s.pendingRollback
	retryRemote := s.pendingRemote
	current := s.launch
	s.launchMu.Unlock()
	if !current.CleanupPending {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var remoteErr, restoreErr error
	if retryRemote {
		remoteErr = s.runtime.StopLaunch(ctx)
	}
	if rollback != nil {
		restoreErr = rollback(ctx)
	}
	if remoteErr == nil && restoreErr == nil {
		s.clearPendingCleanup()
		s.setLaunch(appstate.Launch{Phase: "idle"})
		return nil
	}

	s.launchMu.Lock()
	if restoreErr == nil {
		s.pendingRollback = nil
	}
	if remoteErr == nil {
		s.pendingRemote = false
	}
	s.launchMu.Unlock()
	applicationName := current.Application
	if applicationName == "" {
		applicationName = "The application"
	}
	message := "The private connection is closed, but " + applicationName + "'s local profile still needs attention"
	profileStatus := "needs_review"
	if restoreErr == nil {
		message = "The application profile is restored, but remote revocation could not be confirmed"
		profileStatus = "restored"
	}
	grantStatus := "confirmed"
	if remoteErr != nil {
		grantStatus = "unconfirmed"
	}
	s.setLaunch(appstate.Launch{
		Phase: "recovery", ApplicationID: current.ApplicationID, Application: current.Application,
		Message: message, CleanupPending: true, LocalClosed: true,
		GrantStatus: grantStatus, ProfileStatus: profileStatus,
	})
	return errors.New("Cleanup still needs attention; your private connection remains closed")
}

func (s *desktopService) HideToTray() {
	if s != nil && s.window != nil {
		s.window.Hide()
	}
}

func (s *desktopService) shutdown(ctx context.Context) {
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	_ = s.disconnectLocked(ctx)
	_ = s.runtime.Stop(ctx)
}

func (s *desktopService) SetWindowMode(mode string) error {
	if s == nil || s.window == nil {
		return errors.New("The Alzette Connect window is still starting")
	}
	switch mode {
	case "launcher", "signed-out", "compact", "onboarding":
		s.window.SetSize(720, 640)
	default:
		return errors.New("Unknown window mode")
	}
	s.window.Center()
	return nil
}

func (s *desktopService) Quit() {
	if s != nil && s.app != nil {
		s.app.Quit()
	}
}
