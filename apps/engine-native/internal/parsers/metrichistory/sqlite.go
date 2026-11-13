package metrichistory

import (
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

func (b *builder) sqliteSnapshots(body []byte, o Options) error {
	var f struct {
		Contract  string          `json:"contract"`
		Collector string          `json:"collector_version"`
		Version   string          `json:"producer_version"`
		History   json.RawMessage `json:"history"`
	}
	if err := decodeSource(b.ctx, body, &f, true); err != nil {
		return err
	}
	if f.Contract != "sqlite-pragma-snapshots.v1" || f.Collector != "1.0.0" || f.Version != "3.53.0" {
		return fmt.Errorf("SQLITE_CAPTURE_UNQUALIFIED")
	}
	if o.AllowUnverifiedVersion {
		return fmt.Errorf("SQLITE_CAPTURE_UNVERIFIED_NOT_SUPPORTED")
	}
	if err := b.captureIdentity(o, "sqlite", f.Version, f.Contract, map[string]any{"collector_version": f.Collector, "contract": f.Contract}); err != nil {
		return err
	}
	var h historyFile
	if err := decodeSource(b.ctx, f.History, &h, true); err != nil {
		return err
	}
	if len(h.Snapshots) < 1 || len(h.Snapshots) > 10000 {
		return fmt.Errorf("SQLITE_CAPTURE_SNAPSHOT_BUDGET")
	}
	units := map[string]string{"sqlite.page_count": "{page}", "sqlite.freelist_count": "{page}", "sqlite.page_size": "bytes", "sqlite.data_version": "{version}"}
	var previous uint64
	var identity string
	for i, s := range h.Snapshots {
		end, known := m.Time(s.Time)
		if !known || i > 0 && end <= previous {
			return fmt.Errorf("SQLITE_CAPTURE_TIME_INVALID")
		}
		previous = end
		if len(s.Metrics) != 4 {
			return fmt.Errorf("SQLITE_CAPTURE_METRICS_INVALID")
		}
		seen := map[string]bool{}
		for _, v := range s.Metrics {
			unit, ok := units[v.Name]
			if !ok || seen[v.Name] || v.Unit != unit || v.Kind != "gauge" || v.Temporality != "unspecified" || v.Monotonic || v.Start != nil || v.Histogram != nil || v.Flags != 0 {
				return fmt.Errorf("SQLITE_CAPTURE_METRIC_INVALID")
			}
			seen[v.Name] = true
			var value string
			if json.Unmarshal(v.Value, &value) != nil {
				return fmt.Errorf("SQLITE_CAPTURE_VALUE_INVALID")
			}
			if _, err := m.Uint(value); err != nil {
				return err
			}
			var r map[string]string
			if json.Unmarshal(v.Resource, &r) != nil || len(r) != 2 || r["sqlite_version"] != f.Version || !captureTag.MatchString(r["instance_id"]) {
				return fmt.Errorf("SQLITE_CAPTURE_RESOURCE_INVALID")
			}
			if identity == "" {
				identity = r["instance_id"]
			} else if identity != r["instance_id"] {
				return fmt.Errorf("SQLITE_CAPTURE_IDENTITY_CHANGED")
			}
			var attrs map[string]string
			if json.Unmarshal(v.Attributes, &attrs) != nil || len(attrs) != 1 || attrs["scope"] != "one read-only connection; file gauges; no SQL text or row content" {
				return fmt.Errorf("SQLITE_CAPTURE_SCOPE_INVALID")
			}
			if v.Scope.Name != "manual.sqlite.snapshot" || v.Scope.Version != "1.0.0" || string(v.Scope.Attributes) != "{}" || v.Scope.Dropped != "0" || v.Scope.SchemaURL != "" {
				return fmt.Errorf("SQLITE_CAPTURE_SCOPE_INVALID")
			}
		}
	}
	b.set.Warnings = append(b.set.Warnings, "SQLite 3.53.0 manual read-only PRAGMA gauges from one connection; not an atomic database snapshot. data_version is connection-local, not transaction count; no counter rates or SQL/row contents. File recreation/new connection requires a new source; producer identity remains declared.")
	return b.snapshots(f.History)
}
