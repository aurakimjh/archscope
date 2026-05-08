// ─────────────────────────────────────────────────────────────────────
// [한글] collapsed — async-profiler "collapsed" 텍스트 파서.
//
// 책임/목적
//   async-profiler / perf-script | flamegraph 의 stack-collapsed.pl 등이
//   생산하는 표준 collapsed 텍스트를 stack map 으로 파싱.
//   line 형식: "frame1;frame2;...;leaf <whitespace> 12345" (마지막 토큰이 sample count).
//
// 알고리즘 흐름
//   1) 파일을 line 단위 스캔 (16MB buffer).
//   2) 빈 라인 skip.
//   3) parseCollapsedLine: 마지막 whitespace 위치를 찾아 stack/count 분리.
//   4) count 가 정수 / >0 / 음수 아님을 검증.
//   5) stacks[stack] += samples. 동일 stack 의 중복 라인은 누적.
//
// 진단 분류
//   - MISSING_SAMPLE_COUNT  : whitespace 없는 라인 (수치 없음)
//   - MISSING_STACK         : stack 부분이 비어있음
//   - INVALID_SAMPLE_COUNT  : 마지막 토큰이 정수가 아님
//   - NEGATIVE_SAMPLE_COUNT : 음수 sample
//   - ZERO_SAMPLE_COUNT     : 0 sample (Python 동작과 동일하게 reject)
//   - EMPTY_FILE / NO_VALID_RECORDS : 파일 단위 경고
//
// 주의 / 트리키한 부분
//   • lastWhitespace 는 Unicode whitespace 까지 인식 (Python str.rsplit
//     기본 동작과 동일). ASCII 한정이 아님에 유의.
//   • addDiagnosticError / addDiagnosticWarning 는 svg.go / html.go /
//     jennifer.go 에서도 공유하는 진단 누적 helper.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// timeNow / timeSince are wrappers so tests can stub time without
// importing the real clock everywhere. The defaults forward to
// time.Now / time.Since.
var (
	timeNow   = time.Now
	timeSince = time.Since
)

// Collapsed-parser memory guards. The 70M-wall regression (process
// killed by Windows OOM) was traced to:
//   1. Per-line scanner buffer too small for very deep stacks; some
//      Spring/Hibernate sample lines exceed 1MB.
//   2. Unbounded unique-stack map: a 70MB collapsed profile can hold
//      300k+ distinct stacks, each with 100+ frames, before tree
//      construction.
//
// These constants are the defaults applied via normalizeOptions when
// the renderer doesn't pass explicit values. They're tuned so a
// 70MB wall profile fits in ~1-2GB working-set rather than the
// 8-12GB the previous unbounded path consumed.
// Defaults tightened after the 60 MB / 6 GB OOM regression. The old
// limits (250k stacks × depth 256) made it possible for tree
// construction to spawn ~10M nodes whose Path slices alone consumed
// 6+ GB. Halving each axis caps tree size at ~1.5M nodes, which
// fits comfortably under 1 GB even on the worst inputs while still
// covering >99% of useful samples on real wall profiles.
//
// [한글] 메모리 가드 상수 모음.
//   • defaultCollapsedScannerBuffer (64 MB) : 한 라인 최대 길이.
//     Spring/Hibernate 의 매우 깊은 stack 한 줄이 1MB 를 넘는 경우
//     있어 넉넉히.
//   • defaultMaxUniqueStacks (100k) / MaxStackDepth (128) :
//     70MB wall profile 가 ~1-2GB working-set 에 들어가도록 조정한 cap.
//   • defaultMaxRSSMB (4GB) : soft RSS ceiling. 초과 시 친절한 에러
//     반환 + progress log flush — OS SIGKILL 보다 사용자 경험이 좋음.
//   • defaultMaxFlamegraphNodes (100k) : 렌더러 payload cap. 100k
//     노드 트리는 gzip 시 ~5 MB, canvas 가 끊김 없이 렌더 가능.
//     <0 설정 시 pruning 끔.
const (
	defaultCollapsedScannerBuffer = 1024 * 1024 * 64 // 64 MB per line cap
	defaultMaxUniqueStacks        = 100_000
	defaultMaxStackDepth          = 128
	// defaultMaxRSSMB is the soft RSS ceiling. The analyzer aborts
	// with a friendly error rather than letting the OS issue a hard
	// SIGKILL when alloc crosses this — we lose the result but the
	// progress log is fully flushed so the user knows what happened.
	defaultMaxRSSMB = 4096 // 4 GB
	// defaultMaxFlamegraphNodes caps the renderer payload. A 100k-
	// node tree gzip-encodes to ~5 MB and the canvas can render it
	// without freezing; bigger trees are pruned (top-K children per
	// level + "...other" tail). Set MaxFlamegraphNodes < 0 to skip
	// pruning entirely.
	defaultMaxFlamegraphNodes = 100_000
)

