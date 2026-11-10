package session

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/dumppolicy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestManagerRejectsBodyCaptureBeforeAllocatingSensitiveState(t *testing.T) {
	root := t.TempDir()
	manager := NewManagerForPlatform(root, capture.NopEventSink{}, "windows")
	// Pinned rather than inherited from the host: on a Windows runner with no
	// LocalDumps policy the real preflight verifies, and this test would then
	// assert the accept path while claiming to cover the reject path.
	manager.dumpPolicy = func() dumppolicy.Result {
		return dumppolicy.Result{
			State:  dumppolicy.StateUnavailable,
			Reason: dumppolicy.ReasonLocalDumpsCollecting,
			Detail: "a LocalDumps policy collects a crash dump for this application",
		}
	}
	capturerConstructed := false
	manager.newCapturer = func(proxy.Config) (Capturer, error) {
		capturerConstructed = true
		return nil, errors.New("must not construct capturer")
	}

	_, err := manager.Start(context.Background(), capture.Config{CaptureBodies: true})
	if !errors.Is(err, capture.ErrBodyCapturePreflightPending) {
		t.Fatalf("Start error=%v, want ErrBodyCapturePreflightPending", err)
	}
	if capturerConstructed || manager.authority != nil || manager.Current().ID != "" {
		t.Fatalf("rejected start allocated sensitive state: capturer=%v authority=%v session=%+v", capturerConstructed, manager.authority != nil, manager.Current())
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected start created %d session artifacts", len(entries))
	}
}

// The other half of the gate: a verified preflight must actually enable the
// tier, and the session plus its manifest must record which gate let it
// through. Without this the fail-closed path could be satisfied by code that
// simply always refuses.
func TestManagerStartsBodyCaptureOnVerifiedPreflight(t *testing.T) {
	root := t.TempDir()
	manager := NewManagerForPlatform(root, capture.NopEventSink{}, "windows")
	manager.dumpPolicy = func() dumppolicy.Result {
		return dumppolicy.Result{State: dumppolicy.StateVerified, Detail: "no LocalDumps policy collects a dump"}
	}
	// proxy.New runs its own preflight against the real host, so this test
	// would otherwise be blocked by the very independence it is not testing.
	// The proxy-side accept path is covered in the proxy package instead.
	manager.newCapturer = func(proxy.Config) (Capturer, error) {
		return &drainOnStopCapturer{}, nil
	}

	started, err := manager.Start(context.Background(), capture.Config{
		ListenAddress: "127.0.0.1:0", CaptureBodies: true,
	})
	if err != nil {
		t.Fatalf("Start with a verified preflight failed: %v", err)
	}
	defer func() { _, _ = manager.Stop(context.Background(), started.ID) }()

	if !started.CaptureBodies {
		t.Fatal("session did not record that bodies were captured")
	}
	if started.BodyCaptureGate != capture.BodyGateVerified {
		t.Fatalf("BodyCaptureGate=%q, want %q", started.BodyCaptureGate, capture.BodyGateVerified)
	}
	if started.BodyCaptureGateReason != "" {
		t.Fatalf("a verified gate carried reason %q, want empty", started.BodyCaptureGateReason)
	}
}

// An unavailable preflight must reach the operator with the reason attached.
// A bare refusal would leave "why is the checkbox gone" unanswerable, which is
// the state RD-134 objected to in the opposite direction.
func TestManagerSurfacesPreflightReasonOnRejection(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "windows")
	manager.dumpPolicy = func() dumppolicy.Result {
		return dumppolicy.Result{
			State:  dumppolicy.StateUnavailable,
			Reason: dumppolicy.ReasonExclusionUnconfirmed,
			Detail: "the exclusion was requested but is not present in the effective policy",
		}
	}

	_, err := manager.Start(context.Background(), capture.Config{CaptureBodies: true})
	if !errors.Is(err, capture.ErrBodyCapturePreflightPending) {
		t.Fatalf("Start error=%v, want ErrBodyCapturePreflightPending", err)
	}
	if !strings.Contains(err.Error(), "not present in the effective policy") {
		t.Fatalf("rejection did not carry the preflight detail: %v", err)
	}
}

// A session that did not ask for bodies must not be described by the preflight
// at all. Recording a verified gate on a body-less session would make the
// manifest claim a check mattered to a session it did not govern.
func TestManagerRecordsDisabledGateWhenBodiesNotRequested(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "windows")
	manager.dumpPolicy = func() dumppolicy.Result {
		t.Fatal("preflight ran for a session that did not request bodies")
		return dumppolicy.Result{}
	}

	started, err := manager.Start(context.Background(), capture.Config{ListenAddress: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = manager.Stop(context.Background(), started.ID) }()

	if started.BodyCaptureGate != capture.BodyGateDisabled {
		t.Fatalf("BodyCaptureGate=%q, want %q", started.BodyCaptureGate, capture.BodyGateDisabled)
	}
}

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

type failedPartialCapturer struct{ stops int }

func (*failedPartialCapturer) Name() string                         { return "fixture" }
func (*failedPartialCapturer) Available() (bool, string)            { return true, "" }
func (*failedPartialCapturer) RequiredPrivilege() capture.Privilege { return capture.PrivilegeNone }
func (*failedPartialCapturer) Start(context.Context, capture.Config, func(models.CaptureTransaction) error) (string, error) {
	return "", errors.New("partial start fixture")
}
func (c *failedPartialCapturer) Stop(context.Context) error { c.stops++; return nil }
func TestBackendRejectedBeforeAnyAllocation(t *testing.T) {
	root := t.TempDir()
	manager := NewManagerForPlatform(root, capture.NopEventSink{}, "windows")
	constructed := false
	manager.newCapturer = func(proxy.Config) (Capturer, error) { constructed = true; return nil, errors.New("unexpected") }
	if _, err := manager.Start(context.Background(), capture.Config{Backend: "windows_transparent", CaptureBodies: true}); !errors.Is(err, capture.ErrModeUnavailable) {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if constructed || manager.authority != nil || len(entries) != 0 {
		t.Fatal("unsupported backend allocated resources")
	}
}
func TestPartialStartIsStoppedAndNewAuthorityReleased(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), capture.NopEventSink{}, "windows")
	capturer := &failedPartialCapturer{}
	manager.newCapturer = func(proxy.Config) (Capturer, error) { return capturer, nil }
	if _, err := manager.Start(context.Background(), capture.Config{}); err == nil {
		t.Fatal("expected start failure")
	}
	if capturer.stops != 1 || manager.authority != nil || manager.Current().ID != "" {
		t.Fatalf("stop=%d authority=%v", capturer.stops, manager.authority != nil)
	}
}
