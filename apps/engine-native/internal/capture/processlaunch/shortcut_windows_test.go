//go:build windows

package processlaunch

import (
	"encoding/json"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestShortcutLaunchFixture(t *testing.T) {
	if marker := os.Getenv("ARCHSCOPE_SHORTCUT_MARKER"); marker != "" {
		cwd, _ := os.Getwd()
		data, _ := json.Marshal(struct {
			Directory string
			Args      []string
			Proxy     string
		}{cwd, os.Args, os.Getenv("HTTP_PROXY")})
		if os.WriteFile(marker, data, 0600) != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
}
func TestNativeShortcutLaunchPreservesUnicodeArgumentsAndDirectory(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "한글 working folder")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "marker.json")
	t.Setenv("ARCHSCOPE_SHORTCUT_MARKER", marker)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "fixture.LNK")
	wanted := []string{"-test.run=^TestShortcutLaunchFixture$", "--", "한글 space", `C:\ending\`, "", `quoted"value`}
	raw := ""
	for _, arg := range wanted {
		if raw != "" {
			raw += " "
		}
		raw += syscall.EscapeArg(arg)
	}
	err = withShellLink(func(link *shellLink, file *persistFile) error {
		for _, property := range []struct {
			method uintptr
			value  string
		}{{link.table.SetPath, exe}, {link.table.SetArguments, raw}, {link.table.SetWorkingDirectory, directory}} {
			value, err := windows.UTF16PtrFromString(property.value)
			if err != nil {
				return err
			}
			result, _, _ := syscall.SyscallN(property.method, uintptr(unsafe.Pointer(link)), uintptr(unsafe.Pointer(value)))
			if err := hresult("fixture property", result); err != nil {
				return err
			}
		}
		value, _ := windows.UTF16PtrFromString(linkPath)
		result, _, _ := syscall.SyscallN(file.table.Save, uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(value)), 1)
		return hresult("fixture save", result)
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := Resolve(linkPath, []string{"extra argument"})
	if err != nil {
		t.Fatal(err)
	}
	if target.Path != exe || target.RawArguments != raw || target.WorkingDirectory != directory {
		t.Fatalf("unexpected resolved shortcut: %+v", target)
	}
	child, err := Prepare(target, "127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	defer child.Abort()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("shortcut ran before scope assignment")
	}
	if err := child.Resume(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil {
			var got struct {
				Directory string
				Args      []string
				Proxy     string
			}
			if json.Unmarshal(data, &got) != nil {
				t.Fatal("invalid marker")
			}
			if got.Directory != directory || got.Proxy != "http://127.0.0.1:12345" || !reflect.DeepEqual(got.Args[1:], append(wanted, "extra argument")) {
				t.Fatalf("%+v", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("shortcut child did not complete")
}
