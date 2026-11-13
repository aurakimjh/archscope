package gputrace_test

import (
	"context"
	"database/sql"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	csvexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	htmlexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gputrace"
	rc "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT647GPUExplicitVersionDoesNotRelaxStructure(t *testing.T) {
	for _, c := range []struct{ stem, input string }{{"pytorch-rank-0", "pytorch-rank-0.json"}, {"nsight-summary", "nsight-summary.csv"}, {"nsight", "nsight-sanitized.sqlite"}} {
		t.Run(c.stem, func(t *testing.T) {
			ctx := context.Background()
			o, err := p.DecodeOptions(fixture(t, c.stem+"-options.json"))
			if err != nil {
				t.Fatal(err)
			}
			raw := fixture(t, c.input)
			o.ProducerVersion = "other-version"
			if c.stem == "nsight" {
				path := filepath.Join(t.TempDir(), "source.sqlite")
				if err = os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				_, err = db.Exec("UPDATE META_DATA_EXPORT SET value='other-version' WHERE name='EXPORT_PRODUCT_VERSION'")
				if err != nil {
					t.Fatal(err)
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				raw, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err = p.ParseBytes(ctx, raw, o); err == nil {
				t.Fatal("implicit version expansion")
			}
			o.AllowUnverifiedVersion = true
			set, err := p.ParseBytes(ctx, raw, o)
			if err != nil {
				t.Fatal(err)
			}
			if set.Sources[0].Version != "other-version" || set.Sources[0].Qualification != "unverified_version" {
				t.Fatal(set.Sources)
			}
			b, _ := json.Marshal(set)
			set, err = g.Decode(ctx, b)
			if err != nil {
				t.Fatal(err)
			}
			result, err := a.Build(ctx, set, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			b, _ = json.Marshal(result)
			if !strings.Contains(string(b), `"qualification":"unverified_version"`) {
				t.Fatal("lost qualifier")
			}

			for _, html := range []bool{false, true} {
				view, err := rc.ExportPresentation(result, html)
				if err != nil {
					t.Fatal(err)
				}
				var output []byte
				if html {
					output, err = htmlexport.Marshal(view)
				} else {
					output, err = csvexport.Marshal(view)
				}
				if err != nil || !strings.Contains(string(output), "unverified_version") || !strings.Contains(string(output), "producer_version") {
					t.Fatal("export lost qualification/version", html, err)
				}
			}
			if _, err = p.ParseBytes(ctx, []byte(`{}`), o); err == nil {
				t.Fatal("relaxed structure")
			}
			if c.stem == "nsight" {
				o.ProducerVersion = "conflicting"
				if _, err = p.ParseBytes(ctx, raw, o); err == nil {
					t.Fatal("version conflict accepted")
				}
			}
			o.Format = "gpu-execution-json"
			if _, err = p.ParseBytes(ctx, b, o); err == nil {
				t.Fatal("canonical override accepted")
			}
		})
	}
}
