package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
)

func TestManagerEnforcesSingleActiveSessionAndIdempotentStop(t *testing.T) {
	manager := NewManager(t.TempDir(), capture.NopEventSink{})
	started, err := manager.Start(context.Background(), capture.Config{
		ListenAddress: "127.0.0.1:0", ReserveBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.State != capture.StateRunning || started.ListenAddress == "" {
		t.Fatalf("started=%+v", started)
	}
	if _, err := manager.Start(context.Background(), capture.Config{ListenAddress: "127.0.0.1:0"}); !errors.Is(err, capture.ErrSessionActive) {
		t.Fatalf("second start err=%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stopped, err := manager.Stop(ctx, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != capture.StateFinalized {
		t.Fatalf("stopped=%+v", stopped)
	}
	again, err := manager.Stop(ctx, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.State != capture.StateFinalized || !again.EndedAt.Equal(*stopped.EndedAt) {
		t.Fatalf("idempotent stop changed session: first=%+v second=%+v", stopped, again)
	}
}

func TestManagerRejectsSessionPathTraversal(t *testing.T) {
	manager := NewManager(t.TempDir(), capture.NopEventSink{})
	if _, err := manager.SessionPath(capture.SessionID(`..\outside`)); !errors.Is(err, capture.ErrSessionNotFound) {
		t.Fatalf("err=%v", err)
	}
}
