package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	profile "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/common"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
)

func visualText(value string) string {
	value = common.RedactIndexText(value)
	text, _, _ := resultcontract.DisplayText(value)
	return text
}
func visualCell(value string) resultcontract.Cell {
	return resultcontract.Cell{Text: visualText(value)}
}

// VisualView freezes existing immutable query output. Renderer/export code never
// reparses logs or recomputes observations; floating values are layout only.
func (s *Service) VisualView(ctx context.Context, handle, target string, q visuals.Query) (visuals.View, error) {
	out := visuals.View{TimeScope: "not declared; no clock alignment inferred", Version: visuals.Version, Kind: q.Kind, Query: q, Columns: []string{}, Rows: [][]resultcontract.Cell{}, Nodes: []visuals.Node{}, Warnings: []string{"Declared source identity; no implicit source authentication or clock alignment.", "Known-pattern index masking; inspect the artifact before sharing."}}
	if q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 100_000_000 {
		return out, fmt.Errorf("VISUAL_QUERY_INVALID: limit 1..100 and nonnegative offset required")
	}
	owner, e := s.AcquireResult(handle)
	if e != nil {
		return out, e
	}
	defer s.ReleaseResultLease(owner.ID)
	body, result, e := s.Read(handle)
	if e != nil {
		return out, e
	}
	digest := bytesDigest(body)
	if verdict := s.results.Check(result, "display"); !verdict.Allowed {
		return out, fmt.Errorf("VISUAL_RESULT_UNSUPPORTED: %s", verdict.Reason)
	}
	out.Sources = []visuals.Source{{SHA256: digest, Type: result.Type, Payload: result.Metadata.SchemaVersion}}
	out.Unit = "per-cell units"
	out.Denominator = "not declared by provider"
	out.Population = "per-cell population metadata; no cross-row sum"
	out.ScopeLabel = fmt.Sprintf("provider=%s; requested rows %d..%d; selected columns=%d", q.Provider, q.Offset, q.Offset+q.Limit, len(q.Columns))
	out.QualityLabel = "Provider cell availability/truncation retained; no quality certification"
	out.Quality = "Provider/query quality and truncation are retained; plotted areas are approximate, exact labels and tables are authoritative."
	switch q.Kind {
	case "provider_table":
		if q.Provider == "" {
			return out, fmt.Errorf("VISUAL_PROVIDER_REQUIRED")
		}
		page, e := s.QueryResult(ctx, handle, q.Provider, q.Offset, q.Limit)
		if e != nil {
			return out, e
		}
		if page.ResultSHA256 != digest {
			return out, fmt.Errorf("VISUAL_RESULT_STALE")
		}
		indexes := []int{}
		selected := q.Columns
		if len(selected) == 0 {
			selected = page.Columns
		}
		if len(selected) == 0 || len(selected) > 8 {
			return out, fmt.Errorf("VISUAL_COLUMNS_REQUIRED: explicitly select at most 8 columns")
		}
		seen := map[string]bool{}
		for _, name := range selected {
			if seen[name] {
				return out, fmt.Errorf("VISUAL_COLUMNS_INVALID")
			}
			seen[name] = true
			found := -1
			for i, c := range page.Columns {
				if c == name {
					found = i
				}
			}
			if found < 0 {
				return out, fmt.Errorf("VISUAL_COLUMN_MISSING")
			}
			indexes = append(indexes, found)
			out.Columns = append(out.Columns, visualText(name))
		}
		out.Query.Columns = append([]string(nil), selected...)
		out.ScopeLabel = fmt.Sprintf("provider=%s; selected columns=%s; requested rows %d..%d", q.Provider, strings.Join(out.Columns, ","), q.Offset, q.Offset+q.Limit)
		out.Total = page.Total
		out.Offset = page.Offset
		for _, row := range page.Rows {
			cells := []resultcontract.Cell{}
			for _, i := range indexes {
				cell := row[i]
				cell.Text = visualText(cell.Text)
				cell.Unit = visualText(cell.Unit)
				cell.Reason = visualText(cell.Reason)
				cell.Population = visualText(cell.Population)
				cells = append(cells, cell)
			}
			out.Rows = append(out.Rows, cells)
		}
		if page.Truncated || page.Escaped {
			out.Warnings = append(out.Warnings, "Provider display limits/control escaping apply; this is a bounded row/column selection.")
		}
	case "profile_top", "profile_flamegraph":
		if verdict := s.results.Check(result, "compute"); !verdict.Allowed {
			return out, fmt.Errorf("VISUAL_COMPUTE_UNAVAILABLE: %s", verdict.Reason)
		}
		core, e := profile.ReadCore(result)
		if e != nil {
			return out, e
		}
		query := profilemodel.Query{Selection: core.Selection, View: "top", Sort: "self", Limit: q.Limit}
		if q.Profile != nil {
			query = *q.Profile
			query.Limit = q.Limit
		}
		if visualText(query.Text) != query.Text {
			return out, fmt.Errorf("VISUAL_SENSITIVE_QUERY: clear redactable text filters or use an immutable selected result")
		}
		if q.Kind == "profile_flamegraph" {
			query.View = "tree"
			query.Sort = ""
		} else if query.View != "top" && query.View != "callers" && query.View != "callees" && query.View != "evidence" {
			return out, fmt.Errorf("VISUAL_PROFILE_VIEW_UNSUPPORTED")
		}
		page, e := profilemodel.QuerySet(ctx, core.Model, digest, query)
		if e != nil {
			return out, e
		}
		if query.TimeFrom != nil || query.TimeTo != nil {
			from, to := "unknown", "unknown"
			if query.TimeFrom != nil {
				from = *query.TimeFrom
			}
			if query.TimeTo != nil {
				to = *query.TimeTo
			}
			out.TimeScope = "profile-clock evidence window [" + from + "," + to + ")"
		}
		out.ScopeLabel = fmt.Sprintf("profiles=%s; metric=%s; view=%s; sort=%s; path filters=%d", strings.Join(query.Selection.ProfileIDs, ","), query.Selection.MetricID, query.View, query.Sort, len(query.Selection.Filters))
		known := func(value *string) string {
			if value == nil {
				return "unknown"
			}
			return *value
		}
		out.QualityLabel = fmt.Sprintf("%s; lost=%s; unknown=%s; truncated=%s; symbols=%s; clock=%s; conversion flags=%d", page.Availability, known(page.Quality.Lost), known(page.Quality.Unknown), known(page.Quality.Truncated), known(page.Quality.SymbolResolution), page.Quality.ClockConfidence, len(page.Quality.ConversionLoss))
		out.Query.Profile = &query
		out.Unit = page.Metric.Unit
		out.Denominator = page.Denominator
		out.Population = page.Population
		quality, _ := json.Marshal(page.Quality)
		out.Quality = string(quality)
		out.Total = page.Matched
		out.Offset = 0
		if query.Cursor != "" {
			var cursor struct{ Offset int }
			raw, _ := base64.RawURLEncoding.DecodeString(query.Cursor)
			json.Unmarshal(raw, &cursor)
			out.Offset = cursor.Offset
		}
		out.Columns = []string{"Frame / evidence", "Self", "Inclusive", "Unit", "Reference"}
		for _, row := range page.Rows {
			out.Rows = append(out.Rows, []resultcontract.Cell{visualCell(row.Name), {Text: row.Self}, {Text: row.Inclusive}, visualCell(page.Metric.Unit), visualCell(row.SourceRef + row.FrameID)})
			if q.Kind == "profile_flamegraph" && row.LayoutX != nil && row.LayoutWidth != nil {
				out.Nodes = append(out.Nodes, visuals.Node{Name: visualText(row.Name), Value: row.Inclusive, Self: row.Self, Evidence: row.FrameID, X: *row.LayoutX, Width: *row.LayoutWidth, Depth: row.Depth})
			}
		}
		if q.Kind == "profile_flamegraph" && len(out.Rows) > 0 && len(out.Nodes) == 0 {
			return out, fmt.Errorf("VISUAL_LAYOUT_UNAVAILABLE: signed/unavailable weights do not have an area layout")
		}
		if page.Next != "" {
			out.Warnings = append(out.Warnings, "Only this query page is drawn; hidden nodes retain their full-selection space. Use a focused immutable selection for detail.")
		}
		if page.Availability != "observed" {
			out.Warnings = append(out.Warnings, "Profile availability: "+page.Availability+" "+page.Reason)
		}
	case "profile_diff":
		if target == "" {
			return out, fmt.Errorf("VISUAL_TARGET_REQUIRED")
		}
		targetOwner, e := s.AcquireResult(target)
		if e != nil {
			return out, e
		}
		defer s.ReleaseResultLease(targetOwner.ID)
		targetBody, targetResult, e := s.Read(target)
		if e != nil {
			return out, e
		}
		difference, e := s.ProfileCompare(ctx, handle, target)
		if e != nil {
			return out, e
		}
		if difference.Baseline.ResultSHA256 != digest || difference.Target.ResultSHA256 != bytesDigest(targetBody) {
			return out, fmt.Errorf("VISUAL_RESULT_STALE")
		}
		out.Sources = append(out.Sources, visuals.Source{SHA256: bytesDigest(targetBody), Type: targetResult.Type, Payload: targetResult.Metadata.SchemaVersion})
		out.Columns = []string{"Frame", "Before", "After", "Delta", "Unit", "Reference"}
		names := map[string]string{}
		if core, err := profile.ReadCore(result); err == nil {
			for _, frame := range core.Model.Frames {
				names[frame.ID] = frame.Display
			}
		}
		out.Unit = difference.Metric.Unit
		out.Population = difference.Population
		out.ScopeLabel = "Strict comparison of the two saved selections; separate baseline/target populations"
		out.QualityLabel = fmt.Sprintf("declared identity; quality differences=%d; comparison warnings=%d", len(difference.QualityDifferences), len(difference.Warnings))
		out.Denominator = "separate source populations; no common denominator"
		quality, err := json.Marshal(map[string]any{"baseline": difference.Baseline.Quality, "target": difference.Target.Quality, "quality_differences": difference.QualityDifferences, "identity_basis": difference.IdentityBasis, "comparison_mode": difference.ComparisonMode, "baseline_population": difference.Baseline.Population, "target_population": difference.Target.Population, "baseline_capture_policy": difference.Baseline.CapturePolicy, "target_capture_policy": difference.Target.CapturePolicy})
		if err != nil {
			return out, err
		}
		out.Quality = string(quality)
		out.Total = len(difference.Rows)
		out.Offset = q.Offset
		if q.Offset > out.Total {
			return out, fmt.Errorf("VISUAL_OFFSET_INVALID")
		}
		for _, row := range difference.Rows[q.Offset:min(q.Offset+q.Limit, out.Total)] {
			name := names[row.FrameID]
			if name == "" {
				name = row.FrameID + row.ID
			}
			out.Rows = append(out.Rows, []resultcontract.Cell{visualCell(name), {Text: row.Before}, {Text: row.After}, {Text: row.Delta}, visualCell(out.Unit), visualCell(row.FrameID + row.ID)})
		}
		out.Warnings = append(out.Warnings, difference.Warnings...)
	default:
		return out, fmt.Errorf("VISUAL_KIND_UNSUPPORTED")
	}
	if len(out.Warnings) > 32 {
		out.Warnings = out.Warnings[:32]
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	again, _, e := s.Read(handle)
	if e != nil {
		return out, e
	}
	if bytesDigest(again) != digest {
		return out, fmt.Errorf("VISUAL_RESULT_STALE")
	}
	return out, out.Validate()
}
