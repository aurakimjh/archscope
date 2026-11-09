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
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"golang.org/x/sys/windows"
)

type Suspended struct {
	info windows.ProcessInformation
	Key  models.ProcessKey
}

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
	err = windows.CreateProcess(nil, &commandLine[0], nil, nil, false,
		windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT, &encoded[0], nil, &startup, &info)
	if err != nil {
		return nil, err
	}
	child := &Suspended{info: info, Key: procmap.Lookup(int32(info.ProcessId)).Key}
	if child.Key.StartTime == "" {
		child.Abort()
		return nil, fmt.Errorf("cannot verify launched process identity")
	}
	return child, nil
}

func (s *Suspended) Resume() error {
	_, err := windows.ResumeThread(s.info.Thread)
	if err != nil {
		return err
	}
	windows.CloseHandle(s.info.Thread)
	windows.CloseHandle(s.info.Process)
	return nil
}

func (s *Suspended) Abort() {
	windows.TerminateProcess(s.info.Process, 1)
	windows.CloseHandle(s.info.Thread)
	windows.CloseHandle(s.info.Process)
}
