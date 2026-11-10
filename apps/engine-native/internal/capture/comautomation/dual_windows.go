//go:build windows

package comautomation

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// Tables are process-lived because syscall callback entry points are not freed.
// Slots follow the factual safecall declaration order (HRESULT plus out retval),
// never DISPID order. The standalone table includes its inherited base prefix.
var dualTables = map[string][]uintptr{}

func initDualTables() {
	for name := range referenceContracts.Interfaces {
		methods := interfaceMethods(name)
		if len(methods) == 0 {
			continue
		}
		table := append([]uintptr(nil), dispatchTable[:]...)
		for _, method := range methods {
			m := method
			argc := len(m.Params)
			if m.Returns != "void" {
				argc++
			}
			switch argc {
			case 0:
				table = append(table, windows.NewCallback(func(n *nativeObject) uintptr { return callDual(n, m, nil) }))
			case 1:
				table = append(table, windows.NewCallback(func(n *nativeObject, a uintptr) uintptr { return callDual(n, m, []uintptr{a}) }))
			case 2:
				table = append(table, windows.NewCallback(func(n *nativeObject, a, b uintptr) uintptr { return callDual(n, m, []uintptr{a, b}) }))
			case 3:
				table = append(table, windows.NewCallback(func(n *nativeObject, a, b, c uintptr) uintptr { return callDual(n, m, []uintptr{a, b, c}) }))
			default:
				panic("unsupported automation ABI arity")
			}
		}
		dualTables[name] = table
	}
}
func objectContract(object Object) string {
	if t, ok := object.(interface{ Contract() string }); ok {
		return t.Contract()
	}
	return ""
}
func supportsContract(object Object, id windows.GUID) bool {
	name := objectContract(object)
	for name != "" && name != "IDispatch" {
		contract, ok := referenceContracts.Interfaces[name]
		if !ok {
			return false
		}
		if mustGUID(contract.IID) == id {
			return true
		}
		name = contract.Base
	}
	return false
}
func callDual(object *nativeObject, m abiMethod, words []uintptr) (status uintptr) {
	status = exception
	defer func() {
		if recover() != nil {
			status = exception
		}
	}()
	n := receiver(uintptr(unsafe.Pointer(object)))
	if n == nil || n.server.closing.Load() {
		return noInterface
	}
	if !n.server.sameUserClient() {
		return accessDenied
	}
	var out uintptr
	if m.Returns != "void" {
		out = words[len(words)-1]
		if out == 0 {
			return pointerError
		}
		clearDualResult(m.Returns, out)
	}
	args := make([]any, len(m.Params))
	for i, param := range m.Params {
		switch param.Type {
		case "Integer", "LogType":
			args[i] = int64(int32(words[i]))
		case "WordBool":
			args[i] = int16(words[i]) != 0
		case "WideString":
			args[i] = ole.BstrToString((*uint16)(foreignPointer(words[i])))
		default:
			other := receiver(words[i])
			if other == nil || other.object == nil {
				return invalidArg
			}
			args[i] = other.object
		}
	}
	value, err := n.object.Invoke(context.Background(), m.Name, args, m.Kind == "put")
	if err != nil {
		return exception
	}
	if out != 0 {
		if err = writeDualResult(n.server, m.Returns, out, value); err != nil {
			return exception
		}
	}
	runtime.KeepAlive(object)
	return ok
}
func clearDualResult(kind string, out uintptr) {
	switch kind {
	case "WordBool", "Word":
		*(*uint16)(foreignPointer(out)) = 0
	case "Integer", "LogType":
		*(*int32)(foreignPointer(out)) = 0
	case "Double", "TDateTime":
		*(*uint64)(foreignPointer(out)) = 0
	case "OleVariant":
		*(*ole.VARIANT)(foreignPointer(out)) = ole.VARIANT{}
	default:
		*(*uintptr)(foreignPointer(out)) = 0
	}
}
func writeDualResult(s *Server, kind string, out uintptr, value any) error {
	switch kind {
	case "void":
		return nil
	case "WideString":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		*(*uintptr)(foreignPointer(out)) = uintptr(unsafe.Pointer(ole.SysAllocStringLen(v)))
	case "WordBool":
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("expected boolean")
		}
		if v {
			*(*int16)(foreignPointer(out)) = -1
		}
	case "Integer", "LogType", "Word":
		v, e := integer(value)
		if e != nil {
			return e
		}
		if kind == "Word" {
			if v < 0 || v > 65535 {
				return fmt.Errorf("word overflow")
			}
			*(*uint16)(foreignPointer(out)) = uint16(v)
		} else {
			if v < math.MinInt32 || v > math.MaxInt32 {
				return fmt.Errorf("integer overflow")
			}
			*(*int32)(foreignPointer(out)) = int32(v)
		}
	case "Double", "TDateTime":
		var v float64
		switch x := value.(type) {
		case float64:
			v = x
		case comDate:
			v = float64(x)
		case int:
			v = float64(x)
		case int32:
			v = float64(x)
		case int64:
			v = float64(x)
		default:
			return fmt.Errorf("expected floating value")
		}
		*(*float64)(foreignPointer(out)) = v
	case "OleVariant":
		return setVariant(s, (*ole.VARIANT)(foreignPointer(out)), value)
	default:
		if kind == "IUnknown" {
			if e, ok := value.(Enumeration); ok {
				n := s.wrap(nil, &e, false)
				*(*uintptr)(foreignPointer(out)) = uintptr(unsafe.Pointer(n))
				return nil
			}
		}
		v, ok := value.(Object)
		if !ok || !strings.HasPrefix(kind, "I") {
			return fmt.Errorf("expected interface %s", kind)
		}
		if objectContract(v) != kind {
			return fmt.Errorf("interface result mismatch: %s", kind)
		}
		n := s.wrap(v, nil, false)
		*(*uintptr)(foreignPointer(out)) = uintptr(unsafe.Pointer(n))
	}
	return nil
}

// foreignPointer restores a pointer-sized word delivered by the Windows COM
// ABI. These addresses refer to COM-owned call buffers/interfaces, not Go
// heap offsets. Keep the conversion at this FFI boundary; callers must not
// retain a call buffer after returning to COM.
func foreignPointer(word uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&word))
}
