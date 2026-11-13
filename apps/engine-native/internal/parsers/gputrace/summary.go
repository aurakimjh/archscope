package gputrace

import (
	"bytes"
	"encoding/csv"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	pp "github.com/aurakimjh/archscope/apps/engine-native/internal/producerpolicy"
	"io"
	"strconv"
)

func (b *builder) summary(raw []byte) error {
	version, qualification, err := pp.Select(b.source.Version, "2024.2.3.38", b.o.AllowUnverifiedVersion)
	if err != nil {
		return fmt.Errorf("GPU_TRACE_NSYS_SUMMARY_VERSION: %w", err)
	}
	b.source.Version, b.source.Qualification = version, qualification
	if qualification != "" {
		b.set.Warnings = append(b.set.Warnings, pp.Limitation)
	}
	if b.source.From != nil || b.source.To != nil {
		return fmt.Errorf("GPU_TRACE_SUMMARY_NOT_TIMED")
	}
	b.source.Producer = "nsight-systems"
	b.source.Schema = "cuda_gpu_kern_sum.csv.v1"
	if e := b.scope(); e != nil {
		return e
	}
	r := csv.NewReader(bytes.NewReader(raw))
	header, e := r.Read()
	if e != nil {
		return e
	}
	cols := map[string]int{}
	for i, key := range header {
		if _, ok := cols[key]; ok {
			return fmt.Errorf("GPU_TRACE_SUMMARY_DUPLICATE_HEADER")
		}
		cols[key] = i
	}
	fields := []struct{ key, stat, unit string }{{"Time (%)", "duration_share", "%"}, {"Total Time (ns)", "total", "ns"}, {"Instances", "count", "count"}, {"Avg (ns)", "mean", "ns"}, {"Med (ns)", "median", "ns"}, {"Min (ns)", "min", "ns"}, {"Max (ns)", "max", "ns"}, {"StdDev (ns)", "stddev", "ns"}}
	for _, v := range fields {
		if _, ok := cols[v.key]; !ok {
			return fmt.Errorf("GPU_TRACE_SUMMARY_HEADER: %s", v.key)
		}
	}
	if _, ok := cols["Name"]; !ok {
		return fmt.Errorf("GPU_TRACE_SUMMARY_NAME_REQUIRED")
	}
	for line := 2; ; line++ {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil || len(row) != len(header) {
			return fmt.Errorf("GPU_TRACE_SUMMARY_ROW: %d", line)
		}
		name := row[cols["Name"]]
		if !g.Known(name) {
			return fmt.Errorf("GPU_TRACE_SUMMARY_NAME_INVALID")
		}
		for _, v := range fields {
			if len(b.set.Summaries) >= 4096 {
				return fmt.Errorf("GPU_TRACE_SUMMARY_BUDGET")
			}
			b.set.Summaries = append(b.set.Summaries, g.Summary{Source: b.source.ID, Name: name, Statistic: v.stat, Value: row[cols[v.key]], Unit: v.unit, Position: "csv-row/" + strconv.Itoa(line) + "/" + v.key})
		}
	}
	if len(b.set.Summaries) == 0 {
		return fmt.Errorf("GPU_TRACE_SUMMARY_EMPTY")
	}
	b.set.Warnings = append(b.set.Warnings, "Already aggregated Nsight kernel summary; no execution intervals, stream overlap, CPU launch correlation, rank timing or detailed timeline can be reconstructed. Time (%) is reported duration share, not GPU utilization.")
	return nil
}
