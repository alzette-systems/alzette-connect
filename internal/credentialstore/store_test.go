package credentialstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryLifecycleAndLock(t *testing.T) {
	store := NewMemory()
	if _, err := store.Load(context.Background(), "default"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error=%v", err)
	}
	if err := store.Save(context.Background(), "default", "refresh-credential-value"); err != nil {
		t.Fatal(err)
	}
	if value, err := store.Load(context.Background(), "default"); err != nil || value != "refresh-credential-value" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	release, err := store.Acquire(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	release()
	if err := store.Delete(context.Background(), "default"); err != nil {
		t.Fatal(err)
	}
}

func TestUnavailableNeverFallsBack(t *testing.T) {
	store := Unavailable{Reason: "locked"}
	if err := store.Save(context.Background(), "default", "refresh-secret-value"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("save error=%v", err)
	}
}

func TestFileLockSerializesAndHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	release, err := acquireFileLock(context.Background(), root, "work")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if _, err := acquireFileLock(ctx, root, "work"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contended lock error=%v", err)
	}
	release()
	releaseAgain, err := acquireFileLock(context.Background(), root, "work")
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	releaseAgain()
}
