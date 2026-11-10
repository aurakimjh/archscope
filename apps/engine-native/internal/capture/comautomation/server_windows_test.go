//go:build windows

package comautomation

import (
	"fmt"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows/registry"
	"os"
	"testing"
	"time"
	"unsafe"
)

func TestNativeDispatchObjectVariantAndReferenceABI(t *testing.T) {
	s := &Server{objects: map[uintptr]*nativeObject{}}
	object := s.wrap(Properties{"URL": "한글"}, nil, false)
	if *(*uintptr)(unsafe.Pointer(object)) != uintptr(unsafe.Pointer(&dispatchTable)) {
		t.Fatal("interface address does not point to its vtable")
	}
	var pointer uintptr
	if query(object, &iidDispatch, &pointer) != ok || pointer == 0 {
		t.Fatal("IDispatch query")
	}
	if release(object) != 1 || release(object) != 0 || len(s.objects) != 0 {
		t.Fatal("COM references leaked")
	}
	for _, value := range []any{"한글", int32(42), true, float64(1.5)} {
		var variant ole.VARIANT
		if err := setVariant(s, &variant, value); err != nil {
			t.Fatal(err)
		}
		if _, err := fromVariant(variant); err != nil {
			t.Fatal(err)
		}
		ole.VariantClear(&variant)
	}
	if unsafe.Sizeof(dispatchParams{}) != unsafe.Sizeof(uintptr(0))*2+8 {
		t.Fatal("DISPPARAMS ABI")
	}
	if len(dispatchTable) != 7 || len(enumTable) != 7 || dispatchTable[6] == 0 {
		t.Fatal("vtable ABI")
	}
}
func TestRegistrationUsesOwnedTemporaryKeys(t *testing.T) {
	base := fmt.Sprintf(`Software\ArchScope\Tests\COM-%d`, time.Now().UnixNano())
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		registry.DeleteKey(registry.CURRENT_USER, base+`\CLSID`)
		registry.DeleteKey(registry.CURRENT_USER, base)
	}()
	if err = registerAt(base, exe, false); err != nil {
		t.Fatal(err)
	}
	if err = registerAt(base, exe, true); err != nil {
		t.Fatal(err)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\CLSID\`+ClassID, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	key.SetStringValue("ArchScopeOwner", "other")
	key.Close()
	defer registry.DeleteKey(registry.CURRENT_USER, base+`\CLSID\`+ClassID)
	if registerAt(base, exe, false) == nil {
		t.Fatal("foreign registration overwritten")
	}
}
