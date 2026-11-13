package resultcontract

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Providers are compiled declarations, never imported metadata. Paths select
// immutable envelope data; schemas describe the meaning needed by each consumer.
const MaxProviders = 128
const MaxPageRows = 100
const MaxPageColumns = 32
const MaxCellBytes = 2048
const MaxPageBytes = 256 << 10

type Provider struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Path    string `json:"payload_path"`
	Schema  string `json:"schema"`
	// Semantic columns are mandatory for timeline and SLI adapters. A timestamp
	// alone never certifies cross-source alignment or a causal relationship.
	Time        string `json:"time_column,omitempty"`
	Clock       string `json:"clock_column,omitempty"`
	Evidence    string `json:"evidence_column,omitempty"`
	Numerator   string `json:"numerator_column,omitempty"`
	Denominator string `json:"denominator_column,omitempty"`
	Unit        string `json:"unit_column,omitempty"`
	Population  string `json:"population_column,omitempty"`
}
type Capability struct {
	Provider
	Availability string `json:"availability"`
	Reason       string `json:"reason,omitempty"`
}
type Capabilities struct {
	Panel         *Panel       `json:"panel,omitempty"`
	Version       string       `json:"version"`
	Compatibility Verdict      `json:"compatibility"`
	IdentityBasis string       `json:"identity_basis"`
	Providers     []Capability `json:"providers"`
	Truncated     bool         `json:"truncated"`
}
type Cell struct {
	Text         string `json:"text"`
	Null         bool   `json:"null"`
	Truncated    bool   `json:"truncated"`
	Escaped      bool   `json:"escaped,omitempty"`
	Availability string `json:"availability,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Unit         string `json:"unit,omitempty"`
	Population   string `json:"population,omitempty"`
}
type Page struct {
	Provider  string   `json:"provider"`
	Columns   []string `json:"columns"`
	Rows      [][]Cell `json:"rows"`
	Offset    int      `json:"offset"`
	Next      int      `json:"next"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
	Escaped   bool     `json:"escaped,omitempty"`
}

