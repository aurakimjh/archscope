package html

import (
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jenniferprofile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Export the stored projection with the CLI's scope disclosure and composition
// report. No source artifact is reopened or numerical analysis repeated here.
func marshalJenniferDrilldown(payload map[string]any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var result models.AnalysisResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return []byte(jenniferprofile.RenderHTMLReport(result)), nil
}
