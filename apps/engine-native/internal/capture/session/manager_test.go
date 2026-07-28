package session

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestManagerEnforcesSingleActiveSessionAndIdempotentStop(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "windows")
	started, err := manager.Start(context.Background(), capture.Config{
		ListenAddress: "127.0.0.1:0", ReserveBytes: 0,
		RetainUnattributedMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.State != capture.StateRunning || started.ListenAddress == "" {
		t.Fatalf("started=%+v", started)
	}
	if !started.RetainUnattributedMetadata || !manager.Current().RetainUnattributedMetadata {
		t.Fatalf("active SEC-17 policy was not retained: %+v", started)
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

type drainOnStopCapturer struct {
	startContext  context.Context
	onTransaction func(models.CaptureTransaction) error
	contextAtStop error
	submitError   error
}

func (*drainOnStopCapturer) Name() string                         { return "drain-on-stop" }
func (*drainOnStopCapturer) Available() (bool, string)            { return true, "" }
func (*drainOnStopCapturer) RequiredPrivilege() capture.Privilege { return capture.PrivilegeNone }
func (c *drainOnStopCapturer) Start(
	ctx context.Context,
	_ capture.Config,
	onTransaction func(models.CaptureTransaction) error,
) (string, error) {
	c.startContext = ctx
	c.onTransaction = onTransaction
	return "127.0.0.1:43123", nil
}
func (c *drainOnStopCapturer) Stop(context.Context) error {
	c.contextAtStop = c.startContext.Err()
	c.submitError = c.onTransaction(models.CaptureTransaction{
		ID: "completed-during-stop", Method: http.MethodGet,
		URL: "http://example.test/drain", Scheme: "http",
		Host: "example.test", Path: "/drain", State: models.TxComplete,
		CaptureMode: proxy.CaptureModeMITM, Fidelity: proxy.FidelityDecodedWire,
		Coverage: "confirmed",
		Process: &models.ProcessInstance{
			Name: "test-client", Attribution: "confirmed",
		},
		Request:  models.HTTPMessage{BodyStorage: proxy.BodyStorageOmitted},
		Response: models.HTTPMessage{BodyStorage: proxy.BodyStorageOmitted},
	})
	return c.submitError
}

func TestManagerDrainsCaptureSourceBeforeCancellingSubmissions(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "windows")
	capturer := &drainOnStopCapturer{}
	manager.newCapturer = func(proxy.Config) (Capturer, error) {
		return capturer, nil
	}
	started, err := manager.Start(context.Background(), capture.Config{
		ListenAddress: "127.0.0.1:0", ReserveBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := manager.Stop(context.Background(), started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if capturer.contextAtStop != nil {
		t.Fatalf("submission context was cancelled before source drain: %v", capturer.contextAtStop)
	}
	if capturer.submitError != nil {
		t.Fatalf("transaction completed during stop was not persisted: %v", capturer.submitError)
	}
	stats, err := manager.Stats(started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != capture.StateFinalized ||
		stats.Captured != 1 || stats.Persisted != 1 {
		t.Fatalf("stopped=%+v stats=%+v", stopped, stats)
	}
}

func TestManagerAdvertisesOnlyWindowsAsSupportedLivePlatform(t *testing.T) {
	windowsModes := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "windows").Modes()
	if len(windowsModes) != 1 || !windowsModes[0].Available || windowsModes[0].Reason != "" {
		t.Fatalf("windows modes=%+v", windowsModes)
	}
	other := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "darwin")
	otherModes := other.Modes()
	if len(otherModes) != 1 || otherModes[0].Available || otherModes[0].Reason == "" {
		t.Fatalf("non-windows modes=%+v", otherModes)
	}
	if _, err := other.Start(context.Background(), capture.Config{
		ListenAddress: "127.0.0.1:0",
	}); !errors.Is(err, capture.ErrModeUnavailable) {
		t.Fatalf("non-windows start err=%v", err)
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