func validateProviders(ps []Provider) error {
	if len(ps) > MaxProviders-4 {
		return fmt.Errorf("too many result providers")
	}
	ids := map[string]bool{}
	for _, p := range ps {
		if p.ID == "" || len(p.ID) > 256 || strings.HasPrefix(p.ID, "generic:") || strings.HasPrefix(p.ID, "compare:") || ids[p.ID] {
			return fmt.Errorf("invalid/duplicate provider ID")
		}
		ids[p.ID] = true
		if _, ok := Version(p.Version); !ok || p.Schema == "" {
			return fmt.Errorf("provider version/schema required")
		}
		section, _, e := pointer(p.Path)
		if e != nil || (section != "tables" && section != "series") {
			return fmt.Errorf("provider requires collection pointer")
		}
		switch p.Kind {
		case "table", "detail":
			if p.Schema != "object-rows-v1" {
				return fmt.Errorf("unsupported row schema")
			}
		case "timeline":
			if p.Schema != "timeline-v1" || p.Time == "" || p.Clock == "" || p.Evidence == "" {
				return fmt.Errorf("timeline time/clock/evidence required")
			}
		case "sli":
			if p.Schema != "sli-v1" || p.Numerator == "" || p.Denominator == "" || p.Unit == "" || p.Population == "" {
				return fmt.Errorf("SLI numerator/denominator/unit/population required")
			}
		default:
			return fmt.Errorf("unsupported provider kind")
		}
	}
	return nil
}
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
func rowsAt(r models.AnalysisResult, path string) (reflect.Value, error) {
	section, key, e := pointer(path)
	if e != nil {
		return reflect.Value{}, e
	}
	var raw any
	if section == "tables" {
		raw = r.Tables[key]
	} else if section == "series" {
		raw = r.Series[key]
	} else {
		return reflect.Value{}, fmt.Errorf("expected rows")
	}
	v := reflect.ValueOf(raw)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("collection must be rows")
	}
	return v, nil
}
func objectRow(v reflect.Value) (map[string]any, error) {
	if row, ok := v.Interface().(map[string]any); ok {
		if row == nil {
			return nil, fmt.Errorf("expected object row")
		}
		return row, nil
	}
	// Typed analyzer rows also use the same schema; decoding retains numbers.
	b, e := json.Marshal(v.Interface())
	if e != nil {
		return nil, e
	}
	var row map[string]any
	// Avoid float conversion in typed rows.
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	e = dec.Decode(&row)
	if e == nil && row == nil {
		e = fmt.Errorf("expected object row")
	}
	return row, e
}
func validateProvider(ctx context.Context, r models.AnalysisResult, p Provider, refs referenceIndex) error {
	rows, e := rowsAt(r, p.Path)
	if e != nil {
		return e
	}
	for i := 0; i < rows.Len(); i++ {
		if e = ctx.Err(); e != nil {
			return e
		}
		row, e := objectRow(rows.Index(i))
		if e != nil {
			return e
		}
		text := func(key string) bool { s, ok := row[key].(string); return ok && s != "" }
		switch p.Kind {
		case "timeline":
			timestamp, ok := row[p.Time].(string)
			if _, e := time.Parse(time.RFC3339Nano, timestamp); !ok || e != nil || !text(p.Clock) || !text(p.Evidence) || !refs.has("clock_ids", fmt.Sprint(row[p.Clock])) || !refs.has("evidence_ids", fmt.Sprint(row[p.Evidence])) {
				return fmt.Errorf("invalid timeline payload")
			}
		case "sli":
			n, e := numeric(row[p.Numerator], "json_number")
			if e != nil || n.Sign() < 0 {
				return fmt.Errorf("invalid numerator")
			}
			d, e := numeric(row[p.Denominator], "json_number")
			if e != nil || d.Sign() <= 0 || !text(p.Unit) || !text(p.Population) {
				return fmt.Errorf("invalid denominator/unit/population")
			}
			if !refs.has("population_ids", row[p.Population].(string)) {
				return fmt.Errorf("unknown SLI population")
			}
		}
	}
	return nil
}
func (r *Registry) Capabilities(ctx context.Context, result models.AnalysisResult) (Capabilities, error) {
	if e := ctx.Err(); e != nil {
		return Capabilities{}, e
	}
	refs, e := references(ctx, result)
	if e != nil {
		return Capabilities{}, e
	}
	out := Capabilities{Version: "1.0.0", Compatibility: r.Check(result, "display"), IdentityBasis: "declared", Providers: []Capability{}}
	providerBytes := 1024
	add := func(p Provider, available bool, reason string) {
		if len(p.ID) > 4096 || len(p.Path) > 2048 || len(out.Providers) >= MaxProviders {
			out.Truncated = true
			return
		}
		state := "available"
		if !available {
			state = "unavailable"
		}
		reason, _ = boundedText(reason)
		capability := Capability{p, state, reason}
		body, e := json.Marshal(capability)
		if e != nil || providerBytes+len(body) > 128<<10 {
			out.Truncated = true
			return
		}
		providerBytes += len(body)
		out.Providers = append(out.Providers, capability)
	}
	add(Provider{ID: "generic:summary", Version: "1.0.0", Kind: "summary", Path: "/summary", Schema: "key-value-v1"}, true, "")
	add(Provider{ID: "generic:diagnostics", Version: "1.0.0", Kind: "diagnostics", Path: "/metadata/diagnostics", Schema: "key-value-v1"}, true, "")
	add(Provider{ID: "generic:findings", Version: "1.0.0", Kind: "table", Path: "/metadata/findings", Schema: "object-rows-v1"}, true, "")
	// No imported chart grammar or script is executed by a fallback consumer.
	add(Provider{ID: "generic:charts", Version: "1.0.0", Kind: "chart", Path: "/charts", Schema: "unsupported"}, false, "renderer_not_registered")
	d, known := r.descriptor(result.Type, result.Metadata.SchemaVersion)
	if known && out.Compatibility.Allowed {
		out.Panel = clonePanel(d.Panel)
	}
	if known {
		for _, p := range d.Providers {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			reason := ""
			available := out.Compatibility.Allowed
			if !available {
				reason = out.Compatibility.Reason
			} else if e := validateProvider(ctx, result, p, refs); e != nil {
				if ctx.Err() != nil {
					return out, ctx.Err()
				}
				available = false
				reason = "provider_payload_invalid"
			}
			add(p, available, reason)
		}
		paths := make([]string, 0, len(d.Scalars))
		for path := range d.Scalars {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		v := r.check(result, "compare", out.Compatibility.Allowed)
		measurements, e := Measurements(result)
		for _, path := range paths {
			_, exists := measurements[path]
			reason := v.Reason
			if v.Allowed && (!exists || e != nil) {
				reason = "measurement_absent"
			}
			add(Provider{ID: "compare:" + path, Version: "1.0.0", Kind: "compare", Path: path, Schema: "measurement-delta-v1"}, v.Allowed && exists && e == nil, reason)
		}
	}
	for _, section := range []struct {
		name   string
		values map[string]any
	}{{"tables", result.Tables}, {"series", result.Series}} {
		keys := make([]string, 0, len(section.values))
		for k := range section.values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			path := "/" + section.name + "/" + escapePointer(k)
			// Semantic providers and a fallback table may coexist. Only the former
			// grants a timeline/SLI/detail capability.
			p := Provider{ID: "generic:" + path, Version: "1.0.0", Kind: "table", Path: path, Schema: "object-rows-v1"}
			rows, e := rowsAt(result, path)
			available := e == nil
			if available && rows.Len() > 0 {
				_, e = objectRow(rows.Index(0))
				available = e == nil
			}
			reason := ""
			if !available {
				reason = "not_object_rows"
			}
			add(p, available, reason)
			if out.Truncated {
				break
			}
		}
	}
	return out, nil
}
func boundedText(text string) (string, bool) {
	if len(text) <= MaxCellBytes {
		return text, false
	}
	text = text[:MaxCellBytes]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text, true
}

