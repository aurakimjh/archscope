// Package models defines the AnalysisResult envelope every analyzer
// emits. The shape is the engine ↔ UI contract; all parsers /
// analyzers feed the same struct so the renderer can render any
// result type uniformly.
//
// The profiler-track Go binary already has its own AnalysisResult
// (apps/profiler-native/internal/profiler) tuned to the profiler
// shape — this package generalises that shape so Access Log / GC
// Log / Thread Dump analyzers can also use it. We keep the JSON
// keys identical to the Python implementation so the T-244 / T-390
// parity job can compare outputs byte-for-byte.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] models 패키지 — 모든 분석기가 emit 하는 AnalysisResult
// envelope 의 정의.
//
// 핵심 원칙
//   • 엔진 ↔ UI 사이의 단일 전송 boundary 입니다. 신규 분석기는 이
//     contract 만 만족하면 UI 측 코드를 추가하지 않고도 렌더됩니다.
//   • Python 의 AnalysisResult 와 JSON 키를 byte-for-byte 동일하게
//     유지합니다. parity gate 가 두 엔진 출력을 비교하려면 키 이름,
//     키 순서(MarshalJSON 의 map key 정렬), 빈 컨테이너 표현이
//     모두 정확히 일치해야 합니다.
//
// 왜 map[string]any 인가?
//   각 분석기는 종류가 매우 다른 데이터를 emit 합니다.
//     - access log = 분당 timeline + URL stats 표
//     - profiler   = flame tree
//     - thread dump = 상태 분포 + 멀티 덤프 finding
//   이를 Go 구조체로 강타입화하면 하나의 enum 으로 합쳐야 하는데,
//   결과적으로 모든 분석기가 서로의 타입을 알게 되고 Python 측에
//   대응 dataclass 를 만들어야 합니다. 그래서 의도적으로 dict 형태를
//   유지하고, 렌더러는 Type 문자열로 디스패치합니다.
//
// 주의 사항
//   • New() 가 빈 맵/슬라이스를 미리 채워주지 않으면 Marshal 결과가
//     "field": null 이 되어 UI 측에서 옵셔널 체이닝이 줄줄이 깨집니다.
//   • Metadata 의 Extra 는 Marshal 시 typed 필드와 같은 레벨로
//     flatten 됩니다 — Python dict 모양과 1:1 일치시키기 위함.
//   • Extra 가 typed 필드와 키가 겹치면 typed 가 항상 우선 (오기
//     방지).
package models

