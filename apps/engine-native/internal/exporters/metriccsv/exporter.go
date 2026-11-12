// Package metriccsv exports the dedicated, exact interval projection. Generic
// AnalysisResult CSV remains a summary projection.
package metriccsv

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"strconv"
	"strings"
	"unicode"
)

const MaxBytes = 64 << 20
const Version = "1.0.0"

// Text guards free-text spreadsheet cells. Exact, validated numeric fields are
// emitted unchanged; consumers must import those columns as text to avoid rounding.
func text(s string) string {
	first := strings.TrimLeftFunc(s, unicode.IsSpace)
	if first != "" && (strings.ContainsRune("=+-@", rune(first[0])) || strings.ContainsAny(s[:len(s)-len(first)], "\t\r\n")) {
		return "'" + s
	}
	return s
}
func optional(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func object(value any) string { body, _ := json.Marshal(value); return string(body) }

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > MaxBytes-b.Len() {
		return 0, fmt.Errorf("METRIC_CSV_BUDGET_EXCEEDED: limit=%d", MaxBytes)
	}
	return b.Buffer.Write(p)
}

// Encode selects whole intervals by [from,to) end timestamps, retaining context
// and provenance. It refuses over-budget output instead of truncating rows.
func Encode(ctx context.Context, set m.Set, selection m.Selection, digest string) ([]byte, error) {
	if len(digest) != 64 {
		return nil, fmt.Errorf("METRIC_CSV_DIGEST_INVALID")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return nil, fmt.Errorf("METRIC_CSV_DIGEST_INVALID")
	}
	rows, err := m.SelectRows(ctx, set, selection)
	if err != nil {
		return nil, err
	}
	sources := map[string]m.Source{}
	for _, s := range set.Sources {
		sources[s.ID] = s
	}
	series := map[string]m.Series{}
	for _, s := range set.Series {
		series[s.ID] = s
	}
	var out boundedBuffer
	writer := csv.NewWriter(&out)
	header := []string{"csv_version", "result_sha256", "normalizer", "window_policy", "selection_json", "interval_id", "series_id", "name", "unit", "kind", "temporality", "observed_start_ns", "start_ns", "end_ns", "time", "clock", "value", "special", "delta", "rate", "rate_meaning", "observed_histogram_json", "interval_histogram_json", "status", "gap", "reordered", "point_id", "previous_point_id", "refs_json", "sources_json", "series_json"}
	if err = writer.Write(header); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		refs := []m.Source{}
		seen := map[string]bool{}
		for _, ref := range row.Refs {
			if !seen[ref.SourceID] {
				refs = append(refs, sources[ref.SourceID])
				seen[ref.SourceID] = true
			}
		}
		cells := []string{Version, digest, set.Normalizer, "end_timestamp_selection; intervals are not clipped or redistributed", object(selection), row.ID, row.SeriesID, text(row.Name), text(row.Unit), row.Kind, row.Temporality, optional(row.ObservedStart), optional(row.Start), optional(row.End), optional(row.Timestamp), row.Clock, optional(row.Value), row.Special, optional(row.Delta), optional(row.Rate), row.RateMeaning, object(row.ObservedHistogram), object(row.Histogram), row.Status, strconv.FormatBool(row.Gap), strconv.FormatBool(row.Reordered), row.PointID, row.PreviousPointID, object(row.Refs), object(refs), object(series[row.SeriesID])}
		if err = writer.Write(cells); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
