// Package diagnostics ports archscope_engine.common.diagnostics —
// the ParserDiagnostics builder every parser uses to report total /
// parsed / skipped counts plus per-reason breakdowns and bounded
// sample lists. Same JSON shape as the Python implementation so
// cross-engine debug logs stay diff-able through the T-244 / T-390
// parity gate.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] diagnostics 패키지 — 모든 파서가 공유하는 ParserDiagnostics
// 빌더.
//
// 책임
//   "라인 단위 파싱이 끝났을 때 무엇을 보고할 것인가" 의 표준화.
//   • TotalLines       : 본 파일에서 실제로 읽은 라인 수.
//   • ParsedRecords    : 정상 record 로 변환된 수.
//   • SkippedRecords   : skip 사유 발생 수.
//   • Warnings         : 비치명적 경고 발생 수.
//   • Reasons          : 사유 코드별 카운트 (e.g. INVALID_OTEL_JSON: 5).
//   • Samples/Warnings/Errors : sample 라인 (cap MaxDiagnosticSamples=100).
//
// 두 가지 cap
//   MaxDiagnosticSamples (100)  : 동일 사유가 수만 건 발생해도 sample
//                                  은 100개까지만 보관.
//   RawPreviewLimit       (200)  : sample 의 raw 라인 preview 는 200
//                                  byte 까지만 (큰 라인이 응답을 부풀
//                                  리는 것 방지).
//
// 분석기에서의 활용
//   metadata.diagnostics 에 그대로 직렬화. UI 의 diagnostics 패널이
//   사유별 카운트 + 샘플 라인을 표시 → 사용자가 "이 파일은 N% 만
//   파싱됐고 이런 사유로 skip 됐다" 를 파악 가능.
package diagnostics

import "strings"

const (
	// MaxDiagnosticSamples caps how many sample lines we keep per
	// kind (samples / warnings / errors) so a 50k-line input doesn't
	// blow the response payload.
	MaxDiagnosticSamples = 100
	// RawPreviewLimit truncates the raw line preview shipped with
	// each diagnostic sample.
	RawPreviewLimit = 200
)

// Sample is the per-incident record stored under
// `diagnostics.samples`/`warnings`/`errors`.
type Sample struct {
	LineNumber int    `json:"line_number"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	RawPreview string `json:"raw_preview"`
}

// ParserDiagnostics mirrors the Python dataclass' JSON projection.
type ParserDiagnostics struct {
	SourceFile      *string        `json:"source_file"`
	Format          string         `json:"format"`
	TotalLines      int            `json:"total_lines"`
	ParsedRecords   int            `json:"parsed_records"`
	SkippedLines    int            `json:"skipped_lines"`
	SkippedByReason map[string]int `json:"skipped_by_reason"`
	Samples         []Sample       `json:"samples"`
	WarningCount    int            `json:"warning_count"`
	ErrorCount      int            `json:"error_count"`
	Warnings        []Sample       `json:"warnings"`
	Errors          []Sample       `json:"errors"`
}

// New constructs a fresh diagnostics builder with non-nil maps/slices
// so JSON serialization always emits `{}` and `[]` rather than `null`.
func New(format string) *ParserDiagnostics {
	return &ParserDiagnostics{
		Format:          format,
		SkippedByReason: map[string]int{},
		Samples:         []Sample{},
		Warnings:        []Sample{},
		Errors:          []Sample{},
	}
}

// SetSourceFile captures the absolute source path on the result so
// renderers can surface "where did this come from" without re-walking
// metadata.
func (d *ParserDiagnostics) SetSourceFile(path string) {
	d.SourceFile = &path
}

// AddSkipped records a row-level parser skip — increments the
// skipped counter, bumps the per-reason histogram, and appends to
// the bounded errors + samples lists.
func (d *ParserDiagnostics) AddSkipped(lineNumber int, reason, message, rawLine string) {
	d.SkippedLines++
	d.SkippedByReason[reason]++
	d.addIssue(lineNumber, reason, message, rawLine, "error", true)
}

// AddWarning records a non-fatal warning (e.g. ENCODING_FALLBACK,
// MISSING_PARENT). When `skipped=true` it also bumps the skip
// counter — Python's behaviour for things like CSV duplicate keys.
func (d *ParserDiagnostics) AddWarning(lineNumber int, reason, message, rawLine string, skipped bool) {
	if skipped {
		d.SkippedLines++
		d.SkippedByReason[reason]++
		d.addIssue(lineNumber, reason, message, rawLine, "warning", true)
		return
	}
	d.addIssue(lineNumber, reason, message, rawLine, "warning", true)
}

// AddError records a fatal-but-recoverable parser error without
// bumping the skip counter (e.g. INVALID_SVG when the whole file is
// rejected up front).
func (d *ParserDiagnostics) AddError(lineNumber int, reason, message, rawLine string) {
	d.addIssue(lineNumber, reason, message, rawLine, "error", true)
}

func (d *ParserDiagnostics) addIssue(lineNumber int, reason, message, rawLine, severity string, includeInSamples bool) {
	preview := rawLine
	if len(preview) > RawPreviewLimit {
		preview = preview[:RawPreviewLimit]
	}
	issue := Sample{
		LineNumber: lineNumber,
		Reason:     reason,
		Message:    strings.TrimSpace(message),
		RawPreview: preview,
	}
	if severity == "warning" {
		d.WarningCount++
		if len(d.Warnings) < MaxDiagnosticSamples {
			d.Warnings = append(d.Warnings, issue)
		}
	} else {
		d.ErrorCount++
		if len(d.Errors) < MaxDiagnosticSamples {
			d.Errors = append(d.Errors, issue)
		}
	}
	if includeInSamples && len(d.Samples) < MaxDiagnosticSamples {
		d.Samples = append(d.Samples, issue)
	}
}
