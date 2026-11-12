package inputassist_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/inputassist"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestT633DiscoveryIsBoundedAndSidecarDoesNotInventContext(t *testing.T) {
	ctx := context.Background()
	c, _, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	os.WriteFile(path, body, 0600)
	// Invalid adjacent sidecars must never be discovered implicitly.
	os.WriteFile(path+".capture.json", []byte(`invalid`), 0600)
	report, e := inputassist.Discover(ctx, c, path, "")
	if e != nil || report.Detection.Selected != "server.tomcat" || report.Context.Status != "absent" || !slices.Contains(report.Context.Missing, "collection_start") {
		t.Fatal(report, e)
	}
	if report.ProbeBytes > ingestion.MaxProbeBytes || report.Source.SHA256 == "" {
		t.Fatal(report)
	}
	raw, _ := json.Marshal(report.Draft)
	schema, _ := cases.Contracts.ReadFile("contracts/capture-context.schema.json")
	validate, e := ingestion.CompileSchema(schema)
	if e != nil || validate(raw) != nil {
		t.Fatal("invalid generated draft", e, string(raw))
	}
	capture := report.Draft
	value := "123"
	capture.Entities = []cases.Entity{{ID: "unrelated", PID: &cases.Claim{Value: &value, Basis: "user"}, ProcessStart: &cases.Claim{Value: &value, Basis: "user"}, Host: &cases.Claim{Value: &value, Basis: "user"}}}
	sidecar := filepath.Join(dir, "explicit.json")
	write := func() { b, _ := json.Marshal(capture); os.WriteFile(sidecar, b, 0600) }
	write()
	report, e = inputassist.Discover(ctx, c, path, sidecar)
	if e != nil || report.Context.Status != "matched_declared" || !slices.Contains(report.Context.Missing, "process_lifetime") {
		t.Fatal(report, e)
	}
	capture.Files[0].EntityIDs = []string{"unrelated"}
	write()
	report, e = inputassist.Discover(ctx, c, path, sidecar)
	if e != nil || slices.Contains(report.Context.Missing, "process_lifetime") {
		t.Fatal(report, e)
	}
	capture.Files[0].SHA256 = string(bytes.Repeat([]byte("a"), 64))
	write()
	if _, e = inputassist.Discover(ctx, c, path, sidecar); e == nil {
		t.Fatal("accepted wrong sidecar bytes")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = inputassist.Discover(cancelled, c, path, ""); e == nil {
		t.Fatal("ignored cancellation")
	}
}
func TestT633JFRHintsDoNotAdvertiseGenericSupportAndKitIsSeparate(t *testing.T) {
	c, _, _ := catalog.Builtin()
	dir := t.TempDir()
	for _, body := range []string{"FLR\x00synthetic", `{"recording":{"events":[]}}`} {
		path := filepath.Join(dir, "input.jfr")
		os.WriteFile(path, []byte(body), 0600)
		report, e := inputassist.Discover(context.Background(), c, path, "")
		if e != nil || len(report.Advice) != 1 || report.Advice[0].Route != "jfr" || report.Detection.State != "unsupported" {
			t.Fatal(report, e)
		}
	}
	body, e := inputassist.Kit()
	if e != nil {
		t.Fatal(e)
	}
	again, _ := inputassist.Kit()
	if !bytes.Equal(body, again) {
		t.Fatal("kit not deterministic")
	}
	z, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		t.Fatal(e)
	}
	names := []string{}
	for _, file := range z.File {
		names = append(names, file.Name)
		if file.Mode()&0111 != 0 {
			t.Fatal("executable collector shipped")
		}
	}
	for _, required := range []string{"recipes.json", "INPUT_ASSISTANCE.en.md", "INPUT_ASSISTANCE.ko.md", "capture-context.schema.json"} {
		if !slices.Contains(names, required) {
			t.Fatal("missing", required)
		}
	}
}

func TestT633MappingPreviewKeepsExactSourceAndNormalizedValues(t *testing.T) {
	ctx := context.Background()
	base, _, _ := catalog.Builtin()
	session, e := catalog.OpenPackSession(ctx, t.TempDir(), base)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Manager.Close()
	snapshot, e := session.Provider("")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")
	for _, tc := range []struct{ source, normalized string }{{"12.50", "0.0125"}, {"9007199254740993", "9007199254740.993"}, {"0", "0"}} {
		body := []byte(`{"producer":"synthetic.metrics","producer_version":"1.0.0","metric":"latency","value_ms":"` + tc.source + `"}` + "\n")
		if e = os.WriteFile(path, body, 0600); e != nil {
			t.Fatal(e)
		}
		report, e := inputassist.Discover(ctx, snapshot.Catalog, path, "", snapshot.Mappings...)
		if e != nil || len(report.MappingPreviews) != 1 {
			t.Fatal(report, e)
		}
		preview := report.MappingPreviews[0]
		if preview.Status != "sample_only" || preview.SourceValue != tc.source || preview.NormalizedValue != tc.normalized || preview.Unit != "seconds" || preview.PackSHA256 == "" {
			t.Fatal(preview)
		}
	}
}
