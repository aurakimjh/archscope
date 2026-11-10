//go:build windows

package comautomation

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	ok             uintptr = 0
	falseResult    uintptr = 1
	noInterface    uintptr = 0x80004002
	pointerError   uintptr = 0x80004003
	invalidArg     uintptr = 0x80070057
	notImplemented uintptr = 0x80004001
	memberNotFound uintptr = 0x80020003
	unknownName    uintptr = 0x80020006
	exception      uintptr = 0x80020009
	accessDenied   uintptr = 0x80070005
)

var ole32 = windows.NewLazySystemDLL("ole32.dll")
var user32 = windows.NewLazySystemDLL("user32.dll")
var coRegister = ole32.NewProc("CoRegisterClassObject")
var coRevoke = ole32.NewProc("CoRevokeClassObject")
var coImpersonate = ole32.NewProc("CoImpersonateClient")
var coRevert = ole32.NewProc("CoRevertToSelf")
var peekMessage = user32.NewProc("PeekMessageW")
var getMessage = user32.NewProc("GetMessageW")
var dispatchMessage = user32.NewProc("DispatchMessageW")
var postMessage = user32.NewProc("PostThreadMessageW")
var iidUnknown = mustGUID("{00000000-0000-0000-C000-000000000046}")
var iidDispatch = mustGUID("{00020400-0000-0000-C000-000000000046}")
var iidFactory = mustGUID("{00000001-0000-0000-C000-000000000046}")
var iidEnum = mustGUID("{00020404-0000-0000-C000-000000000046}")

func mustGUID(value string) windows.GUID {
	g, err := windows.GUIDFromString(value)
	if err != nil {
		panic(err)
	}
	return g
}

type message struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}
type dispatchParams struct {
	Arguments         *ole.VARIANT
	Named             *int32
	Count, NamedCount uint32
}
type exceptionInfo struct {
	Code, Reserved            uint16
	Source, Description, Help *int16
	Context                   uint32
	ReservedPointer, Fill     uintptr
	Status                    int32
}
type nativeObject struct {
	// COM dereferences the interface address as a vtable pointer. Keep this
	// field first; Go bookkeeping must never precede the native ABI prefix.
	Table           *uintptr
	container       *connectionContainer
	connectionPoint *nativeObject
	point           *pointState
	connections     *connectionEnumeration
	nameIDs         map[string]int32
	memberIDs       map[int32]string
	references      atomic.Uint32
	pin             runtime.Pinner
	server          *Server
	object          Object
	enumeration     *Enumeration
	position        int
	names           []string
	factory         bool
}

// Fail the Windows build if a future edit moves the vtable off offset zero.
var _ [0 - unsafe.Offsetof(nativeObject{}.Table)]byte

type Server struct {
	typeInfos         map[string]uintptr
	metadataDirectory string
	integrity         uint32
	elevated          bool
	mu                sync.Mutex
	objects           map[uintptr]*nativeObject
	execute           automation.Executor
	thread            uint32
	cookie            uint32
	owner             string
	done              chan struct{}
	closing           atomic.Bool
	closeOnce         sync.Once
}

var dispatchTable, factoryTable, enumTable [7]uintptr

func init() {
	dispatchTable = [7]uintptr{windows.NewCallback(query), windows.NewCallback(addRef), windows.NewCallback(release), windows.NewCallback(typeInfoCount), windows.NewCallback(typeInfo), windows.NewCallback(names), windows.NewCallback(invoke)}
	factoryTable = [7]uintptr{windows.NewCallback(query), windows.NewCallback(addRef), windows.NewCallback(release), windows.NewCallback(createInstance), windows.NewCallback(lockServer)}
	enumTable = [7]uintptr{windows.NewCallback(query), windows.NewCallback(addRef), windows.NewCallback(release), windows.NewCallback(enumNext), windows.NewCallback(enumSkip), windows.NewCallback(enumReset), windows.NewCallback(enumClone)}

	initDualTables()
}

// A process-wide table keeps callback receiver pointers alive until final Release.
var receivers sync.Map

