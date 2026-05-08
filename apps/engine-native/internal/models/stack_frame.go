// [한글] models/stack_frame.go — 스택 프레임과 락 핸들의 정규화된 모델.
//
// 이 파일이 포함하는 두 개의 모델은 모든 thread-dump 플러그인이 공통
// 으로 사용합니다. Java jstack, Go goroutine, Python py-spy, Node.js
// diagnostic-report, .NET clrstack, Jennifer profile 모두가 같은 형태
// 로 변환되어 multi-thread 분석기/lock contention 분석기로 흘러갑니다.
//
// 포인터 필드의 의미
//   *string / *int 로 선언된 필드는 "값 없음" 과 "빈 문자열"/0 을
//   구별하기 위함입니다. JSON 으로 직렬화될 때 nil 은 null 로 출력되어
//   Python `Optional[...]` 의 동작과 1:1 매칭됩니다. parity gate 가
//   바이트 단위로 결과를 비교하므로 이 구분이 중요합니다.
//
// LockHandle vs StackFrame 분리 이유
//   대부분의 런타임은 락 정보를 별도의 라인 또는 별도 키 (waiting on
//   <0x...> ...) 로 표현합니다. StackFrame 안에 락 정보를 끼워 넣는
//   대신 LockHandle 로 분리해야 lock contention 분석기가 owner/waiter
//   그래프를 단순한 join 으로 만들 수 있습니다.
package models

import "strconv"

// LockHandle identifies a single monitor / lock object. JVM jstack
// exposes `<0x000000076ab62208>` plus a class hint; other runtimes
// usually leave LockID empty and only carry LockClass. Mirrors the
// Python `LockHandle` dataclass.
//
// LockClass is `*string` so the JSON shape can emit `null` (matching
// Python `str | None`). WaitMode is omitted entirely when empty to
// match Python's `if self.wait_mode: payload["wait_mode"] = ...`.
type LockHandle struct {
	LockID    string  `json:"lock_id"`
	LockClass *string `json:"lock_class"`
	WaitMode  string  `json:"wait_mode,omitempty"`
}

// StackFrame is one normalized stack frame. Function is the only
// required field; everything else is best-effort capture from the
// source format. Mirrors `models/thread_snapshot.py:StackFrame`.
//
// Module / File / Line / Language are all pointer types so unset
// fields emit JSON `null` (Python parity).
type StackFrame struct {
	Function string  `json:"function"`
	Module   *string `json:"module"`
	File     *string `json:"file"`
	Line     *int    `json:"line"`
	Language *string `json:"language"`
}

// Render is the human-readable single-line rendering used for stack
// signatures. Matches `StackFrame.render()` in Python so signatures
// stay byte-identical across the two implementations.
//
// [한글] 알고리즘
//   1) head = Function (예: "doRequest")
//   2) Module 이 있으면 head = Module + "." + Function
//      (예: "com.foo.Bar.doRequest").
//   3) File 정보가 없으면 그대로 head 반환.
//   4) File 만 있으면 "head (file)".
//      File+Line 이면 "head (file:line)".
//
//   왜 이 형식인가? Java/Python/Go stack signature 가 문자열 비교로
//   정확히 같아져야 multi-thread 분석기의 PERSISTENT_BLOCKED_THREAD
//   같은 finding 이 같은 스택을 같은 그룹으로 묶을 수 있습니다.
//   Python 측의 동일 함수와 한 글자도 어긋나면 안 됩니다.
func (f StackFrame) Render() string {
	head := f.Function
	if f.Module != nil && *f.Module != "" {
		head = *f.Module + "." + f.Function
	}
	if f.File == nil || *f.File == "" {
		return head
	}
	location := *f.File
	if f.Line != nil {
		location = *f.File + ":" + strconv.Itoa(*f.Line)
	}
	return head + " (" + location + ")"
}
