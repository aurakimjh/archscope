// Package runtimestack ports the four Python runtime stack-trace
// parsers (`dotnet_parser`, `go_panic_parser`, `nodejs_stack_parser`,
// `python_traceback_parser`) to Go. They all emit the same
// RuntimeStackRecord shape so they live in one package with one file
// per language and a shared types.go for the records and Options.
//
// Each language exposes a `Parse<Lang>File` entrypoint mirroring
// Python's `parse_<lang>(...)`. The entrypoints share the Strict /
// MaxLines knobs the access-log reference parser (T-310) uses so
// analyzers can dispatch through a uniform Options struct.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] runtimestack — 4개 런타임 stack trace 파서를 한 패키지로 통합.
//
// 파일 구성
//   types.go             : 공통 RuntimeStackRecord / IisAccessRecord /
//                          Options (이 파일).
//   dotnet.go            : .NET 예외 + IIS W3C access 혼합 스트림.
//   gopanic.go           : Go panic + goroutine 덤프.
//   nodejs.go            : Node.js Error / Caused by 체인.
//   pythontraceback.go   : Python Traceback 블록.
//
// 통합 자료구조 (RuntimeStackRecord)
//   • Runtime    : "dotnet"/"go"/"nodejs"/"python" 라벨.
//   • RecordType : 한 런타임 안에서의 변종 식별자
//                  (예: dotnet 의 "exception" vs "iis_access").
//   • Headline   : "TypeName: message" 형태의 한 줄 요약.
//   • Message    : 포인터 — Python 의 None 과 빈 문자열을 구별
//                  (python_traceback 만 두 케이스 발생).
//   • Stack      : 프레임 string slice.
//   • Signature  : 분석기 dedup 키 (type + 첫 frame).
//   • RawBlock   : 원본 텍스트 보존 (debug / 보고서용).
//
// IisAccessRecord
//   .NET 입력 파일에 IIS W3C access log 가 같이 섞여 있는 케이스 —
//   `#Fields: ...` 지시어 뒤의 라인들을 access record 로 별도 파싱.
//   분석기는 두 종류 record 를 모두 보고 IIS_5XX_PRESENT,
//   DOTNET_EXCEPTIONS_PRESENT 같은 finding 을 emit.
//
// 공통 옵션 (Options)
//   Strict / MaxLines — 다른 파서와 동일 의미. 4개 entrypoint 가
//   같은 Options 구조체를 받음 → uniform dispatch 가능.
package runtimestack

import "time"

// RuntimeStackRecord mirrors `models.runtime_stack.RuntimeStackRecord`
// — the cross-runtime aggregation key the exception/runtime analyzers
// (T-333) consume. `Message` is a pointer because Python distinguishes
// "no header message" (None) from "header message was the empty string"
// for python_traceback; the other runtimes only ever produce non-empty
// messages but use the same field for parity.
type RuntimeStackRecord struct {
	Runtime    string
	RecordType string
	Headline   string
	Message    *string
	Stack      []string
	Signature  string
	RawBlock   string
}

// IisAccessRecord mirrors `models.runtime_stack.IisAccessRecord`. Only
// the .NET parser produces these (the IIS W3C log block is interleaved
// with .NET exception text in the same input file).
type IisAccessRecord struct {
	Method      string
	URI         string
	Status      int
	TimeTakenMS *int
	RawLine     string
}

// Options mirrors the per-parser knobs the access-log reference
// surfaces. `MaxLines` is honoured before line-level dispatch so
// strict-mode rejections from beyond the cap don't fire. `Strict`
// turns the first per-line skip into a fatal error.
type Options struct {
	MaxLines int
	Strict   bool

	// StartTime / EndTime are accepted for API symmetry with
	// access-log; the runtime stack inputs aren't time-stamped, so
	// the fields are ignored.
	StartTime *time.Time
	EndTime   *time.Time
}

// stringPtr / intPtr build the optional pointer fields on
// RuntimeStackRecord and IisAccessRecord. Python uses None for
// "absent"; we map that to nil and reserve an actual pointer for
// "present, possibly empty".
func stringPtr(s string) *string { return &s }

func intPtr(n int) *int { return &n }