func receiver(pointer uintptr) *nativeObject {
	value, _ := receivers.Load(pointer)
	if value == nil {
		return nil
	}
	return value.(*nativeObject)
}
func (s *Server) wrap(object Object, enumeration *Enumeration, factory bool) *nativeObject {
	n := &nativeObject{Table: &dispatchTable[0], server: s, object: object, enumeration: enumeration, factory: factory}
	if factory {
		n.Table = &factoryTable[0]
	} else if enumeration != nil {
		n.Table = &enumTable[0]
	} else {
		if table := dualTables[objectContract(object)]; len(table) > 0 {
			n.Table = &table[0]
			n.pin.Pin(&table[0])
		}
		n.names = object.Members()
		sort.Strings(n.names)
		n.nameIDs = dispatchIDs(object)
		n.memberIDs = map[int32]string{}
		for _, name := range n.names {
			n.memberIDs[n.nameIDs[strings.ToLower(name)]] = name
		}
	}
	n.references.Store(1)
	n.pin.Pin(n)
	pointer := uintptr(unsafe.Pointer(n))
	s.mu.Lock()
	s.objects[pointer] = n
	s.mu.Unlock()
	receivers.Store(pointer, n)
	return n
}
func query(object *nativeObject, iid *windows.GUID, result *uintptr) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil || iid == nil || result == nil {
		return pointerError
	}
	*result = 0
	id := *iid
	if id == iidContainer && logController(n) != nil {
		if n.container == nil {
			n.container = &connectionContainer{Table: &containerTable[0], Owner: n}
			n.pin.Pin(n.container)
		}
		n.references.Add(1)
		*result = uintptr(unsafe.Pointer(n.container))
		return ok
	}
	supported := id == iidUnknown || (!n.factory && n.enumeration == nil && id == iidDispatch) || (n.factory && id == iidFactory) || (n.enumeration != nil && id == iidEnum)
	if n.point != nil {
		supported = id == iidUnknown || id == iidPoint
	}
	if n.connections != nil {
		supported = id == iidUnknown || (!n.connections.Points && id == iidConnectionEnum) || (n.connections.Points && id == iidPointEnum)
	}
	if !supported && !n.factory && n.enumeration == nil && n.object != nil {
		supported = supportsContract(n.object, id)
	}
	if !supported {
		return noInterface
	}
	n.references.Add(1)
	*result = this
	return ok
}
func addRef(object *nativeObject) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil {
		return 0
	}
	return uintptr(n.references.Add(1))
}
func release(object *nativeObject) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil {
		return 0
	}
	remaining := n.references.Add(^uint32(0))
	if remaining == 0 {
		cleanupConnections(n)
		receivers.Delete(this)
		n.server.mu.Lock()
		delete(n.server.objects, this)
		n.server.mu.Unlock()
		n.pin.Unpin()
	}
	return uintptr(remaining)
}
func typeInfoCount(object *nativeObject, count *uint32) uintptr { return metadataCount(object, count) }
func typeInfo(object *nativeObject, index, locale uint32, result *uintptr) uintptr {
	return metadataInfo(object, index, locale, result)
}
func names(object *nativeObject, iid *windows.GUID, stringsPointer **uint16, count, locale uint32, idsPointer *int32) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil || stringsPointer == nil || idsPointer == nil || count == 0 || count > 128 {
		return invalidArg
	}
	input := unsafe.Slice(stringsPointer, int(count))
	output := unsafe.Slice(idsPointer, int(count))
	status := ok
	for i, pointer := range input {
		output[i] = -1
		key := strings.ToLower(windows.UTF16PtrToString(pointer))
		for _, member := range n.names {
			if strings.EqualFold(member, key) {
				output[i] = n.nameIDs[key]
				break
			}
		}
		if output[i] == -1 {
			status = unknownName
		}
	}
	return status
}
func (s *Server) sameUserClient() bool {
	result, _, _ := coImpersonate.Call()
	if int32(uint32(result)) < 0 {
		return false
	}
	defer coRevert.Call()
	var token windows.Token
	if windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token) != nil {
		return false
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	level, levelErr := tokenIntegrity(token)
	return err == nil && levelErr == nil && user.User.Sid.String() == s.owner && level >= s.integrity && (!s.elevated || token.IsElevated())
}
func tokenIntegrity(token windows.Token) (uint32, error) {
	buffer := make([]byte, 256)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenIntegrityLevel, &buffer[0], uint32(len(buffer)), &size); err != nil {
		return 0, err
	}
	if size < uint32(unsafe.Sizeof(windows.Tokenmandatorylabel{})) {
		return 0, fmt.Errorf("invalid token integrity label")
	}
	label := (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buffer[0]))
	if label.Label.Sid == nil {
		return 0, fmt.Errorf("missing token integrity SID")
	}
	start := uintptr(unsafe.Pointer(&buffer[0]))
	sid := uintptr(unsafe.Pointer(label.Label.Sid))
	if sid < start || sid-start > uintptr(len(buffer)-8) {
		return 0, fmt.Errorf("integrity SID is outside token buffer")
	}
	offset := int(sid - start)
	count := int(buffer[offset+1])
	if buffer[offset] != 1 || count < 1 || count > 15 || offset+8+4*count > len(buffer) {
		return 0, fmt.Errorf("invalid integrity SID")
	}
	// Read the copied SID in the original Go allocation. x/sys's native
	// subauthority helpers return interior addresses through uintptr, which
	// checkptr correctly cannot associate with this caller-owned buffer.
	value := binary.LittleEndian.Uint32(buffer[offset+8+4*(count-1):])
	runtime.KeepAlive(buffer)
	return value, nil
}

