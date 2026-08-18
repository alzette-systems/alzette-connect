//go:build windows

package updater

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/windows"
)

func startInstall(assetPath string) error {
	executable, err := currentExecutable()
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "alzette-connect-helper-")
	if err != nil {
		return errors.New("prepare update helper")
	}
	helper := filepath.Join(directory, "alzette-connect-updater.exe")
	if err := copyWindowsFile(executable, helper); err != nil {
		_ = os.RemoveAll(directory)
		return errors.New("prepare update helper")
	}
	command := exec.Command(helper, helperFlag, strconv.Itoa(os.Getpid()), assetPath, executable)
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(directory)
		return errors.New("start update helper")
	}
	return command.Process.Release()
}

func applyUpdate(rawPID, assetPath, executable string) error {
	pid, err := strconv.ParseUint(rawPID, 10, 32)
	if err != nil || pid <= 1 {
		return errors.New("invalid update process")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil && !errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return errors.New("Alzette Connect could not confirm that the previous version closed")
	}
	if err == nil {
		defer windows.CloseHandle(handle)
		result, waitErr := windows.WaitForSingleObject(handle, 45_000)
		if waitErr != nil || result != windows.WAIT_OBJECT_0 {
			return errors.New("Alzette Connect did not close in time for the update")
		}
	}
	command := exec.Command(assetPath, "/S")
	if err := command.Run(); err != nil {
		return errors.New("the verified update installer failed")
	}
	time.Sleep(500 * time.Millisecond)
	if err := exec.Command(executable).Start(); err != nil {
		return errors.New("the updated application could not be reopened")
	}
	_ = os.Remove(assetPath)
	return nil
}

func copyWindowsFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ErrUnsafeRelease
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
