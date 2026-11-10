package admission

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestDeadlineLifecycleAndGeneration(t *testing.T) {
	h := New()
	h.Timeout = 20 * time.Millisecond
	entered := make(chan struct{}, 8)
	key, release, err := h.Register(func(ctx context.Context, _ string, _ models.CaptureTransaction) (bool, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return false, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err = h.Begin("s", key, 0); err != nil {
		t.Fatal(err)
	}
	tx := models.CaptureTransaction{}
	if ok, e := h.Decide(context.Background(), "s", tx); ok || !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(ok, e)
	}
	<-entered
	for i, action := range []func(){func() { h.CancelPending("s") }, func() { h.Advance("s", 1) }, func() { h.End("s") }} {
		done := make(chan error, 1)
		go func() { _, e := h.Decide(context.Background(), "s", tx); done <- e }()
		<-entered
		action()
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("cancellation was ignored")
			}
		case <-time.After(time.Second):
			t.Fatal("pending decision did not cancel")
		}
		if i == 1 {
			tx.Generation = 1
		}
	}
	if ok, _ := h.Decide(context.Background(), "s", tx); ok {
		t.Fatal("closed source admitted")
	}
	h.Forget("s")
	if ok, e := h.Decide(context.Background(), "unowned", tx); !ok || e != nil {
		t.Fatal(ok, e)
	}
}

func TestNoListenersDoesNotChangeOrdinaryStopAndReleaseRevokesOwner(t *testing.T) {
	h := New()
	var active atomic.Bool
	key, release, err := h.RegisterActive(func(context.Context, string, models.CaptureTransaction) (bool, error) { return false, nil }, active.Load)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Begin("s", key, 0); err != nil {
		t.Fatal(err)
	}
	h.End("s")
	if ok, e := h.Decide(context.Background(), "s", models.CaptureTransaction{}); !ok || e != nil {
		t.Fatal(ok, e)
	}
	active.Store(true)
	if ok, _ := h.Decide(context.Background(), "s", models.CaptureTransaction{}); ok {
		t.Fatal("closed active policy admitted")
	}
	release()
	release()
	if h.Validate(key) == nil {
		t.Fatal("released owner accepted")
	}
}
