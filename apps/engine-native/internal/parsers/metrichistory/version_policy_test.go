package metrichistory_test

import (
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	csvexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	htmlexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	rc "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"strings"
	"testing"
)

func TestT647CapturedVersionsRemainAuthoritative(t *testing.T) {
	for _, c := range []struct{ input, key, version string }{{"system-db-postgres-capture.json", "postgres_version", "16.13"}, {"system-db-linux-ss-capture.json", "iproute2_version", "6.2.0"}, {"apple-ollama-capture.json", "ollama_version", "0.40.17"}} {
		t.Run(c.key, func(t *testing.T) {
			ctx := context.Background()
			raw, err := os.ReadFile("../../catalog/contracts/" + c.input)
			if err != nil {
				t.Fatal(err)
			}
			raw = changedCapture(t, raw, func(v map[string]any) {
				v[c.key] = c.version
				if c.key == "postgres_version" {
					v["server_version_num"] = "160013"
				}
			})
			if _, err = p.ParseBytes(ctx, raw, "capture", p.Options{}); err == nil {
				t.Fatal("implicit qualification")
			}
			o := p.Options{AllowUnverifiedVersion: true}
			set, err := p.ParseBytes(ctx, raw, "capture", o)
			if err != nil {
				t.Fatal(err)
			}
			if set.Sources[0].ProducerVersion != c.version || set.Sources[0].Qualification != "unverified_version" {
				t.Fatal(set.Sources)
			}
			b, _ := json.Marshal(set)
			set, err = m.Decode(b)
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, s := range set.Series {
				ids = append(ids, s.ID)
			}
			result, err := a.Build(ctx, set, m.Selection{SeriesIDs: ids}, "capture")
			if err != nil {
				t.Fatal(err)
			}
			b, _ = json.Marshal(result)
			if !strings.Contains(string(b), `"qualification":"unverified_version"`) {
				t.Fatal("lost qualification")
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
			o.ProducerVersion = "conflicting"
			if _, err = p.ParseBytes(ctx, raw, "capture", o); err == nil {
				t.Fatal("declared version overrides capture")
			}
			o.ProducerVersion = ""
			bad := changedCapture(t, raw, func(v map[string]any) { v["snapshots"] = []any{} })
			if _, err = p.ParseBytes(ctx, bad, "capture", o); err == nil {
				t.Fatal("opt-in relaxed snapshots")
			}
			set.Sources[0].Qualification = "verified"
			if set.Validate(ctx) == nil {
				t.Fatal("forged qualification")
			}
		})
	}
}
