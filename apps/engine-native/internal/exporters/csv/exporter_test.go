package csv

import (
	encodingcsv "encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// readCSV parses bytes through encoding/csv with FieldsPerRecord=-1
// because our single-CSV layout intentionally mixes section comments,
// summary rows, and series tables of different widths.
func readCSV(t *testing.T, body []byte) [][]string {
	t.Helper()
	r := encodingcsv.NewReader(strings.NewReader(string(body)))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	return rows
}

// findRow returns the index of the first row whose first cell equals
// `marker`, or -1.
func findRow(rows [][]string, marker string) int {
	for i, r := range rows {
		if len(r) > 0 && r[0] == marker {
			return i
		}
	}
	return -1
}

func TestMarshalFlattensSummaryAndSeries(t *testing.T) {
	result := models.New("access_log", "nginx")
	result.Summary = map[string]any{
		"total_requests":  42,
		"avg_response_ms": 123.4,
		"label":           "한글",
	}
	result.Series = map[string]any{
		"requests_per_minute": []map[string]any{
			{"time": "2026-04-29T10:00:00+0900", "value": 1},
			{"time": "2026-04-29T10:01:00+0900", "value": 7},
		},
	}

	body, err := Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rows := readCSV(t, body)

	// Summary header.
	if findRow(rows, "# summary") < 0 {
		t.Fatalf("missing # summary header in:\n%s", body)
	}

	// Verify summary rows present (sorted by key).
	want := map[string]string{
		"avg_response_ms": "123.4",
		"label":           "한글",
		"total_requests":  "42",
	}
	for k, v := range want {
		found := false
		for _, r := range rows {
			if len(r) == 2 && r[0] == k && r[1] == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("summary row %s=%s not found in:\n%s", k, v, body)
		}
	}

	// Series section header + table.
	idx := findRow(rows, "# series:requests_per_minute")
	if idx < 0 {
		t.Fatalf("missing # series:requests_per_minute header in:\n%s", body)
	}
	header := rows[idx+1]
	if len(header) != 2 || header[0] != "time" || header[1] != "value" {
		t.Errorf("series header = %v, want [time value]", header)
	}
	// Two data rows.
	if rows[idx+2][1] != "1" || rows[idx+3][1] != "7" {
		t.Errorf("series data rows wrong: %v / %v", rows[idx+2], rows[idx+3])
	}
}

func TestMarshalNonASCIIRoundTrips(t *testing.T) {
	result := models.New("access_log", "nginx")
	result.Summary = map[string]any{"label": "한글, comma"}
	body, err := Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "한글, comma") {
		t.Errorf("non-ASCII not preserved verbatim: %q", body)
	}
	// And the comma-bearing value must be quoted by encoding/csv.
	if !strings.Contains(string(body), `"한글, comma"`) {
		t.Errorf("comma cell should be quoted: %q", body)
	}
}

func TestMarshalEmptyResult(t *testing.T) {
	result := models.New("empty", "none")
	body, err := Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rows := readCSV(t, body)
	if findRow(rows, "# summary") != 0 {
		t.Errorf("expected # summary as first row, got %v", rows)
	}
	// Header row "key,value" should follow even with zero pairs.
	if len(rows) < 2 || rows[1][0] != "key" || rows[1][1] != "value" {
		t.Errorf("expected key,value header row; got %v", rows)
	}
	// And no series sections (since Series is empty).
	for _, r := range rows {
		if len(r) > 0 && strings.HasPrefix(r[0], "# series:") {
			t.Errorf("unexpected series section in empty result: %v", r)
		}
	}
}

func TestMarshalAcceptsPlainMap(t *testing.T) {
	payload := map[string]any{
		"summary": map[string]any{"hello": "world"},
		"series": map[string]any{
			"timeline": []any{
				map[string]any{"t": "00:00", "v": 1.5},
			},
		},
	}
	body, err := Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "hello") || !strings.Contains(string(body), "world") {
		t.Errorf("summary cells missing: %q", body)
	}
	if !strings.Contains(string(body), "# series:timeline") {
		t.Errorf("series header missing: %q", body)
	}
}

func TestMarshalSeriesScalarSlice(t *testing.T) {
	result := models.New("ext", "x")
	result.Series = map[string]any{
		"counts": []any{1, 2, 3},
	}
	body, err := Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rows := readCSV(t, body)
	idx := findRow(rows, "# series:counts")
	if idx < 0 {
		t.Fatalf("missing series header in:\n%s", body)
	}
	if rows[idx+1][0] != "value" {
		t.Errorf("expected single 'value' column header, got %v", rows[idx+1])
	}
	if rows[idx+2][0] != "1" || rows[idx+3][0] != "2" || rows[idx+4][0] != "3" {
		t.Errorf("scalar series rows wrong: %v", rows[idx+1:idx+5])
	}
}

