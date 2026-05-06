// Package csv ports archscope_engine.exporters.csv_exporter — a
// spreadsheet-friendly flattener for AnalysisResult `summary` and
// `series` sections.
//
// The Python source is a stub:
//
//	def write_csv_table() -> None:
//	    raise NotImplementedError("CSV export is planned for a later phase.")
//
// Per work_status T-343 the intent is "summary/series flattening for
// spreadsheets". This Go port implements that minimal flattener so
// downstream tooling (Excel, gnuplot, pandas) can consume an
// AnalysisResult without re-parsing JSON.
//
// Two surface modes mirror T-340's `Write` / `Marshal`:
//
//   - Single-CSV mode (`Write`, `Marshal`): one file with section
//     headers (`# summary`, `# series:<key>`, ...). Suitable for a
//     quick spreadsheet dump or sharing one artefact.
//   - Directory mode (`WriteAll`): one CSV per logical table —
//     `summary.csv` plus `series_<key>.csv` per series key. Suitable
//     for tools that don't tolerate multiple sections in one file.
//
// Encoding follows RFC 4180 via `encoding/csv`: CRLF line endings,
// quoted fields when they contain commas / quotes / newlines, doubled
// internal quotes. UTF-8 throughout.
package csv

import (
	"bytes"
	encodingcsv "encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Write marshals `result` into a single CSV with section headers and
// writes it to `path`, creating parent directories as needed. Mirrors
// the JSON exporter's `Write` surface.
func Write(path string, result any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// Marshal returns the single-CSV bytes Write would persist.
//
// Layout:
//
//	# summary
//	key,value
//	<rows>
//
//	# series:<name>
//	<header columns>
//	<rows>
//
// An empty result emits just `# summary\nkey,value\n` so consumers
// get a valid CSV instead of zero bytes.
func Marshal(result any) ([]byte, error) {
	summary, series := extractSections(result)

	var buf bytes.Buffer
	w := encodingcsv.NewWriter(&buf)

	if err := writeSection(w, "# summary", summaryRows(summary)); err != nil {
		return nil, err
	}

	// Stable order across runs — Go map iteration is randomised.
	for _, name := range sortedKeys(series) {
		if err := w.Write([]string{}); err != nil {
			return nil, fmt.Errorf("blank separator: %w", err)
		}
		header, rows := seriesRows(series[name])
		if err := writeSection(w, "# series:"+name, append([][]string{header}, rows...)); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush: %w", err)
	}
	return buf.Bytes(), nil
}

// WriteAll writes one CSV file per logical table under `dir`:
// `summary.csv` plus `series_<name>.csv` per series key. Empty
// sections are still emitted (header-only) so downstream tooling can
// rely on the file existing.
func WriteAll(dir string, result any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	summary, series := extractSections(result)

	// summaryRows already prepends the `key,value` header row.
	if err := writeFile(filepath.Join(dir, "summary.csv"), summaryRows(summary)); err != nil {
		return err
	}
	for _, name := range sortedKeys(series) {
		header, rows := seriesRows(series[name])
		path := filepath.Join(dir, "series_"+sanitiseFilename(name)+".csv")
		if err := writeFile(path, append([][]string{header}, rows...)); err != nil {
			return err
		}
	}
	return nil
}

// extractSections accepts either `models.AnalysisResult`, a pointer
// to one, or a `map[string]any` envelope and returns its `summary`
// and `series` maps. Anything else yields empty sections — Marshal
// stays best-effort because the Tier-4 exporters all share this entry
// point.
func extractSections(result any) (summary, series map[string]any) {
	switch v := result.(type) {
	case models.AnalysisResult:
		return cloneMap(v.Summary), cloneMap(v.Series)
	case *models.AnalysisResult:
		if v == nil {
			return map[string]any{}, map[string]any{}
		}
		return cloneMap(v.Summary), cloneMap(v.Series)
	case map[string]any:
		s, _ := v["summary"].(map[string]any)
		r, _ := v["series"].(map[string]any)
		return cloneMap(s), cloneMap(r)
	default:
		// Fall back to JSON round-trip for arbitrary structs whose
		// JSON shape matches the AnalysisResult envelope.
		body, err := json.Marshal(result)
		if err != nil {
			return map[string]any{}, map[string]any{}
		}
		var generic map[string]any
		if err := json.Unmarshal(body, &generic); err != nil {
			return map[string]any{}, map[string]any{}
		}
		s, _ := generic["summary"].(map[string]any)
		r, _ := generic["series"].(map[string]any)
		return cloneMap(s), cloneMap(r)
	}
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// summaryRows flattens the summary map to a list of [key, value]
// pairs sorted by key. Non-scalar values (maps, slices) are JSON-
// encoded so the cell still round-trips. The header row is added by
// the caller.
func summaryRows(summary map[string]any) [][]string {
	rows := make([][]string, 0, len(summary)+1)
	rows = append(rows, []string{"key", "value"})
	for _, k := range sortedKeys(summary) {
		rows = append(rows, []string{k, formatScalar(summary[k])})
	}
	return rows
}

// seriesRows flattens one `series[name]` value into (header, rows).
//
// Cases handled:
//   - []map[string]any / []any of maps → wide table; columns = union
//     of keys across all rows, sorted alphabetically.
//   - []any of scalars → single-column "value" table.
//   - map[string]any → 2-column key/value table (same shape as
//     summary).
//   - anything else → 1-row "value" table with a JSON-encoded cell.
func seriesRows(value any) (header []string, rows [][]string) {
	switch v := value.(type) {
	case []map[string]any:
		return tableFromRecords(toAnySlice(v))
	case []any:
		return tableFromAnySlice(v)
	case map[string]any:
		header = []string{"key", "value"}
		for _, k := range sortedKeys(v) {
			rows = append(rows, []string{k, formatScalar(v[k])})
		}
		return header, rows
	default:
		// Use reflection so we accept []SomeStruct / []map[string]string
		// without forcing every analyzer to emit []any.
		if rv := reflect.ValueOf(value); rv.IsValid() && rv.Kind() == reflect.Slice {
			converted := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				converted[i] = rv.Index(i).Interface()
			}
			return tableFromAnySlice(converted)
		}
		return []string{"value"}, [][]string{{formatScalar(value)}}
	}
}

func toAnySlice(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, m := range in {
		out[i] = m
	}
	return out
}

// tableFromAnySlice picks between the records-table layout and the
// scalar-list layout based on whether every element is a map.
func tableFromAnySlice(items []any) (header []string, rows [][]string) {
	if len(items) == 0 {
		return []string{"value"}, nil
	}
	allMaps := true
	for _, it := range items {
		if _, ok := asStringMap(it); !ok {
			allMaps = false
			break
		}
	}
	if allMaps {
		return tableFromRecords(items)
	}
	header = []string{"value"}
	for _, it := range items {
		rows = append(rows, []string{formatScalar(it)})
	}
	return header, rows
}

// tableFromRecords expects every item to be coercible to a string-
// keyed map and emits a wide table with the union of keys as columns.
func tableFromRecords(items []any) (header []string, rows [][]string) {
	keySet := map[string]struct{}{}
	maps := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m, ok := asStringMap(it)
		if !ok {
			// A non-map element — fall back to scalar rendering for
			// robustness rather than panicking.
			maps = append(maps, map[string]any{"value": it})
			keySet["value"] = struct{}{}
			continue
		}
		maps = append(maps, m)
		for k := range m {
			keySet[k] = struct{}{}
		}
	}
	header = make([]string, 0, len(keySet))
	for k := range keySet {
		header = append(header, k)
	}
	sort.Strings(header)
	if len(header) == 0 {
		// Every record was empty — emit a single placeholder column
		// rather than zero columns, so the CSV stays well-formed.
		header = []string{"value"}
		for range maps {
			rows = append(rows, []string{""})
		}
		return header, rows
	}
	for _, m := range maps {
		row := make([]string, len(header))
		for i, k := range header {
			if val, ok := m[k]; ok {
				row[i] = formatScalar(val)
			}
		}
		rows = append(rows, row)
	}
	return header, rows
}

// asStringMap accepts map[string]any and (via reflection) any other
// map keyed by string so analyzer-specific record shapes (e.g.
// map[string]int) flatten cleanly.
func asStringMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Kind() != reflect.Map {
		return nil, false
	}
	if rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}

// formatScalar renders a single cell. Numbers use their narrowest
// natural form (no trailing `.0` for whole floats), bools use
// "true"/"false", nil renders as "", and complex values JSON-encode
// so spreadsheets at least see structured text rather than a Go
// pointer-print.
func formatScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.FormatInt(int64(x), 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return string(x)
	case []byte:
		return string(x)
	default:
		body, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(body)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// writeSection emits a `# section` comment row followed by the
// supplied rows (which include their own header as the first entry).
func writeSection(w *encodingcsv.Writer, header string, rows [][]string) error {
	if err := w.Write([]string{header}); err != nil {
		return fmt.Errorf("section header: %w", err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("row: %w", err)
		}
	}
	return nil
}

// writeFile is the WriteAll-mode helper: open a file, dump `rows`
// through `encoding/csv`, close.
func writeFile(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	w := encodingcsv.NewWriter(f)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write row %s: %w", path, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}
	return nil
}

// sanitiseFilename keeps `series_<name>.csv` portable across OSes —
// only `[A-Za-z0-9._-]` survive; everything else collapses to '_'.
func sanitiseFilename(s string) string {
	if s == "" {
		return "unnamed"
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.', c == '_', c == '-':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
