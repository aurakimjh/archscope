// Package json ports archscope_engine.exporters.json_exporter — a
// tiny round-trip-stable AnalysisResult writer. The exported payload
// is `json.MarshalIndent` output with a trailing newline so diffs and
// editors play nicely with it; UTF-8 native (no escaping non-ASCII).
//
// Foundation for the other Tier-4 exporters (HTML / PPTX / CSV /
// report-diff) that all need to load & re-emit AnalysisResult JSON.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] json exporter — AnalysisResult 의 라운드트립 안정 직렬화.
//
// 출력 형식
//   • 2-space indent.
//   • 끝에 한 줄 띄움 (\n).
//   • 비-ASCII 문자(한글 등) 은 escape 하지 않고 raw UTF-8.
//
// 왜 이 형식인가?
//   • diff/git/editor 친화적 — indent 와 trailing newline 이 깔끔한
//     diff 를 만들어 줌.
//   • Python `ensure_ascii=False` 와 자동 동치 — Go `encoding/json` 의
//     기본 동작이 raw UTF-8.
//   • parity gate 가 두 엔진 출력을 비교할 때 노이즈 0.
//
// 다른 exporter 의 토대
//   T-340 의 exporter 표면 (Write / Marshal) 을 처음 정의한 곳.
//   HTML / PPTX / CSV / report-diff 가 모두 이 스타일을 그대로 모방
//   (alias swap 가능).
//
// 디렉토리 자동 생성
//   Write 가 부모 디렉토리를 mkdir -p 처리 — 보고서를 처음 생성하는
//   경로에서 ENOENT 로 실패하지 않도록.
package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Write marshals `result` (typically `models.AnalysisResult` or any
// JSON-serializable payload), creating any missing parent directories,
// and writes it with 2-space indent + trailing newline. Mirrors Python
// `write_json_result(payload, path)` byte-for-byte.
//
// `ensure_ascii=False` parity is automatic: Go's `encoding/json`
// emits non-ASCII as raw UTF-8 by default, matching Python's
// `ensure_ascii=False`.
func Write(path string, result any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// Marshal returns the byte slice that `Write` would persist — useful
// for callers that want to ship JSON over stdout / HTTP / a buffer
// without touching the filesystem.
//
// Uses an explicit Encoder with SetIndent + SetEscapeHTML(false) so
// `<`, `>`, `&` round-trip verbatim (Python's default).
func Marshal(result any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	// json.Encoder.Encode appends '\n' automatically — matches
	// Python's `json.dumps(...) + "\n"`.
	return buf.Bytes(), nil
}
