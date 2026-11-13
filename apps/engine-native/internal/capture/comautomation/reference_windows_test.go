//go:build windows

package comautomation

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// REGKIND_NONE reads installed metadata without registration or vendor execution.
// Both libraries are needed: the EXE imports the companion core interfaces.
func TestInstalledReferenceTypeLibrary(t *testing.T) {
	path := os.Getenv("ARCHSCOPE_REFERENCE_TYPELIB")
	if path == "" {
		t.Skip("set ARCHSCOPE_REFERENCE_TYPELIB to the installed Stand-alone EXE")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		t.Fatal(err)
	}
	defer windows.CoUninitialize()
	load := func(path string) uintptr {
		t.Helper()
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		var lib uintptr
		hr, _, _ := windows.NewLazySystemDLL("oleaut32.dll").NewProc("LoadTypeLibEx").Call(uintptr(unsafe.Pointer(name)), 2, uintptr(unsafe.Pointer(&lib)))
		if int32(uint32(hr)) < 0 {
			t.Fatalf("LoadTypeLibEx: 0x%08x", uint32(hr))
		}
		return lib
	}
	standalone := load(path)
	defer releaseCOM(standalone)
	core := load(filepath.Join(filepath.Dir(path), "HTTPAnalyzer.tlb"))
	defer releaseCOM(core)
	generated, err := buildTypeInfos()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, p := range generated {
			releaseCOM(p)
		}
	}()
	keys := make([]string, 0, len(referenceContracts.Interfaces))
	for key := range referenceContracts.Interfaces {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	checked := 0
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			lib := core
			if key == "IHTTPAnalyzerStandAlone" {
				lib = standalone
			}
			id := mustGUID(referenceContracts.Interfaces[key].IID)
			var dispatch uintptr
			if err := comCall(lib, 6, uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&dispatch))); err != nil {
				t.Fatal(err)
			}
			defer releaseCOM(dispatch)
			var href uint32
			if err := comCall(dispatch, 8, uintptr(^uint32(0)), uintptr(unsafe.Pointer(&href))); err != nil {
				t.Fatal(err)
			}
			var info uintptr
			if err := comCall(dispatch, 14, uintptr(href), uintptr(unsafe.Pointer(&info))); err != nil {
				t.Fatal(err)
			}
			defer releaseCOM(info)
			var attr *ole.TYPEATTR
			if err := comCall(info, 3, uintptr(unsafe.Pointer(&attr))); err != nil {
				t.Fatal(err)
			}
			count, kind := attr.CFuncs, attr.Typekind
			comCall(info, 19, uintptr(unsafe.Pointer(attr)))
			if kind != 3 || count == 0 {
				t.Fatalf("expected nonempty dual vtable: kind=%d count=%d", kind, count)
			}
			for index := 0; index < int(count); index++ {
				var original *functionDescription
				if err := comCall(info, 5, uintptr(index), uintptr(unsafe.Pointer(&original))); err != nil {
					t.Fatal(err)
				}
				func() {
					defer comCall(info, 20, uintptr(unsafe.Pointer(original)))
					word := int(unsafe.Sizeof(uintptr(0)))
					slot := int(original.Offset) / word
					if int(original.Offset)%word != 0 || slot < 7 || slot-7 >= len(interfaceMethods(key)) {
						t.Fatalf("reference slot %d is outside %s", slot, key)
					}
					var local *functionDescription
					if err := comCall(generated[key], 5, uintptr(slot-7), uintptr(unsafe.Pointer(&local))); err != nil {
						t.Fatal(err)
					}
					defer comCall(generated[key], 20, uintptr(unsafe.Pointer(local)))
					a, b := referenceFunctionSignature(t, info, original), referenceFunctionSignature(t, generated[key], local)
					if a != b {
						t.Errorf("slot %d reference=%s generated=%s", slot, a, b)
					}
					checked++
				}()
			}
			t.Logf("compared %d installed dual methods including parameter flags and referenced types", count)
		})
	}
	t.Logf("installed reference methods compared: %d", checked)
}

func referenceFunctionSignature(t *testing.T, info uintptr, fd *functionDescription) string {
	t.Helper()
	value := fmt.Sprintf("id=%d kind=%d invoke=%d cc=%d result=%s", fd.ID, fd.Kind, fd.InvokeKind, fd.CallConvention, referenceABIType(t, info, fd.Result.Type, 0))
	for _, p := range unsafe.Slice(fd.Params, int(fd.Count)) {
		value += fmt.Sprintf("/%d:%s", p.Param.Flags, referenceABIType(t, info, p.Type, 0))
	}
	return value
}
func referenceABIType(t *testing.T, info uintptr, d typeDescription, depth int) string {
	t.Helper()
	if depth > 8 {
		t.Fatal("recursive reference ABI type")
	}
	if d.VT == 26 {
		return "ptr(" + referenceABIType(t, info, *(*typeDescription)(foreignPointer(d.Value)), depth+1) + ")"
	}
	if d.VT != 29 {
		return fmt.Sprint(d.VT)
	}
	var target uintptr
	if err := comCall(info, 14, d.Value, uintptr(unsafe.Pointer(&target))); err != nil {
		t.Fatal(err)
	}
	defer releaseCOM(target)
	var attr *ole.TYPEATTR
	if err := comCall(target, 3, uintptr(unsafe.Pointer(&attr))); err != nil {
		t.Fatal(err)
	}
	defer comCall(target, 19, uintptr(unsafe.Pointer(attr)))
	if attr.Typekind == 0 {
		return "3"
	} // Automation enums are signed 32-bit.
	if attr.Typekind == 6 {
		return fmt.Sprint(attr.TdescAlias.VT)
	}
	return "iid:" + strings.ToLower(attr.Guid.String())
}
