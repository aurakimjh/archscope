//go:build windows

package comautomation

import (
	"context"
	"runtime"
	"sort"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var iidContainer = mustGUID("{B196B284-BAB4-101A-B69C-00AA00341D07}")
var iidPoint = mustGUID("{B196B286-BAB4-101A-B69C-00AA00341D07}")
var iidPointEnum = mustGUID("{B196B285-BAB4-101A-B69C-00AA00341D07}")
var iidConnectionEnum = mustGUID("{B196B287-BAB4-101A-B69C-00AA00341D07}")
var iidLogEvents = mustGUID("{B2077F52-4E22-4A6E-9055-1BE6DE830661}")

type connectionContainer struct {
	Table *uintptr
	Owner *nativeObject
}
type connectionData struct {
	Unknown uintptr
	Cookie  uint32
}
type pointState struct {
	Owner  *nativeObject
	Next   uint32
	Sinks  map[uint32]uintptr
	Cursor eventCursor
}
type connectionEnumeration struct {
	Points   bool
	Items    []connectionData
	Position int
}

func containerQuery(c *connectionContainer, iid *windows.GUID, out *uintptr) uintptr {
	return query(c.Owner, iid, out)
}
func containerAddRef(c *connectionContainer) uintptr  { return addRef(c.Owner) }
func containerRelease(c *connectionContainer) uintptr { return release(c.Owner) }
func logController(n *nativeObject) *Controller {
	if t, ok := n.object.(Typed); ok {
		if l, ok := t.Object.(liveLog); ok {
			return l.controller
		}
	}
	return nil
}
func getPoint(owner *nativeObject) *nativeObject {
	if owner.connectionPoint != nil && receiver(uintptr(unsafe.Pointer(owner.connectionPoint))) != nil {
		addRef(owner.connectionPoint)
		return owner.connectionPoint
	}
	n := owner.server.wrap(nil, &Enumeration{}, false)
	n.enumeration = nil
	n.Table = &pointTable[0]
	n.point = &pointState{Owner: owner, Sinks: map[uint32]uintptr{}}
	owner.connectionPoint = n
	addRef(owner)
	return n
}
func findPoint(c *connectionContainer, iid *windows.GUID, out *uintptr) uintptr {
	if out == nil || iid == nil {
		return pointerError
	}
	*out = 0
	if !c.Owner.server.sameUserClient() {
		return accessDenied
	}
	if *iid != iidLogEvents {
		return 0x80040200
	}
	n := getPoint(c.Owner)
	*out = uintptr(unsafe.Pointer(n))
	return ok
}
func enumPoints(c *connectionContainer, out *uintptr) uintptr {
	if out == nil {
		return pointerError
	}
	*out = 0
	if !c.Owner.server.sameUserClient() {
		return accessDenied
	}
	p := getPoint(c.Owner)
	n := newConnectionEnum(c.Owner.server, true, []connectionData{{Unknown: uintptr(unsafe.Pointer(p))}})
	release(p)
	*out = uintptr(unsafe.Pointer(n))
	return ok
}
func pointInterface(n *nativeObject, out *windows.GUID) uintptr {
	if n.point == nil || out == nil {
		return pointerError
	}
	*out = iidLogEvents
	return ok
}
func pointContainer(n *nativeObject, out *uintptr) uintptr {
	if n.point == nil || out == nil {
		return pointerError
	}
	return query(n.point.Owner, &iidContainer, out)
}
func pointAdvise(n *nativeObject, sink unsafe.Pointer, cookie *uint32) uintptr {
	if cookie == nil || sink == nil || n.point == nil {
		return pointerError
	}
	*cookie = 0
	if !n.server.sameUserClient() {
		return accessDenied
	}
	if len(n.point.Sinks) >= 16 {
		return 0x80040201
	}
	value, e := queryCOM(uintptr(sink), iidLogEvents)
	if e != nil {
		return 0x80040202
	}
	controller := logController(n.point.Owner)
	if len(n.point.Sinks) == 0 {
		if _, e = controller.pollEvents(context.Background(), &n.point.Cursor, true); e != nil {
			releaseCOM(value)
			return exception
		}
	}
	n.point.Next++
	if n.point.Next == 0 {
		n.point.Next++
	}
	n.point.Sinks[n.point.Next] = value
	*cookie = n.point.Next
	return ok
}
func pointUnadvise(n *nativeObject, cookie uint32) uintptr {
	if n.point == nil {
		return noInterface
	}
	if !n.server.sameUserClient() {
		return accessDenied
	}
	value, ok := n.point.Sinks[cookie]
	if !ok {
		return 0x80040200
	}
	delete(n.point.Sinks, cookie)
	releaseCOM(value)
	return 0
}
func newConnectionEnum(s *Server, points bool, items []connectionData) *nativeObject {
	n := s.wrap(nil, &Enumeration{}, false)
	n.enumeration = nil
	n.Table = &pointEnumTable[0]
	n.connections = &connectionEnumeration{Points: points, Items: append([]connectionData(nil), items...)}
	for _, item := range items {
		_ = comCall(item.Unknown, 1)
	}
	return n
}
func enumConnections(n *nativeObject, out *uintptr) uintptr {
	if out == nil || n.point == nil {
		return pointerError
	}
	*out = 0
	if !n.server.sameUserClient() {
		return accessDenied
	}
	keys := make([]int, 0, len(n.point.Sinks))
	for k := range n.point.Sinks {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	items := make([]connectionData, 0, len(keys))
	for _, k := range keys {
		items = append(items, connectionData{n.point.Sinks[uint32(k)], uint32(k)})
	}
	v := newConnectionEnum(n.server, false, items)
	*out = uintptr(unsafe.Pointer(v))
	return ok
}
func connectionNext(n *nativeObject, count uint32, out unsafe.Pointer, fetched *uint32) uintptr {
	if n.connections == nil || out == nil || count > 1024 || (fetched == nil && count != 1) {
		return invalidArg
	}
	if !n.server.sameUserClient() {
		return accessDenied
	}
	if fetched != nil {
		*fetched = 0
	}
	written := uint32(0)
	for written < count && n.connections.Position < len(n.connections.Items) {
		item := n.connections.Items[n.connections.Position]
		_ = comCall(item.Unknown, 1)
		if n.connections.Points {
			unsafe.Slice((*uintptr)(out), int(count))[written] = item.Unknown
		} else {
			unsafe.Slice((*connectionData)(out), int(count))[written] = item
		}
		n.connections.Position++
		written++
	}
	if fetched != nil {
		*fetched = written
	}
	if written < count {
		return falseResult
	}
	return ok
}
func connectionSkip(n *nativeObject, count uint32) uintptr {
	if n.connections == nil {
		return invalidArg
	}
	remaining := len(n.connections.Items) - n.connections.Position
	if uint64(count) > uint64(remaining) {
		n.connections.Position = len(n.connections.Items)
		return falseResult
	}
	n.connections.Position += int(count)
	return ok
}
func connectionReset(n *nativeObject) uintptr {
	if n.connections == nil {
		return invalidArg
	}
	n.connections.Position = 0
	return ok
}
func connectionClone(n *nativeObject, out *uintptr) uintptr {
	if n.connections == nil || out == nil {
		return pointerError
	}
	v := newConnectionEnum(n.server, n.connections.Points, n.connections.Items)
	v.connections.Position = n.connections.Position
	*out = uintptr(unsafe.Pointer(v))
	return ok
}

// Called on the server STA after its posted poll message. Never invoke a client
// from a capture worker or while a controller/server lock is held.
func (s *Server) publishLogEvents() {
	s.mu.Lock()
	points := []*nativeObject{}
	for _, n := range s.objects {
		if n.point != nil && len(n.point.Sinks) > 0 {
			addRef(n)
			points = append(points, n)
		}
	}
	s.mu.Unlock()
	for _, n := range points {
		controller := logController(n.point.Owner)
		events, e := controller.pollEvents(context.Background(), &n.point.Cursor, false)
		if e == nil {
			source := n.point.Cursor.Source
			for _, event := range events {
				keys := make([]uint32, 0, len(n.point.Sinks))
				for k := range n.point.Sinks {
					keys = append(keys, k)
				}
				discarded := false
				for _, k := range keys {
					sink := n.point.Sinks[k]
					if sink == 0 {
						continue
					}
					_ = comCall(sink, 1)
					discard, e := sendLogEvent(s, sink, event)
					releaseCOM(sink)
					if e == nil && discard {
						discarded = true
					}
				}
				if discarded {
					controller.discardEntry(event.ID, source)
				}
			}
		}
		release(n)
	}
}
func sendLogEvent(s *Server, sink uintptr, event logEvent) (bool, error) {
	var entry ole.VARIANT
	if e := setVariant(s, &entry, event.Entry); e != nil {
		return false, e
	}
	defer ole.VariantClear(&entry)
	discard := int16(0)
	var pin runtime.Pinner
	pin.Pin(&discard)
	defer pin.Unpin()
	args := []ole.VARIANT{entry}
	id := uintptr(202)
	if event.New {
		id = 201
		args = []ole.VARIANT{{VT: ole.VT_BOOL | ole.VT_BYREF, Val: int64(uintptr(unsafe.Pointer(&discard)))}, entry}
	}
	params := dispatchParams{Arguments: &args[0], Count: uint32(len(args))}
	table := *(*uintptr)(foreignPointer(sink))
	method := *(*uintptr)(unsafe.Add(foreignPointer(table), 6*unsafe.Sizeof(uintptr(0))))
	// IDispatch::Invoke takes the outgoing DISPID and reversed VARIANT args.
	var empty windows.GUID
	hr, _, _ := syscall.SyscallN(method, sink, id, uintptr(unsafe.Pointer(&empty)), 0, 1, uintptr(unsafe.Pointer(&params)), 0, 0, 0)
	runtime.KeepAlive(args)
	runtime.KeepAlive(discard)
	if int32(uint32(hr)) < 0 {
		return false, syscall.Errno(hr)
	}
	return discard != 0, nil
}

func cleanupConnections(n *nativeObject) {
	if n.point != nil {
		for k, p := range n.point.Sinks {
			delete(n.point.Sinks, k)
			releaseCOM(p)
		}
		owner := n.point.Owner
		n.point = nil
		if owner.connectionPoint == n {
			owner.connectionPoint = nil
		}
		release(owner)
	}
	if n.connections != nil {
		for _, item := range n.connections.Items {
			releaseCOM(item.Unknown)
		}
		n.connections = nil
	}
}

var containerTable [5]uintptr
var pointTable [8]uintptr
var pointEnumTable [7]uintptr

func init() {
	containerTable = [5]uintptr{windows.NewCallback(containerQuery), windows.NewCallback(containerAddRef), windows.NewCallback(containerRelease), windows.NewCallback(enumPoints), windows.NewCallback(findPoint)}
	pointTable = [8]uintptr{windows.NewCallback(query), windows.NewCallback(addRef), windows.NewCallback(release), windows.NewCallback(pointInterface), windows.NewCallback(pointContainer), windows.NewCallback(pointAdvise), windows.NewCallback(pointUnadvise), windows.NewCallback(enumConnections)}
	pointEnumTable = [7]uintptr{windows.NewCallback(query), windows.NewCallback(addRef), windows.NewCallback(release), windows.NewCallback(connectionNext), windows.NewCallback(connectionSkip), windows.NewCallback(connectionReset), windows.NewCallback(connectionClone)}
}
