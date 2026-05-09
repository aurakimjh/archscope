// ─────────────────────────────────────────────────────────────────────
// [한글] pprof — collapsed stack map 을 Google pprof.proto 로 export.
//
// 책임/목적
//   profiler 결과(map[stackKey]samples) 를 표준 pprof 바이너리 형식으로
//   저장한다. 결과 파일은 `go tool pprof`, speedscope, pprof.me 등 다양한
//   생태계 툴에서 그대로 열 수 있다. Python 측 exporter 와 같은 wire 포맷.
//
// 변환 규칙
//   - SampleType: ("samples","count") 가 디폴트. intervalMs>0 이면 추가
//     SampleType("duration","milliseconds") 를 붙여 시간 단위 차트도 가능.
//   - PeriodType: intervalMs>0 일 때 ("wall","milliseconds") + Period.
//   - Function/Location: frame 이름 단위 dedupe (functions / locations map).
//     pprof 는 ID 1 부터 시작.
//   - Sample.Location: pprof 는 leaf-first 순서를 요구하므로 collapsed
//     (root-first) 를 reverse 해서 넣는다.
//
// 트리키한 부분
//   • prof.Write 는 자동 gzip 압축. 호출부는 .gz 따로 처리할 필요 없음.
//   • collapsed 빈 frame 은 trim 후 drop (Python 동작과 동일).
//   • CheckValid 는 location/function ID 무결성 검증. 우리 빌더가 단조
//     증가 ID 를 보장하므로 보통 성공하지만 sanity check 로 호출.
// ─────────────────────────────────────────────────────────────────────

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
//
// [한ст] ExportToPprof — collapsed map → pprof.gz 변환.
// 매개변수: stacks(필수), outPath(필수), sampleType("samples" 디폴트),
// unit("count" 디폴트), intervalMs(>0 이면 duration 정보 추가).
// 빈 outPath 면 에러. 0 이하 sample 은 skip.
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
