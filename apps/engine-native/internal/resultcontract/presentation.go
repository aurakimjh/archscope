package resultcontract

import (
	"encoding/json"
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
