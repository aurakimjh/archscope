//go:build windows

package comautomation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestCOMRemotingHelper(t *testing.T) {
	if os.Getenv("ARCHSCOPE_COM_TEST_CHILD") != "1" {
		t.Skip("subprocess fixture only")
	}
	class := os.Getenv("ARCHSCOPE_COM_TEST_CLASS")
	root := os.Getenv("ARCHSCOPE_COM_TEST_DIR")
	s, e := startWithClass(func(_ context.Context, call automation.Call) (any, error) {
		if call.Command == "capabilities" {
			return map[string]string{"buildCommit": "synthetic-com-fixture"}, nil
		}
		return nil, fmt.Errorf("unsupported fixture command")
	}, class)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = os.WriteFile(filepath.Join(root, "ready"), []byte("ready"), 0600); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		if _, e = os.Stat(filepath.Join(root, "stop")); e == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("fixture client did not stop")
}

// Real out-of-process activation/IDispatch/ITypeInfo, with a random test CLSID.
// Only missing interface mappings are temporarily owned by this fixture. No
// existing registration, capture, browser cache or GUI is changed.
func TestNativeOutOfProcessDispatchAndTypeInfo(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if e := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); e != nil {
		t.Fatal(e)
	}
	defer windows.CoUninitialize()
	id, e := windows.GenerateGUID()
	if e != nil {
		t.Fatal(e)
	}
	class := id.String()
	root := t.TempDir()
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	// Exercise the universal marshaler on a clean user view, then remove only
	// our own temporary mappings. Existing vendor registrations are untouched.
	classes, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Classes`, registry.READ|registry.WRITE)
	if e != nil {
		t.Fatal(e)
	}
	defer classes.Close()
	if e = metadataPreflight(classes, executable); e != nil {
		t.Skip(e)
	}
	filename := filepath.Join(root, "client-types.tlb")
	infos, e := buildTypeInfosAt(filename)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range infos {
		releaseCOM(p)
	}
	if e = writeRegistryValues(classes, metadataValues(executable, filename, filename)); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := removeOwnedMetadata(classes, executable); e != nil {
			t.Error(e)
		}
	}()
	keyPath := `Software\Classes\CLSID\` + class
	key, _, e := registry.CreateKey(registry.CURRENT_USER, keyPath+`\LocalServer32`, registry.SET_VALUE)
	if e != nil {
		t.Fatal(e)
	}
	e = key.SetStringValue("", `"`+executable+`" -test.run=^TestCOMRemotingHelper$`)
	key.Close()
	if e != nil {
		t.Fatal(e)
	}
	defer registry.DeleteKey(registry.CURRENT_USER, keyPath)
	defer registry.DeleteKey(registry.CURRENT_USER, keyPath+`\LocalServer32`)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCOMRemotingHelper$")
	cmd.Env = append(os.Environ(), "ARCHSCOPE_COM_TEST_CHILD=1", "ARCHSCOPE_COM_TEST_CLASS="+class, "ARCHSCOPE_COM_TEST_DIR="+root)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if !finished {
			cancel()
			<-done
		}
	}()
	deadline := time.Now().Add(25 * time.Second)
	for {
		if _, e = os.Stat(filepath.Join(root, "ready")); e == nil {
			break
		}
		select {
		case e := <-done:
			finished = true
			t.Fatalf("COM fixture exited: %v\n%s", e, output.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("COM fixture readiness timed out")
		}
		time.Sleep(20 * time.Millisecond)
	}
	unknown, e := ole.CreateInstance(ole.NewGUID(class), ole.IID_IDispatch)
	if e != nil {
		t.Fatal(e)
	}
	dispatch := (*ole.IDispatch)(unsafe.Pointer(unknown))
	defer dispatch.Release()
	// Request normal authenticated caller impersonation for the same-user gate.
	hr, _, _ := ole32.NewProc("CoSetProxyBlanket").Call(uintptr(unsafe.Pointer(dispatch)), 0xffffffff, 0xffffffff, 0, 0, 3, 0, 0)
	if int32(uint32(hr)) < 0 {
		t.Fatalf("client proxy security: 0x%x", hr)
	}
	value, e := oleutil.GetProperty(dispatch, "BuildCommit")
	if e != nil {
		t.Fatal(e)
	}
	if value.ToString() != "synthetic-com-fixture" {
		t.Fatal("wrong remote controller")
	}
	value.Clear()
	for _, name := range []string{"IHTTPAnalyzer", "IHTTPAnalyzerStandAlone"} {
		custom, e := queryCOM(uintptr(unsafe.Pointer(dispatch)), mustGUID(referenceContracts.Interfaces[name].IID))
		if e != nil {
			t.Fatal("clean-host custom IID", name, e)
		}
		hr, _, _ = ole32.NewProc("CoSetProxyBlanket").Call(custom, 0xffffffff, 0xffffffff, 0, 0, 3, 0, 0)
		if int32(uint32(hr)) < 0 {
			t.Fatalf("custom proxy security: 0x%x", hr)
		}
		checked := false
		for index, m := range interfaceMethods(name) {
			if m.Name == "MaxLogSize" && m.Kind == "get" {
				var count int32 = -1
				if e = comCall(custom, 7+index, uintptr(unsafe.Pointer(&count))); e != nil || count != 0 {
					t.Fatal("remote typed integer", count, e)
				}
				checked = true
			}
		}
		releaseCOM(custom)
		if !checked {
			t.Fatal("fixture did not call an early-bound slot")
		}
	}
	var count uint32
	if e = comCall(uintptr(unsafe.Pointer(dispatch)), 3, uintptr(unsafe.Pointer(&count))); e != nil || count != 1 {
		t.Fatal("remote type count", count, e)
	}
	var info uintptr
	if e = comCall(uintptr(unsafe.Pointer(dispatch)), 4, 0, 0, uintptr(unsafe.Pointer(&info))); e != nil || info == 0 {
		t.Fatal("remote type info", e)
	}
	var attr *ole.TYPEATTR
	if e = comCall(info, 3, uintptr(unsafe.Pointer(&attr))); e != nil {
		releaseCOM(info)
		t.Fatal("remote type attribute", e)
	}
	if attr.WTypeFlags&0x40 == 0 {
		t.Fatal("remote metadata is not dual")
	}
	comCall(info, 19, uintptr(unsafe.Pointer(attr)))
	releaseCOM(info)
	if e = os.WriteFile(filepath.Join(root, "stop"), []byte("stop"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		finished = true
		t.Fatalf("COM fixture failed: %v\n%s", e, output.String())
	}
	finished = true
}
