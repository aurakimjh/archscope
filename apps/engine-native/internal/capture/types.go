// Package capture defines the live HTTP-capture engine boundary. It owns
// session lifecycle and counters; parsers and analyzers consume the persisted
// canonical transaction model and do not depend on a running capturer.
package capture

import (
	"context"
	"errors"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	LiveSchemaVersion            = 1
	LiveCaptureContractVersion   = 2
	DefaultLiveTransactionRowCap = 500
)

// Body-capture gate. Retaining plaintext request/response bodies in a MITM
// proxy is the exact exposure the SEC-10 crash-dump exclusion preflight bounds,
// so the tier is opt-in per session and every session records which gate was in
// force. `BodyGatePreflightPending` is the honest state today: the preflight is
// not implemented, so an operator who enables bodies is accepting the residual
// risk rather than passing a check.
const (
	BodyGateDisabled         = "disabled"
	BodyGatePreflightPending = "sec10_preflight_pending"

	DefaultCaptureMaxBodyBytes = 256 << 10
)

type LiveCaptureContract struct {
	SchemaVersion                      int  `json:"schemaVersion"`
	TransactionRowCap                  int  `json:"transactionRowCap"`
	ResyncOnEventSkip                  bool `json:"resyncOnEventSkip"`
	RestoreCurrentSessionOnPageReentry bool `json:"restoreCurrentSessionOnPageReentry"`
	FinalizedSessionUsesAnalysisResult bool `json:"finalizedSessionUsesAnalysisResult"`
	// BodyCaptureAvailable tells the renderer whether to offer the opt-in at
	// all, and BodyCaptureGate names the gate the session will record. The
	// renderer must not infer either from the absence of body previews.
	BodyCaptureAvailable bool   `json:"bodyCaptureAvailable"`
	BodyCaptureGate      string `json:"bodyCaptureGate"`
	MaxBodyBytes         int    `json:"maxBodyBytes"`
	// UpstreamTimingsAvailable tells the renderer that finalized rows carry a
	// `proxyUpstream` phase breakdown, so a missing waterfall means the phase
	// was not observed rather than that the engine never emits one.
	UpstreamTimingsAvailable bool `json:"upstreamTimingsAvailable"`
}

func DefaultLiveCaptureContract() LiveCaptureContract {
	return LiveCaptureContract{
		SchemaVersion:                      LiveCaptureContractVersion,
		TransactionRowCap:                  DefaultLiveTransactionRowCap,
		ResyncOnEventSkip:                  true,
		RestoreCurrentSessionOnPageReentry: true,
		FinalizedSessionUsesAnalysisResult: true,
		BodyCaptureAvailable:               true,
		BodyCaptureGate:                    BodyGatePreflightPending,
		MaxBodyBytes:                       DefaultCaptureMaxBodyBytes,
		UpstreamTimingsAvailable:           true,
	}
}

type SessionID string
type SessionState string
type Privilege string

const (
	StateCreated     SessionState = "created"
	StateStarting    SessionState = "starting"
	StateRunning     SessionState = "running"
	StateStopping    SessionState = "stopping"
	StateFinalized   SessionState = "finalized"
	StateFailed      SessionState = "failed"
	StateRecoverable SessionState = "recoverable"

	PrivilegeNone  Privilege = "none"
	PrivilegeAdmin Privilege = "admin"
)

var (
	ErrSessionActive    = errors.New("a capture session is already active")
	ErrSessionNotFound  = errors.New("capture session not found")
	ErrBackpressureHard = errors.New("capture write queue reached its hard byte limit")
	ErrModeUnavailable  = errors.New("live capture mode is unavailable on this platform")
)

type OverflowPolicy string

const (
	OverflowStop     OverflowPolicy = "stop"
	OverflowBodyOnly OverflowPolicy = "body_only"
	OverflowRotate   OverflowPolicy = "rotate"
)

