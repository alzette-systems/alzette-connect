package clientconfig

import (
	"context"
	"errors"
)

const (
	JanSupportedVersion   = "0.8.4"
	GooseSupportedVersion = "1.46.0"
)

var (
	ErrUnsupported   = errors.New("client configuration is unsupported")
	ErrWrongVersion  = errors.New("unsupported client version")
	ErrUnsafePath    = errors.New("unsafe client configuration path")
	ErrClientRunning = errors.New("client must be closed before configuration")
	ErrConflict      = errors.New("client configuration conflicts with an unowned entry")
	ErrSecretStore   = errors.New("client protected credential store is unavailable")
	ErrStaleRollback = errors.New("configuration changed since it was applied")
)

type Client string

const (
	Jan   Client = "jan"
	Goose Client = "goose"
)

type Connection struct {
	// BaseURL is the Connect proxy URL and must be an http loopback URL ending
	// in /v1. Capability is written only to the client's protected store.
	BaseURL    string
	Capability string
	Models     []string
}

type JanRequest struct {
	Connection Connection
	// AppSettingsPath overrides the canonical Jan settings file. It is intended
	// for an explicitly discovered portable/custom installation and tests.
	AppSettingsPath string
	ExecutablePath  string
}

type GooseRequest struct {
	Connection Connection
	// ConfigDir overrides Goose's canonical config directory.
	ConfigDir      string
	AppASARPath    string
	ExecutablePath string
}

type Status string

const (
	Configured Status = "configured"
	Unchanged  Status = "unchanged"
)

type Result struct {
	Client       Client
	Version      string
	Status       Status
	ChangedFiles []string
	BackupFiles  []string
	rollback     func(context.Context) error
}

// Rollback restores only state still matching this Configure result. This
// prevents an old result from clobbering a later capability rotation.
func (r *Result) Rollback(ctx context.Context) error {
	if r == nil || r.rollback == nil || r.Status == Unchanged {
		return nil
	}
	return r.rollback(ctx)
}

// SecretStore is deliberately shaped like the Rust keyring entries used by
// Jan and Goose. Implementations must use native protected storage and must
// not fall back to plaintext.
type SecretStore interface {
	Get(context.Context, string, string) (value string, found bool, err error)
	Set(context.Context, string, string, string) error
	Delete(context.Context, string, string) error
}

type RunningChecker interface {
	Running(context.Context, string) (bool, error)
}

type Options struct {
	GOOS          string
	HomeDir       string
	UserConfigDir string
	UserDataDir   string
	SecretStore   SecretStore
	Running       RunningChecker
}

type Manager struct {
	goos          string
	homeDir       string
	userConfigDir string
	userDataDir   string
	secrets       SecretStore
	running       RunningChecker
}