// ContainsDisplayControls is shared by display DTOs and constrained identifier
// interpreters. Ordinary Unicode letters, accents and joiners stay unchanged.
func ContainsDisplayControls(text string) bool {
	for _, c := range text {
		if displayControl(c) {
			return true
		}
	}
	return false
}
func displayControl(c rune) bool {
	return unicode.IsControl(c) || c == 0x061c || c == 0x200e || c == 0x200f || c >= 0x202a && c <= 0x202e || c >= 0x2066 && c <= 0x2069
}

// DisplayText exposes invisible ordering/control characters, bounding the output
// after expansion. It never changes an immutable result's original value.
func DisplayText(text string) (string, bool, bool) {
	var out strings.Builder
	escaped := false
	for i, c := range text {
		part := string(c)
		if displayControl(c) {
			part = fmt.Sprintf("⟦U+%04X⟧", c)
			escaped = true
		}
		if out.Len()+len(part) > MaxCellBytes {
			return out.String(), true, escaped
		}
		out.WriteString(part)
		if i+utf8.RuneLen(c) >= len(text) {
			return out.String(), false, escaped
		}
	}
	return out.String(), false, escaped
}
func DisplayCell(value any) Cell { return displayCell(value) }
func displayCell(value any) Cell {
	c := Cell{Null: value == nil}
	if value == nil {
		c.Text = "—"
		return c
	}
	switch v := value.(type) {
	case string:
		c.Text = v
	case bool:
		c.Text = fmt.Sprint(v)
	case json.Number:
		c.Text = string(v)
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Map, reflect.Slice, reflect.Struct, reflect.Array:
			c.Text = "[structured value: backend export]"
		default:
			b, e := json.Marshal(value)
			if e != nil {
				c.Text = "[unavailable]"
			} else {
				c.Text = string(b)
			}
		}
	}
	c.Text, c.Truncated, c.Escaped = DisplayText(c.Text)
	return c
}
func (r *Registry) Project(ctx context.Context, result models.AnalysisResult, id string, offset, limit int) (Page, error) {
	if offset < 0 || limit < 1 || limit > MaxPageRows {
		return Page{}, fmt.Errorf("invalid page bounds")
	}
	snapshot, e := r.prepare(ctx, result)
	if e != nil {
		return Page{}, e
	}
	return snapshot.Project(ctx, id, offset, limit)
}
func (snapshot *Projection) Project(ctx context.Context, id string, offset, limit int) (Page, error) {
	page := Page{Provider: id, Columns: []string{}, Rows: [][]Cell{}, Offset: offset, Next: -1}
	if offset < 0 || limit < 1 || limit > MaxPageRows {
		return page, fmt.Errorf("invalid page bounds")
	}
	if e := ctx.Err(); e != nil {
		return page, e
	}
	caps := snapshot.capabilities

	var p Provider
	found := false
	for _, c := range caps.Providers {
		if c.ID == id {
			if c.Availability != "available" {
				return page, fmt.Errorf("provider unavailable: %s", c.Reason)
			}
			p = c.Provider
			found = true
			break
		}
	}
	if !found || p.Kind == "compare" || p.Kind == "chart" {
		return page, fmt.Errorf("unknown/non-display provider")
	}
	measurement := snapshot.measurements
	var e error
	budget := 512 + len(id)
	appendRow := func(cells []Cell) bool {
		b, _ := json.Marshal(cells)
		if budget+len(b) > MaxPageBytes {
			page.Truncated = true
			return false
		}
		budget += len(b)
		page.Rows = append(page.Rows, cells)
		for _, c := range cells {
			page.Truncated = page.Truncated || c.Truncated
			page.Escaped = page.Escaped || c.Escaped
		}
		return true
	}
	decorate := func(value any, m Measurement) Cell {
		c := displayCell(value)
		for _, field := range []struct {
			dest  *string
			value string
		}{{&c.Availability, m.Availability}, {&c.Reason, m.Reason}, {&c.Unit, m.Unit}, {&c.Population, m.Population}} {
			text, truncated, escaped := DisplayText(field.value)
			*field.dest = text
			c.Truncated = c.Truncated || truncated
			c.Escaped = c.Escaped || escaped
		}
		return c
	}
	if p.Kind == "summary" || p.Kind == "diagnostics" {
		index, e := snapshot.values(ctx, p)
		if e != nil {
			return page, e
		}
		values, keys := index.values, index.keys
		page.Total = len(keys)
		page.Columns = []string{"key", "value"}
		for i := offset; i < len(keys) && len(page.Rows) < limit; i++ {
			if e = ctx.Err(); e != nil {
				return page, e
			}
			k := keys[i]
			m := Measurement{}
			if p.Kind == "summary" {
				m = measurement["/summary/"+escapePointer(k)]
			}
			if !appendRow([]Cell{displayCell(k), decorate(values[k], m)}) {
				break
			}
		}
	} else {
		index, e := snapshot.collection(ctx, p)
		if e != nil {
			return page, e
		}
		rows := index.rows
		page.Total = rows.Len()
		page.Truncated = index.truncated
		page.Escaped = index.escaped
		page.Columns = append(page.Columns, index.labels...)
		end := rows.Len()
		if offset < end && limit < end-offset {
			end = offset + limit
		}
		selected := []map[string]any{}
		for i := offset; i < end; i++ {
			if e := ctx.Err(); e != nil {
				return page, e
			}
			row, e := objectRow(rows.Index(i))
			if e != nil {
				return page, e
			}
			selected = append(selected, row)
		}

		columnBytes, _ := json.Marshal(page.Columns)
		budget += len(columnBytes)
		for _, row := range selected {
			cells := []Cell{}
			for _, key := range index.keys {
				v, exists := row[key]
				m := measurement[p.Path].Columns[key]
				if m.AvailabilityColumn != "" {
					m.Availability, _ = row[m.AvailabilityColumn].(string)
				}
				if m.ReasonColumn != "" {
					m.Reason, _ = row[m.ReasonColumn].(string)
				}
				c := decorate(v, m)
				if !exists {
					c.Availability = "absent"
					c.Reason = "field_absent"
				}
				cells = append(cells, c)
			}
			if !appendRow(cells) {
				break
			}
		}
	}
	if offset > page.Total {
		return page, fmt.Errorf("page offset beyond result")
	}
	if offset < page.Total && len(page.Rows) == 0 {
		return Page{}, &ProjectionBudgetError{"generic row bytes"}
	}
	if offset+len(page.Rows) < page.Total {
		page.Next = offset + len(page.Rows)
	}
	body, e := json.Marshal(page)
	if e != nil {
		return page, e
	}
	if len(body) > MaxPageBytes {
		return Page{}, &ProjectionBudgetError{"generic page bytes"}
	}
	return page, nil
}
