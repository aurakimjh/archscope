package resultcontract

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"reflect"
	"sort"
)

// Independent output ceilings prevent a bounded table from expanding into an
// unbounded rows-by-columns appendix. Overflow refuses the whole projection.
const MaxProjectionEntries = 100_000
const MaxProjectionBytes = 16 << 20

type ProjectionBudgetError struct{ Resource string }

func (e *ProjectionBudgetError) Error() string {
	return "EXPORT_PROJECTION_BUDGET_EXCEEDED: " + e.Resource
}

// ExportPresentation is a disposable display projection, never a stored result
// or input for calculations. Exact strings remain strings; null CSV cells stay
// empty and their availability/reason/population travel as a separate table.
func ExportPresentation(r models.AnalysisResult, html bool) (models.AnalysisResult, error) {
	m, e := Measurements(r)
	if e != nil {
		return r, e
	}
	// Count using slice lengths before making either the result copy or appendix.
	entries := 0
	for path, d := range m {
		section, key, e := pointer(path)
		if e != nil {
			return r, e
		}
		if section == "summary" {
			if entries == MaxProjectionEntries {
				return r, &ProjectionBudgetError{"entry count"}
			}
			entries++
			continue
		}
		var target any
		if section == "series" {
			target = r.Series[key]
		} else {
			target = r.Tables[key]
		}
		v := reflect.ValueOf(target)
		if !v.IsValid() || v.Kind() != reflect.Slice {
			return r, fmt.Errorf("collection must be rows")
		}
		columns := len(d.Columns)
		if columns > 0 && v.Len() > (MaxProjectionEntries-entries)/columns {
			return r, &ProjectionBudgetError{"entry count"}
		}
		entries += v.Len() * columns
	}
	b, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	copy, e := Decode(b)
	if e != nil {
		return r, e
	}
	if len(m) == 0 {
		return copy, nil
	}
	if _, collision := copy.Series["measurement_availability"]; collision {
		return r, fmt.Errorf("export projection conflicts with existing series.measurement_availability")
	}
	rows := make([]map[string]any, 0, entries)
	projectionBytes := 2 // JSON array brackets
	appendRow := func(row map[string]any) error {
		body, e := json.Marshal(row)
		if e != nil {
			return e
		}
		if len(body)+1 > MaxProjectionBytes-projectionBytes {
			return &ProjectionBudgetError{"byte count"}
		}
		projectionBytes += len(body) + 1 // conservative comma, including the last row
		rows = append(rows, row)
		return nil
	}
	paths := make([]string, 0, len(m))
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		d := m[path]
		section, key, e := pointer(path)
		if e != nil {
			return r, e
		}
		if section != "summary" {
			var target any
			if section == "series" {
				target = copy.Series[key]
			} else {
				target = copy.Tables[key]
			}
			values, ok := target.([]any)
			if !ok {
				return r, fmt.Errorf("collection must be rows")
			}
			columns := make([]string, 0, len(d.Columns))
			for col := range d.Columns {
				columns = append(columns, col)
			}
			sort.Strings(columns)
			for index, value := range values {
				row, ok := value.(map[string]any)
				if !ok {
					return r, fmt.Errorf("collection must be object rows")
				}
				for _, col := range columns {
					cm := d.Columns[col]
					availability, reason := cm.Availability, cm.Reason
					if cm.AvailabilityColumn != "" {
						availability, _ = row[cm.AvailabilityColumn].(string)
					}
					if cm.ReasonColumn != "" {
						reason, _ = row[cm.ReasonColumn].(string)
					}
					if e = appendRow(map[string]any{"path": path, "row": index, "column": col, "availability": availability, "reason": reason, "unit": cm.Unit, "population": cm.Population, "numeric_encoding": cm.Encoding}); e != nil {
						return r, e
					}
					if html && availability == "unavailable" {
						if v, present := row[col]; present && v == nil {
							row[col] = "—"
						}
					}
				}
			}
			continue
		}
		if e = appendRow(map[string]any{"path": path, "availability": d.Availability, "reason": d.Reason, "unit": d.Unit, "population": d.Population, "numeric_encoding": d.Encoding}); e != nil {
			return r, e
		}
		if html && d.Availability == "unavailable" {
			if value, present := copy.Summary[key]; present && value == nil {
				copy.Summary[key] = "—"
			}
		}
	}
	copy.Series["measurement_availability"] = rows
	if html {
		return addMetricIntervals(copy)
	}
	return copy, nil
}
