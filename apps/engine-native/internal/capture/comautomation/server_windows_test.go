//go:build windows

package comautomation

import (
	"fmt"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"runtime"
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

func TestConnectionPointIdentityAndLifetime(t *testing.T) {
	s := &Server{objects: map[uintptr]*nativeObject{}}
	controller := New(nil)
	owner := s.wrap(Typed{"ILog", liveLog{controller}}, nil, false)
	var address uintptr
	if query(owner, &iidContainer, &address) != ok || address == 0 {
		t.Fatal("connection container missing")
	}
	c := (*connectionContainer)(foreignPointer(address))
	var unknown uintptr
	if containerQuery(c, &iidUnknown, &unknown) != ok || unknown != uintptr(unsafe.Pointer(owner)) {
		t.Fatal("controlling unknown changed")
	}
	release(owner)
	point := getPoint(owner)
	var outgoing windows.GUID
	if pointInterface(point, &outgoing) != ok || outgoing != iidLogEvents {
		t.Fatal(outgoing)
	}
	enum := newConnectionEnum(s, true, []connectionData{{Unknown: uintptr(unsafe.Pointer(point))}})
	if query(enum, &iidPointEnum, &address) != ok {
		t.Fatal("point enumeration IID")
	}
	release(enum)
	var clone uintptr
	if connectionClone(enum, &clone) != ok {
		t.Fatal("clone")
	}
	release(receiver(clone))
	release(enum)
	if containerRelease(c) != 2 {
		t.Fatal("container reference")
	}
	release(point)
	release(owner)
	if len(s.objects) != 0 {
		t.Fatal("connection objects leaked", len(s.objects))
	}
}

type testEventSink struct {
	Table        *uintptr
	New, Updates int
}

var testEventTable [7]uintptr

func TestNativeEventVariantOrderAndByRefCancel(t *testing.T) {
	testEventTable[6] = windows.NewCallback(func(sink *testEventSink, id int32, iid *windows.GUID, locale uint32, flags uint16, params *dispatchParams, result *ole.VARIANT, info *exceptionInfo, arg *uint32) uintptr {
		values := unsafe.Slice(params.Arguments, int(params.Count))
		if id == 201 {
			if len(values) != 2 || values[0].VT != ole.VT_BOOL|ole.VT_BYREF || values[1].VT != ole.VT_DISPATCH {
				return invalidArg
			}
			*(*int16)(foreignPointer(uintptr(values[0].Val))) = -1
			sink.New++
		} else if id == 202 {
			if len(values) != 1 || values[0].VT != ole.VT_DISPATCH {
				return invalidArg
			}
			sink.Updates++
		} else {
			return memberNotFound
		}
		return ok
	})
	sink := &testEventSink{Table: &testEventTable[0]}
	var pin runtime.Pinner
	pin.Pin(sink)
	defer pin.Unpin()
	s := &Server{objects: map[uintptr]*nativeObject{}}
	event := logEvent{ID: "one", New: true, Entry: Typed{"ILogEntry", Properties{"URL": "https://example.test"}}}
	discard, e := sendLogEvent(s, uintptr(unsafe.Pointer(sink)), event)
	if e != nil || !discard || sink.New != 1 {
		t.Fatal(discard, e, sink.New)
	}
	event.New = false
	discard, e = sendLogEvent(s, uintptr(unsafe.Pointer(sink)), event)
	if e != nil || discard || sink.Updates != 1 || len(s.objects) != 0 {
		t.Fatal(discard, e, sink.Updates, len(s.objects))
	}
}

func TestNativeDualPrefixAndTypeInformation(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if e := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); e != nil {
		t.Fatal(e)
	}
	defer windows.CoUninitialize()
	infos, e := buildTypeInfos()
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		for _, info := range infos {
			releaseCOM(info)
		}
	}()
	s := &Server{objects: map[uintptr]*nativeObject{}, typeInfos: infos}
	n := s.wrap(New(nil), nil, false)
	defer release(n)
	for _, name := range []string{"IHTTPAnalyzerStandAlone", "IHTTPAnalyzer"} {
		id := mustGUID(referenceContracts.Interfaces[name].IID)
		var p uintptr
		if query(n, &id, &p) != ok || p != uintptr(unsafe.Pointer(n)) {
			t.Fatal(name)
		}
		release(n)
	}
	base, child := dualTables["IHTTPAnalyzer"], dualTables["IHTTPAnalyzerStandAlone"]
	if len(base) != 27 || len(child) != 47 {
		t.Fatal("incorrect inherited vtable size", len(base), len(child))
	}
	var count uint32
	if typeInfoCount(n, &count) != ok || count != 1 {
		t.Fatal("missing type info")
	}
	var info uintptr
	if typeInfo(n, 0, 0, &info) != ok || info == 0 {
		t.Fatal("GetTypeInfo")
	}
	defer releaseCOM(info)
	var attr *ole.TYPEATTR
	if e = comCall(info, 3, uintptr(unsafe.Pointer(&attr))); e != nil {
		t.Fatal(e)
	}
	if attr == nil || attr.WTypeFlags&0x40 == 0 {
		t.Fatal("not a dual interface")
	}
	comCall(info, 19, uintptr(unsafe.Pointer(attr)))
	var fd *functionDescription
	// SaveLog is the seventh custom method: HRESULT (BSTR, enum), no retval.
	if e = comCall(info, 5, 6, uintptr(unsafe.Pointer(&fd))); e != nil {
		t.Fatal(e)
	}
	if fd.ID != 212 || fd.Count != 2 || fd.Result.Type.VT != 25 {
		t.Fatal("incorrect SaveLog ABI", fd)
	}
	comCall(info, 20, uintptr(unsafe.Pointer(fd)))
	var bad uintptr
	if typeInfo(n, 1, 0, &bad) != 0x8002000B || bad != 0 {
		t.Fatal("bad type index accepted")
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
