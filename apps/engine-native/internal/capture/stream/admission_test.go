package stream

import (
	"context"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/admission"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestAdmissionRejectsRedactedCompletionAndAbortedProgressBeforeStorage(t *testing.T) {
	st := testStore(t)
	calls := 0
	p, e := New(Config{Store: st, SessionID: "s", RetainUnattributedMetadata: true, Admit: func(_ context.Context, tx models.CaptureTransaction) (bool, error) {
		calls++
		if tx.URL != "http://example.test/?token=%5BREDACTED%5D" || tx.Request.BodyPreview != "" {
			t.Errorf("unredacted admission: %+v", tx)
		}
		return false, nil
	}})
	if e != nil {
		t.Fatal(e)
	}
	tx := models.CaptureTransaction{ID: "denied", URL: "http://example.test/?token=secret", State: models.TxComplete, Request: models.HTTPMessage{BodyPreview: "payload"}}
	if e = p.Submit(context.Background(), tx); e != nil {
		t.Fatal(e)
	}
	tx.ID = "unfinished"
	tx.State = models.TxRequestSent
	if e = p.TrackProgress(tx); e != nil {
		t.Fatal(e)
	}
	if e = p.Close(); e != nil {
		t.Fatal(e)
	}
	page, e := st.Fetch(store.Filter{}, "", 10)
	if e != nil || len(page.Items) != 0 || calls != 2 {
		t.Fatal(page, calls, e)
	}
	stats := p.Stats(capture.StateFinalized)
	if stats.Persisted != 0 || stats.Captured != 0 || stats.Dropped != 2 || len(p.LiveWindow()) != 0 {
		t.Fatal(stats, p.LiveWindow())
	}
}

func TestPendingAdmissionIsReleasedBeforeClearAndClose(t *testing.T) {
	for _, clear := range []bool{true, false} {
		t.Run(map[bool]string{true: "clear", false: "stop"}[clear], func(t *testing.T) {
			h := admission.New()
			entered := make(chan struct{}, 1)
			key, release, e := h.Register(func(ctx context.Context, _ string, _ models.CaptureTransaction) (bool, error) {
				entered <- struct{}{}
				<-ctx.Done()
				return false, ctx.Err()
			})
			if e != nil {
				t.Fatal(e)
			}
			defer release()
			h.Begin("s", key, 0)
			p, e := New(Config{Store: testStore(t), SessionID: "s", RetainUnattributedMetadata: true, Admit: func(ctx context.Context, tx models.CaptureTransaction) (bool, error) { return h.Decide(ctx, "s", tx) }})
			if e != nil {
				t.Fatal(e)
			}
			submitted := make(chan error, 1)
			go func() {
				submitted <- p.Submit(context.Background(), models.CaptureTransaction{ID: "pending", State: models.TxComplete})
			}()
			<-entered
			done := make(chan error, 1)
			go func() {
				if clear {
					h.CancelPending("s")
					_, e := p.Clear()
					done <- e
				} else {
					h.End("s")
					done <- p.Close()
				}
			}()
			select {
			case e := <-done:
				if e != nil {
					t.Fatal(e)
				}
			case <-time.After(time.Second):
				t.Fatal("lifecycle deadlock")
			}
			if e := <-submitted; e != nil {
				t.Fatal(e)
			}
			if clear {
				p.Close()
			}
		})
	}
}
