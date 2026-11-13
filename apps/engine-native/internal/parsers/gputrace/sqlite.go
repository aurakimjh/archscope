package gputrace

import (
	"bytes"
	"database/sql"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	pp "github.com/aurakimjh/archscope/apps/engine-native/internal/producerpolicy"
	"math/big"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (b *builder) sqlite(raw []byte) error {
	if !bytes.HasPrefix(raw, []byte("SQLite format 3\x00")) {
		return fmt.Errorf("GPU_TRACE_SQLITE_HEADER_REQUIRED")
	}
	dir, e := os.MkdirTemp("", "gpu-trace-sqlite-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "source.sqlite")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		return e
	}
	uriPath := filepath.ToSlash(path)
	// Drive-letter paths need the leading slash in file:///C:/...; otherwise
	// the drive is parsed as a URI authority instead of a local filename.
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro&immutable=1"}).String()
	db, e := sql.Open("sqlite", uri)
	if e != nil {
		return fmt.Errorf("GPU_TRACE_SQLITE_OPEN")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{"PRAGMA query_only=ON", "PRAGMA trusted_schema=OFF", "PRAGMA cache_size=-4096"} {
		if _, e = db.ExecContext(b.ctx, q); e != nil {
			return fmt.Errorf("GPU_TRACE_SQLITE_READONLY_SETUP")
		}
	}
	tables := map[string]bool{}
	objects := map[string]string{}
	rows, e := db.QueryContext(b.ctx, "SELECT name,type,sql FROM sqlite_schema LIMIT 257")
	if e != nil {
		return fmt.Errorf("GPU_TRACE_SQLITE_SCHEMA")
	}
	count := 0
	for rows.Next() {
		var name, kind string
		var ddl sql.NullString
		if e = rows.Scan(&name, &kind, &ddl); e != nil {
			rows.Close()
			return e
		}
		count++
		objects[name] = kind
		if len(ddl.String) > 16<<10 {
			rows.Close()
			return fmt.Errorf("GPU_TRACE_SQLITE_SCHEMA_BYTES_BUDGET")
		}
		if kind == "table" && !strings.Contains(strings.ToUpper(ddl.String), "CREATE VIRTUAL TABLE") {
			tables[name] = true
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if count > 256 {
		return fmt.Errorf("GPU_TRACE_SQLITE_TABLE_BUDGET")
	}
	for _, table := range []string{"META_DATA_EXPORT", "StringIds", "CUPTI_ACTIVITY_KIND_RUNTIME", "CUPTI_ACTIVITY_KIND_KERNEL", "CUPTI_ACTIVITY_KIND_MEMCPY", "NVTX_EVENTS"} {
		if objects[table] != "" && !tables[table] {
			return fmt.Errorf("GPU_TRACE_NSYS_SELECTED_OBJECT_REFUSED: %s must be an ordinary table; views/virtual tables refused", table)
		}
	}
	for _, table := range []string{"CUPTI_ACTIVITY_KIND_RUNTIME", "CUPTI_ACTIVITY_KIND_KERNEL", "CUPTI_ACTIVITY_KIND_MEMCPY", "NVTX_EVENTS"} {
		state := "present"
		if !tables[table] {
			state = "absent; unavailable, not proof of zero activity"
		}
		b.set.Warnings = append(b.set.Warnings, "Nsight supported table "+table+": "+state)
	}
	if !tables["META_DATA_EXPORT"] || !tables["StringIds"] {
		return fmt.Errorf("GPU_TRACE_NSYS_SCHEMA_REQUIRED: official SQLite tables, not views")
	}
	for _, table := range []string{"META_DATA_EXPORT", "StringIds", "CUPTI_ACTIVITY_KIND_RUNTIME", "CUPTI_ACTIVITY_KIND_KERNEL", "CUPTI_ACTIVITY_KIND_MEMCPY", "NVTX_EVENTS"} {
		if !tables[table] {
			continue
		}
		r, err := db.QueryContext(b.ctx, "PRAGMA table_xinfo('"+table+"')")
		if err != nil {
			return fmt.Errorf("GPU_TRACE_SQLITE_COLUMN_LAYOUT")
		}
		for r.Next() {
			var id, notnull, pk, hidden int
			var name, typ string
			var def any
			if err = r.Scan(&id, &name, &typ, &notnull, &def, &pk, &hidden); err != nil {
				r.Close()
				return err
			}
			if hidden != 0 {
				r.Close()
				return fmt.Errorf("GPU_TRACE_SQLITE_GENERATED_COLUMN_REFUSED")
			}
		}
		err = r.Err()
		r.Close()
		if err != nil {
			return err
		}
	}
	rows, e = db.QueryContext(b.ctx, "SELECT name,value FROM META_DATA_EXPORT WHERE name IN ('EXPORT_PRODUCT_NAME','EXPORT_PRODUCT_VERSION','EXPORT_SCHEMA_VERSION')")
	if e != nil {
		return fmt.Errorf("GPU_TRACE_NSYS_METADATA")
	}
	meta := map[string]string{}
	for rows.Next() {
		var k, v string
		if e = rows.Scan(&k, &v); e != nil {
			rows.Close()
			return e
		}
		if !g.Text(v) || meta[k] != "" {
			rows.Close()
			return fmt.Errorf("GPU_TRACE_NSYS_METADATA_INVALID")
		}
		meta[k] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if meta["EXPORT_PRODUCT_NAME"] != "NVIDIA Nsight Systems" || meta["EXPORT_SCHEMA_VERSION"] != "3.11.1" {
		return fmt.Errorf("GPU_TRACE_NSYS_VERSION: qualified export 2024.2.3.38 / schema3.11.1 required")
	}
	if b.o.ProducerVersion != "" && b.o.ProducerVersion != meta["EXPORT_PRODUCT_VERSION"] {
		return fmt.Errorf("GPU_TRACE_NSYS_VERSION_CONFLICT")
	}
	b.source.Producer = "nsight-systems"
	version, qualification, err := pp.Select(meta["EXPORT_PRODUCT_VERSION"], "2024.2.3.38", b.o.AllowUnverifiedVersion)
	if err != nil {
		return fmt.Errorf("GPU_TRACE_NSYS_VERSION: %w", err)
	}
	b.source.Version, b.source.Qualification = version, qualification
	if qualification != "" {
		b.set.Warnings = append(b.set.Warnings, pp.Limitation)
	}
	b.source.Schema = meta["EXPORT_SCHEMA_VERSION"]
	if e = b.scope(); e != nil {
		return e
	}
	for _, def := range []struct{ table, kind, name string }{{"CUPTI_ACTIVITY_KIND_RUNTIME", "cuda_api", "nameId"}, {"CUPTI_ACTIVITY_KIND_KERNEL", "gpu_kernel", "shortName"}, {"CUPTI_ACTIVITY_KIND_MEMCPY", "memcpy", ""}, {"NVTX_EVENTS", "annotation", ""}} {
		if !tables[def.table] {
			continue
		}
		cols := map[string]bool{}
		r, e := db.QueryContext(b.ctx, "PRAGMA table_info('"+def.table+"')")
		if e != nil {
			return e
		}
		for r.Next() {
			var a, d, f int
			var name, typ string
			var defaultValue any
			if e = r.Scan(&a, &name, &typ, &d, &defaultValue, &f); e != nil {
				r.Close()
				return e
			}
			cols[name] = true
		}
		e = r.Err()
		r.Close()
		if e != nil {
			return e
		}
		required := []string{"start", "end"}
		if def.kind == "cuda_api" {
			required = append(required, "globalTid", "correlationId", def.name)
		} else if g.In(def.kind, "gpu_kernel", "memcpy") {
			required = append(required, "globalPid", "deviceId", "contextId", "streamId", "correlationId")
			if def.name != "" {
				required = append(required, def.name)
			}
		} else {
			required = append(required, "globalTid", "eventType")
		}
		for _, key := range required {
			if !cols[key] {
				return fmt.Errorf("GPU_TRACE_NSYS_COLUMN_REQUIRED: %s.%s", def.table, key)
			}
		}
		expr := func(key string) string {
			if !cols[key] {
				return "''"
			}
			return "COALESCE(CAST(t.\"" + key + "\" AS TEXT),'')"
		}
		process := expr("globalPid")
		thread := "''"
		if def.kind == "cuda_api" || def.kind == "annotation" {
			thread = expr("globalTid")
			process = thread
		}
		name := "''"
		join := ""
		if def.name != "" {
			name = "CASE WHEN length(s.value)<=1024 THEN s.value ELSE NULL END"
			join = " LEFT JOIN StringIds s ON t.\"" + def.name + "\"=s.id"
		}
		if def.kind == "annotation" {
			name = "'annotation-sha256-excluded'"
		}
		where := " WHERE t.start IS NOT NULL AND t.end IS NOT NULL"
		args := []any{}
		if b.source.From != nil {
			bound, _ := new(big.Int).SetString(*b.source.To, 10)
			if bound.BitLen() > 63 {
				return fmt.Errorf("GPU_TRACE_NSYS_SELECTION_SIGNED_RANGE")
			}
			where += " AND t.start < CAST(? AS INTEGER) AND (t.end > CAST(? AS INTEGER) OR (t.start=t.end AND t.start>=CAST(? AS INTEGER)))"
			args = append(args, *b.source.To, *b.source.From, *b.source.From)
		}
		q := "SELECT CAST(t.rowid AS TEXT)," + expr("start") + "," + expr("end") + "," + process + "," + thread + "," + expr("deviceId") + "," + expr("contextId") + "," + expr("streamId") + "," + expr("correlationId") + "," + name + " FROM \"" + def.table + "\" t" + join + where + " ORDER BY t.start,t.rowid LIMIT " + strconv.Itoa(g.MaxEvents+1)
		r, e = db.QueryContext(b.ctx, q, args...)
		if e != nil {
			return fmt.Errorf("GPU_TRACE_NSYS_LAYOUT_INVALID: %s", def.table)
		}
		for r.Next() {
			var row, from, to, pid, tid, device, contextID, stream, corr string
			var name sql.NullString
			if e = r.Scan(&row, &from, &to, &pid, &tid, &device, &contextID, &stream, &corr, &name); e != nil {
				r.Close()
				return fmt.Errorf("GPU_TRACE_NSYS_ROW_INVALID")
			}
			if !g.Uint(from) || !g.Uint(to) || g.Cmp(from, to) > 0 || !g.Uint(pid) || !name.Valid {
				r.Close()
				return fmt.Errorf("GPU_TRACE_NSYS_TIME_OR_NAME_INVALID at %s/rowid/%s", def.table, row)
			}
			if tid != "" {
				n, _ := new(big.Int).SetString(pid, 10)
				pid = n.Lsh(n.Rsh(n, 24), 24).String()
			}
			if corr != "" && !g.Uint(corr) {
				r.Close()
				return fmt.Errorf("GPU_TRACE_NSYS_CORRELATION_INVALID")
			}
			if g.In(def.kind, "gpu_kernel", "memcpy") && (!g.Uint(device) || !g.Uint(contextID) || !g.Uint(stream)) {
				r.Close()
				return fmt.Errorf("GPU_TRACE_NSYS_STREAM_INVALID")
			}
			if def.kind == "memcpy" {
				name.String = "memcpy"
			}
			t := g.Track{Kind: def.kind, Process: pid, Thread: tid, Context: contextID, Stream: stream}
			if g.In(def.kind, "gpu_kernel", "memcpy") {
				t.DeviceIndex = device
				t.Device = b.o.DeviceMap[device]
				if t.Device == "" {
					r.Close()
					return fmt.Errorf("GPU_TRACE_DEVICE_MAPPING_REQUIRED: device index %s", device)
				}
			}
			if e = b.add(t, g.Event{Name: name.String, From: from, To: to, Correlation: corr, Position: def.table + "/rowid/" + row}); e != nil {
				r.Close()
				return e
			}
		}
		e = r.Err()
		r.Close()
		if e != nil {
			return e
		}
	}
	if len(b.set.Events) == 0 {
		return fmt.Errorf("GPU_TRACE_NSYS_NO_EVENTS: choose a window containing execution intervals")
	}
	b.set.Warnings = append(b.set.Warnings, "Nsight SQLite2024.2.3.38/schema3.11.1: immutable read-only copy, fixed table/column queries; environment/command-line/tensor/NVTX text excluded. Packed globalPid/globalTid stay capture-scoped serialized IDs, not OS PID/TID.", "Explicit window selects intersecting whole intervals without clipping or rescaling. No hardware utilization, instruction counters, causal critical path, per-request allocation or NCCL diagnosis is inferred.")
	return nil
}
