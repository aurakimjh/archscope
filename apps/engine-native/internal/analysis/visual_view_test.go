package analysis_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
)

func TestVisualProfileAndProviderFreezeExactSelection(t *testing.T) {
	service, _, _ := setup(t, nil)
	job := importModel(t, service, "duration")
	ctx := context.Background()
	view, e := service.VisualView(ctx, job.ResultHandle, "", visuals.Query{Kind: "profile_top", Limit: 25})
	if e != nil {
		t.Fatal(e)
	}
	if view.Denominator != "0.35" || view.Unit != "seconds" || view.Query.Profile.Sort != "self" || len(view.Rows) == 0 {
		t.Fatal(view)
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "float64") {
		t.Fatal("weight coercion")
	}
	tree, e := service.VisualView(ctx, job.ResultHandle, "", visuals.Query{Kind: "profile_flamegraph", Limit: 25})
	if e != nil || len(tree.Nodes) == 0 || tree.Denominator != view.Denominator {
		t.Fatal(tree, e)
	}
	difference, e := service.VisualView(ctx, job.ResultHandle, job.ResultHandle, visuals.Query{Kind: "profile_diff", Limit: 25})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(difference.Quality, `"baseline":`) || !strings.Contains(difference.Quality, `"target":`) || len(difference.Sources) != 2 {
		t.Fatal("comparison quality/source provenance missing")
	}
	provider, e := service.VisualView(ctx, job.ResultHandle, "", visuals.Query{Kind: "provider_table", Provider: "generic:summary", Limit: 25, Columns: []string{"key", "value"}})
	if e != nil || len(provider.Rows) == 0 {
		t.Fatal(provider, e)
	}
	if _, e = service.VisualView(ctx, job.ResultHandle, "", visuals.Query{Kind: "provider_table", Provider: "generic:summary", Limit: 25, Columns: []string{"does-not-exist"}}); e == nil {
		t.Fatal("unknown column accepted")
	}
	spec, e := visuals.NewSpec(view, visuals.DefaultPresentation(), "")
	if e != nil {
		t.Fatal(e)
	}
	changed := view
	changed.Denominator = "35"
	if spec.Validate(changed, false) == nil {
		t.Fatal("wrong denominator accepted")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = service.VisualView(cancelCtx, job.ResultHandle, "", visuals.Query{Kind: "profile_top", Limit: 25}); e == nil {
		t.Fatal("cancellation ignored")
	}
}
