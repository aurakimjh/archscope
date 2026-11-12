package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/metriccsv"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	source "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
)

type MetricInventory struct {
	ResultSHA256 string      `json:"result_sha256"`
	Selection    m.Selection `json:"selection"`
	Series       []m.Series  `json:"series"`
	Sources      []m.Source  `json:"sources"`
	Warnings     []string    `json:"warnings"`
}

func withMetric[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, *metricSemantic, string) (T, error)) (out T, err error) {
	ctx, finish, err := s.beginLoad(parent)
	if err != nil {
		return out, err
	}
	defer finish()
	owner, err := s.AcquireResult(handle)
	if err != nil {
		return out, err
	}
	defer s.ReleaseResultLease(owner.ID)
	body, actual, err := s.readVerifiedResult(ctx, handle)
	if err != nil {
		return out, err
	}
	if digest != "" && digest != actual {
		return out, fmt.Errorf("METRIC_RESULT_STALE")
	}
	prepared, err := s.preparedMetric(ctx, body, actual)
	if err != nil {
		return out, err
	}
	return run(ctx, prepared, actual)
}
func (s *Service) MetricInventory(ctx context.Context, handle string) (MetricInventory, error) {
	return withMetric(s, ctx, handle, "", func(ctx context.Context, prepared *metricSemantic, digest string) (MetricInventory, error) {
		core := prepared.core
		value := MetricInventory{digest, core.Selection, core.Model.Series, core.Model.Sources, append(source.InspectionWarnings(core.Model), "Queries use only series retained in this result; reopen the original to add omitted series.")}
		raw, e := json.Marshal(value)
		if e != nil {
			return MetricInventory{}, e
		}
		var owned MetricInventory
		if e = json.Unmarshal(raw, &owned); e != nil {
			return owned, e
		}
		return owned, ctx.Err()
	})
}
func (s *Service) QueryMetricHistory(ctx context.Context, handle, digest string, q m.Query) (m.Page, error) {
	if digest == "" {
		return m.Page{}, fmt.Errorf("METRIC_RESULT_STALE")
	}
	return withMetric(s, ctx, handle, digest, func(ctx context.Context, prepared *metricSemantic, actual string) (m.Page, error) {
		return prepared.index.Query(ctx, actual, q)
	})
}
func (s *Service) MaterializeMetricSelection(ctx context.Context, handle, digest string, selection m.Selection) (ResultView, ResultLease, error) {
	var view ResultView
	var lease ResultLease
	if digest == "" {
		return view, lease, fmt.Errorf("METRIC_RESULT_STALE")
	}
	body, err := withMetric(s, ctx, handle, digest, func(ctx context.Context, prepared *metricSemantic, _ string) ([]byte, error) {
		result, e := a.Build(ctx, prepared.core.Model, selection, "explicit selected metric history")
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

// MetricIntervalCSV uses the same owned, byte-verified snapshot as metric queries.
func (s *Service) MetricIntervalCSV(ctx context.Context, handle, digest string, selection m.Selection) ([]byte, error) {
	if digest == "" {
		return nil, fmt.Errorf("METRIC_RESULT_STALE")
	}
	return withMetric(s, ctx, handle, digest, func(ctx context.Context, prepared *metricSemantic, actual string) ([]byte, error) {
		return metriccsv.Encode(ctx, prepared.core.Model, selection, actual)
	})
}
