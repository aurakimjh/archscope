package executionmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
)

const DistributionVersion = "1.1.0"
const MaxJTLRows = 1_000_000
const MaxJTLBins = 100_000
const RetainedJTLSamples = 1000

type DurationCount struct {
	Seconds string `json:"seconds"`
	Count   int    `json:"count"`
}
type Distribution struct {
	GroupSHA string          `json:"group_sha256"`
	Count    int             `json:"count"`
	Errors   int             `json:"errors"`
	Values   []DurationCount `json:"latency_distribution"`
}

// DistributionMetrics calculates exact nearest-rank quantiles over counts,
// never combines already calculated percentiles. Buckets strictly increase.
func DistributionMetrics(ctx context.Context, d Distribution) ([]Metric, error) {
	if !Digest(d.GroupSHA) || d.Count < 1 || d.Count > MaxJTLRows || d.Errors < 0 || d.Errors > d.Count || len(d.Values) == 0 || len(d.Values) > MaxJTLBins {
		return nil, fmt.Errorf("EXECUTION_DISTRIBUTION_INVALID")
	}
	count := 0
	sum := new(big.Rat)
	var previous *big.Rat
	for _, v := range d.Values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		x, err := Number(v.Seconds)
		if err != nil || x.Sign() < 0 || x.RatString() != v.Seconds || previous != nil && x.Cmp(previous) <= 0 || v.Count < 1 || v.Count > MaxJTLRows-count {
			return nil, fmt.Errorf("EXECUTION_DISTRIBUTION_BUCKET_INVALID")
		}
		count += v.Count
		sum.Add(sum, new(big.Rat).Mul(x, big.NewRat(int64(v.Count), 1)))
		previous = x
	}
	if count != d.Count {
		return nil, fmt.Errorf("EXECUTION_DISTRIBUTION_COUNT_MISMATCH")
	}
	out := []Metric{}
	add := func(name, stat, unit, value, pop string) {
		v := Metric{Name: name, Statistic: stat, Unit: unit, Population: pop, GroupSHA: d.GroupSHA, Policy: "jtl-nearest-rank.v1", Value: Ptr(value), OriginalUnit: "derived from elapsed milliseconds", Samples: Ptr(strconv.Itoa(d.Count)), Position: "label-sha256/" + d.GroupSHA}
		v.ID = MetricID(v)
		out = append(out, v)
	}
	add("samples", "count", "count", strconv.Itoa(d.Count), "all_samples")
	add("sample_errors", "count", "count", strconv.Itoa(d.Errors), "failed_samples")
	add("sample_latency", "mean", "seconds", new(big.Rat).Quo(sum, big.NewRat(int64(d.Count), 1)).RatString(), "all_samples")
	add("sample_latency", "min", "seconds", d.Values[0].Seconds, "all_samples")
	add("sample_latency", "max", "seconds", d.Values[len(d.Values)-1].Seconds, "all_samples")
	for _, p := range []int{50, 90, 95, 99} {
		rank := (int64(d.Count)*int64(p) + 99) / 100
		cumulative := int64(0)
		for _, v := range d.Values {
			cumulative += int64(v.Count)
			if cumulative >= rank {
				add("sample_latency", fmt.Sprintf("p%d", p), "seconds", v.Seconds, "all_samples")
				break
			}
		}
	}
	return out, nil
}

func (s Set) validateDistributions(ctx context.Context) error {
	if s.Version == Version && len(s.Distributions) > 0 {
		return fmt.Errorf("EXECUTION_DISTRIBUTION_VERSION")
	}
	if s.Version != DistributionVersion {
		return nil
	}
	if s.Kind != "benchmark" || s.Source.Format != "jmeter-jtl-csv" || len(s.Samples) != 0 || len(s.Distributions) == 0 || len(s.Distributions) > 128 {
		return fmt.Errorf("EXECUTION_DISTRIBUTION_SCOPE")
	}
	metrics := []Metric{}
	rows, bins := 0, 0
	previous := ""
	for _, d := range s.Distributions {
		if d.GroupSHA <= previous {
			return fmt.Errorf("EXECUTION_DISTRIBUTION_ORDER")
		}
		previous = d.GroupSHA
		// Validate per-distribution bounds before arithmetic (including 32-bit).
		m, err := DistributionMetrics(ctx, d)
		if err != nil {
			return err
		}
		rows += d.Count
		bins += len(d.Values)
		if rows > MaxJTLRows || bins > MaxJTLBins {
			return fmt.Errorf("EXECUTION_DISTRIBUTION_BUDGET")
		}
		metrics = append(metrics, m...)
	}
	actual, _ := json.Marshal(s.Metrics)
	expected, _ := json.Marshal(metrics)
	if string(actual) != string(expected) {
		return fmt.Errorf("EXECUTION_DISTRIBUTION_METRICS_MISMATCH")
	}
	return nil
}

type DistributionRow struct {
	GroupSHA string `json:"group_sha256"`
	Seconds  string `json:"latency_seconds"`
	Count    int    `json:"count"`
}

func DistributionRows(s Set) []DistributionRow {
	out := []DistributionRow{}
	for _, d := range s.Distributions {
		for _, v := range d.Values {
			out = append(out, DistributionRow{d.GroupSHA, v.Seconds, v.Count})
		}
	}
	return out
}
