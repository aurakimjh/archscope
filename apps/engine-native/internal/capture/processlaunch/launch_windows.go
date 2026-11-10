//go:build windows

package processlaunch

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/procmap"

	"golang.org/x/sys/windows"
)

// Prepare installs the inherited proxy environment before creating a suspended
// process. The manager can bind its PID/start time before the first instruction.
func Prepare(path string, args []string, proxy string) (*Suspended, error) {
	command := syscall.EscapeArg(path)
	for _, arg := range args {
		command += " " + syscall.EscapeArg(arg)
	}
	commandLine, err := windows.UTF16FromString(command)
	if err != nil {
		return nil, err
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY":
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment, "HTTP_PROXY=http://"+proxy, "HTTPS_PROXY=http://"+proxy, "NO_PROXY=")
	sort.Slice(environment, func(i, j int) bool { return strings.ToUpper(environment[i]) < strings.ToUpper(environment[j]) })
	var encoded []uint16
	for _, entry := range environment {
		value, e := windows.UTF16FromString(entry)
		if e != nil {
			return nil, e
		}
		encoded = append(encoded, value...)
	}
	encoded = append(encoded, 0)
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var info windows.ProcessInformation
	applicationName, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	err = windows.CreateProcess(applicationName, &commandLine[0], nil, nil, false,
		windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT, &encoded[0], nil, &startup, &info)
	if err != nil {
		return nil, err
	}
	child := &Suspended{Key: procmap.Lookup(int32(info.ProcessId)).Key, state: "prepared",
		resume: func() error {
			previous, err := windows.ResumeThread(info.Thread)
			if err != nil {
				return err
			}
			if previous != 1 {
				return fmt.Errorf("launched thread was not in the expected suspended state")
			}
			return nil
		},
		abort:   func() { _ = windows.TerminateProcess(info.Process, 1) },
		release: func() { _ = windows.CloseHandle(info.Thread); _ = windows.CloseHandle(info.Process) },
	}
	if child.Key.StartTime == "" {
		child.Abort()
		return nil, fmt.Errorf("cannot verify launched process identity")
	}
	return child, nil
}