func invoke(object *nativeObject, id int32, iid *windows.GUID, locale uint32, flags uint16, paramsPointer *dispatchParams, resultPointer *ole.VARIANT, errorPointer *exceptionInfo, argumentError *uint32) (status uintptr) {
	this := uintptr(unsafe.Pointer(object))
	status = exception
	defer func() {
		if recover() != nil {
			status = exception
		}
	}()
	n := receiver(this)
	if n == nil || n.server.closing.Load() {
		return noInterface
	}
	if !n.server.sameUserClient() {
		return accessDenied
	}
	var params dispatchParams
	if paramsPointer != nil {
		params = *paramsPointer
	}
	if params.Count > 128 || (params.Count > 0 && params.Arguments == nil) || params.NamedCount > 1 {
		return invalidArg
	}
	name, exists := n.memberIDs[id]
	if !exists {
		return memberNotFound
	}
	if params.NamedCount > 0 {
		if flags&12 == 0 || params.Named == nil || *params.Named != -3 {
			return invalidArg
		}
	}
	args := make([]any, int(params.Count))
	variants := unsafe.Slice(params.Arguments, int(params.Count))
	for i, variant := range variants {
		value, err := fromVariant(variant)
		if err != nil {
			if argumentError != nil {
				*argumentError = uint32(i)
			}
			return invalidArg
		}
		args[len(args)-1-i] = value
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	value, err := n.object.Invoke(ctx, name, args, flags&12 != 0)
	if err != nil {
		if errorPointer != nil {
			info := errorPointer
			*info = exceptionInfo{Source: ole.SysAllocStringLen("ArchScope"), Description: ole.SysAllocStringLen(err.Error()), Status: -2147352567}
		}
		return exception
	}
	if resultPointer != nil {
		if err = setVariant(n.server, resultPointer, value); err != nil {
			return exception
		}
	}
	return ok
}
func fromVariant(value ole.VARIANT) (any, error) {
	switch value.VT {
	case ole.VT_EMPTY, ole.VT_NULL:
		return nil, nil
	case ole.VT_BSTR:
		return value.ToString(), nil
	case ole.VT_BOOL:
		return int16(value.Val) != 0, nil
	case ole.VT_I2:
		return int64(int16(value.Val)), nil
	case ole.VT_I4, ole.VT_INT:
		return int64(int32(value.Val)), nil
	case ole.VT_UI4, ole.VT_UINT:
		return int64(uint32(value.Val)), nil
	case ole.VT_I8:
		return value.Val, nil
	case ole.VT_R8:
		return math.Float64frombits(uint64(value.Val)), nil
	}
	return nil, fmt.Errorf("unsupported COM argument type")
}
func setVariant(server *Server, target *ole.VARIANT, value any) error {
	*target = ole.VARIANT{}
	switch v := value.(type) {
	case nil:
		return nil
	case bool:
		target.VT = ole.VT_BOOL
		if v {
			target.Val = -1
		}
	case string:
		target.VT = ole.VT_BSTR
		target.Val = int64(uintptr(unsafe.Pointer(ole.SysAllocStringLen(v))))
	case int:
		target.VT = ole.VT_I4
		target.Val = int64(v)
	case int32:
		target.VT = ole.VT_I4
		target.Val = int64(v)
	case int64:
		target.VT = ole.VT_I8
		target.Val = v
	case comDate:
		target.VT = ole.VT_DATE
		target.Val = int64(math.Float64bits(float64(v)))
	case float64:
		target.VT = ole.VT_R8
		target.Val = int64(math.Float64bits(v))
	case Object:
		n := server.wrap(v, nil, false)
		target.VT = ole.VT_DISPATCH
		target.Val = int64(uintptr(unsafe.Pointer(n)))
	case Enumeration:
		n := server.wrap(nil, &v, false)
		target.VT = ole.VT_UNKNOWN
		target.Val = int64(uintptr(unsafe.Pointer(n)))
	default:
		return fmt.Errorf("unsupported COM result type")
	}
	return nil
}
func enumNext(object *nativeObject, count uint32, output *ole.VARIANT, fetched *uint32) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil || n.enumeration == nil || count > 1024 || output == nil || (fetched == nil && count != 1) {
		return invalidArg
	}
	if !n.server.sameUserClient() {
		return accessDenied
	}
	if fetched != nil {
		*fetched = 0
	}
	values := unsafe.Slice(output, int(count))
	written := 0
	for written < int(count) && n.position < len(n.enumeration.Items) {
		if setVariant(n.server, &values[written], n.enumeration.Items[n.position]) != nil {
			return exception
		}
		written++
		n.position++
	}
	if fetched != nil {
		*fetched = uint32(written)
	}
	if written < int(count) {
		return falseResult
	}
	return ok
}
func enumSkip(object *nativeObject, count uint32) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil || n.enumeration == nil {
		return invalidArg
	}
	remaining := len(n.enumeration.Items) - n.position
	if uint64(count) > uint64(remaining) {
		n.position = len(n.enumeration.Items)
		return falseResult
	}
	n.position += int(count)
	return ok
}
func enumReset(object *nativeObject) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil {
		return invalidArg
	}
	n.position = 0
	return ok
}
func enumClone(object *nativeObject, output *uintptr) uintptr {
	this := uintptr(unsafe.Pointer(object))
	n := receiver(this)
	if n == nil || n.enumeration == nil || output == nil {
		return invalidArg
	}
	copy := n.server.wrap(nil, n.enumeration, false)
	copy.position = n.position
	*output = uintptr(unsafe.Pointer(copy))
	return ok
}
func createInstance(factory *nativeObject, outer unsafe.Pointer, iid *windows.GUID, result *uintptr) uintptr {
	this := uintptr(unsafe.Pointer(factory))
	n := receiver(this)
	if n == nil || outer != nil || result == nil {
		return invalidArg
	}
	if !n.server.sameUserClient() {
		return accessDenied
	}
	object := n.server.wrap(New(n.server.execute), nil, false)
	status := query(object, iid, result)
	release(object)
	return status
}
func lockServer(this *nativeObject, lock int32) uintptr  { return ok }
func Start(execute automation.Executor) (*Server, error) { return startWithClass(execute, ClassID) }
func startWithClass(execute automation.Executor, classID string) (*Server, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	level, err := tokenIntegrity(windows.GetCurrentProcessToken())
	if err != nil {
		return nil, err
	}
	server := &Server{integrity: level, elevated: windows.GetCurrentProcessToken().IsElevated(), execute: execute, owner: user.User.Sid.String(), objects: map[uintptr]*nativeObject{}, done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(server.done)
		err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
		if err != nil && !errors.Is(err, syscall.Errno(1)) {
			ready <- err
			return
		}
		defer func() {
			windows.CoUninitialize()
			server.mu.Lock()
			objects := server.objects
			server.objects = map[uintptr]*nativeObject{}
			server.mu.Unlock()
			for pointer := range objects {
				receivers.Delete(pointer)
			}
			for _, object := range objects {
				cleanupConnections(object)
				object.pin.Unpin()
			}
			for _, info := range server.typeInfos {
				releaseCOM(info)
			}
			server.typeInfos = nil
			if server.metadataDirectory != "" {
				_ = os.RemoveAll(server.metadataDirectory)
			}
		}()
		server.metadataDirectory, err = os.MkdirTemp("", "archscope-com-types-")
		if err != nil {
			ready <- err
			return
		}
		server.typeInfos, err = buildTypeInfosAt(filepath.Join(server.metadataDirectory, "ArchScope.Automation.tlb"))
		if err != nil {
			ready <- err
			return
		}
		server.thread = windows.GetCurrentThreadId()
		var msg message
		peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
		factory := server.wrap(nil, nil, true)
		class := mustGUID(classID)
		result, _, _ := coRegister.Call(uintptr(unsafe.Pointer(&class)), uintptr(unsafe.Pointer(factory)), 4, 1, uintptr(unsafe.Pointer(&server.cookie)))
		if int32(uint32(result)) < 0 {
			release(factory)
			ready <- fmt.Errorf("register COM class: 0x%x", result)
			return
		}
		ready <- nil
		go func() {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-server.done:
					return
				case <-ticker.C:
					if !server.closing.Load() {
						postMessage.Call(uintptr(server.thread), 0x8001, 0, 0)
					}
				}
			}
		}()
		for {
			result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if result == 0 || int32(uint32(result)) == -1 {
				break
			}
			if msg.Message == 0x8001 {
				server.publishLogEvents()
			} else {
				dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
			}
		}
		server.closing.Store(true)
		coRevoke.Call(uintptr(server.cookie))
		release(factory)

	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return server, nil
}
func (s *Server) Close() error {
	s.closeOnce.Do(func() { s.closing.Store(true); postMessage.Call(uintptr(s.thread), 0x12, 0, 0) })
	select {
	case <-s.done:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("COM shutdown is waiting for an active call")
	}
}
func Register(executable string) error   { return registerViews(executable, false) }
func Unregister(executable string) error { return registerViews(executable, true) }
func registerViews(executable string, remove bool) error {
	succeeded := false
	for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
		root, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes`, registry.READ|registry.WRITE|view)
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if err != nil {
			return err
		}
		err = registerKey(root, executable, remove)
		root.Close()
		if err != nil {
			return err
		}
		succeeded = true
	}
	if !succeeded {
		return fmt.Errorf("no supported COM registry view")
	}
	return nil
}
func registerAt(base, executable string, remove bool) error {
	root, _, err := registry.CreateKey(registry.CURRENT_USER, base, registry.READ|registry.WRITE)
	if err != nil {
		return err
	}
	defer root.Close()
	return registerKey(root, executable, remove)
}
func registerKey(root registry.Key, executable string, remove bool) error {
	if !filepath.IsAbs(executable) || !strings.EqualFold(filepath.Ext(executable), ".exe") || strings.ContainsAny(executable, "\x00\"") {
		return fmt.Errorf("choose an absolute .exe path")
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("desktop executable is unavailable")
	}
	class := `CLSID\` + ClassID
	program := ProgID
	command := `"` + executable + `" --com-server`
	programKey, programErr := registry.OpenKey(root, program+`\CLSID`, registry.QUERY_VALUE)
	if programErr == nil {
		value, _, _ := programKey.GetStringValue("")
		programKey.Close()
		if value != ClassID {
			return fmt.Errorf("ProgID is not owned by ArchScope")
		}
	} else if !errors.Is(programErr, windows.ERROR_FILE_NOT_FOUND) {
		return programErr
	}
	key, err := registry.OpenKey(root, class, registry.QUERY_VALUE)
	if err == nil {
		owner, _, _ := key.GetStringValue("ArchScopeOwner")
		key.Close()
		if owner != ClassID {
			return fmt.Errorf("COM class registration is not owned by ArchScope")
		}
	} else if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return err
	}
	if remove {
		key, err := registry.OpenKey(root, class+`\LocalServer32`, registry.QUERY_VALUE)
		if err != nil {
			return err
		}
		current, _, err := key.GetStringValue("")
		key.Close()
		if err != nil || current != command {
			return fmt.Errorf("registration points to a different executable")
		}
		for _, path := range []string{class + `\LocalServer32`, class + `\ProgID`, class, program + `\CLSID`, program} {
			if err := registry.DeleteKey(root, path); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				return err
			}
		}
		return nil
	}
	values := []struct{ path, name, value string }{{class, "ArchScopeOwner", ClassID}, {class, "", "ArchScope HTTP capture automation"}, {class + `\LocalServer32`, "", command}, {class + `\ProgID`, "", ProgID}, {program + `\CLSID`, "", ClassID}, {program, "", "ArchScope HTTP capture automation"}}
	for _, value := range values {
		key, _, err := registry.CreateKey(root, value.path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = key.SetStringValue(value.name, value.value)
		key.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
