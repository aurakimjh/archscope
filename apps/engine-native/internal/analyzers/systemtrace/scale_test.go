package systemtrace_test

import (
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/systemtrace"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestT637Bounded10k20kSampleScale(t *testing.T) {
	for _, count := range []int{10000, 20000} {
		s := model(t, "perf-set.json")
		r := s.Records[0]
		s.Records = nil
		for i := 0; i < count; i++ {
			copy := r
			copy.Line = i + 1
			copy.Byte = int64(i * 100)
			copy.Time = m.Ptr(strconv.FormatInt(int64(i)*1000000+1, 10))
			copy.ID = m.RecordID(s.SourceSHA256, copy)
			s.Records = append(s.Records, copy)
		}
		result, e := a.Build(context.Background(), s, "synthetic scale")
		if e != nil {
			t.Fatal(e)
		}
		body, _ := json.Marshal(result)
		if len(body) > 64<<20 {
			t.Fatal("saved result cannot reopen")
		}
		projection, e := a.Project(context.Background(), s, "period/cpu-clock:HG", nil)
		if e != nil {
			t.Fatal(e)
		}
		tree, e := p.BuildTree(context.Background(), projection, p.Selection{MetricID: "period/cpu-clock:HG", ProfileIDs: []string{projection.Profiles[0].ID}})
		if e != nil || tree.Root.Inclusive != strconv.FormatInt(int64(count)*1000000, 10) {
			t.Fatal(tree, e)
		}
		if dir := os.Getenv("ARCHSCOPE_SYSTEM_SCALE_EVIDENCE_DIR"); dir != "" {
			os.MkdirAll(dir, 0700)
			os.WriteFile(filepath.Join(dir, strconv.Itoa(count)+"-result.json"), body, 0600)
		}
		t.Logf("synthetic samples=%d result bytes=%d denominator=%s ns", count, len(body), tree.Root.Inclusive)
	}
}
