//go:build windows

package comautomation

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Build real dual type descriptions, with HRESULT returns and [out, retval]
// parameters. CreateDispTypeInfo's simplified direct-return model would describe
// the safecall vtables incorrectly. No vendor typelib is loaded or registered.
type typeDescription struct {
	Value uintptr
	VT    uint16
}
type parameterDescription struct {
	Extra uintptr
	Flags uint16
}
type elementDescription struct {
	Type  typeDescription
	Param parameterDescription
}
type functionDescription struct {
	ID                                  int32
	Scodes                              uintptr
	Params                              *elementDescription
	Kind, InvokeKind, CallConvention    int32
	Count, Optional, Offset, ScodeCount int16
	Result                              elementDescription
	Flags                               uint16
}

var iidTypeInfo = mustGUID("{00020401-0000-0000-C000-000000000046}")

func comCall(pointer uintptr, slot int, args ...uintptr) error {
	if pointer == 0 {
		return fmt.Errorf("nil COM metadata interface")
	}
	table := *(*uintptr)(foreignPointer(pointer))
	method := *(*uintptr)(unsafe.Add(foreignPointer(table), uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	words := append([]uintptr{pointer}, args...)
	hr, _, _ := syscall.SyscallN(method, words...)
	if int32(uint32(hr)) < 0 {
		return fmt.Errorf("COM metadata slot %d: 0x%08x", slot, uint32(hr))
	}
	return nil
}
func releaseCOM(pointer uintptr) {
	if pointer != 0 {
		_ = comCall(pointer, 2)
	}
}
func queryCOM(pointer uintptr, id windows.GUID) (uintptr, error) {
	var out uintptr
	e := comCall(pointer, 0, uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&out)))
	return out, e
}

