package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type ResultCapabilities struct {
	PayloadVersion   string                `json:"payload_version"`
	Handle           string                `json:"handle"`
	ResultSHA256     string                `json:"result_sha256"`
	Type             string                `json:"type"`
	Sources          []resultcontract.Cell `json:"sources"`
	SourcesTruncated bool                  `json:"sources_truncated"`
	resultcontract.Capabilities
}
type ProviderPage struct {
	Handle       string `json:"handle"`
	ResultSHA256 string `json:"result_sha256"`
	resultcontract.Page
}
type ProviderComparison struct {
	Allowed bool                  `json:"allowed"`
	Code    string                `json:"code,omitempty"`
	Reason  string                `json:"reason,omitempty"`
	Delta   *resultcontract.Delta `json:"delta,omitempty"`
}

func (s *Service) DescribeResult(parent context.Context, handle string) (ResultCapabilities, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return ResultCapabilities{}, e
	}
	defer finish()
	snapshot, e := s.projection(ctx, handle)
	if e != nil {
		return ResultCapabilities{}, e
	}
	sources, truncated := snapshot.Sources()
	caps := snapshot.Capabilities()
	if snapshot.Type() == "metrics_timeseries" && caps.Compatibility.Allowed {
		inv, err := s.MetricInventory(ctx, handle)
		if err != nil {
			return ResultCapabilities{}, err
		}
		if inv.ResultSHA256 != snapshot.Digest() {
			return ResultCapabilities{}, fmt.Errorf("METRIC_RESULT_STALE")
		}
		caps.Providers = append(caps.Providers, resultcontract.Capability{Provider: resultcontract.Provider{ID: metricIntervalsProvider, Version: "1.0.0", Kind: "detail", Path: "/metadata/metrics_core", Schema: "object-rows-v1"}, Availability: "available"})
	}
	return ResultCapabilities{Handle: handle, ResultSHA256: snapshot.Digest(), Type: snapshot.Type(), PayloadVersion: snapshot.PayloadVersion(), Sources: sources, SourcesTruncated: truncated, Capabilities: caps}, nil
}

func (s *Service) QueryResult(parent context.Context, handle, provider string, offset, limit int) (ProviderPage, error) {
	if provider == metricIntervalsProvider {
		return s.metricIntervalPage(parent, handle, offset, limit)
	}
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return ProviderPage{}, e
	}
	defer finish()
	snapshot, e := s.projection(ctx, handle)
	if e != nil {
		return ProviderPage{}, e
	}
	page, e := snapshot.Project(ctx, provider, offset, limit)
	if e != nil {
		return ProviderPage{}, e
	}
	return ProviderPage{Handle: handle, ResultSHA256: snapshot.Digest(), Page: page}, nil
}

func (s *Service) CompareResult(parent context.Context, baseline, target, provider string) (ProviderComparison, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return ProviderComparison{}, e
	}
	defer finish()
	snapshot, e := s.projection(ctx, baseline)
	if e != nil {
		return ProviderComparison{}, e
	}
	caps := snapshot.Capabilities()
	for _, c := range caps.Providers {
		if c.ID != provider || c.Kind != "compare" {
			continue
		}
		if c.Availability != "available" {
			return ProviderComparison{Code: c.Reason, Reason: "Comparison is unavailable for this result."}, nil
		}
		if e = ctx.Err(); e != nil {
			return ProviderComparison{}, e
		}
		other, e := s.projection(ctx, target)
		if e != nil {
			return ProviderComparison{}, e
		}
		delta, e := snapshot.Compare(ctx, other, c.Path)
		if e != nil {
			var refusal *resultcontract.ComparisonError
			if errors.As(e, &refusal) {
				return ProviderComparison{Code: refusal.Code, Reason: refusal.Message}, nil
			}
			return ProviderComparison{}, e
		}
		if delta.Comparison != nil {
			delta.Comparison.Baseline.Handle = baseline
			delta.Comparison.Baseline.ResultSHA256 = snapshot.Digest()
			delta.Comparison.Target.Handle = target
			delta.Comparison.Target.ResultSHA256 = other.Digest()
		}
		return ProviderComparison{Allowed: delta.Value != nil, Code: delta.Reason, Delta: &delta}, nil
	}
	return ProviderComparison{}, fmt.Errorf("unknown comparison provider")
}