type Config struct {
	ListenAddress              string         `json:"listenAddress,omitempty"`
	StoreRoot                  string         `json:"storeRoot,omitempty"`
	MaxSessionBytes            int64          `json:"maxSessionBytes,omitempty"`
	ReserveBytes               int64          `json:"reserveBytes,omitempty"`
	BodyFraction               float64        `json:"bodyFraction,omitempty"`
	OverflowPolicy             OverflowPolicy `json:"overflowPolicy,omitempty"`
	WriteHighWaterBytes        int64          `json:"writeHighWaterBytes,omitempty"`
	WriteHardLimitBytes        int64          `json:"writeHardLimitBytes,omitempty"`
	LiveWindow                 int            `json:"liveWindow,omitempty"`
	AllowPassthrough           []string       `json:"allowPassthrough,omitempty"`
	PassthroughTTLSeconds      int            `json:"passthroughTtlSeconds,omitempty"`
	RetainUnattributedMetadata bool           `json:"retainUnattributedMetadata,omitempty"`
	// CaptureBodies opts this session into retaining request/response payloads.
	// Off by default; see the body-capture gate constants above.
	CaptureBodies bool `json:"captureBodies,omitempty"`
	MaxBodyBytes  int  `json:"maxBodyBytes,omitempty"`
}

type Stats struct {
	SessionID       SessionID    `json:"sessionId"`
	State           SessionState `json:"state"`
	Observed        uint64       `json:"observed"`
	Captured        uint64       `json:"captured"`
	Persisted       uint64       `json:"persisted"`
	BodyOmitted     uint64       `json:"bodyOmitted"`
	EventSkipped    uint64       `json:"eventSkipped"`
	KernelDropped   uint64       `json:"kernelDropped"`
	ParseFailed     uint64       `json:"parseFailed"`
	Unsupported     uint64       `json:"unsupported"`
	Passthrough     uint64       `json:"passthrough"`
	Unattributed    uint64       `json:"unattributed"`
	Dropped         uint64       `json:"dropped"`
	Backpressured   bool         `json:"backpressured"`
	SnapshotVersion uint64       `json:"snapshotVersion"`
	Sequence        uint64       `json:"sequence"`
	StoreBytes      int64        `json:"storeBytes"`
}

type Session struct {
	ID                         SessionID    `json:"sessionId"`
	State                      SessionState `json:"state"`
	ListenAddress              string       `json:"listenAddress"`
	StorePath                  string       `json:"storePath"`
	StartedAt                  time.Time    `json:"startedAt"`
	EndedAt                    *time.Time   `json:"endedAt,omitempty"`
	Error                      string       `json:"error,omitempty"`
	RetainUnattributedMetadata bool         `json:"retainUnattributedMetadata"`
	// CaptureBodies / BodyCaptureGate are the policy this session actually
	// enforced, fixed at start. The renderer shows these rather than the local
	// checkbox so a running session can never be described by a setting that
	// will only take effect on the next start.
	CaptureBodies   bool   `json:"captureBodies"`
	BodyCaptureGate string `json:"bodyCaptureGate"`
	MaxBodyBytes    int    `json:"maxBodyBytes,omitempty"`
}

type Mode struct {
	Name              string    `json:"name"`
	Available         bool      `json:"available"`
	Reason            string    `json:"reason,omitempty"`
	RequiredPrivilege Privilege `json:"requiredPrivilege"`
	Fidelity          string    `json:"fidelity"`
}

type EventSink interface {
	Started(Session)
	Progress(SessionID, []models.CaptureTransaction)
	Transactions(SessionID, uint64, uint64, []models.CaptureTransaction)
	Aggregate(SessionID, uint64, uint64, any)
	Stats(Stats)
	Stopped(Session)
	Error(SessionID, string)
}

type NopEventSink struct{}

func (NopEventSink) Started(Session)                                                     {}
func (NopEventSink) Progress(SessionID, []models.CaptureTransaction)                     {}
func (NopEventSink) Transactions(SessionID, uint64, uint64, []models.CaptureTransaction) {}
func (NopEventSink) Aggregate(SessionID, uint64, uint64, any)                            {}
func (NopEventSink) Stats(Stats)                                                         {}
func (NopEventSink) Stopped(Session)                                                     {}
func (NopEventSink) Error(SessionID, string)                                             {}

type Capturer interface {
	Name() string
	Available() (bool, string)
	RequiredPrivilege() Privilege
	Start(context.Context, Config, func(models.CaptureTransaction) error) (string, error)
	Stop(context.Context) error
}
