//go:build darwin && cgo

package mackeychain

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNativeKeychainLifecycle(t *testing.T) {
	service := "systems.alzette.Connect.test"
	account := fmt.Sprintf("test-%d-%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() {
		if err := Delete(service, account); err != nil {
			t.Errorf("cleanup Keychain item: %v", err)
		}
	})

	if _, err := Get(service, account); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() before Set() error = %v, want ErrNotFound", err)
	}
	if err := Set(service, account, []byte("first-refresh-credential")); err != nil {
		t.Fatalf("Set() fresh item: %v", err)
	}
	if got, err := Get(service, account); err != nil || string(got) != "first-refresh-credential" {
		t.Fatalf("Get() fresh item = %q, %v", got, err)
	}
	if err := Set(service, account, []byte("rotated-refresh-credential")); err != nil {
		t.Fatalf("Set() existing item: %v", err)
	}
	if got, err := Get(service, account); err != nil || string(got) != "rotated-refresh-credential" {
		t.Fatalf("Get() rotated item = %q, %v", got, err)
	}
	if err := Delete(service, account); err != nil {
		t.Fatalf("Delete(): %v", err)
	}
	if _, err := Get(service, account); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
	if err := Delete(service, account); err != nil {
		t.Fatalf("Delete() absent item: %v", err)
	}
}
