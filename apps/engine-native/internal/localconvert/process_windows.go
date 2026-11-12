package localconvert

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"unsafe"
)

func prepareProcess(ctx context.Context, cmd *exec.Cmd) (func() error, func(), error) {
	job, e := windows.CreateJobObject(nil, nil)
	if e != nil {
		return nil, nil, e
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, e = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); e != nil {
		windows.CloseHandle(job)
		return nil, nil, e
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW}
	cmd.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	started := func() error {
		process, e := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if e != nil {
			return e
		}
		defer windows.CloseHandle(process)
		if e = windows.AssignProcessToJobObject(job, process); e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			_ = windows.TerminateJobObject(job, 1)
			return e
		}
		snapshot, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
		if e != nil {
			return e
		}
		defer windows.CloseHandle(snapshot)
		entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
		for e = windows.Thread32First(snapshot, &entry); e == nil; e = windows.Thread32Next(snapshot, &entry) {
			if entry.OwnerProcessID == uint32(cmd.Process.Pid) {
				thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
				if e != nil {
					return e
				}
				_, e = windows.ResumeThread(thread)
				windows.CloseHandle(thread)
				return e
			}
		}
		return fmt.Errorf("suspended converter thread unavailable")
	}
	return started, func() { windows.CloseHandle(job) }, nil
}
