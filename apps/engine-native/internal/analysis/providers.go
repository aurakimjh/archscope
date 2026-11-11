package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type ResultCapabilities struct {
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
	body, result, e := s.Read(handle)
	if e != nil {
		return ResultCapabilities{}, e
	}
	capabilities, e := s.results.Capabilities(ctx, result)
	if e != nil {
		return ResultCapabilities{}, e
	}
	sum := sha256.Sum256(body)
	out := ResultCapabilities{Handle: handle, ResultSHA256: hex.EncodeToString(sum[:]), Type: result.Type, Sources: []resultcontract.Cell{}, Capabilities: capabilities}
	for i, source := range result.SourceFiles {
		if i == 32 {
			out.SourcesTruncated = true
			break
		}
		if len(source) > 2048 {
			source = "[source label exceeds display limit]"
			out.SourcesTruncated = true
		}
		out.Sources = append(out.Sources, resultcontract.Cell{Text: source})
	}
	return out, nil
}
func (s *Service) QueryResult(parent context.Context, handle, provider string, offset, limit int) (ProviderPage, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return ProviderPage{}, e
	}
	defer finish()
	body, result, e := s.Read(handle)
	if e != nil {
		return ProviderPage{}, e
	}
	page, e := s.results.Project(ctx, result, provider, offset, limit)
	if e != nil {
		return ProviderPage{}, e
	}
	sum := sha256.Sum256(body)
	return ProviderPage{Handle: handle, ResultSHA256: hex.EncodeToString(sum[:]), Page: page}, nil
}
func (s *Service) CompareResult(parent context.Context, baseline, target, provider string) (ProviderComparison, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return ProviderComparison{}, e
	}
	defer finish()
	_, result, e := s.Read(baseline)
	if e != nil {
		return ProviderComparison{}, e
	}
	caps, e := s.results.Capabilities(ctx, result)
	if e != nil {
		return ProviderComparison{}, e
	}
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
		delta, e := s.Compare(baseline, target, c.Path)
		if e != nil {
			var refusal *resultcontract.ComparisonError
			if errors.As(e, &refusal) {
				return ProviderComparison{Code: refusal.Code, Reason: refusal.Message}, nil
			}
			return ProviderComparison{}, e
		}
		return ProviderComparison{Allowed: delta.Value != nil, Code: delta.Reason, Delta: &delta}, nil
	}
	return ProviderComparison{}, fmt.Errorf("unknown comparison provider")
}
