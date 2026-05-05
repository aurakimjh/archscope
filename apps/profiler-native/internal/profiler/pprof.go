package profiler

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/pprof/profile"
)

// ExportToPprof writes a gzip-compressed profile.proto file to `outPath` from
// the supplied collapsed-style stacks (frame1;frame2;...;leaf -> samples).
//
// The result is loadable by `go tool pprof` and any UI that consumes the
// pprof wire format. The Python engine's exporter follows the same model.
//
// `unit` defaults to "samples" when empty; `intervalMs` is multiplied into
// each sample so a `cpu`-style profile is annotated with realistic time.
func ExportToPprof(stacks map[string]int, outPath string, sampleType, unit string, intervalMs float64) error {
	if outPath == "" {
		return fmt.Errorf("outPath is required")
	}
	if sampleType == "" {
		sampleType = "samples"
	}
	if unit == "" {
		unit = "count"
	}

	prof := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: sampleType, Unit: unit},
		},
		DefaultSampleType: sampleType,
	}
	if intervalMs > 0 {
		prof.SampleType = append(prof.SampleType, &profile.ValueType{Type: "duration", Unit: "milliseconds"})
		prof.PeriodType = &profile.ValueType{Type: "wall", Unit: "milliseconds"}
		prof.Period = int64(intervalMs)
	}

	functions := map[string]*profile.Function{}
	locations := map[string]*profile.Location{}
	var nextFuncID, nextLocID uint64 = 1, 1

	functionFor := func(name string) *profile.Function {
		if fn, ok := functions[name]; ok {
			return fn
		}
		fn := &profile.Function{
			ID:         nextFuncID,
			Name:       name,
			SystemName: name,
		}
		nextFuncID++
		functions[name] = fn
		prof.Function = append(prof.Function, fn)
		return fn
	}
	locationFor := func(name string) *profile.Location {
		if loc, ok := locations[name]; ok {
			return loc
		}
		loc := &profile.Location{
			ID: nextLocID,
			Line: []profile.Line{
				{Function: functionFor(name)},
			},
		}
		nextLocID++
		locations[name] = loc
		prof.Location = append(prof.Location, loc)
		return loc
	}

	for stack, samples := range stacks {
		if samples <= 0 {
			continue
		}
		frames := strings.Split(stack, ";")
		// pprof expects locations leaf-first; collapsed stacks are root-first.
		filtered := make([]string, 0, len(frames))
		for _, frame := range frames {
			frame = strings.TrimSpace(frame)
			if frame != "" {
				filtered = append(filtered, frame)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		// Reverse to leaf-first.
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
		locs := make([]*profile.Location, 0, len(filtered))
		for _, frame := range filtered {
			locs = append(locs, locationFor(frame))
		}
		values := []int64{int64(samples)}
		if intervalMs > 0 {
			values = append(values, int64(float64(samples)*intervalMs))
		}
		prof.Sample = append(prof.Sample, &profile.Sample{
			Location: locs,
			Value:    values,
		})
	}

	if err := prof.CheckValid(); err != nil {
		return fmt.Errorf("pprof validation: %w", err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	// `Profile.Write` already gzip-compresses the proto payload.
	return prof.Write(out)
}
