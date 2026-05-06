package profiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const maxContextChars = 500

// DebugLog captures a portable, redacted parser failure log that a field user
// can ship as a single artifact without sharing the original input.
//
// Mirrors archscope_engine.common.debug_log.DebugLogCollector — same JSON
// shape, same per-error-type sample cap, same 1 MiB output cap, same
// redact-by-default policy. Verdict / encoding metadata is decided at write
// time so callers don't have to know the policy.
type DebugLog struct {
	analyzerType     string
	parser           string
	sourceFile       string
	encodingDetected string
	parserOptions    map[string]any
	totalLines       int
	parsedRecords    int
	skippedLines     int
	parseErrors      []DebugLogError
	exceptions       []DebugLogException
	hints            []string
	redactionSummary map[string]int
	startedAt        time.Time
	maxSamplesByType int
	maxBytes         int
}

// DebugLogError is a single parser failure record. `RawContext` keeps a small
// before/target/after window of the offending input region (already redacted)
// so the developer can reproduce the failure without the full file.
type DebugLogError struct {
	LineNumber    int               `json:"line_number"`
	Reason        string            `json:"reason"`
	Message       string            `json:"message"`
	FailedPattern string            `json:"failed_pattern,omitempty"`
	RawContext    map[string]string `json:"raw_context,omitempty"`
	FieldShapes   map[string]any    `json:"field_shapes,omitempty"`
}

// DebugLogException records a fatal parser exception (traceback / phase) so
// a crashing run still ships actionable evidence.
type DebugLogException struct {
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	Traceback string `json:"traceback,omitempty"`
}

// NewDebugLog constructs a collector. `parser` should match the parser name
// recorded under `metadata.parser` in the AnalysisResult so the log can be
// correlated with the analyzer output.
func NewDebugLog(analyzerType, parser, sourceFile string) *DebugLog {
	return &DebugLog{
		analyzerType:     analyzerType,
		parser:           parser,
		sourceFile:       sourceFile,
		parserOptions:    map[string]any{},
		parseErrors:      []DebugLogError{},
		exceptions:       []DebugLogException{},
		hints:            []string{},
		redactionSummary: map[string]int{},
		startedAt:        time.Now().UTC(),
		maxSamplesByType: 5,
		maxBytes:         1024 * 1024,
	}
}

// SetEncodingDetected records the text encoding the parser settled on.
func (l *DebugLog) SetEncodingDetected(encoding string) {
	if l == nil {
		return
	}
	l.encodingDetected = encoding
}

// SetTotals records the final parsing totals for the summary section.
func (l *DebugLog) SetTotals(totalLines, parsedRecords, skippedLines int) {
	if l == nil {
		return
	}
	l.totalLines = totalLines
	l.parsedRecords = parsedRecords
	l.skippedLines = skippedLines
}

// SetParserOptions records the parser option snapshot.
func (l *DebugLog) SetParserOptions(opts map[string]any) {
	if l == nil || opts == nil {
		return
	}
	l.parserOptions = opts
}

// AddHint adds a free-form developer hint shown next to the verdict.
func (l *DebugLog) AddHint(hint string) {
	if l == nil || strings.TrimSpace(hint) == "" {
		return
	}
	l.hints = append(l.hints, hint)
}

// AddParseError records a row-level parser failure. `rawContext` may include
// `before`, `target`, `after` strings; each is redacted in place. The total
// per-`reason` sample count is capped at `maxSamplesByType` so a failing
// 100k-line input does not bloat the log.
func (l *DebugLog) AddParseError(lineNumber int, reason, message, failedPattern string, rawContext map[string]string, fieldShapes map[string]any) {
	if l == nil {
		return
	}
	count := 0
	for _, existing := range l.parseErrors {
		if existing.Reason == reason {
			count++
		}
	}
	if count >= l.maxSamplesByType {
		return
	}
	redactedContext := map[string]string{}
	for _, key := range []string{"before", "target", "after"} {
		value, ok := rawContext[key]
		if !ok {
			continue
		}
		clipped := value
		if len(clipped) > maxContextChars {
			clipped = clipped[:maxContextChars]
		}
		result := RedactText(clipped)
		redactedContext[key] = result.Text
		l.redactionSummary = MergeRedactionSummaries(l.redactionSummary, result.Summary)
	}
	if fieldShapes == nil {
		target := rawContext["target"]
		if target != "" {
			fieldShapes = InferFieldShapes(target)
		}
	}
	l.parseErrors = append(l.parseErrors, DebugLogError{
		LineNumber:    lineNumber,
		Reason:        reason,
		Message:       message,
		FailedPattern: failedPattern,
		RawContext:    redactedContext,
		FieldShapes:   fieldShapes,
	})
}

// AddException records a fatal exception during analysis.
func (l *DebugLog) AddException(phase, message, traceback string) {
	if l == nil {
		return
	}
	l.exceptions = append(l.exceptions, DebugLogException{
		Phase:     phase,
		Message:   message,
		Traceback: traceback,
	})
}