// ParseCollapsedFile is the back-compat entry that uses default
// memory guards. Callers that want to override the caps should use
// ParseCollapsedFileWithOptions.
//
// [한글] ParseCollapsedFile — collapsed text 파일을 stack map 으로.
// 진단(diagnostics) 에 라인 통계와 skip 사유 누적, 빈 파일/파싱 0건
// 경고도 부착. 메모리 가드를 default 값으로 적용한 wrapper —
// override 가 필요하면 ParseCollapsedFileWithOptions 사용.
func ParseCollapsedFile(path string) (map[string]int, ParserDiagnostics, error) {
	return ParseCollapsedFileWithOptions(path, Options{})
}

func ParseCollapsedFileWithOptions(path string, opts Options) (map[string]int, ParserDiagnostics, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, ParserDiagnostics{}, err
	}
	defer file.Close()

	maxStacks := opts.MaxUniqueStacks
	if maxStacks <= 0 {
		maxStacks = defaultMaxUniqueStacks
	}
	maxDepth := opts.MaxStackDepth
	if maxDepth <= 0 {
		maxDepth = defaultMaxStackDepth
	}

	source := path
	diagnostics := ParserDiagnostics{
		SourceFile:      &source,
		Format:          "async_profiler_collapsed",
		SkippedByReason: map[string]int{},
	}
	if info, statErr := file.Stat(); statErr == nil {
		diagnostics.BytesRead = info.Size()
	}

	pl := opts.ProgressLog
	pl.Phase("parse-start", fmt.Sprintf("file=%s size=%d cap_stacks=%d cap_depth=%d", path, diagnostics.BytesRead, maxStacks, maxDepth))

	stacks := map[string]int{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), defaultCollapsedScannerBuffer)

	// Tick cadence (revised after the 60 MB / 6 GB regression): we
	// want a heartbeat well before the first 64 MB chunk completes
	// because OOM kills tend to fire mid-build. Tighter cadence:
	// every 50k lines OR every 4 MB OR every 750 ms — whichever
	// hits first. The overhead is negligible compared to a missing
	// log on a 6 GB OOM.
	const tickEveryLines = 50_000
	const tickEveryBytes = 4 * 1024 * 1024
	const tickEveryMs = 750
	bytesProcessed := int64(0)
	bytesAtLastTick := int64(0)
	startTime := timeNow()
	lastTickAt := startTime

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		diagnostics.TotalLines++
		raw := scanner.Text()
		bytesProcessed += int64(len(raw)) + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		stack, samples, reason, message := parseCollapsedLine(line)
		if reason != "" {
			addDiagnosticError(&diagnostics, lineNumber, reason, message, raw)
			continue
		}
		// Depth guard: an unbounded path slice is the dominant
		// allocator when wall-mode profiles include thread dumps with
		// 1k+ frames. Truncating preserves the leaf classification
		// (last frame) which is what the timeline cares about.
		if maxDepth > 0 {
			depth := strings.Count(stack, ";") + 1
			if depth > diagnostics.MaxObservedDepth {
				diagnostics.MaxObservedDepth = depth
			}
			if depth > maxDepth {
				stack = truncateStackDepth(stack, maxDepth)
				diagnostics.OverDepthRecords++
			}
		}
		// Unique-stack guard: once the cap is hit, drop new entries
		// rather than letting the map grow without bound. Existing
		// entries still aggregate, so the dominant stacks keep their
		// counts even when the long-tail is truncated.
		if maxStacks > 0 && len(stacks) >= maxStacks {
			if _, ok := stacks[stack]; !ok {
				diagnostics.DroppedStacks++
				if diagnostics.DroppedStackReason == "" {
					diagnostics.DroppedStackReason = fmt.Sprintf("MAX_UNIQUE_STACKS_REACHED (cap=%d)", maxStacks)
				}
				continue
			}
		}
		stacks[stack] += samples
		diagnostics.ParsedRecords++

		if pl != nil {
			now := timeNow()
			elapsedSinceTick := now.Sub(lastTickAt).Milliseconds()
			if lineNumber%tickEveryLines == 0 ||
				bytesProcessed-bytesAtLastTick >= tickEveryBytes ||
				elapsedSinceTick >= tickEveryMs {
				bytesAtLastTick = bytesProcessed
				lastTickAt = now
				elapsed := timeSince(startTime)
				rate := float64(bytesProcessed) / elapsed.Seconds() / (1024 * 1024)
				pl.Tick("parse line=%d bytes=%d/%dMB unique=%d dropped=%d depth_max=%d rate=%.1fMB/s",
					lineNumber, bytesProcessed/(1024*1024), diagnostics.BytesRead/(1024*1024),
					len(stacks), diagnostics.DroppedStacks, diagnostics.MaxObservedDepth, rate)
				pl.Mem("parse")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		pl.Phase("parse-error", fmt.Sprintf("scanner: %v", err))
		return nil, diagnostics, err
	}
	pl.Phase("parse-end", fmt.Sprintf("lines=%d unique=%d dropped=%d", lineNumber, len(stacks), diagnostics.DroppedStacks))
	if diagnostics.TotalLines == 0 {
		addDiagnosticWarning(&diagnostics, 0, "EMPTY_FILE", "Collapsed profiler file is empty.", "")
	}
	if diagnostics.ParsedRecords == 0 && diagnostics.TotalLines > 0 {
		addDiagnosticWarning(&diagnostics, 0, "NO_VALID_RECORDS", "No valid collapsed profiler records were parsed.", "")
	}
	if diagnostics.DroppedStacks > 0 {
		addDiagnosticWarning(&diagnostics, 0, "MAX_UNIQUE_STACKS_REACHED",
			fmt.Sprintf("Dropped %d unique stacks beyond the cap of %d. Increase MaxUniqueStacks in options to retain more.", diagnostics.DroppedStacks, maxStacks), "")
	}
	if diagnostics.OverDepthRecords > 0 {
		addDiagnosticWarning(&diagnostics, 0, "STACK_DEPTH_TRUNCATED",
			fmt.Sprintf("Truncated %d records beyond depth %d (max observed depth=%d).", diagnostics.OverDepthRecords, maxDepth, diagnostics.MaxObservedDepth), "")
	}
	return stacks, diagnostics, nil
}

