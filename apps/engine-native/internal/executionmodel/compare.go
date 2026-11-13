package executionmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
)

type Delta struct {
	Policy     string  `json:"statistical_policy"`
	Metric     string  `json:"metric_id"`
	Name       string  `json:"name"`
	Statistic  string  `json:"statistic"`
	Unit       string  `json:"unit"`
	Population string  `json:"population"`
	GroupSHA   string  `json:"group_sha256"`
	Baseline   *string `json:"baseline"`
	Target     *string `json:"target"`
	Difference *string `json:"difference"`
	Percent    *string `json:"percent_change"`
	Status     string  `json:"status"`
}
type Comparison struct {
	VerificationScope string   `json:"verification_scope"`
	IdentityBasis     string   `json:"identity_basis"`
	Version           string   `json:"version"`
	BaselineSHA       string   `json:"baseline_result_sha256"`
	TargetSHA         string   `json:"target_result_sha256"`
	BaselineSource    string   `json:"baseline_source_sha256"`
	TargetSource      string   `json:"target_source_sha256"`
	ContextSHA        string   `json:"context_sha256"`
	Status            string   `json:"status"`
	Reasons           []string `json:"reasons"`
	Rows              []Delta  `json:"rows"`
	Next              string   `json:"next"`
	Total             int      `json:"total"`
	QuerySHA          string   `json:"query_sha256"`
}

func ContextIdentity(s Set) (string, []string) {
	reasons := []string{}
	if s.Kind != "benchmark" || s.Context == nil {
		return "", []string{"benchmark_context_required; crash context is not a numeric baseline"}
	}
	if s.Source.Qualification != "" {
		reasons = append(reasons, "producer_version_unverified")
	}
	c := *s.Context
	for _, v := range []struct{ name, value string }{{"scenario", c.Scenario}, {"workload", c.WorkloadSHA}, {"client_environment", c.ClientSHA}, {"target_environment", c.TargetSHA}, {"arrival_model", c.Arrival}, {"phase", c.Phase}, {"boundary", c.Boundary}, {"protocol", c.Protocol}} {
		if !Known(v.value) {
			reasons = append(reasons, v.name+"_unavailable")
		}
	}
	if c.Duration == nil || *c.Duration == "0" {
		reasons = append(reasons, "duration_unavailable")
	}
	if c.Warmup == nil {
		reasons = append(reasons, "warmup_unavailable")
	}
	if c.Streaming == nil {
		reasons = append(reasons, "streaming_condition_unavailable")
	}
	if c.Phase == "cold" && c.Warmup != nil && *c.Warmup != "0" {
		reasons = append(reasons, "cold_phase_with_declared_warmup")
	}
	if c.Phase == "mixed" {
		reasons = append(reasons, "mixed_warmup_population")
	}
	if c.Arrival == "open" && c.ArrivalRate == nil {
		reasons = append(reasons, "arrival_rate_unavailable")
	}
	if c.Arrival == "closed" && (c.Concurrency == nil || *c.Concurrency == "0") {
		reasons = append(reasons, "concurrency_unavailable")
	}
	if c.Kind == "llm" {
		if c.InputMean == nil || c.InputStddev == nil || c.OutputMean == nil || c.OutputStddev == nil {
			reasons = append(reasons, "requested_token_distribution_unavailable")
		}
		for _, v := range []string{c.Model, c.Revision, c.Tokenizer, c.InputTokensSHA, c.OutputTokensSHA} {
			if !Known(v) {
				reasons = append(reasons, "llm_model_token_conditions_unavailable")
				break
			}
		}
	}
	c.RunID = ""
	return Hash([]any{"execution-comparison.v1", s.Source.Format, s.Source.Producer, s.Source.Version, c}), reasons
}

// Compare never merges percentiles or treats a crash as a cause of run latency.
func Compare(ctx context.Context, left, right Set, leftSHA, rightSHA string, offset, limit int) (Comparison, error) {
	out := Comparison{VerificationScope: "validated_normalized_snapshots_not_original_reimport", IdentityBasis: "declared", Version: "1.0.0", BaselineSHA: leftSHA, TargetSHA: rightSHA, BaselineSource: left.Source.SHA, TargetSource: right.Source.SHA, Status: "incompatible", Rows: []Delta{}, Reasons: []string{}}
	if !Digest(leftSHA) || !Digest(rightSHA) || offset < 0 || limit < 1 || limit > 100 {
		return out, fmt.Errorf("EXECUTION_COMPARISON_QUERY_INVALID")
	}
	a, ar := ContextIdentity(left)
	b, br := ContextIdentity(right)
	out.Reasons = append(out.Reasons, ar...)
	out.Reasons = append(out.Reasons, br...)
	if a != b {
		out.Reasons = append(out.Reasons, "collection_or_producer_conditions_differ")
	}
	if len(out.Reasons) > 0 {
		return out, ctx.Err()
	}
	out.ContextSHA = a
	out.Status = "compatible_declarations"
	out.Reasons = []string{"conditions are source/caller declarations; not provenance authentication or statistical significance"}
	l := map[string]Metric{}
	r := map[string]Metric{}
	order := []string{}
	for _, v := range left.Metrics {
		l[v.ID] = v
		order = append(order, v.ID)
	}
	for _, v := range right.Metrics {
		r[v.ID] = v
		if _, ok := l[v.ID]; !ok {
			order = append(order, v.ID)
		}
	}
	out.QuerySHA = Hash([]any{leftSHA, rightSHA, a, limit})
	out.Total = len(order)
	for i, id := range order {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if i < offset || len(out.Rows) >= limit {
			continue
		}
		a, aok := l[id]
		b, bok := r[id]
		definition := a
		if !aok {
			definition = b
		}
		row := Delta{Policy: definition.Policy, Metric: id, Name: definition.Name, Statistic: definition.Statistic, Unit: definition.Unit, Population: definition.Population, GroupSHA: definition.GroupSHA, Baseline: a.Value, Target: b.Value, Status: "missing_or_unavailable"}
		if aok && bok && a.Value != nil && b.Value != nil {
			x, e := Number(*a.Value)
			if e != nil {
				return out, e
			}
			y, e := Number(*b.Value)
			if e != nil {
				return out, e
			}
			d := new(big.Rat).Sub(y, x)
			row.Difference = Ptr(d.RatString())
			row.Status = "observed_difference; no significance claim"
			if x.Sign() != 0 {
				row.Percent = Ptr(new(big.Rat).Mul(new(big.Rat).Quo(d, x), big.NewRat(100, 1)).RatString())
			} else {
				row.Status = "baseline_zero; percent_unavailable"
			}
		}
		out.Rows = append(out.Rows, row)
	}
	if offset > out.Total {
		return out, fmt.Errorf("EXECUTION_CURSOR_OFFSET_INVALID")
	}
	return out, ctx.Err()
}

// JSONCopy prevents semantic cache pointers from escaping to callers.
func JSONCopy[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	json.Unmarshal(raw, &out)
	return out
}
