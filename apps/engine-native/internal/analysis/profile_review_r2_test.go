package analysis_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
)

func TestR5ProfileOversizedStringHasBudgetJobDetail(t *testing.T) {
	s, _, root := setup(t, nil)
	raw, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	var model map[string]any
	json.Unmarshal(raw, &model)
	model["extensions"] = map[string]any{"synthetic_padding": strings.Repeat("x", (4<<20)+1)}
	body, e := json.Marshal(model)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "over-string.json")
	os.WriteFile(path, body, 0600)
	started := start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: json.RawMessage(`{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`)})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	job, e := s.Wait(ctx, started.ID, started.Generation)
	if e != nil || job.State != "failed" || job.ResultHandle != "" || job.Problem == nil || job.Problem.Code != "IMPORT_BUDGET_EXCEEDED" || job.Problem.Detail == nil {
		t.Fatal(job, e)
	}
	detail := job.Problem.Detail
	if detail.ReasonCode != "profile_json_string_bytes" || detail.Resource != "json_string_bytes" || detail.Actual == nil || *detail.Actual != "4194305" || detail.Limit == nil || *detail.Limit != "4194304" {
		t.Fatal(detail)
	}
	clean(t, root)
}