func TestMarshalSeriesUnionOfKeys(t *testing.T) {
	// Records with different keys should emit a wide table whose
	// columns are the union, alphabetically sorted, with blank cells
	// where a record didn't carry that key.
	result := models.New("ext", "x")
	result.Series = map[string]any{
		"mixed": []map[string]any{
			{"a": 1, "b": 2},
			{"b": 3, "c": 4},
		},
	}
	body, err := Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rows := readCSV(t, body)
	idx := findRow(rows, "# series:mixed")
	if idx < 0 {
		t.Fatalf("series header missing")
	}
	header := rows[idx+1]
	if strings.Join(header, ",") != "a,b,c" {
		t.Errorf("header = %v, want [a b c]", header)
	}
	row1 := rows[idx+2]
	row2 := rows[idx+3]
	if row1[0] != "1" || row1[1] != "2" || row1[2] != "" {
		t.Errorf("row1 = %v, want [1 2 \"\"]", row1)
	}
	if row2[0] != "" || row2[1] != "3" || row2[2] != "4" {
		t.Errorf("row2 = %v, want [\"\" 3 4]", row2)
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "deeply", "nested", "result.csv")
	result := models.New("ext", "x")
	result.Summary = map[string]any{"k": "v"}
	if err := Write(out, result); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "k") {
		t.Errorf("written file missing summary content: %q", body)
	}
}

func TestWriteAllOneFilePerSeries(t *testing.T) {
	tmp := t.TempDir()
	result := models.New("access_log", "nginx")
	result.Summary = map[string]any{"total": 10}
	result.Series = map[string]any{
		"requests_per_minute": []map[string]any{{"t": "00:00", "v": 1}},
		"errors_per_minute":   []map[string]any{{"t": "00:00", "v": 0}},
	}

	if err := WriteAll(tmp, result); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}

	for _, name := range []string{"summary.csv", "series_requests_per_minute.csv", "series_errors_per_minute.csv"} {
		path := filepath.Join(tmp, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}

	// summary.csv should have a header + one data row.
	body, err := os.ReadFile(filepath.Join(tmp, "summary.csv"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	rows := readCSV(t, body)
	if len(rows) < 2 || rows[0][0] != "key" || rows[1][0] != "total" || rows[1][1] != "10" {
		t.Errorf("summary.csv unexpected rows: %v", rows)
	}

	// series file should have a 2-col header.
	body, err = os.ReadFile(filepath.Join(tmp, "series_requests_per_minute.csv"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	rows = readCSV(t, body)
	if len(rows) < 2 || strings.Join(rows[0], ",") != "t,v" {
		t.Errorf("series file header = %v", rows[0])
	}
	if rows[1][0] != "00:00" || rows[1][1] != "1" {
		t.Errorf("series row = %v", rows[1])
	}
}

func TestWriteAllSanitisesFilename(t *testing.T) {
	tmp := t.TempDir()
	result := models.New("ext", "x")
	result.Series = map[string]any{
		"weird/name with spaces": []any{1, 2},
	}
	if err := WriteAll(tmp, result); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	expected := filepath.Join(tmp, "series_weird_name_with_spaces.csv")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected sanitised filename %s, stat err: %v", expected, err)
	}
}

func TestFormatScalarShapes(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hi", "hi"},
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{int64(-7), "-7"},
		{uint(9), "9"},
		{1.5, "1.5"},
		{float64(2.0), "2"},
		{[]byte("bytes"), "bytes"},
		{map[string]any{"k": 1}, `{"k":1}`},
		{[]int{1, 2}, `[1,2]`},
	}
	for _, tc := range cases {
		got := formatScalar(tc.in)
		if got != tc.want {
			t.Errorf("formatScalar(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSeriesMapValueRendersAsKeyValue(t *testing.T) {
	// A series entry that's a map (not a slice) should render as a
	// 2-column key/value table, mirroring summary flattening.
	result := models.New("ext", "x")
	result.Series = map[string]any{
		"distribution": map[string]any{
			"RUNNABLE": 3,
			"BLOCKED":  1,
		},
	}
	body, err := Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rows := readCSV(t, body)
	idx := findRow(rows, "# series:distribution")
	if idx < 0 {
		t.Fatalf("series header missing in:\n%s", body)
	}
	if rows[idx+1][0] != "key" || rows[idx+1][1] != "value" {
		t.Errorf("expected key,value header for map series, got %v", rows[idx+1])
	}
	// Sorted by key: BLOCKED first, RUNNABLE second.
	if rows[idx+2][0] != "BLOCKED" || rows[idx+2][1] != "1" {
		t.Errorf("row1 = %v", rows[idx+2])
	}
	if rows[idx+3][0] != "RUNNABLE" || rows[idx+3][1] != "3" {
		t.Errorf("row2 = %v", rows[idx+3])
	}
}

func TestMarshalNilResult(t *testing.T) {
	body, err := Marshal(nil)
	if err != nil {
		t.Fatalf("Marshal(nil): %v", err)
	}
	rows := readCSV(t, body)
	if findRow(rows, "# summary") != 0 {
		t.Errorf("nil result should still produce a summary section: %v", rows)
	}
}
