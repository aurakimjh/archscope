//go:build windows

package processlaunch

import (
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"syscall"
	"unsafe"
)

// IShellLinkW/IPersistFile vtables follow the Windows SDK shobjidl_core.h.
// Only load/getters are used: no ShellExecute, link tracking, search or UI.
type unknownTable struct{ QueryInterface, AddRef, Release uintptr }
type linkTable struct {
	unknownTable
	GetPath, GetIDList, SetIDList, GetDescription, SetDescription                  uintptr
	GetWorkingDirectory, SetWorkingDirectory, GetArguments, SetArguments           uintptr
	GetHotkey, SetHotkey, GetShowCmd, SetShowCmd, GetIconLocation, SetIconLocation uintptr
	SetRelativePath, Resolve, SetPath                                              uintptr
}
type fileTable struct {
	unknownTable
	GetClassID, IsDirty, Load, Save, SaveCompleted, GetCurFile uintptr
}
type shellLink struct{ table *linkTable }
type persistFile struct{ table *fileTable }

var createLink = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")
var shellLinkClass = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
var shellLinkInterface = windows.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
var persistFileInterface = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}

func hresult(operation string, code uintptr) error {
	if int32(uint32(code)) < 0 {
		return fmt.Errorf("shortcut %s failed (HRESULT 0x%08x)", operation, uint32(code))
	}
	return nil
}
func withShellLink(use func(*shellLink, *persistFile) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	// S_FALSE means COM was already initialized in this apartment; it still
	// requires one matching CoUninitialize. A changed apartment is an error.
	if err != nil && err != syscall.Errno(1) {
		return fmt.Errorf("shortcut COM apartment: %w", err)
	}
	defer windows.CoUninitialize()
	var link *shellLink
	result, _, _ := createLink.Call(uintptr(unsafe.Pointer(&shellLinkClass)), 0, 1, uintptr(unsafe.Pointer(&shellLinkInterface)), uintptr(unsafe.Pointer(&link)))
	if err := hresult("create", result); err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("shortcut interface unavailable")
	}
	defer syscall.SyscallN(link.table.Release, uintptr(unsafe.Pointer(link)))
	var file *persistFile
	result, _, _ = syscall.SyscallN(link.table.QueryInterface, uintptr(unsafe.Pointer(link)), uintptr(unsafe.Pointer(&persistFileInterface)), uintptr(unsafe.Pointer(&file)))
	if err := hresult("persistence", result); err != nil {
		return err
	}
	if file == nil {
		return fmt.Errorf("shortcut persistence unavailable")
	}
	defer syscall.SyscallN(file.table.Release, uintptr(unsafe.Pointer(file)))
	return use(link, file)
}
func readShortcut(path string) (shortcutProperties, error) {
	var properties shortcutProperties
	if err := checkShortcutHeader(path); err != nil {
		return properties, err
	}
	err := withShellLink(func(link *shellLink, file *persistFile) error {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		result, _, _ := syscall.SyscallN(file.table.Load, uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(name)), 0)
		if err := hresult("load", result); err != nil {
			return err
		}
		get := func(method uintptr, path bool) (string, error) {
			buffer := make([]uint16, 32768)
			var code uintptr
			if path {
				code, _, _ = syscall.SyscallN(method, uintptr(unsafe.Pointer(link)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
			} else {
				code, _, _ = syscall.SyscallN(method, uintptr(unsafe.Pointer(link)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
			}
			if err := hresult("read", code); err != nil {
				return "", err
			}
			count := 0
			for count < len(buffer) && buffer[count] != 0 {
				count++
			}
			if count >= len(buffer)-1 {
				return "", fmt.Errorf("shortcut property is truncated or too long")
			}
			return windows.UTF16ToString(buffer[:count]), nil
		}
		properties.Path, err = get(link.table.GetPath, true)
		if err != nil {
			return err
		}
		properties.Arguments, err = get(link.table.GetArguments, false)
		if err != nil {
			return err
		}
		properties.WorkingDirectory, err = get(link.table.GetWorkingDirectory, false)
		return err
	})
	if err != nil {
		return properties, err
	}
	properties.Path, err = expandShortcutPath(properties.Path)
	if err != nil {
		return properties, err
	}
	properties.WorkingDirectory, err = expandShortcutPath(properties.WorkingDirectory)
	return properties, err
}
func expandShortcutPath(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	input, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, 32768)
	length, err := windows.ExpandEnvironmentStrings(input, &buffer[0], uint32(len(buffer)))
	if err != nil {
		return "", err
	}
	if length == 0 || length > uint32(len(buffer)) {
		return "", fmt.Errorf("expanded shortcut path is too long")
	}
	return windows.UTF16ToString(buffer), nil
}
