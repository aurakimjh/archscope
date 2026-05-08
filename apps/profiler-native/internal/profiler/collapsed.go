package profiler

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
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
const (
	defaultCollapsedScannerBuffer = 1024 * 1024 * 64 // 64 MB per line cap
	defaultMaxUniqueStacks        = 250_000
	defaultMaxStackDepth          = 256
)

// ParseCollapsedFile is the back-compat entry that uses default
// memory guards. Callers that want to override the caps should use
// ParseCollapsedFileWithOptions.
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

	stacks := map[string]int{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), defaultCollapsedScannerBuffer)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		diagnostics.TotalLines++
		raw := scanner.Text()
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
		// rather than letting the map grow without bound. The kept
		// entries are the most-frequent ones because they were
		// accumulated first under the typical async-profiler ordering.
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
	}
	if err := scanner.Err(); err != nil {
		return nil, diagnostics, err
	}
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
func truncateStackDepth(stack string, keep int) string {
	parts := strings.Split(stack, ";")
	if len(parts) <= keep {
		return stack
	}
	tail := parts[len(parts)-keep:]
	return "...truncated;" + strings.Join(tail, ";")
}

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
