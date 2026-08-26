//go:build darwin

package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func startInstall(assetPath, expectedVersion string) error {
	executable, err := currentExecutable()
	if err != nil {
		return err
	}
	if err := preflightMacInstall(executable); err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "alzette-connect-helper-")
	if err != nil {
		return errors.New("prepare update helper")
	}
	helper := filepath.Join(directory, "alzette-connect-updater")
	if err := copyRegularFile(executable, helper, 0o700); err != nil {
		_ = os.RemoveAll(directory)
		return errors.New("prepare update helper")
	}
	command := exec.Command(helper, helperFlag, strconv.Itoa(os.Getpid()), assetPath, executable, expectedVersion)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(directory)
		return errors.New("start update helper")
	}
	return command.Process.Release()
}

func applyUpdate(rawPID, assetPath, executable, expectedVersion string) error {
	pid, err := strconv.Atoi(rawPID)
	if err != nil || pid <= 1 {
		return errors.New("invalid update process")
	}
	currentApp, err := macBundleForExecutable(executable)
	if err != nil {
		return err
	}
	if err := waitForProcess(pid, 45*time.Second); err != nil {
		return err
	}
	parent := filepath.Dir(currentApp)
	stage, err := os.MkdirTemp(parent, ".alzette-connect-update-")
	if err != nil {
		return errors.New("the application folder is not writable")
	}
	defer os.RemoveAll(stage)
	if output, err := exec.Command("/usr/bin/ditto", "-x", "-k", assetPath, stage).CombinedOutput(); err != nil || len(output) > 4096 {
		return errors.New("the verified update could not be unpacked")
	}
	newApp := filepath.Join(stage, "Alzette Connect.app")
	if err := verifyMacBundle(newApp, expectedVersion); err != nil {
		return err
	}
	backup := currentApp + ".previous-" + time.Now().UTC().Format("20060102T150405Z")
	if err := os.Rename(currentApp, backup); err != nil {
		return errors.New("the installed application could not be prepared for update")
	}
	if err := os.Rename(newApp, currentApp); err != nil {
		_ = os.Rename(backup, currentApp)
		return errors.New("the update could not replace the installed application")
	}
	if err := exec.Command("/usr/bin/open", "-n", currentApp).Start(); err != nil {
		_ = os.RemoveAll(currentApp)
		_ = os.Rename(backup, currentApp)
		return errors.New("the updated application could not be reopened")
	}
	_ = os.Remove(assetPath)
	return nil
}

func verifyMacBundle(path, expectedVersion string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrUnsafeRelease
	}
	command := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", path)
	if err := command.Run(); err != nil {
		return errors.New("the update application signature is invalid")
	}
	identifier, err := macBundleIdentifier(path)
	if err != nil || identifier != "systems.alzette.Connect" {
		return ErrUnsafeRelease
	}
	output, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", filepath.Join(path, "Contents", "Info.plist")).Output()
	if err != nil || normalizeVersion(strings.TrimSpace(string(output))) != expectedVersion {
		return errors.New("the update application version is invalid")
	}
	binary := filepath.Join(path, "Contents", "MacOS", "alzette-connect")
	if info, err := os.Lstat(binary); err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return ErrUnsafeRelease
	}
	return nil
}

func macBundleForExecutable(executable string) (string, error) {
	clean := filepath.Clean(executable)
	if !strings.HasSuffix(clean, "/Contents/MacOS/alzette-connect") {
		return "", errors.New("Alzette Connect is not running from an application bundle")
	}
	bundle := filepath.Clean(filepath.Join(filepath.Dir(clean), "..", ".."))
	info, err := os.Lstat(bundle)
	if filepath.Ext(bundle) != ".app" || err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unexpected application bundle")
	}
	identifier, err := macBundleIdentifier(bundle)
	if err != nil || identifier != "systems.alzette.Connect" {
		return "", errors.New("unexpected application bundle")
	}
	return bundle, nil
}

func macBundleIdentifier(bundle string) (string, error) {
	output, err := exec.Command(
		"/usr/libexec/PlistBuddy",
		"-c",
		"Print :CFBundleIdentifier",
		filepath.Join(bundle, "Contents", "Info.plist"),
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func preflightMacInstall(executable string) error {
	bundle, err := macBundleForExecutable(executable)
	if err != nil {
		return err
	}
	if strings.Contains(bundle, "/AppTranslocation/") {
		return errors.New("Move Alzette Connect to Applications, reopen it there, then update")
	}
	probe, err := os.MkdirTemp(filepath.Dir(bundle), ".alzette-connect-update-check-")
	if err != nil {
		return errors.New("Move Alzette Connect to a writable Applications folder, then update")
	}
	if err := os.Remove(probe); err != nil {
		return errors.New("the application folder could not be prepared for update")
	}
	return nil
}

func reopenAfterUpdateFailure(executable string) error {
	bundle, err := macBundleForExecutable(executable)
	if err != nil {
		return err
	}
	return exec.Command("/usr/bin/open", "-n", bundle).Start()
}

func waitForProcess(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return nil
		}
		if err != nil && err != syscall.EPERM {
			return fmt.Errorf("wait for Alzette Connect to close")
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("Alzette Connect did not close in time for the update")
}

func copyRegularFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ErrUnsafeRelease
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := output.ReadFrom(input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