func buildTypeInfos() (map[string]uintptr, error) {
	// The library is in memory; SaveAllChanges and global typelib registration
	// are deliberately not called. ITypeInfo references own their lifetimes.
	var library, standard uintptr
	var infos = map[string]uintptr{}
	var creators = map[string]uintptr{}
	success := false
	defer func() {
		for _, p := range creators {
			releaseCOM(p)
		}
		releaseCOM(library)
		releaseCOM(standard)
		if !success {
			for _, p := range infos {
				releaseCOM(p)
			}
		}
	}()
	name, _ := windows.UTF16PtrFromString("ArchScope.Automation.tlb")
	syskind := uintptr(3)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		syskind = 0
	}
	hr, _, _ := windows.NewLazySystemDLL("oleaut32.dll").NewProc("CreateTypeLib2").Call(syskind, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&library)))
	if int32(uint32(hr)) < 0 {
		return nil, fmt.Errorf("create automation type library: 0x%x", hr)
	}
	libID := mustGUID("{02AAEB6C-A963-4D5D-9CFA-A2DBB6E5E233}")
	if e := comCall(library, 6, uintptr(unsafe.Pointer(&libID))); e != nil {
		return nil, e
	}
	if e := comCall(library, 5, 1, 0); e != nil {
		return nil, e
	}
	label, _ := windows.UTF16PtrFromString("ArchScopeAutomation")
	if e := comCall(library, 4, uintptr(unsafe.Pointer(label))); e != nil {
		return nil, e
	}
	stdID := mustGUID("{00020430-0000-0000-C000-000000000046}")
	hr, _, _ = windows.NewLazySystemDLL("oleaut32.dll").NewProc("LoadRegTypeLib").Call(uintptr(unsafe.Pointer(&stdID)), 2, 0, 0, uintptr(unsafe.Pointer(&standard)))
	if int32(uint32(hr)) < 0 {
		return nil, fmt.Errorf("load standard automation types: 0x%x", hr)
	}
	var dispatchInfo uintptr
	if e := comCall(standard, 6, uintptr(unsafe.Pointer(&iidDispatch)), uintptr(unsafe.Pointer(&dispatchInfo))); e != nil {
		return nil, e
	}
	defer releaseCOM(dispatchInfo)
	keys := make([]string, 0, len(referenceContracts.Interfaces))
	for n := range referenceContracts.Interfaces {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	for _, n := range keys {
		if len(interfaceMethods(n)) == 0 {
			continue
		}
		text, _ := windows.UTF16PtrFromString(n)
		var creator uintptr
		if e := comCall(library, 3, uintptr(unsafe.Pointer(text)), 3, uintptr(unsafe.Pointer(&creator))); e != nil {
			return nil, e
		} // TKIND_INTERFACE
		creators[n] = creator
		guid := mustGUID(referenceContracts.Interfaces[n].IID)
		if e := comCall(creator, 3, uintptr(unsafe.Pointer(&guid))); e != nil {
			return nil, e
		}
		if e := comCall(creator, 4, 0x1140); e != nil {
			return nil, e
		} // dual, oleautomation, dispatchable
		info, e := queryCOM(creator, iidTypeInfo)
		if e != nil {
			return nil, e
		}
		infos[n] = info
	}
	for _, n := range keys {
		creator := creators[n]
		if creator == 0 {
			continue
		}
		var href uint32
		if e := comCall(creator, 8, dispatchInfo, uintptr(unsafe.Pointer(&href))); e != nil {
			return nil, e
		}
		if e := comCall(creator, 10, 0, uintptr(href)); e != nil {
			return nil, e
		}
		for index, method := range interfaceMethods(n) {
			if e := addTypeMethod(creator, n, index, method, infos); e != nil {
				return nil, fmt.Errorf("%s.%s: %w", n, method.Name, e)
			}
		}
	}
	for _, n := range keys {
		if creator := creators[n]; creator != 0 {
			if e := comCall(creator, 25); e != nil {
				return nil, fmt.Errorf("layout %s: %w", n, e)
			}
		}
	}
	success = true
	return infos, nil
}
func addTypeMethod(creator uintptr, contract string, index int, m abiMethod, infos map[string]uintptr) error {
	// Pointer descriptors stay alive until AddFuncDesc has copied their graphs.
	var keep []*typeDescription
	ptr := func(t typeDescription) typeDescription {
		p := new(typeDescription)
		*p = t
		keep = append(keep, p)
		return typeDescription{Value: uintptr(unsafe.Pointer(p)), VT: 26}
	}
	dataType := func(kind string) (typeDescription, error) {
		switch kind {
		case "Integer", "LogType":
			return typeDescription{VT: 3}, nil
		case "WordBool":
			return typeDescription{VT: 11}, nil
		case "Word":
			return typeDescription{VT: 18}, nil
		case "WideString":
			return typeDescription{VT: 8}, nil
		case "Double":
			return typeDescription{VT: 5}, nil
		case "TDateTime":
			return typeDescription{VT: 7}, nil
		case "OleVariant":
			return typeDescription{VT: 12}, nil
		case "IUnknown":
			return typeDescription{VT: 13}, nil
		}
		var href uint32
		info := infos[kind]
		if info == 0 {
			return typeDescription{}, fmt.Errorf("unknown automation type %s", kind)
		}
		if e := comCall(creator, 8, info, uintptr(unsafe.Pointer(&href))); e != nil {
			return typeDescription{}, e
		}
		return ptr(typeDescription{Value: uintptr(href), VT: 29}), nil
	}
	params := make([]elementDescription, 0, len(m.Params)+1)
	for _, p := range m.Params {
		d, e := dataType(p.Type)
		if e != nil {
			return e
		}
		params = append(params, elementDescription{Type: d, Param: parameterDescription{Flags: 1}})
	}
	if m.Returns != "void" {
		d, e := dataType(m.Returns)
		if e != nil {
			return e
		}
		params = append(params, elementDescription{Type: ptr(d), Param: parameterDescription{Flags: 10}})
	}
	id, ok := referenceContracts.Interfaces[contract].IDs[m.Name]
	if !ok && contract == "IHTTPAnalyzerStandAlone" {
		id, ok = dispatchIDs(New(nil))[strings.ToLower(m.Name)]
	}
	if !ok {
		return fmt.Errorf("missing dispatch identifier")
	}
	invokeKind := int32(1)
	if m.Kind == "get" {
		invokeKind = 2
	}
	if m.Kind == "put" {
		invokeKind = 4
	}
	fd := functionDescription{ID: id, Kind: 1, InvokeKind: invokeKind, CallConvention: 4, Count: int16(len(params)), Offset: int16((7 + index) * int(unsafe.Sizeof(uintptr(0)))), Result: elementDescription{Type: typeDescription{VT: 25}}}
	if len(params) > 0 {
		fd.Params = &params[0]
	}
	if e := comCall(creator, 9, uintptr(index), uintptr(unsafe.Pointer(&fd))); e != nil {
		return e
	}
	paramNames := []string{m.Name}
	for _, p := range m.Params {
		paramNames = append(paramNames, p.Name)
	}
	// SetFuncAndParamNames excludes the return-value parameter.
	names := make([]*uint16, len(paramNames))
	for i, n := range paramNames {
		names[i], _ = windows.UTF16PtrFromString(n)
	}
	e := comCall(creator, 15, uintptr(index), uintptr(unsafe.Pointer(&names[0])), uintptr(len(names)))
	runtime.KeepAlive(params)
	runtime.KeepAlive(keep)
	runtime.KeepAlive(names)
	return e
}

func metadataFor(n *nativeObject) uintptr { return n.server.typeInfos[objectContract(n.object)] }
func metadataCount(object *nativeObject, count *uint32) uintptr {
	n := receiver(uintptr(unsafe.Pointer(object)))
	if n == nil || count == nil {
		return pointerError
	}
	*count = 0
	if metadataFor(n) != 0 {
		*count = 1
	}
	return ok
}
func metadataInfo(object *nativeObject, index, locale uint32, result *uintptr) uintptr {
	if result == nil {
		return pointerError
	}
	*result = 0
	n := receiver(uintptr(unsafe.Pointer(object)))
	if n == nil {
		return pointerError
	}
	if index != 0 {
		return 0x8002000B
	}
	info := metadataFor(n)
	if info == 0 {
		return notImplemented
	}
	if e := comCall(info, 1); e != nil {
		return exception
	}
	*result = info
	return ok
}
