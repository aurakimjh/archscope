package execution_test

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func model(t *testing.T, name string) e.Set {
	t.Helper()
	raw, err := os.ReadFile("../../catalog/contracts/execution-evidence-" + name + "-set.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.Decode(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestComparisonConditionsPrecisionAndPagination(t *testing.T) {
	ctx := context.Background()
	left := model(t, "k6")
	right := e.JSONCopy(left)
	right.Context.RunID = "target"
	right.Source.SHA = strings.Repeat("b", 64)
	id := 0
	for i, v := range right.Metrics {
		if v.Name == "requests" {
			id = i
			break
		}
	}
	left.Metrics[id].Value = e.Ptr("9007199254740993")
	right.Metrics[id].Value = e.Ptr("9007199254740994")
	sha := strings.Repeat("a", 64)
	target := strings.Repeat("b", 64)
	q := a.CompareQuery{Limit: 1}
	page, err := a.CompareSets(ctx, left, right, sha, target, q)
	if err != nil || page.Status != "compatible_declarations" || page.Next == "" {
		t.Fatal(page, err)
	}
	for page.Next != "" {
		q.Cursor = page.Next
		next, err := a.CompareSets(ctx, left, right, sha, target, q)
		if err != nil {
			t.Fatal(err)
		}
		page = next
	}
	all, err := a.CompareSets(ctx, left, right, sha, target, a.CompareQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range all.Rows {
		if row.Metric == right.Metrics[id].ID {
			found = true
			if *row.Difference != "1" {
				t.Fatal(row)
			}
		}
	}
	if !found {
		t.Fatal("metric lost")
	}
	q.Cursor = ""
	page, _ = a.CompareSets(ctx, left, right, sha, target, q)
	q.Cursor = page.Next
	if _, err = a.CompareSets(ctx, left, right, target, sha, q); err == nil {
		t.Fatal("ordered cursor reused")
	}
	cases := map[string]func(*e.Set){"warmup": func(s *e.Set) { s.Context.Warmup = nil }, "mixed": func(s *e.Set) { s.Context.Phase = "mixed" }, "client": func(s *e.Set) { s.Context.ClientSHA = "unknown" }, "arrival": func(s *e.Set) { s.Context.Arrival = "open"; s.Context.ArrivalRate = nil }, "concurrency": func(s *e.Set) { s.Context.Concurrency = e.Ptr("3") }, "workload": func(s *e.Set) { s.Context.WorkloadSHA = strings.Repeat("c", 64) }, "duration": func(s *e.Set) { s.Context.Duration = nil }, "producer": func(s *e.Set) { s.Source.Version = "different" }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			bad := e.JSONCopy(right)
			change(&bad)
			out, err := a.CompareSets(ctx, left, bad, sha, target, a.CompareQuery{Limit: 25})
			if err != nil || out.Status != "incompatible" || len(out.Rows) != 0 {
				t.Fatal(out, err)
			}
		})
	}
	crash := model(t, "crash")
	out, err := a.CompareSets(ctx, left, crash, sha, target, a.CompareQuery{Limit: 25})
	if err != nil || out.Status != "incompatible" {
		t.Fatal(out, err)
	}
	right = e.JSONCopy(left)
	right.Metrics[id].Value = e.Ptr("0")
	left.Metrics[id].Value = e.Ptr("0")
	out, err = a.CompareSets(ctx, left, right, sha, target, a.CompareQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range out.Rows {
		if row.Metric == right.Metrics[id].ID && row.Percent != nil {
			t.Fatal("zero baseline percent fabricated")
		}
	}
}
func TestSavedPayloadForgeryAndBoundedQueries(t *testing.T) {
	ctx := context.Background()
	s := model(t, "jmeter")
	result, err := a.Build(ctx, s, "sanitized")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Validate(result); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	_ = raw
	result.Summary["sample_records"] = "100"
	if a.Validate(result) == nil {
		t.Fatal("forged summary")
	}
	sha := strings.Repeat("a", 64)
	q := a.Query{Kind: "samples", Limit: 1}
	page, err := a.QuerySet(ctx, s, sha, q)
	if err != nil || page.Total != 3 || len(page.Rows) != 1 {
		t.Fatal(page, err)
	}
	q.Cursor = page.Next
	q.GroupSHA = s.Samples[0].GroupSHA
	if _, err = a.QuerySet(ctx, s, sha, q); err == nil {
		t.Fatal("selector cursor reused")
	}
	s.Metrics[0].Unit = "tokens"
	if s.Validate(ctx) == nil {
		t.Fatal("physical unit mismatch")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = a.QuerySet(ctx, s, sha, a.Query{Kind: "samples", Limit: 25}); err == nil {
		t.Fatal("cancel ignored")
	}
}
func TestTwentyThousandSamplesByteAndCountBudgetsAndConcurrentComparison(t *testing.T) {
	s := model(t, "jmeter")
	base := s.Samples[0]
	s.Samples = nil
	for i := 0; i < 20000; i++ {
		v := base
		v.Position = fmt.Sprint(i)
		v.ID = e.SampleID(v)
		s.Samples = append(s.Samples, v)
	}
	result, err := a.Build(context.Background(), s, "20k")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Validate(result); err != nil {
		t.Fatal(err)
	}
	s.Samples = append(s.Samples, make([]e.Sample, e.MaxSamples)...)
	if s.Validate(context.Background()) == nil {
		t.Fatal("sample budget bypass")
	}
	left := model(t, "llmperf")
	before := e.JSONCopy(left)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.CompareSets(context.Background(), left, left, strings.Repeat("a", 64), strings.Repeat("a", 64), a.CompareQuery{Limit: 25}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(before, left) {
		t.Fatal("comparison mutated model")
	}
}
