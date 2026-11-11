//go:build windows

package comautomation

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestOwnedMetadataMappingsAndForeignPreservation(t *testing.T) {
	base := fmt.Sprintf(`Software\ArchScope\Tests\Metadata-%d`, time.Now().UnixNano())
	root, _, e := registry.CreateKey(registry.CURRENT_USER, base, registry.READ|registry.WRITE)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	defer registry.DeleteKey(registry.CURRENT_USER, base)
	exe, _ := os.Executable()
	if e = metadataPreflight(root, exe); e != nil {
		t.Fatal(e)
	}
	if e = writeRegistryValues(root, metadataValues(exe, `C:\owned32.tlb`, `C:\owned64.tlb`)); e != nil {
		t.Fatal(e)
	}
	if e = metadataPreflight(root, exe); e != nil {
		t.Fatal(e)
	}
	if e = metadataPreflight(root, exe+".other"); e == nil {
		t.Fatal("foreign owner accepted")
	}
	if e = removeOwnedMetadata(root, exe+".other"); e == nil {
		t.Fatal("foreign owner removed")
	}
	if e = removeOwnedMetadata(root, exe); e != nil {
		t.Fatal(e)
	}
	// Empty grouping keys are ours in this isolated fixture.
	registry.DeleteKey(root, "Interface")
	registry.DeleteKey(root, "TypeLib")
	p := `Interface\` + referenceContracts.Interfaces["ILog"].IID
	k, _, e := registry.CreateKey(root, p, registry.READ|registry.WRITE)
	if e != nil {
		t.Fatal(e)
	}
	k.SetStringValue("", "vendor sentinel")
	k.Close()
	if e = metadataPreflight(root, exe); e == nil {
		t.Fatal("unowned vendor key accepted")
	}
	k, e = registry.OpenKey(root, p, registry.READ)
	if e != nil {
		t.Fatal(e)
	}
	value, _, e := k.GetStringValue("")
	k.Close()
	if e != nil || value != "vendor sentinel" {
		t.Fatal(value, e)
	}
	registry.DeleteKey(root, p)
	registry.DeleteKey(root, "Interface")
}

func TestBothArchitectureTypeLibrariesDescribeTargetOffsets(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if e := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); e != nil {
		t.Fatal(e)
	}
	defer windows.CoUninitialize()
	for _, target := range []struct {
		kind uintptr
		word int
	}{{1, 4}, {3, 8}} {
		filename := filepath.Join(t.TempDir(), "types.tlb")
		infos, e := buildTypeInfosFor(filename, target.kind)
		if e != nil {
			t.Fatal(target.kind, e)
		}
		for name, info := range infos {
			var fd *functionDescription
			if e = comCall(info, 5, 0, uintptr(unsafe.Pointer(&fd))); e != nil {
				t.Fatal(e)
			}
			if fd.Offset != int16(7*target.word) {
				t.Errorf("%s offset=%d kind=%d", name, fd.Offset, target.kind)
			}
			comCall(info, 20, uintptr(unsafe.Pointer(fd)))
			releaseCOM(info)
		}
		if st, e := os.Stat(filename); e != nil || st.Size() == 0 {
			t.Fatal("type library not persisted", e)
		}
	}
}

// Run the explicit public lifecycle only when all affected product/interface
// keys are absent. Never commandeer an installed product's registration.
func TestExplicitRegistrationLifecycleOnCleanHost(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	roots := []registry.Key{}
	defer func() {
		for _, root := range roots {
			root.Close()
		}
	}()
	for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
		root, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Classes`, registry.READ|registry.WRITE|view)
		if e != nil {
			t.Fatal(e)
		}
		roots = append(roots, root)
		for _, p := range append(metadataRoots(), `CLSID\`+ClassID, ProgID) {
			k, e := registry.OpenKey(root, p, registry.READ)
			if e == nil {
				k.Close()
				t.Skip("existing registration preserved", p)
			}
		}
		machine, e := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Classes`, registry.READ|view)
		if e != nil {
			t.Fatal(e)
		}
		for _, p := range metadataRoots() {
			k, e := registry.OpenKey(machine, p, registry.READ)
			if e == nil {
				k.Close()
				machine.Close()
				t.Skip("existing machine registration preserved", p)
			}
		}
		machine.Close()
	}
	if e = Register(exe); e != nil {
		t.Fatal(e)
	}
	registered := true
	defer func() {
		if registered {
			if e := Unregister(exe); e != nil {
				t.Error("cleanup", e)
			}
		}
	}()
	first := registeredMetadataFiles(roots)
	if len(first) != 2 {
		t.Fatal("missing architecture libraries", first)
	}
	if e = Register(exe); e != nil {
		t.Fatal("replace", e)
	}
	for _, p := range first {
		if _, e = os.Stat(p); !os.IsNotExist(e) {
			t.Fatal("replaced library retained", p, e)
		}
	}
	second := registeredMetadataFiles(roots)
	if e = Unregister(exe); e != nil {
		t.Fatal("unregister", e)
	}
	registered = false
	for _, p := range second {
		if _, e = os.Stat(p); !os.IsNotExist(e) {
			t.Fatal("unregistered library retained", p, e)
		}
	}
	for _, root := range roots {
		for _, p := range append(metadataRoots(), `CLSID\`+ClassID, ProgID) {
			if k, e := registry.OpenKey(root, p, registry.READ); e == nil {
				k.Close()
				t.Error("owned registration leaked", p)
			}
		}
	}
}
