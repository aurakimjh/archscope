package threaddump

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"maps"
	"strings"
)

const MaxCorrelationContextBytes = 64 << 10

type CorrelationThread struct {
	RuntimeID string `json:"runtime_id"`
	Start     string `json:"thread_start_ns"`
}
type CorrelationContext struct {
	Version   string              `json:"schema_version"`
	Format    string              `json:"source_format"`
	Host      string              `json:"host_id"`
	Boot      string              `json:"boot_id"`
	Namespace string              `json:"pid_namespace"`
	PID       string              `json:"pid"`
	Start     string              `json:"process_start_ns"`
	Threads   []CorrelationThread `json:"threads"`
}

func DecodeCorrelationContext(raw []byte) (CorrelationContext, error) {
	var c CorrelationContext
	if len(raw) > MaxCorrelationContextBytes {
		return c, fmt.Errorf("THREAD_CORRELATION_CONTEXT_BUDGET")
	}
	if err := metricmodel.Strict(raw, &c); err != nil {
		return c, err
	}
	for _, v := range []string{c.Format, c.Host, c.Boot, c.Namespace} {
		if v == "" || v == "unknown" || len(v) > 1024 || strings.ContainsAny(v, "\x00\r\n") {
			return c, fmt.Errorf("THREAD_CORRELATION_PROCESS_CONTEXT_REQUIRED")
		}
	}
	if c.Version != "1.0.0" || len(c.Threads) == 0 || len(c.Threads) > 4096 {
		return c, fmt.Errorf("THREAD_CORRELATION_CONTEXT_INVALID")
	}
	for _, v := range []string{c.PID, c.Start} {
		if _, err := metricmodel.Uint(v); err != nil {
			return c, err
		}
	}
	ids := map[string]bool{}
	for _, t := range c.Threads {
		if t.RuntimeID == "" || len(t.RuntimeID) > 1024 || strings.ContainsAny(t.RuntimeID, "\x00\r\n") || ids[t.RuntimeID] {
			return c, fmt.Errorf("THREAD_CORRELATION_THREAD_INVALID")
		}
		ids[t.RuntimeID] = true
		if _, err := metricmodel.Uint(t.Start); err != nil {
			return c, err
		}
		a, _ := metricmodel.Uint(t.Start)
		b, _ := metricmodel.Uint(c.Start)
		if a.Cmp(b) < 0 {
			return c, fmt.Errorf("THREAD_CORRELATION_THREAD_PRECEDES_PROCESS")
		}
	}
	return c, nil
}

// ApplyCorrelationContext never invents an ID or falls back to a display name.
// Unmapped observations remain isolated and are reported by the analyzer.
func ApplyCorrelationContext(bundles []models.ThreadDumpBundle, c CorrelationContext) ([]models.ThreadDumpBundle, error) {
	raw, _ := json.Marshal(c)
	if _, err := DecodeCorrelationContext(raw); err != nil {
		return nil, err
	}
	starts := map[string]string{}
	for _, t := range c.Threads {
		starts[t.RuntimeID] = t.Start
	}
	seen := map[string]bool{}
	for _, b := range bundles {
		if b.SourceFormat != c.Format {
			return nil, fmt.Errorf("THREAD_CORRELATION_FORMAT_MISMATCH")
		}
		for _, s := range b.Snapshots {
			if s.ThreadID != nil {
				if _, ok := starts[*s.ThreadID]; ok {
					seen[*s.ThreadID] = true
				}
			}
		}
	}
	for id := range starts {
		if !seen[id] {
			return nil, fmt.Errorf("THREAD_CORRELATION_RUNTIME_ID_NOT_OBSERVED")
		}
	}
	out := append([]models.ThreadDumpBundle(nil), bundles...)
	for i := range out {
		b := &out[i]
		b.Metadata = maps.Clone(b.Metadata)
		b.Snapshots = append([]models.ThreadSnapshot(nil), b.Snapshots...)
		if b.Metadata == nil {
			b.Metadata = map[string]any{}
		}
		b.Metadata["correlation_context"] = map[string]string{"host_id": c.Host, "boot_id": c.Boot, "pid_namespace": c.Namespace, "pid": c.PID, "process_start_ns": c.Start}
		b.Metadata["correlation_context_basis"] = "caller_declared; not source authentication"
		for j := range b.Snapshots {
			s := &b.Snapshots[j]
			s.Metadata = maps.Clone(s.Metadata)
			if s.ThreadID != nil {
				if start, ok := starts[*s.ThreadID]; ok {
					if s.Metadata == nil {
						s.Metadata = map[string]any{}
					}
					s.Metadata["thread_start_ns"] = start
					s.Metadata["thread_lifetime_basis"] = "caller_declared"
				}
			}
		}
	}
	return out, nil
}