import (
	"encoding/json"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

// AnalysisResult is the universal envelope. Each analyzer populates
// only the fields it needs; the rest stay as their zero value.
//
// `Series` / `Tables` / `Charts` / `Summary` are kept as
// `map[string]any` rather than typed structs because the Python
// implementation projects different shapes per analyzer (access log
// = per-minute timeline; profiler = flame tree; thread dump = state
// distribution). The renderer uses the `Type` field to decide which
// keys to look for; the parity job just diffs the JSON.
//
// [한글] 필드별 의미
//   Type        : 분석기 식별자(예: "access_log", "thread_dump_multi").
//                 UI 가 어떤 페이지·차트로 렌더할지 결정.
//   SourceFiles : 분석기가 받은 입력 파일 경로들. 보고서에 출처를
//                 표시할 때 사용.
//   CreatedAt   : RFC3339Nano. parity gate 비교에서 노이즈가 되지
//                 않도록 분석기 호출 직전이 아닌 결과 생성 시점.
//   Summary     : 메트릭 카드(상단 KPI)에 표시할 스칼라 모음.
//   Series      : 차트 시계열/배열 데이터.
//   Tables      : shadcn/D3 표용 행 모음.
//   Charts      : flame tree 같은 raw 차트 데이터.
//   Metadata    : 파서·진단·findings 등 부수 정보(아래 참고).
type AnalysisResult struct {
	Type        string         `json:"type"`
	SourceFiles []string       `json:"source_files"`
	CreatedAt   string         `json:"created_at"`
	Summary     map[string]any `json:"summary"`
	Series      map[string]any `json:"series"`
	Tables      map[string]any `json:"tables"`
	Charts      map[string]any `json:"charts"`
	Metadata    Metadata       `json:"metadata"`
}

// Metadata carries per-analyzer plumbing (parser name, schema
// version, parser diagnostics, findings list, AI interpretation
// payload, etc.). `Findings` and `Extra` stay open-ended for the
// same reason the section dicts above do.
type Metadata struct {
	Parser        string                         `json:"parser"`
	SchemaVersion string                         `json:"schema_version"`
	Diagnostics   *diagnostics.ParserDiagnostics `json:"diagnostics,omitempty"`
	// Findings is always emitted (even when empty) so the renderer
	// can rely on `metadata.findings.length` without null-checking.
	Findings []map[string]any `json:"findings"`
	// Extra carries analyzer-specific metadata that doesn't fit any
	// of the above (e.g. profiler timeline_scope, JFR command info,
	// OTel cross-service paths). Marshalled inline.
	Extra map[string]any `json:"-"`
}

// New constructs an envelope with non-nil section maps + a fresh
// RFC3339 timestamp so `json.Marshal` always emits `{}` rather than
// `null`. Mirrors Python's `AnalysisResult.__init__` defaults.
//
// [한글] 빈 컨테이너 사전 채우기가 핵심.
//   nil map/slice 를 그대로 두면 json.Marshal 이 "summary": null 처럼
//   직렬화하고, UI 의 result.summary.foo 가 즉시 TypeError. 그래서
//   여기서 모든 섹션을 빈 map/slice 로 미리 초기화합니다.
//   schema_version "0.1.0" 은 contract 변경 시 수동 bump.
func New(resultType, parser string) AnalysisResult {
	return AnalysisResult{
		Type:        resultType,
		SourceFiles: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Summary:     map[string]any{},
		Series:      map[string]any{},
		Tables:      map[string]any{},
		Charts:      map[string]any{},
		Metadata: Metadata{
			Parser:        parser,
			SchemaVersion: "0.1.0",
			Findings:      []map[string]any{},
			Extra:         map[string]any{},
		},
	}
}

// AttachSource appends a path to `SourceFiles`.
func (r *AnalysisResult) AttachSource(path string) {
	r.SourceFiles = append(r.SourceFiles, path)
}

// AddFinding appends a finding row in the canonical
// {severity, code, message, evidence?} shape.
func (r *AnalysisResult) AddFinding(severity, code, message string, evidence map[string]any) {
	row := map[string]any{
		"severity": severity,
		"code":     code,
		"message":  message,
	}
	if evidence != nil {
		row["evidence"] = evidence
	}
	r.Metadata.Findings = append(r.Metadata.Findings, row)
}

// MarshalJSON merges the typed fields with `Extra` so analyzer-specific
// keys (access_log: format / analysis_options; profiler:
// timeline_scope; etc.) appear at the same level as `parser` /
// `schema_version`, matching the Python `metadata` dict shape.
//
// Extra never overrides typed fields — if the analyzer accidentally
// writes "parser" into Extra, the typed Parser still wins.
//
// [한글] 알고리즘 (flatten merge)
//   1) 새 map 을 만들고 Extra 의 모든 키를 먼저 복사.
//   2) typed 필드(parser, schema_version, diagnostics, findings) 를
//      덮어 쓰기 — 따라서 Extra 와 키가 충돌해도 typed 가 항상 승리.
//   3) findings 가 nil 이면 빈 슬라이스로 치환해서 항상 [] 로 직렬화
//      (UI 의 .length 체크가 안전해짐).
//   4) Diagnostics 는 nil 일 때만 키 자체를 생략(Python 의
//      `if diagnostics: data["diagnostics"] = ...` 와 동치).
//   결과 JSON 의 키 순서는 Go map 의 순회 순서가 비결정적이지만,
//   parity gate 는 비교 전에 양쪽 결과를 indented JSON 으로 다시 쓰기
//   때문에 키 정렬이 자동으로 같아져 비교에 문제가 없습니다.
func (m Metadata) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(m.Extra)+4)
	for k, v := range m.Extra {
		out[k] = v
	}
	out["parser"] = m.Parser
	out["schema_version"] = m.SchemaVersion
	if m.Diagnostics != nil {
		out["diagnostics"] = m.Diagnostics
	}
	findings := m.Findings
	if findings == nil {
		findings = []map[string]any{}
	}
	out["findings"] = findings
	return json.Marshal(out)
}
