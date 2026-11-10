//go:build windows

package comautomation

import (
	"context"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/admission"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type admissionRequest struct {
	controller *Controller
	session    string
	tx         models.CaptureTransaction
	ctx        context.Context
	answer     chan admissionAnswer
}
type admissionAnswer struct {
	allowed bool
	err     error
}

func setAdmissionHub(s *Server, hub *admission.Hub) { s.hub = hub }
func nativeController(object Object) *Controller {
	if c, ok := object.(*Controller); ok {
		return c
	}
	if t, ok := object.(Typed); ok {
		if l, ok := t.Object.(liveLog); ok {
			return l.controller
		}
	}
	return nil
}
func (s *Server) newController() (*Controller, error) {
	c := New(s.execute)
	if s.hub == nil {
		return c, nil
	}
	key, release, err := s.hub.RegisterActive(func(ctx context.Context, session string, tx models.CaptureTransaction) (bool, error) {
		req := &admissionRequest{controller: c, session: session, tx: tx, ctx: ctx, answer: make(chan admissionAnswer, 1)}
		select {
		case s.admissionRequests <- req:
		default:
			return false, fmt.Errorf("admission queue is full")
		}
		postMessage.Call(uintptr(s.thread), 0x8002, 0, 0)
		select {
		case result := <-req.answer:
			return result.allowed, result.err
		case <-ctx.Done():
			return false, ctx.Err()
		case <-s.done:
			return false, context.Canceled
		}
	}, func() bool { return c.listeners.Load() > 0 })
	if err != nil {
		return nil, err
	}
	c.admissionOwner = key
	c.admissionRelease = release
	c.admissionHub = s.hub
	return c, nil
}
func (s *Server) serveAdmission() {
	for {
		var req *admissionRequest
		select {
		case req = <-s.admissionRequests:
		default:
			return
		}
		if req.ctx.Err() != nil {
			continue
		}
		s.mu.Lock()
		points := []*nativeObject{}
		for _, n := range s.objects {
			if n.point != nil && logController(n.point.Owner) == req.controller && len(n.point.Sinks) > 0 {
				addRef(n)
				points = append(points, n)
			}
		}
		s.mu.Unlock()
		allowed := true
		var failure error
		for _, point := range points {
			keys := make([]uint32, 0, len(point.point.Sinks))
			for key := range point.point.Sinks {
				keys = append(keys, key)
			}
			req.controller.mu.Lock()
			entry := entryObject(req.tx).(Typed)
			entry.Object.(Properties)["IsFiltered"] = !req.controller.filter.matches(req.tx)
			req.controller.mu.Unlock()
			for _, key := range keys {
				if req.ctx.Err() != nil {
					allowed = false
					break
				}
				sink := point.point.Sinks[key]
				if sink == 0 {
					continue
				}
				_ = comCall(sink, 1)
				discard, err := sendLogEvent(s, sink, logEvent{ID: req.tx.ID, New: true, Entry: entry})
				releaseCOM(sink)
				if discard || err != nil {
					allowed = false
					failure = err
				}
			}
			release(point)
		}
		if req.ctx.Err() != nil {
			allowed = false
			failure = req.ctx.Err()
		}
		if allowed {
			req.controller.mu.Lock()
			if req.controller.preNotified == nil {
				req.controller.preNotified = map[string]bool{}
			}
			if req.controller.sessionID == req.session {
				key := fmt.Sprintf("%s:%d:%s", req.session, req.tx.Generation, req.tx.ID)
				if len(req.controller.preOrder) >= 100064 {
					delete(req.controller.preNotified, req.controller.preOrder[0])
					req.controller.preOrder = req.controller.preOrder[1:]
				}
				req.controller.preOrder = append(req.controller.preOrder, key)
				req.controller.preNotified[key] = true
			}
			req.controller.mu.Unlock()
		}
		req.answer <- admissionAnswer{allowed, failure}
	}
}
