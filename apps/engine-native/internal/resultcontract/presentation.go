package resultcontract

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"sort"
)

// ExportPresentation is a disposable display projection, never a stored result
// or input for calculations. Exact strings remain strings; null CSV cells stay
// empty and their availability/reason/population travel as a separate table.
func ExportPresentation(r models.AnalysisResult, html bool) (models.AnalysisResult, error) {
	b, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	copy, e := Decode(b)
	if e != nil {
		return r, e
	}
	m, e := Measurements(copy)
	if e != nil {
		return r, e
	}
	if len(m) == 0 {
		return copy, nil
	}
	if _, collision := copy.Series["measurement_availability"]; collision {
		return r, fmt.Errorf("export projection conflicts with existing series.measurement_availability")
	}
	rows := []map[string]any{}
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
					rows = append(rows, map[string]any{"path": path, "row": index, "column": col, "availability": availability, "reason": reason, "unit": cm.Unit, "population": cm.Population, "numeric_encoding": cm.Encoding})
					if html && availability == "unavailable" {
						if v, present := row[col]; present && v == nil {
							row[col] = "—"
						}
					}
				}
			}
			continue
		}
		rows = append(rows, map[string]any{"path": path, "availability": d.Availability, "reason": d.Reason, "unit": d.Unit, "population": d.Population, "numeric_encoding": d.Encoding})
		if html && d.Availability == "unavailable" {
			if value, present := copy.Summary[key]; present && value == nil {
				copy.Summary[key] = "—"
			}
		}
	}
	copy.Series["measurement_availability"] = rows
	return copy, nil
}
