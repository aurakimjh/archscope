//go:build windows

package comautomation

import (
	"context"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/admission"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

type admissionTestSink struct {
	Table   *uintptr
	action  func()
	discard bool
}

var admissionTestTable [7]uintptr

func init() {
	admissionTestTable[1] = windows.NewCallback(func(*admissionTestSink) uintptr { return 2 })
	admissionTestTable[2] = windows.NewCallback(func(*admissionTestSink) uintptr { return 1 })
	admissionTestTable[6] = windows.NewCallback(func(sink *admissionTestSink, id int32, _ *windows.GUID, _ uint32, _ uint16, params *dispatchParams, _ *ole.VARIANT, _ *exceptionInfo, _ *uint32) uintptr {
		if id != 201 {
			return invalidArg
		}
		if sink.action != nil {
			sink.action()
		}
		if sink.discard {
			*(*int16)(foreignPointer(uintptr(params.Arguments.Val))) = -1
		}
		return ok
	})
}

func TestAdmissionSTARejectsDiscardAndReentrantStop(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "discard", true: "reentrant-stop"}[stop], func(t *testing.T) {
			h := admission.New()
			s := &Server{hub: h, objects: map[uintptr]*nativeObject{}, admissionRequests: make(chan *admissionRequest, 64), done: make(chan struct{})}
			c, e := s.newController()
			if e != nil {
				t.Fatal(e)
			}
			defer c.admissionRelease()
			c.sessionID = "s"
			c.listeners.Store(1)
			h.Begin("s", c.admissionOwner, 0)
			owner := s.wrap(Typed{"ILog", liveLog{c}}, nil, false)
			point := getPoint(owner)
			sink := &admissionTestSink{Table: &admissionTestTable[0], discard: !stop}
			if stop {
				sink.action = func() { h.End("s") }
			}
			var pin runtime.Pinner
			pin.Pin(sink)
			defer pin.Unpin()
			point.point.Sinks[1] = uintptr(unsafe.Pointer(sink))
			result := make(chan bool, 1)
			go func() {
				ok, _ := h.Decide(context.Background(), "s", models.CaptureTransaction{ID: "synthetic"})
				result <- ok
			}()
			deadline := time.Now().Add(time.Second)
			for len(s.admissionRequests) == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			s.serveAdmission()
			select {
			case ok := <-result:
				if ok {
					t.Fatal("discard or reentrant cancellation admitted")
				}
			case <-time.After(time.Second):
				t.Fatal("STA admission deadlock")
			}
			release(point)
			release(owner)
			if len(s.objects) != 0 {
				t.Fatal("native references leaked")
			}
		})
	}
}
