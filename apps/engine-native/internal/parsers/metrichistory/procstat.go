package metrichistory

import (
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

func (b *builder) procStatSnapshots(body []byte, o Options) error {
	var f struct {
		Contract  string          `json:"contract"`
		Collector string          `json:"collector_version"`
		Version   string          `json:"producer_version"`
		History   json.RawMessage `json:"history"`
	}
	if err := decodeSource(b.ctx, body, &f, true); err != nil {
		return err
	}
	if f.Contract != "linux-proc-stat.v1" || f.Collector != "1.0.0" || f.Version != "7.0.14-linuxkit" {
		return fmt.Errorf("PROC_STAT_CAPTURE_UNQUALIFIED")
	}
	if err := b.captureIdentity(o, "linux-proc-stat", f.Version, f.Contract, map[string]any{"collector_version": f.Collector, "contract": f.Contract}); err != nil {
		return err
	}
	var h historyFile
	if err := decodeSource(b.ctx, f.History, &h, true); err != nil {
		return err
	}
	if len(h.Snapshots) < 1 || len(h.Snapshots) > 10000 {
		return fmt.Errorf("PROC_STAT_CAPTURE_SNAPSHOT_BUDGET")
	}
	fields := map[string]bool{"user": true, "nice": true, "system": true, "idle": true, "iowait": true, "irq": true, "softirq": true, "steal": true, "guest": true, "guest_nice": true}
	var previous uint64
	var identity string
	for i, s := range h.Snapshots {
		end, known := m.Time(s.Time)
		if !known || i > 0 && end <= previous {
			return fmt.Errorf("PROC_STAT_CAPTURE_TIME_INVALID")
		}
		previous = end
		if len(s.Metrics) != 10 {
			return fmt.Errorf("PROC_STAT_CAPTURE_METRICS_INVALID")
		}
		seen := map[string]bool{}
		for _, v := range s.Metrics {
			prefix := "linux.proc.stat.cpu."
			if len(v.Name) <= len(prefix) || v.Name[:len(prefix)] != prefix || !fields[v.Name[len(prefix):]] || seen[v.Name] || v.Unit != "{tick}" || v.Kind != "gauge" || v.Temporality != "unspecified" || v.Monotonic || v.Start != nil || v.Histogram != nil || v.Flags != 0 {
				return fmt.Errorf("PROC_STAT_CAPTURE_METRIC_INVALID")
			}
			seen[v.Name] = true
			var value string
			if json.Unmarshal(v.Value, &value) != nil {
				return fmt.Errorf("PROC_STAT_CAPTURE_VALUE_INVALID")
			}
			if _, err := m.Uint(value); err != nil {
				return err
			}
			var r map[string]string
			if json.Unmarshal(v.Resource, &r) != nil || len(r) != 2 || r["kernel_release"] != f.Version || !captureTag.MatchString(r["host_id"]) {
				return fmt.Errorf("PROC_STAT_CAPTURE_RESOURCE_INVALID")
			}
			if identity == "" {
				identity = r["host_id"]
			} else if identity != r["host_id"] {
				return fmt.Errorf("PROC_STAT_CAPTURE_IDENTITY_CHANGED")
			}
			var attrs map[string]string
			if json.Unmarshal(v.Attributes, &attrs) != nil || len(attrs) != 2 || attrs["scope"] != "Docker shared kernel aggregate; not container or process CPU; guest overlaps user and iowait may decrease" || attrs["user_hz"] != "100" {
				return fmt.Errorf("PROC_STAT_CAPTURE_SCOPE_INVALID")
			}
			if v.Scope.Name != "manual.linux.proc-stat" || v.Scope.Version != "1.0.0" || string(v.Scope.Attributes) != "{}" || v.Scope.Dropped != "0" || v.Scope.SchemaURL != "" {
				return fmt.Errorf("PROC_STAT_CAPTURE_SCOPE_INVALID")
			}
		}
	}
	b.set.Warnings = append(b.set.Warnings, "Actual Linux 7.0.14-linuxkit Docker shared-kernel /proc/stat aggregate; USER_HZ=100. Exact raw tick gauges only. Guest overlaps user; iowait may decrease. No sums, percentages, counter rates, container/process attribution or bare-metal/version qualification.")
	return b.snapshots(f.History)
}
