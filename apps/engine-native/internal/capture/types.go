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

const LiveSchemaVersion = 1

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
