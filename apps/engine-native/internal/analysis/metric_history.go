package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

type MetricInventory struct {
	ResultSHA256 string      `json:"result_sha256"`
	Selection    m.Selection `json:"selection"`
	Series       []m.Series  `json:"series"`
	Sources      []m.Source  `json:"sources"`
	Warnings     []string    `json:"warnings"`
}

func withMetric[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, a.Core, string) (T, error)) (out T, err error) {
	ctx, finish, err := s.beginLoad(parent)
	if err != nil {
		return out, err
	}
	defer finish()
	body, result, err := s.Read(handle)
	if err != nil {
		return out, err
	}
	if verdict := s.results.Check(result, "compute"); !verdict.Allowed {
		return out, fmt.Errorf("%s", verdict.Reason)
	}
	actual := bytesDigest(body)
	if digest != "" && digest != actual {
		return out, fmt.Errorf("METRIC_RESULT_STALE")
	}
	core, err := a.ReadCore(result)
	if err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	return run(ctx, core, actual)
}
func (s *Service) MetricInventory(ctx context.Context, handle string) (MetricInventory, error) {
	return withMetric(s, ctx, handle, "", func(ctx context.Context, core a.Core, digest string) (MetricInventory, error) {
		return MetricInventory{digest, core.Selection, core.Model.Series, core.Model.Sources, core.Model.Warnings}, ctx.Err()
	})
}
func (s *Service) QueryMetricHistory(ctx context.Context, handle, digest string, q m.Query) (m.Page, error) {
	if digest == "" {
		return m.Page{}, fmt.Errorf("METRIC_RESULT_STALE")
	}
	return withMetric(s, ctx, handle, digest, func(ctx context.Context, core a.Core, actual string) (m.Page, error) {
		return m.QuerySet(ctx, core.Model, actual, q)
	})
}
func (s *Service) MaterializeMetricSelection(ctx context.Context, handle, digest string, selection m.Selection) (ResultView, ResultLease, error) {
	var view ResultView
	var lease ResultLease
	if digest == "" {
		return view, lease, fmt.Errorf("METRIC_RESULT_STALE")
	}
	body, err := withMetric(s, ctx, handle, digest, func(ctx context.Context, core a.Core, _ string) ([]byte, error) {
		result, e := a.Build(ctx, core.Model, selection, "explicit selected metric history")
		if e != nil {
			return nil, e
		}
		return json.Marshal(result)
	})
	if err != nil {
		return view, lease, err
	}
	return s.LoadOwnedSnapshot(ctx, body)
}
