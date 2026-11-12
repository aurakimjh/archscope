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
	groups := map[string][]e.Sample{}
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
		if len(s.Samples) >= e.MaxSamples {
			return fmt.Errorf("EXECUTION_SAMPLE_BUDGET: split original JTL")
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
		v.ID = e.SampleID(v)
		s.Samples = append(s.Samples, v)
		groups[v.GroupSHA] = append(groups[v.GroupSHA], v)
		if len(groups) > 128 {
			return fmt.Errorf("EXECUTION_JTL_GROUP_BUDGET: split original labels")
		}
	}
	if len(s.Samples) == 0 {
		return fmt.Errorf("EXECUTION_JTL_SAMPLES_REQUIRED")
	}
	keys := []string{}
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rows := groups[key]
		values := []*big.Rat{}
		sum := new(big.Rat)
		errors := 0
		for _, row := range rows {
			x, _ := e.Number(*row.Latency)
			values = append(values, x)
			sum.Add(sum, x)
			if !row.Success {
				errors++
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
		add := func(name, stat, unit, value, pop string) {
			v := e.Metric{Name: name, Statistic: stat, Unit: unit, Population: pop, GroupSHA: key, Policy: "jtl-nearest-rank.v1", Value: e.Ptr(value), OriginalUnit: "derived from elapsed milliseconds", Samples: e.Ptr(strconv.Itoa(len(rows))), Position: "label-sha256/" + key}
			v.ID = e.MetricID(v)
			s.Metrics = append(s.Metrics, v)
		}
		add("samples", "count", "count", strconv.Itoa(len(rows)), "all_samples")
		add("sample_errors", "count", "count", strconv.Itoa(errors), "failed_samples")
		add("sample_latency", "mean", "seconds", new(big.Rat).Quo(sum, big.NewRat(int64(len(rows)), 1)).RatString(), "all_samples")
		add("sample_latency", "min", "seconds", values[0].RatString(), "all_samples")
		add("sample_latency", "max", "seconds", values[len(values)-1].RatString(), "all_samples")
		for _, p := range []int{50, 90, 95, 99} {
			index := (len(values)*p+99)/100 - 1
			add("sample_latency", fmt.Sprintf("p%d", p), "seconds", values[index].RatString(), "all_samples")
		}
	}
	s.Limitations = append(s.Limitations, "JTL timeStamp is explicitly epoch milliseconds. Label populations are SHA-256 scoped; response/failure messages, URL and thread names are excluded. Latency quantiles use nearest-rank.v1 per label; exported summary percentiles are never merged.")
	return nil
}
