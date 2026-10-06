package credentialstore

import (
	"errors"
	"strings"
	"testing"
)

type recordMemory struct {
	values    map[string][]byte
	failWrite int
	writes    int
}

func (m *recordMemory) read(name string) ([]byte, error) {
	v, ok := m.values[name]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}
func (m *recordMemory) write(name string, value []byte) error {
	m.writes++
	if m.writes == m.failWrite {
		return ErrUnavailable
	}
	if len(value) > 2560 {
		return ErrUnavailable
	}
	m.values[name] = append([]byte(nil), value...)
	return nil
}
func (m *recordMemory) remove(name string) error { delete(m.values, name); return nil }
func (m *recordMemory) names(prefix string) ([]string, error) {
	var names []string
	for name := range m.values {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names, nil
}

func TestProtectedRefreshRotationHandlesLongTokensAndRemovesOldParts(t *testing.T) {
	m := &recordMemory{values: map[string][]byte{}}
	for _, length := range []int{16, 4096, 16384, 2401, 32} {
		value := strings.Repeat("a", length)
		if err := saveRefreshRecord(m, "profile", value); err != nil {
			t.Fatal(err)
		}
		loaded, err := loadRefreshRecord(m, "profile")
		if err != nil || loaded != value {
			t.Fatal("rotation round trip failed")
		}
		if len(m.values) != 1+(length+protectedPartSize-1)/protectedPartSize {
			t.Fatal("old protected records survived rotation")
		}
	}
	if err := deleteRefreshRecord(m, "profile"); err != nil || len(m.values) != 0 {
		t.Fatal("deletion left protected records")
	}
}

func TestProtectedRefreshInterruptedRotationKeepsPreviousToken(t *testing.T) {
	for failure := 1; failure <= 4; failure++ {
		m := &recordMemory{values: map[string][]byte{}}
		if err := saveRefreshRecord(m, "profile", strings.Repeat("old", 1200)); err != nil {
			t.Fatal(err)
		}
		count := len(m.values)
		m.failWrite = m.writes + failure
		if err := saveRefreshRecord(m, "profile", strings.Repeat("new", 2000)); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("failure %d was not simulated", failure)
		}
		loaded, err := loadRefreshRecord(m, "profile")
		if err != nil || loaded != strings.Repeat("old", 1200) || len(m.values) != count {
			t.Fatal("failed rotation destroyed previous credential or leaked partial records")
		}
	}
}

func TestProtectedRefreshFailsClosedOnMissingOrModifiedPart(t *testing.T) {
	m := &recordMemory{values: map[string][]byte{}}
	if err := saveRefreshRecord(m, "profile", strings.Repeat("token", 900)); err != nil {
		t.Fatal(err)
	}
	r, _ := readRefreshRecord(m, "profile")
	key := partTarget("profile", r.Generation, 0)
	m.values[key][0] ^= 1
	if _, err := loadRefreshRecord(m, "profile"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("modified credential accepted")
	}
	delete(m.values, key)
	if _, err := loadRefreshRecord(m, "profile"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("incomplete credential accepted")
	}
}
