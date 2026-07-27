package session

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
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

func TestManagerAdvertisesOnlyWindowsAsSupportedLivePlatform(t *testing.T) {
	modes := NewManager(t.TempDir(), capture.NopEventSink{}).Modes()
	if len(modes) != 1 {
		t.Fatalf("modes=%+v", modes)
	}
	if runtime.GOOS == "windows" {
		if !modes[0].Available || modes[0].Reason != "" {
			t.Fatalf("windows mode=%+v", modes[0])
		}
		return
	}
	if modes[0].Available || modes[0].Reason == "" {
		t.Fatalf("non-windows mode=%+v", modes[0])
	}
}

func TestManagerRejectsSessionPathTraversal(t *testing.T) {
	manager := NewManager(t.TempDir(), capture.NopEventSink{})
	if _, err := manager.SessionPath(capture.SessionID(`..\outside`)); !errors.Is(err, capture.ErrSessionNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestLiveMetadataRequiresConfirmedAttributionOrExplicitOptIn(t *testing.T) {
	unknown := models.CaptureTransaction{}
	if retainLiveMetadata(unknown, false) {
		t.Fatal("nil process metadata was exposed without opt-in")
	}
	unknown.Process = &models.ProcessInstance{Attribution: "unknown"}
	if retainLiveMetadata(unknown, false) {
		t.Fatal("unknown process metadata was exposed without opt-in")
	}
	if !retainLiveMetadata(unknown, true) {
		t.Fatal("explicit metadata-only opt-in did not retain unknown attribution")
	}
	confirmed := models.CaptureTransaction{
		Process: &models.ProcessInstance{Attribution: "confirmed"},
	}
	if !retainLiveMetadata(confirmed, false) {
		t.Fatal("confirmed process metadata was dropped")
	}
}
