package metrichistory

import (
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"regexp"
	"strconv"
	"strings"
)

// Closed manual exports: import never executes SQL, tools or network I/O.
var captureTag = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
var captureKernel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.+-]{0,127}$`)

type captureWindow struct {
	Start *string `json:"capture_start_ns"`
	End   *string `json:"capture_end_ns"`
}

func captureTime(w captureWindow, previous uint64, index int) (uint64, error) {
	a, ak := m.Time(w.Start)
	z, zk := m.Time(w.End)
	if !ak || !zk || a > z || index > 0 && a <= previous {
		return 0, fmt.Errorf("SNAPSHOT_CAPTURE_TIME: snapshots/%d requires increasing nonoverlapping canonical bounds", index)
	}
	return z, nil
}
func captureUint(v *string, pos string) error {
	if v == nil || len(*v) > 20 {
		return fmt.Errorf("SNAPSHOT_CAPTURE_VALUE: %s requires canonical uint64", pos)
	}
	if _, e := m.Uint(*v); e != nil {
		return fmt.Errorf("SNAPSHOT_CAPTURE_VALUE: %s requires canonical uint64", pos)
	}
	return nil
}
func (b *builder) captureIdentity(o Options, producer, version, contract string, query any) error {
	if o.Producer != "" && o.Producer != producer || o.ProducerVersion != "" && o.ProducerVersion != version || o.ExportID != "" || o.Namespace != "" || string(b.set.Sources[0].Query) != "{}" {
		return fmt.Errorf("SNAPSHOT_CAPTURE_OPTIONS: capture owns identity; no query/namespace/export overrides")
	}
	b.set.Sources[0].Producer = producer
	b.set.Sources[0].ProducerVersion = version
	b.set.Sources[0].Query = appleJSON(query)
	b.origin = contract + ":collector:1.0.0:" + producer + ":" + version
	return nil
}
func (b *builder) capturePoint(name, unit, description, kind string, resource, attrs json.RawMessage, start, end, value *string, pos string) error {
	temporal := "unspecified"
	if kind == "sum" {
		temporal = "cumulative"
	}
	id, e := b.addSeries(m.Series{Name: name, Unit: unit, Description: description, Kind: kind, Temporality: temporal, Monotonic: kind == "sum", Resource: resource, Attributes: attrs, Scope: m.Scope{Name: "manual.system-db-snapshots", Version: "1.0.0"}})
	if e != nil {
		return e
	}
	return b.addPoint(m.Point{SeriesID: id, Start: start, End: end, Value: value}, pos)
}

type pgCounts struct {
	Commit    *string `json:"xact_commit"`
	Rollback  *string `json:"xact_rollback"`
	Read      *string `json:"blks_read"`
	Hit       *string `json:"blks_hit"`
	Returned  *string `json:"tup_returned"`
	Fetched   *string `json:"tup_fetched"`
	Inserted  *string `json:"tup_inserted"`
	Updated   *string `json:"tup_updated"`
	Deleted   *string `json:"tup_deleted"`
	Conflicts *string `json:"conflicts"`
	TempFiles *string `json:"temp_files"`
	TempBytes *string `json:"temp_bytes"`
	Deadlocks *string `json:"deadlocks"`
	Sessions  *string `json:"sessions"`
}
type pgSnapshot struct {
	captureWindow
	pgCounts
	Postmaster string  `json:"postmaster_start_ns"`
	Reset      *string `json:"stats_reset_ns"`
	Backends   *string `json:"numbackends"`
}
type pgCapture struct {
	Contract    string       `json:"contract"`
	Collector   string       `json:"collector_version"`
	Version     string       `json:"postgres_version"`
	VersionNum  string       `json:"server_version_num"`
	Consistency string       `json:"stats_fetch_consistency"`
	TrackCounts *bool        `json:"track_counts"`
	Host        string       `json:"host_id"`
	Boot        string       `json:"boot_id"`
	Instance    string       `json:"instance_id"`
	Database    string       `json:"database_tag"`
	OID         string       `json:"database_oid"`
	Snapshots   []pgSnapshot `json:"snapshots"`
}

func (b *builder) postgres(body []byte, o Options) error {
	if len(body) > 4<<20 {
		return fmt.Errorf("SNAPSHOT_CAPTURE_BUDGET: maximum4MiB")
	}
	var c pgCapture
	if e := decodeSource(b.ctx, body, &c, true); e != nil {
		return e
	}
	if c.Contract != "postgres-stat-database.v1" || c.Collector != "1.0.0" || c.Version != "16.14" || c.VersionNum != "160014" {
		return fmt.Errorf("POSTGRES_CAPTURE_VERSION: qualified PostgreSQL16.14/server_version_num160014 and collector1.0.0 required")
	}
	if c.Consistency != "snapshot" || c.TrackCounts == nil || !*c.TrackCounts {
		return fmt.Errorf("POSTGRES_CAPTURE_STATS: track_counts=true and stats_fetch_consistency=snapshot required")
	}
	for _, v := range []string{c.Host, c.Boot, c.Instance, c.Database} {
		if !captureTag.MatchString(v) {
			return fmt.Errorf("SNAPSHOT_CAPTURE_IDENTITY: sanitized host/boot/instance/database tags required")
		}
	}
	oid, e := strconv.ParseUint(c.OID, 10, 32)
	if e != nil || oid == 0 || strconv.FormatUint(oid, 10) != c.OID {
		return fmt.Errorf("POSTGRES_CAPTURE_DATABASE: canonical nonzero database OID required; shared row excluded")
	}
	if len(c.Snapshots) == 0 || len(c.Snapshots) > 256 {
		return fmt.Errorf("SNAPSHOT_CAPTURE_BUDGET:1..256 snapshots required")
	}
	if e := b.captureIdentity(o, "postgresql", c.Version, c.Contract, nil); e != nil {
		return e
	}
	b.set.Normalizer = m.EpochNormalizer
	windows := []map[string]any{}
	var previous uint64
	for i, s := range c.Snapshots {
		end, e := captureTime(s.captureWindow, previous, i)
		if e != nil {
			return e
		}
		previous = end
		pm, ok := m.Time(&s.Postmaster)
		if !ok || pm > end {
			return fmt.Errorf("POSTGRES_CAPTURE_TIME: snapshots/%d/postmaster_start_ns", i)
		}
		if s.Reset != nil {
			r, ok := m.Time(s.Reset)
			if !ok || r > end {
				return fmt.Errorf("POSTGRES_CAPTURE_RESET: snapshots/%d/stats_reset_ns", i)
			}
		}
		windows = append(windows, map[string]any{"capture_start_ns": s.Start, "capture_end_ns": s.End, "postmaster_start_ns": s.Postmaster, "stats_reset_ns": s.Reset})
		resource := appleJSON(map[string]string{"host_id": c.Host, "boot_id": c.Boot, "instance_id": c.Instance, "database_tag": c.Database, "database_oid": c.OID, "postmaster_start_ns": s.Postmaster, "engine": "postgresql", "engine_version": c.Version})
		attrs := appleJSON(map[string]string{"view": "pg_stat_database", "observation": "delayed_cumulative_statistics", "clock": "unix_server_clock"})
		for _, v := range []struct {
			name, unit string
			value      *string
		}{
			{"xact_commit", "{transaction}", s.Commit}, {"xact_rollback", "{transaction}", s.Rollback}, {"blks_read", "{block}", s.Read}, {"blks_hit", "{block}", s.Hit},
			{"tup_returned", "{tuple}", s.Returned}, {"tup_fetched", "{tuple}", s.Fetched}, {"tup_inserted", "{tuple}", s.Inserted}, {"tup_updated", "{tuple}", s.Updated}, {"tup_deleted", "{tuple}", s.Deleted},
			{"conflicts", "{conflict}", s.Conflicts}, {"temp_files", "{file}", s.TempFiles}, {"temp_bytes", "By", s.TempBytes}, {"deadlocks", "{deadlock}", s.Deadlocks}, {"sessions", "{session}", s.Sessions},
		} {
			pos := fmt.Sprintf("snapshots/%d/%s", i, v.name)
			if e := captureUint(v.value, pos); e != nil {
				return e
			}
			if e := b.capturePoint("postgres.database."+v.name, v.unit, "Database-wide delayed cumulative statistic; no per-query attribution. stats_reset is the counter epoch, not postmaster start.", "sum", resource, attrs, s.Reset, s.End, v.value, pos); e != nil {
				return e
			}
		}
		pos := fmt.Sprintf("snapshots/%d/numbackends", i)
		if e := captureUint(s.Backends, pos); e != nil {
			return e
		}
		if e := b.capturePoint("postgres.database.numbackends", "{backend}", "Instantaneous connected backends, including collection backend; not active queries or pool queue.", "gauge", resource, appleJSON(map[string]string{"view": "pg_stat_database", "observation": "instantaneous_backend_count", "clock": "unix_server_clock"}), s.Start, s.End, s.Backends, pos); e != nil {
			return e
		}
	}
	b.set.Sources[0].Query = appleJSON(map[string]any{"contract": c.Contract, "collector_version": c.Collector, "stats_fetch_consistency": c.Consistency, "track_counts": true, "capture_windows": windows})
	b.set.Warnings = append(b.set.Warnings, "PostgreSQL16.14 counters are delayed cumulative statistics; capture windows are read skew, not workload or aggregation intervals. numbackends is an instantaneous gauge including the collector. SQL text/database names are excluded.", "stats_reset, including null/unknown, is preserved as the cumulative start; postmaster start scopes instance lifetime and never substitutes for a missing reset. Clean restarts may retain older statistics. Unknown starts/decreases do not yield valid rates. Known reset rates cover only the post-reset epoch, not the missing pre-reset interval.", "blks_hit measures PostgreSQL buffer hits; blks_read is not proven physical disk I/O. No inferred cache percentage, I/O latency, query percentile, replication lag or bottleneck. Tags and producer identity are declarations, without authentication or cross-source clock alignment.")
	return nil
}

type ssSnapshot struct {
	captureWindow
	Summary string `json:"summary"`
}
type ssCapture struct {
	Contract  string       `json:"contract"`
	Collector string       `json:"collector_version"`
	Version   string       `json:"iproute2_version"`
	Kernel    string       `json:"kernel_release"`
	Host      string       `json:"host_id"`
	Boot      string       `json:"boot_id"`
	Namespace string       `json:"network_namespace"`
	Snapshots []ssSnapshot `json:"snapshots"`
}

var ssTotal = regexp.MustCompile(`^Total:\s+([0-9]+)$`)
var ssTCP = regexp.MustCompile(`^TCP:\s+([0-9]+) \(estab ([0-9]+), closed (-?[0-9]+), orphaned ([0-9]+), timewait ([0-9]+)\)$`)

func (b *builder) socketSummary(body []byte, o Options) error {
	if len(body) > 4<<20 {
		return fmt.Errorf("SNAPSHOT_CAPTURE_BUDGET: maximum4MiB")
	}
	var c ssCapture
	if e := decodeSource(b.ctx, body, &c, true); e != nil {
		return e
	}
	if c.Contract != "linux-ss-summary.v1" || c.Collector != "1.0.0" || c.Version != "6.1.0" {
		return fmt.Errorf("SS_CAPTURE_VERSION: qualified iproute2-6.1.0 and collector1.0.0 required")
	}
	for _, v := range []string{c.Host, c.Boot, c.Namespace} {
		if !captureTag.MatchString(v) {
			return fmt.Errorf("SNAPSHOT_CAPTURE_IDENTITY: sanitized host/boot/network namespace tags required")
		}
	}
	if !captureKernel.MatchString(c.Kernel) {
		return fmt.Errorf("SS_CAPTURE_KERNEL: release required")
	}
	if len(c.Snapshots) == 0 || len(c.Snapshots) > 256 {
		return fmt.Errorf("SNAPSHOT_CAPTURE_BUDGET:1..256 snapshots required")
	}
	resource := appleJSON(map[string]string{"host_id": c.Host, "boot_id": c.Boot, "network_namespace": c.Namespace, "kernel_release": c.Kernel, "producer": "iproute2", "producer_version": c.Version})
	if e := b.captureIdentity(o, "iproute2", c.Version, c.Contract, nil); e != nil {
		return e
	}
	windows := []captureWindow{}
	var previous uint64
	for i, s := range c.Snapshots {
		end, e := captureTime(s.captureWindow, previous, i)
		if e != nil {
			return e
		}
		previous = end
		windows = append(windows, s.captureWindow)
		if len(s.Summary) > 16<<10 {
			return fmt.Errorf("SS_CAPTURE_BUDGET: snapshots/%d/summary exceeds16KiB", i)
		}
		lines := []string{}
		for _, line := range strings.Split(strings.ReplaceAll(s.Summary, "\r\n", "\n"), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) != 8 {
			return fmt.Errorf("SS_CAPTURE_LAYOUT: snapshots/%d requires Total,TCP,transport header and five rows; use LC_ALL=C ss -s only", i)
		}
		total, tcp := ssTotal.FindStringSubmatch(lines[0]), ssTCP.FindStringSubmatch(lines[1])
		if total == nil || tcp == nil || strings.Join(strings.Fields(lines[2]), " ") != "Transport Total IP IPv6" {
			return fmt.Errorf("SS_CAPTURE_LAYOUT: snapshots/%d unsupported summary header", i)
		}
		add := func(name, value, scope string) error {
			pos := fmt.Sprintf("snapshots/%d/summary/%s", i, name)
			if name == "tcp.reported_closed" {
				n, e := strconv.ParseInt(value, 10, 64)
				if e != nil || strconv.FormatInt(n, 10) != value {
					return fmt.Errorf("SS_CAPTURE_VALUE: %s", pos)
				}
			} else if e := captureUint(&value, pos); e != nil {
				return e
			}
			attrs := appleJSON(map[string]string{"scope": scope, "capture_semantics": "sequential_proc_summary_read", "clock": "unix_collector_clock"})
			unit := "{socket}"
			if strings.HasPrefix(name, "transport.frag.") {
				unit = "{fragment_queue}"
			}
			return b.capturePoint("linux.ss."+name, unit, "Producer-reported instantaneous gauge; fields overlap and mix shared-kernel allocation with namespace counts. No sums, rates or flow attribution.", "gauge", resource, attrs, s.Start, s.End, &value, pos)
		}
		if e := add("reported_total", total[1], "producer_reported_summary"); e != nil {
			return e
		}
		for j, name := range []string{"reported_total", "established", "reported_closed", "orphaned", "timewait"} {
			scope := "producer_mixed_kernel_namespace"
			if name == "established" || name == "timewait" {
				scope = "network_namespace"
			}
			if e := add("tcp."+name, tcp[j+1], scope); e != nil {
				return e
			}
		}
		for j, name := range []string{"RAW", "UDP", "TCP", "INET", "FRAG"} {
			fields := strings.Fields(lines[j+3])
			if len(fields) != 4 || fields[0] != name {
				return fmt.Errorf("SS_CAPTURE_LAYOUT: snapshots/%d/transport/%d", i, j)
			}
			for k, family := range []string{"total", "ipv4", "ipv6"} {
				if e := add("transport."+strings.ToLower(name)+"."+family, fields[k+1], "producer_reported_transport_summary"); e != nil {
					return e
				}
			}
		}
	}
	b.set.Sources[0].Query = appleJSON(map[string]any{"contract": c.Contract, "collector_version": c.Collector, "command": "LC_ALL=C ss -s", "capture_windows": windows})
	b.set.Warnings = append(b.set.Warnings, "iproute2-6.1.0 ss -s is a sequential /proc summary, not packet/flow evidence or cumulative counters. All fields remain gauges without rates. Linux qualification is the recorded kernel/environment, not netstat/per-process/Windows support.", "TCP reported total/closed/orphan allocation may include shared-kernel accounting outside the declared network namespace; context does not prove every value is namespace-local. Never sum overlapping totals/transport rows or infer throughput, drops, RTT, bandwidth, local ownership or causation. Signed reported_closed is retained because separately read counts enter the producer calculation.")
	return nil
}
