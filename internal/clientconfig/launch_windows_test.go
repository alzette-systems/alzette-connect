//go:build windows

package clientconfig

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Each helper is a real child process. Tests inspect the descendants through
// retained process handles, so PID reuse cannot produce a false pass.
func TestWindowsSupervisionHelper(t *testing.T) {
	mode := os.Getenv("ALZETTE_JOB_TEST_MODE")
	if mode == "" {
		return
	}
	executable, _ := os.Executable()
	marker := os.Getenv("ALZETTE_JOB_TEST_MARKER")
	if mode == "supervisor" {
		_, err := launchObserved(context.Background(), executable, []string{"-test.run=^TestWindowsSupervisionHelper$"}, []string{"ALZETTE_JOB_TEST_MODE=root", "ALZETTE_JOB_TEST_MARKER=" + marker}, nil)
		if err != nil {
			t.Fatal(err)
		}
	} else if mode == "root" {
		child := exec.Command(executable, "-test.run=^TestWindowsSupervisionHelper$")
		child.Env = append(launchEnvironment(os.Environ()), "ALZETTE_JOB_TEST_MODE=leaf")
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal([]int{os.Getpid(), child.Process.Pid})
		if err := os.WriteFile(marker, data, 0o600); err != nil {
			t.Fatal(err)
		}
		defer child.Wait()
	} else if mode != "leaf" {
		t.Fatal("unknown helper mode")
	}
	time.Sleep(time.Minute)
}

func jobTestHandles(t *testing.T, marker string) []windows.Handle {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(marker)
		if err == nil && json.Unmarshal(data, &pids) == nil && len(pids) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(pids) != 2 {
		t.Fatal("helper tree did not start")
	}
	handles := []windows.Handle{}
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
		if err != nil {
			t.Fatalf("retain process %s: %v", strconv.Itoa(pid), err)
		}
		handles = append(handles, h)
		t.Cleanup(func() { windows.CloseHandle(h) })
	}
	return handles
}

func assertJobStopped(t *testing.T, handles []windows.Handle) {
	t.Helper()
	for _, h := range handles {
		result, err := windows.WaitForSingleObject(h, 5000)
		if err != nil || result != windows.WAIT_OBJECT_0 {
			t.Fatalf("owned descendant survived: result=%d err=%v", result, err)
		}
	}
}

func TestWindowsStopTerminatesOwnedDescendants(t *testing.T) {
	executable, _ := os.Executable()
	marker := filepath.Join(t.TempDir(), "tree.json")
	p, err := launchObserved(context.Background(), executable, []string{"-test.run=^TestWindowsSupervisionHelper$"}, []string{"ALZETTE_JOB_TEST_MODE=root", "ALZETTE_JOB_TEST_MARKER=" + marker}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Stop(context.Background()) })
	handles := jobTestHandles(t, marker)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	assertJobStopped(t, handles)
}

func TestWindowsSupervisorCrashTerminatesOwnedDescendants(t *testing.T) {
	executable, _ := os.Executable()
	marker := filepath.Join(t.TempDir(), "tree.json")
	supervisor := exec.Command(executable, "-test.run=^TestWindowsSupervisionHelper$")
	supervisor.Env = append(launchEnvironment(os.Environ()), "ALZETTE_JOB_TEST_MODE=supervisor", "ALZETTE_JOB_TEST_MARKER="+marker)
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = supervisor.Process.Kill(); _ = supervisor.Wait() })
	handles := jobTestHandles(t, marker)
	if err := supervisor.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	assertJobStopped(t, handles)
}
