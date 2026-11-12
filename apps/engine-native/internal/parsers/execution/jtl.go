package execution

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

func parseJTL(ctx context.Context, raw []byte, s *e.Set) error {
	if s.Context.Kind != "http" {
		return fmt.Errorf("EXECUTION_JTL_HTTP_CONTEXT_REQUIRED")
	}
	lines := bufio.NewScanner(bytes.NewReader(raw))
	lines.Buffer(make([]byte, 64<<10), 1<<20)
	for lines.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if lines.Err() != nil {
		return fmt.Errorf("EXECUTION_JTL_LINE_BUDGET")
	}
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return err
	}
	cols := map[string]int{}
	for i, v := range header {
		v = strings.TrimPrefix(v, "\ufeff")
		if _, ok := cols[v]; ok {
			return fmt.Errorf("EXECUTION_JTL_DUPLICATE_HEADER")
		}
		cols[v] = i
	}
	for _, key := range []string{"timeStamp", "elapsed", "label", "success"} {
		if _, ok := cols[key]; !ok {
			return fmt.Errorf("EXECUTION_JTL_HEADER_REQUIRED: %s", key)
		}
	}
	type accumulator struct {
		distribution e.Distribution
		counts       map[string]int
	}
	groups := map[string]*accumulator{}
	rows, bins := 0, 0
	for line := 2; ; line++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) != len(header) {
			return fmt.Errorf("EXECUTION_JTL_ROW_INVALID: row=%d", line)
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		rows++
		if rows > e.MaxJTLRows {
			return fmt.Errorf("EXECUTION_SAMPLE_BUDGET: rows=%d limit=%d; reduce run scope; split-file percentiles cannot be combined", rows, e.MaxJTLRows)
		}
		get := func(key string) string {
			if i, ok := cols[key]; ok {
				return row[i]
			}
			return ""
		}
		parseDuration := func(key string) (*string, error) {
			v := get(key)
			if v == "" {
				return nil, nil
			}
			n, err := decimal(v)
			if err != nil {
				return nil, err
			}
			x, _ := e.Number(*n)
			return e.Ptr(new(big.Rat).Quo(x, big.NewRat(1000, 1)).RatString()), nil
		}
		ms := get("timeStamp")
		n, err := strconv.ParseUint(ms, 10, 64)
		if err != nil || strconv.FormatUint(n, 10) != ms || n > ^uint64(0)/1000000 {
			return fmt.Errorf("EXECUTION_JTL_EPOCH_MILLISECONDS_REQUIRED: row=%d", line)
		}
		elapsed, err := parseDuration("elapsed")
		if err != nil || elapsed == nil {
			return fmt.Errorf("EXECUTION_JTL_ELAPSED_INVALID: row=%d", line)
		}
		first, err := parseDuration("Latency")
		if err != nil {
			return err
		}
		connect, err := parseDuration("Connect")
		if err != nil {
			return err
		}
		success := get("success")
		if success != "true" && success != "false" {
			return fmt.Errorf("EXECUTION_JTL_SUCCESS_INVALID: row=%d", line)
		}
		threads := get("allThreads")
		var concurrency *string
		if threads != "" {
			concurrency = e.Ptr(threads)
			if !e.Uint(concurrency) {
				return fmt.Errorf("EXECUTION_JTL_THREADS_INVALID: row=%d", line)
			}
		}
		code := get("responseCode")
		if len(code) > 5 {
			code = ""
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				code = ""
				break
			}
		}
		v := e.Sample{GroupSHA: Digest([]byte(get("label"))), Start: e.Ptr(strconv.FormatUint(n*1000000, 10)), Latency: elapsed, FirstByte: first, Connect: connect, Success: success == "true", Concurrency: concurrency, ResponseCode: code, Position: fmt.Sprintf("csv-row/%d", line)}
		if rows <= e.RetainedJTLSamples {
			v.ID = e.SampleID(v)
			s.Samples = append(s.Samples, v)
		} else if rows == e.RetainedJTLSamples+1 {
			s.Samples = []e.Sample{}
		}
		group := groups[v.GroupSHA]
		if group == nil {
			group = &accumulator{distribution: e.Distribution{GroupSHA: v.GroupSHA}, counts: map[string]int{}}
			groups[v.GroupSHA] = group
		}
		if group.counts[*elapsed] == 0 {
			bins++
			if bins > e.MaxJTLBins {
				return fmt.Errorf("EXECUTION_DISTRIBUTION_BUDGET: rows=%d distinct_values=%d limit=%d; reduce run scope; split-file percentiles cannot be combined", rows, bins, e.MaxJTLBins)
			}
		}
		group.counts[*elapsed]++
		group.distribution.Count++
		if !v.Success {
			group.distribution.Errors++
		}
		if len(groups) > 128 {
			return fmt.Errorf("EXECUTION_JTL_GROUP_BUDGET: split original labels")
		}
	}
	if rows == 0 {
		return fmt.Errorf("EXECUTION_JTL_SAMPLES_REQUIRED")
	}
	keys := []string{}
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		d := group.distribution
		for seconds, count := range group.counts {
			d.Values = append(d.Values, e.DurationCount{Seconds: seconds, Count: count})
		}
		sort.Slice(d.Values, func(i, j int) bool {
			x, _ := e.Number(d.Values[i].Seconds)
			y, _ := e.Number(d.Values[j].Seconds)
			return x.Cmp(y) < 0
		})
		metrics, err := e.DistributionMetrics(ctx, d)
		if err != nil {
			return err
		}
		s.Metrics = append(s.Metrics, metrics...)
		if rows > e.RetainedJTLSamples {
			s.Distributions = append(s.Distributions, d)
		}
	}
	if rows > e.RetainedJTLSamples {
		s.Version = e.DistributionVersion
		s.Limitations = append(s.Limitations, "Large JTL retains exact per-label latency distributions and error counts, not per-request timestamps/first-byte/connect/concurrency/code or timeline. No percentile composition across split files. Row limit 1000000, total distinct latency bins 100000, groups 128, model 16 MiB.")
	}
	s.Limitations = append(s.Limitations, "JTL timeStamp is explicitly epoch milliseconds. Label populations are SHA-256 scoped; response/failure messages, URL and thread names are excluded. Latency quantiles use nearest-rank.v1 per label; exported summary percentiles are never merged.")
	return nil
}
