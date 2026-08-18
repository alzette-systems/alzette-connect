package credentialstore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
)

var (
	ErrNotFound    = errors.New("credential not found")
	ErrUnavailable = errors.New("protected credential store unavailable")
)

var validProfile = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Store owns only the rotating identity refresh credential. OAuth access
// tokens, Alzette inference tokens, and localhost capabilities must never be
// passed to this interface.
type Store interface {
	Load(context.Context, string) (string, error)
	Save(context.Context, string, string) error
	Delete(context.Context, string) error
	Acquire(context.Context, string) (func(), error)
	Kind() string
}

func validate(profile, credential string, requireCredential bool) error {
	if !validProfile.MatchString(profile) {
		return errors.New("credential profile is invalid")
	}
	if requireCredential && (len(credential) < 16 || len(credential) > 16<<10) {
		return errors.New("refresh credential is invalid")
	}
	return nil
}

// Memory is an explicit development/test store. It is never selected as a
// silent fallback when the OS credential store is unavailable.
type Memory struct {
	mu     sync.Mutex
	values map[string]string
	locks  map[string]*sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{values: make(map[string]string), locks: make(map[string]*sync.Mutex)}
}

func (m *Memory) Kind() string { return "memory" }

func (m *Memory) Load(_ context.Context, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[profile]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (m *Memory) Save(_ context.Context, profile, credential string) error {
	if err := validate(profile, credential, true); err != nil {
		return err
	}
	m.mu.Lock()
	m.values[profile] = credential
	m.mu.Unlock()
	return nil
}

func (m *Memory) Delete(_ context.Context, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.values, profile)
	m.mu.Unlock()
	return nil
}

func (m *Memory) Acquire(ctx context.Context, profile string) (func(), error) {
	if err := validate(profile, "", false); err != nil {
		return nil, err
	}
	m.mu.Lock()
	lock := m.locks[profile]
	if lock == nil {
		lock = &sync.Mutex{}
		m.locks[profile] = lock
	}
	m.mu.Unlock()
	locked := make(chan struct{})
	go func() {
		lock.Lock()
		close(locked)
	}()
	select {
	case <-ctx.Done():
		go func() {
			<-locked
			lock.Unlock()
		}()
		return nil, ctx.Err()
	case <-locked:
		var once sync.Once
		return func() { once.Do(lock.Unlock) }, nil
	}
}

type Unavailable struct{ Reason string }

func (u Unavailable) Kind() string { return "unavailable" }
func (u Unavailable) Load(context.Context, string) (string, error) {
	return "", u.err()
}
func (u Unavailable) Save(context.Context, string, string) error { return u.err() }
func (u Unavailable) Delete(context.Context, string) error       { return u.err() }
func (u Unavailable) Acquire(context.Context, string) (func(), error) {
	return nil, u.err()
}
func (u Unavailable) err() error {
	if u.Reason == "" {
		return ErrUnavailable
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, u.Reason)
}
