//go:build windows

package clientconfig

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Assign the suspended root before it executes so every descendant belongs to
// this launch. The non-inherited job handle kills the tree even if Connect
// crashes. We never terminate another instance by application name or PID scan.
func prepareProcessSupervision(command *exec.Cmd) (func() error, func() error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return nil, nil, err
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	var once sync.Once
	var stopError error
	stop := func() error {
		once.Do(func() {
			defer windows.CloseHandle(job)
			if err := windows.TerminateJobObject(job, 1); err != nil {
				stopError = err
				return
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				var accounting struct {
					UserTime, KernelTime, PeriodUserTime, PeriodKernelTime               int64
					PageFaultCount, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
				}
				if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
					stopError = err
					return
				}
				if accounting.ActiveProcesses == 0 {
					return
				}
				if time.Now().After(deadline) {
					stopError = errors.New("client process tree did not stop")
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
		})
		return stopError
	}
	start := func() error {
		process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
		if err != nil {
			return err
		}
		defer windows.CloseHandle(process)
		if err := windows.AssignProcessToJobObject(job, process); err != nil {
			return err
		}
		// os/exec closes the primary thread handle. Before execution the new
		// process has exactly one thread; recover that handle and resume it.
		snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
		if err != nil {
			return err
		}
		defer windows.CloseHandle(snapshot)
		entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
		for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
			if entry.OwnerProcessID != uint32(command.Process.Pid) {
				continue
			}
			thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if e != nil {
				return e
			}
			_, e = windows.ResumeThread(thread)
			windows.CloseHandle(thread)
			return e
		}
		return fmt.Errorf("cannot resume suspended desktop client: %w", err)
	}
	return start, stop, nil
}
