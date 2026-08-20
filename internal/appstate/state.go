package appstate

import (
	"sort"
	"sync"
	"time"
)

// Phase is deliberately small and UI-oriented. It never carries a credential
// or a raw protocol error.
type Phase string

const (
	Starting       Phase = "starting"
	SignInRequired Phase = "sign_in_required"
	SigningIn      Phase = "signing_in"
	Ready          Phase = "ready"
	NoAccess       Phase = "no_access"
	AccessRemoved  Phase = "access_removed"
	Offline        Phase = "offline"
	Stopping       Phase = "stopping"
	Failed         Phase = "failed"
)

type Context struct {
	ID           string   `json:"id"`
	Organisation string   `json:"organisation"`
	Project      string   `json:"project"`
	Environment  string   `json:"environment"`
	Models       []string `json:"models"`
}

type Application struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Version      string `json:"version,omitempty"`
	Detail       string `json:"detail,omitempty"`
	DeliveryMode string `json:"delivery_mode"`
	ModelCount   int    `json:"model_count"`
	Installed    bool   `json:"installed"`
	Configured   bool   `json:"configured"`
}

// Launch is a credential-free view of the application lifecycle. It contains
// only the information needed to explain progress and recovery in the native
// UI; process handles, paths, local capabilities, and grants never cross the
// Wails boundary.
type Launch struct {
	Phase          string    `json:"phase"`
	ApplicationID  string    `json:"application_id,omitempty"`
	Application    string    `json:"application,omitempty"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	ModelCount     int       `json:"model_count,omitempty"`
	CleanupPending bool      `json:"cleanup_pending,omitempty"`
	LocalClosed    bool      `json:"local_closed,omitempty"`
	GrantStatus    string    `json:"grant_status,omitempty"`
	ProfileStatus  string    `json:"profile_status,omitempty"`
}

// Update is presentation-only release state. It never includes a download URL,
// local path, digest, or any credential-bearing value.
type Update struct {
	State            string `json:"state"`
	CurrentVersion   string `json:"current_version"`
	AvailableVersion string `json:"available_version,omitempty"`
	Message          string `json:"message,omitempty"`
}

type Snapshot struct {
	Phase             Phase         `json:"phase"`
	Message           string        `json:"message"`
	ErrorCode         string        `json:"error_code,omitempty"`
	Contexts          []Context     `json:"contexts,omitempty"`
	SelectedContextID string        `json:"selected_context_id,omitempty"`
	Applications      []Application `json:"applications,omitempty"`
	Launch            Launch        `json:"launch"`
	Update            Update        `json:"update"`
	Platform          string        `json:"platform,omitempty"`
	TrayAvailable     bool          `json:"tray_available"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// Model is the concurrency-safe handoff between the security core and a tray
// frontend. Snapshots are copied so frontend code cannot mutate core state.
type Model struct {
	mu       sync.RWMutex
	snapshot Snapshot
	watchers map[uint64]chan Snapshot
	nextID   uint64
}

func New(now time.Time) *Model {
	return &Model{snapshot: Snapshot{Phase: Starting, Message: "Starting Alzette Connect", UpdatedAt: now.UTC()}, watchers: make(map[uint64]chan Snapshot)}
}

func (m *Model) Current() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clone(m.snapshot)
}

func (m *Model) Set(next Snapshot) {
	next.UpdatedAt = next.UpdatedAt.UTC()
	next.Contexts = cloneContexts(next.Contexts)
	next.Applications = append([]Application(nil), next.Applications...)
	m.mu.Lock()
	m.snapshot = next
	for _, watcher := range m.watchers {
		select {
		case watcher <- clone(next):
		default:
		}
	}
	m.mu.Unlock()
}

func (m *Model) Subscribe() (<-chan Snapshot, func()) {
	m.mu.Lock()
	m.nextID++
	id := m.nextID
	updates := make(chan Snapshot, 1)
	m.watchers[id] = updates
	updates <- clone(m.snapshot)
	m.mu.Unlock()
	return updates, func() {
		m.mu.Lock()
		if watcher, ok := m.watchers[id]; ok {
			delete(m.watchers, id)
			close(watcher)
		}
		m.mu.Unlock()
	}
}

func clone(value Snapshot) Snapshot {
	value.Contexts = cloneContexts(value.Contexts)
	value.Applications = append([]Application(nil), value.Applications...)
	return value
}

func cloneContexts(values []Context) []Context {
	result := append([]Context(nil), values...)
	for index := range result {
		result[index].Models = append([]string(nil), result[index].Models...)
		sort.Strings(result[index].Models)
	}
	return result
}
