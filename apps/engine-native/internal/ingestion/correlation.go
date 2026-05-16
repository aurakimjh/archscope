package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const DefaultCorrelationWindow = 5 * time.Second

type TimestampWindow struct {
	StartUnixNanos int64  `json:"start_unix_nanos,omitempty"`
	EndUnixNanos   int64  `json:"end_unix_nanos,omitempty"`
	StartTime      string `json:"start_time,omitempty"`
	EndTime        string `json:"end_time,omitempty"`
	WindowMillis   int64  `json:"window_ms,omitempty"`
}

// CorrelationKeys is the cross-source key model used by Mid-Term Plus evidence
// stitching. TenantID and CustomerID should only carry sanitized or explicitly
// allow-listed identifiers.
type CorrelationKeys struct {
	TraceID      string           `json:"trace_id,omitempty"`
	SpanID       string           `json:"span_id,omitempty"`
	ParentSpanID string           `json:"parent_span_id,omitempty"`
	RequestID    string           `json:"request_id,omitempty"`
	TenantID     string           `json:"tenant_id,omitempty"`
	CustomerID   string           `json:"customer_id,omitempty"`
	ContainerID  string           `json:"container_id,omitempty"`
	PodUID       string           `json:"pod_uid,omitempty"`
	HostID       string           `json:"host_id,omitempty"`
	PID          int              `json:"pid,omitempty"`
	Window       *TimestampWindow `json:"timestamp_window,omitempty"`
	StableID     string           `json:"stable_id,omitempty"`
}

func NormalizeCorrelationKeys(keys CorrelationKeys) CorrelationKeys {
	out := keys
	out.TraceID = lowerID(out.TraceID)
	out.SpanID = lowerID(out.SpanID)
	out.ParentSpanID = lowerID(out.ParentSpanID)
	out.RequestID = trimID(out.RequestID)
	out.TenantID = trimID(out.TenantID)
	out.CustomerID = trimID(out.CustomerID)
	out.ContainerID = lowerID(out.ContainerID)
	out.PodUID = lowerID(out.PodUID)
	out.HostID = lowerID(out.HostID)
	out.StableID = StableCorrelationID(out)
	return out
}

func TimestampWindowFor(ts time.Time, window time.Duration) *TimestampWindow {
	if ts.IsZero() {
		return nil
	}
	if window <= 0 {
		window = DefaultCorrelationWindow
	}
	utc := ts.UTC()
	half := window / 2
	start := utc.Add(-half)
	end := start.Add(window)
	return &TimestampWindow{
		StartUnixNanos: start.UnixNano(),
		EndUnixNanos:   end.UnixNano(),
		StartTime:      start.Format(time.RFC3339Nano),
		EndTime:        end.Format(time.RFC3339Nano),
		WindowMillis:   window.Milliseconds(),
	}
}

func StableCorrelationID(keys CorrelationKeys) string {
	parts := CanonicalCorrelationParts(keys)
	if len(parts) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:24]
}

func CanonicalCorrelationParts(keys CorrelationKeys) []string {
	values := map[string]string{
		"trace_id":       lowerID(keys.TraceID),
		"span_id":        lowerID(keys.SpanID),
		"parent_span_id": lowerID(keys.ParentSpanID),
		"request_id":     trimID(keys.RequestID),
		"tenant_id":      trimID(keys.TenantID),
		"customer_id":    trimID(keys.CustomerID),
		"container_id":   lowerID(keys.ContainerID),
		"pod_uid":        lowerID(keys.PodUID),
		"host_id":        lowerID(keys.HostID),
	}
	if keys.PID > 0 {
		values["pid"] = strconv.Itoa(keys.PID)
	}
	if keys.Window != nil {
		if keys.Window.StartUnixNanos != 0 {
			values["window_start_unix_nanos"] = strconv.FormatInt(keys.Window.StartUnixNanos, 10)
		}
		if keys.Window.EndUnixNanos != 0 {
			values["window_end_unix_nanos"] = strconv.FormatInt(keys.Window.EndUnixNanos, 10)
		}
	}
	names := make([]string, 0, len(values))
	for name, value := range values {
		if value != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+"="+values[name])
	}
	return out
}

func AttachCorrelationKeys(result *models.AnalysisResult, keys []CorrelationKeys) {
	if result == nil || len(keys) == 0 {
		return
	}
	normalized := make([]CorrelationKeys, 0, len(keys))
	for _, key := range keys {
		next := NormalizeCorrelationKeys(key)
		if next.StableID != "" {
			normalized = append(normalized, next)
		}
	}
	if len(normalized) == 0 {
		return
	}
	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["correlation_keys"] = normalized
}

func lowerID(value string) string {
	return strings.ToLower(trimID(value))
}

func trimID(value string) string {
	return strings.TrimSpace(value)
}
