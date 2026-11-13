package execution_test

import (
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	csvexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	htmlexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
	rc "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"strings"
	"testing"
)

func TestT647BenchmarkQualificationPersistsAndRefusesBaseline(t *testing.T) {
	for _, c := range []struct{ name, file string }{{"k6", "k6-1.3.0.json"}, {"llmperf", "llmperf-summary.json"}, {"jmeter", "jmeter-5.6.3.csv"}} {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			o, err := p.DecodeOptions(fixture(t, c.name+"-options.json"))
			if err != nil {
				t.Fatal(err)
			}
			raw := fixture(t, c.file)
			o.ProducerVersion = "other-version"
			if _, err = p.ParseBytes(ctx, raw, o); err == nil {
				t.Fatal("implicit version expansion")
			}
			o.AllowUnverifiedVersion = true
			for _, version := range []string{"other-version", ""} {
				o.ProducerVersion = version
				set, err := p.ParseBytes(ctx, raw, o)
				if err != nil {
					t.Fatal(err)
				}
				want := version
				if want == "" {
					want = "unknown"
				}
				if set.Source.Version != want || set.Source.Qualification != "unverified_version" {
					t.Fatal(set.Source)
				}
				b, _ := json.Marshal(set)
				loaded, err := e.Decode(ctx, b)
				if err != nil {
					t.Fatal(err)
				}
				result, err := a.Build(ctx, loaded, "fixture")
				if err != nil {
					t.Fatal(err)
				}
				b, _ = json.Marshal(result)
				if !strings.Contains(string(b), `"qualification":"unverified_version"`) {
					t.Fatal("qualification lost from result/export")
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
				comparison, err := e.Compare(ctx, loaded, loaded, strings.Repeat("a", 64), strings.Repeat("b", 64), 0, 10)
				if err != nil || comparison.Status != "incompatible" || !strings.Contains(strings.Join(comparison.Reasons, " "), "unverified") {
					t.Fatal(comparison, err)
				}
				set.Source.Qualification = "verified"
				if set.Validate(ctx) == nil {
					t.Fatal("forged qualification")
				}
			}
			if _, err = p.ParseBytes(ctx, []byte(`{"metrics":{}}`), o); err == nil {
				t.Fatal("opt-in bypassed structure")
			}
		})
	}
}