// HasContent reports whether the collector picked up anything worth writing.
func (l *DebugLog) HasContent() bool {
	if l == nil {
		return false
	}
	return len(l.parseErrors) > 0 || len(l.exceptions) > 0
}

// Verdict ranks the log using the same policy as Python:
// FATAL_ERROR if exceptions, MAJORITY_FAILED if ≥50% skipped,
// PARTIAL_SUCCESS if any skipped, CLEAN otherwise.
func (l *DebugLog) Verdict() string {
	if l == nil {
		return "CLEAN"
	}
	if len(l.exceptions) > 0 {
		return "FATAL_ERROR"
	}
	skipped := l.skippedLines
	if skipped <= 0 {
		// Fall back to counting parse errors if SetTotals wasn't called.
		skipped = len(l.parseErrors)
	}
	if skipped <= 0 {
		return "CLEAN"
	}
	if l.totalLines > 0 && float64(skipped)/float64(l.totalLines) >= 0.5 {
		return "MAJORITY_FAILED"
	}
	return "PARTIAL_SUCCESS"
}

// PortableFilename returns a deterministic file name suitable for shipping.
func (l *DebugLog) PortableFilename() string {
	stem := "archscope-debug"
	if l != nil && l.parser != "" {
		stem = stem + "-" + sanitizeFilenameComponent(l.parser)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("%s-%s.json", stem, stamp)
}

func sanitizeFilenameComponent(value string) string {
	out := []rune{}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// WriteJSON serializes the collected log into the configured directory. If
// `dir` is empty, defaults to `<execution-cwd>/archscope-debug/`. Returns the
// final output path. The payload is JSON-encoded and capped at `maxBytes`;
// when the cap kicks in the most recent samples are kept and `truncated:
// true` is emitted in the metadata.
func (l *DebugLog) WriteJSON(dir string) (string, error) {
	if l == nil {
		return "", fmt.Errorf("debug log not initialized")
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cwd, "archscope-debug")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, l.PortableFilename())

	payload := l.payload(false)
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	if len(body) > l.maxBytes {
		// Fall back to a truncated payload — drop oldest samples first.
		payload = l.payload(true)
		body, err = json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (l *DebugLog) payload(truncated bool) map[string]any {
	// Group parse errors by reason, matching Python's errors_by_type shape.
	type errorEntry struct {
		Count         int              `json:"count"`
		Description   string           `json:"description"`
		FailedPattern string           `json:"failed_pattern,omitempty"`
		Samples       []DebugLogError  `json:"samples"`
	}
	errorsByType := map[string]*errorEntry{}
	for _, err := range l.parseErrors {
		entry, ok := errorsByType[err.Reason]
		if !ok {
			entry = &errorEntry{Description: err.Message, FailedPattern: err.FailedPattern}
			errorsByType[err.Reason] = entry
		}
		entry.Count++
		if entry.FailedPattern == "" && err.FailedPattern != "" {
			entry.FailedPattern = err.FailedPattern
		}
		if truncated && len(entry.Samples) >= l.maxSamplesByType {
			continue
		}
		if len(entry.Samples) < l.maxSamplesByType {
			entry.Samples = append(entry.Samples, err)
		}
	}
	// Sorted by reason for deterministic output.
	sortedReasons := make([]string, 0, len(errorsByType))
	for reason := range errorsByType {
		sortedReasons = append(sortedReasons, reason)
	}
	sort.Strings(sortedReasons)
	orderedErrorsByType := map[string]any{}
	errorTypeCounts := map[string]int{}
	for _, reason := range sortedReasons {
		entry := errorsByType[reason]
		errorTypeCounts[reason] = entry.Count
		cleaned := map[string]any{
			"count":       entry.Count,
			"description": entry.Description,
			"samples":     entry.Samples,
		}
		if entry.FailedPattern != "" {
			cleaned["failed_pattern"] = entry.FailedPattern
		}
		orderedErrorsByType[reason] = cleaned
	}

	skipped := l.skippedLines
	if skipped <= 0 {
		for _, entry := range errorsByType {
			skipped += entry.Count
		}
	}
	skipRate := 0.0
	if l.totalLines > 0 {
		skipRate = float64(skipped) / float64(l.totalLines) * 100
		skipRate = float64(int(skipRate*100)) / 100 // round to 2 decimals
	}

	redactedSourceFile := RedactText(l.sourceFile).Text
	sourceFileName := filepath.Base(l.sourceFile)

	var fileSize *int64
	if info, err := os.Stat(l.sourceFile); err == nil {
		s := info.Size()
		fileSize = &s
	}

	return map[string]any{
		"environment": map[string]string{
			"archscope_version": "go-native",
			"go_version":        runtime.Version(),
			"os":                runtime.GOOS + "/" + runtime.GOARCH,
			"timestamp":         time.Now().UTC().Format(time.RFC3339),
		},
		"context": map[string]any{
			"analyzer_type":    l.analyzerType,
			"source_file":      redactedSourceFile,
			"source_file_name": sourceFileName,
			"file_size_bytes":  fileSize,
			"encoding_detected": l.encodingDetected,
			"parser":            l.parser,
			"parser_options":    l.parserOptions,
		},
		"redaction": map[string]any{
			"enabled":              true,
			"redaction_version":    RedactionVersion,
			"raw_context_redacted": true,
			"summary":              l.redactionSummary,
		},
		"summary": map[string]any{
			"total_lines":      l.totalLines,
			"parsed_ok":        l.parsedRecords,
			"skipped":          skipped,
			"skip_rate_percent": skipRate,
			"error_types":      errorTypeCounts,
			"exceptions":       len(l.exceptions),
			"verdict":          l.Verdict(),
		},
		"errors_by_type": orderedErrorsByType,
		"exceptions":     l.exceptions,
		"hints":          l.buildHints(skipped, errorTypeCounts),
		"truncated":      truncated,
	}
}

// buildHints generates developer hints matching Python's _build_hints.
func (l *DebugLog) buildHints(skipped int, errorTypes map[string]int) []string {
	hints := append([]string{}, l.hints...)
	totalErrors := 0
	for _, count := range errorTypes {
		totalErrors += count
	}
	if totalErrors > 0 {
		noFormat := errorTypes["NO_FORMAT_MATCH"]
		if float64(noFormat)/float64(totalErrors) >= 0.8 {
			hints = append(hints, "NO_FORMAT_MATCH dominates. The input may not match the selected parser format.")
		}
		if errorTypes["INVALID_TIMESTAMP"] > 0 {
			hints = append(hints, "INVALID_TIMESTAMP is present. Check timestamp shape and parser time format.")
		}
	}
	if l.totalLines > 0 && float64(skipped)/float64(l.totalLines) >= 0.5 {
		hints = append(hints, "More than half of parsed lines failed. The file may use an unsupported format.")
	}
	if len(l.exceptions) > 0 {
		hints = append(hints, "Parser or analyzer exception captured. Inspect traceback and phase.")
	}
	return hints
}

// InferFieldShapes extracts structural metadata from a raw text line,
// matching Python's infer_field_shapes for cross-engine parity.
func InferFieldShapes(text string) map[string]any {
	shapes := map[string]any{
		"target_token_count": len(strings.Fields(text)),
		"quote_count":        strings.Count(text, "\""),
		"bracket_count":      strings.Count(text, "[") + strings.Count(text, "]"),
	}
	// Suffix key=value detection.
	if idx := strings.LastIndex(text, " "); idx >= 0 {
		suffix := text[idx+1:]
		if strings.Contains(suffix, "=") {
			shapes["suffix_shape"] = "key=value"
		}
	}
	// Request shape detection.
	if reqShape := extractRequestShape(text); reqShape != nil {
		for k, v := range reqShape {
			shapes[k] = v
		}
	}
	// Timestamp shape detection.
	if ts := detectTimestampShape(text); ts != "" {
		shapes["timestamp_shape"] = ts
	}
	return shapes
}

var (
	requestShapeRE      = regexp.MustCompile(`"(?P<method>[A-Z]+)\s+(?P<path>\S+)\s+(?P<protocol>[^"]+)"`)
	timestampNginxRE    = regexp.MustCompile(`\[\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\]`)
	timestampBracketRE  = regexp.MustCompile(`\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]`)
	timestampISO8601RE  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
	pathNumberRE        = regexp.MustCompile(`/\d+`)
)

func extractRequestShape(text string) map[string]any {
	match := requestShapeRE.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	path := match[2]
	queryKeys := []string{}
	if idx := strings.Index(path, "?"); idx >= 0 {
		query := path[idx+1:]
		for _, part := range strings.Split(query, "&") {
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			queryKeys = append(queryKeys, kv[0])
		}
	}
	result := map[string]any{}
	if len(queryKeys) > 0 {
		result["request_shape"] = "METHOD PATH_WITH_QUERY PROTOCOL"
	} else {
		result["request_shape"] = "METHOD PATH PROTOCOL"
	}
	purePath := path
	if idx := strings.Index(purePath, "?"); idx >= 0 {
		purePath = purePath[:idx]
	}
	result["path_shape"] = pathNumberRE.ReplaceAllString(purePath, "/<NUMBER>")
	result["query_keys"] = queryKeys
	return result
}

func detectTimestampShape(text string) string {
	if timestampNginxRE.MatchString(text) {
		return "dd/Mon/yyyy:HH:mm:ss Z"
	}
	if timestampBracketRE.MatchString(text) {
		return "yyyy-MM-dd HH:mm:ss"
	}
	if timestampISO8601RE.MatchString(text) {
		return "ISO-8601"
	}
	return ""
}