// truncateStackDepth keeps the deepest `keep` frames of a collapsed
// stack. We preserve the leaf side because the timeline classifier
// keys off the deepest frame; a leading "...truncated..." marker
// makes the truncation visible in top-stacks tables.
//
// [한글] truncateStackDepth — stack 의 가장 깊은 keep 개 프레임만 유지.
// 잎쪽을 보존하는 이유: timeline 분류기가 가장 깊은 프레임을 키로 사용.
// 잘렸음을 알리는 "...truncated;" 헤드 마커로 사용자가 잘림을 인식 가능.
func truncateStackDepth(stack string, keep int) string {
	parts := strings.Split(stack, ";")
	if len(parts) <= keep {
		return stack
	}
	tail := parts[len(parts)-keep:]
	return "...truncated;" + strings.Join(tail, ";")
}

// [한글] parseCollapsedLine — 한 라인 검증 및 (stack, samples) 분리.
// 반환 (stack, samples, reason, message). reason!="" 면 진단 사유.
func parseCollapsedLine(line string) (string, int, string, string) {
	index := lastWhitespace(line)
	if index < 0 {
		return "", 0, "MISSING_SAMPLE_COUNT", "Collapsed line must end with a sample count."
	}
	stack := strings.TrimSpace(line[:index])
	countText := strings.TrimSpace(line[index:])
	if stack == "" {
		return "", 0, "MISSING_STACK", "Collapsed line must contain at least one stack frame."
	}
	samples, err := strconv.Atoi(countText)
	if err != nil {
		return "", 0, "INVALID_SAMPLE_COUNT", fmt.Sprintf("Invalid sample count: %q.", countText)
	}
	if samples < 0 {
		return "", 0, "NEGATIVE_SAMPLE_COUNT", "Sample count must not be negative."
	}
	if samples == 0 {
		return "", 0, "ZERO_SAMPLE_COUNT", "Sample count must be positive."
	}
	return stack, samples, "", ""
}

func lastWhitespace(value string) int {
	last := -1
	for index, r := range value {
		if unicode.IsSpace(r) {
			last = index
		}
	}
	return last
}

func addDiagnosticError(diagnostics *ParserDiagnostics, lineNumber int, reason, message, raw string) {
	sample := DiagnosticSample{
		LineNumber: lineNumber,
		Reason:     reason,
		Message:    message,
		RawPreview: safePreview(raw),
	}
	diagnostics.SkippedLines++
	diagnostics.SkippedByReason[reason]++
	diagnostics.Samples = append(diagnostics.Samples, sample)
	diagnostics.Errors = append(diagnostics.Errors, sample)
	diagnostics.ErrorCount = len(diagnostics.Errors)
}

func addDiagnosticWarning(diagnostics *ParserDiagnostics, lineNumber int, reason, message, raw string) {
	sample := DiagnosticSample{
		LineNumber: lineNumber,
		Reason:     reason,
		Message:    message,
		RawPreview: safePreview(raw),
	}
	diagnostics.Samples = append(diagnostics.Samples, sample)
	diagnostics.Warnings = append(diagnostics.Warnings, sample)
	diagnostics.WarningCount = len(diagnostics.Warnings)
}
