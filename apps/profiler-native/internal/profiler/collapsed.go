package profiler

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func ParseCollapsedFile(path string) (map[string]int, ParserDiagnostics, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, ParserDiagnostics{}, err
	}
	defer file.Close()

	source := path
	diagnostics := ParserDiagnostics{
		SourceFile:      &source,
		Format:          "async_profiler_collapsed",
		SkippedByReason: map[string]int{},
	}
	stacks := map[string]int{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024*16)

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
	return stacks, diagnostics, nil
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
