//go:build windows

package credentialstore

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestWindowsNativeLongRefreshLifecycle(t *testing.T) {
	if os.Getenv("ALZETTE_WINDOWS_NATIVE_QA") != "1" {
		t.Skip("requires interactive Windows QA desktop")
	}
	ctx := context.Background()
	store := NewPlatform()
	profile := "native-long-refresh-qa"
	defer store.Delete(ctx, profile)
	for _, length := range []int{16, 4096, 16384, 32} {
		value := strings.Repeat("x", length)
		if err := store.Save(ctx, profile, value); err != nil {
			t.Fatal(err)
		}
		loaded, err := NewPlatform().Load(ctx, profile)
		if err != nil || loaded != value {
			t.Fatal("native protected credential round trip failed")
		}
	}
	if err := store.Delete(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPlatform().Load(ctx, profile); !errors.Is(err, ErrNotFound) {
		t.Fatal("credential survived native deletion")
	}
}
