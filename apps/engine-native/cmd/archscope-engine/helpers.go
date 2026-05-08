// Shared helpers for the archscope-engine Cobra CLI. Each subcommand
// file (cmd_accesslog.go, cmd_gclog.go, …) keeps its own flag-binding
// and Run* function, but the JSON write path, time-flag parsing, and
// repeatable input-flag plumbing are common across them and live here.
//
// [한글] archscope-engine Cobra CLI 의 공용 도우미.
//
// 책임 분리:
//   • cmd_*.go : 분석기 단위로 플래그 바인딩과 RunE 핸들러 보유.
//                "이 명령은 어떤 옵션을 받고 어떤 분석기를 호출하는가" 를
//                담당합니다.
//   • helpers.go (이 파일) : 모든 cmd_*.go 가 공유하는 작은 유틸.
//        - writeJSONResult / writeJSONAny : AnalysisResult 또는 임의
//          JSON 페이로드를 indent JSON 으로 stdout 또는 파일에 기록.
//        - parseTimeFlag                 : --start-time / --end-time 등
//          시간 플래그를 RFC3339 로 파싱하고 빈 문자열이면 nil 반환.
//        - readJSONFile                   : `report` 그룹이 입력 JSON 을
//          generic map 으로 다시 읽기 위한 round-trip 헬퍼.
//        - splitCommaSeparated            : `--in a.txt --in b.txt,c.txt`
//          처럼 반복+콤마 혼합된 입력을 정규화.
//
// 헬퍼 자체는 분석기 로직과 무관합니다. 즉 여기에 새 함수를 추가할 때는
// "여러 cmd_*.go 가 정말로 동일한 것을 필요로 하는지" 를 먼저 따져봐야
// 합니다 (단발성이면 cmd_*.go 안에 로컬 헬퍼로 두는 편이 추적이 쉬움).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// writeJSONResult marshals the AnalysisResult envelope to indented
// JSON and writes it to `out` (`-` for stdout). Mirrors the original
// flag-based emitResult.
//
// [한글] AnalysisResult 전용 wrapper. 사실상 writeJSONAny 로 바로
// 위임합니다. 이름이 따로 있는 이유는 cmd_*.go 의 호출 지점에서
// "분석 결과를 쓰는 것" 임을 즉각 식별 가능하게 하기 위함입니다.
func writeJSONResult(result models.AnalysisResult, out string) error {
	return writeJSONAny(result, out)
}

// writeJSONAny writes any JSON-marshalable payload (the `to-collapsed`
// command emits `map[string]int`; the `report json` command emits a
// generic `map[string]any`).
//
// [한글] 알고리즘 흐름:
//   1) json.MarshalIndent 로 두 칸 들여쓰기 JSON 직렬화.
//   2) 끝에 '\n' 한 바이트를 덧붙임 — POSIX 라인 규약 준수와
//      쉘 prompt 에서의 가독성 둘 다를 위해.
//   3) out 이 "" 또는 "-" 이면 stdout 으로 직접 기록(파이프라인 친화).
//      그 외에는 0644 권한으로 파일에 기록.
//   4) 부분 파일 생성을 막기 위해 일괄 WriteFile 을 사용 — Marshal 이
//      먼저 메모리에서 끝났으므로 디스크에는 성공/실패가 atomic 에
//      가깝게 기록됩니다.
func writeJSONAny(payload any, out string) error {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if out == "-" || out == "" {
		_, err := os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(out, body, 0o644)
}

// parseTimeFlag converts a user-supplied --start-time / --end-time
// value to *time.Time. Returns (nil, nil) when `value` is empty so
// callers can wire the flag straight into Options without checking.
//
// [한글] 빈 입력 처리가 핵심입니다. nil 을 반환하면 호출 측은
// "이 시간 필터는 비활성" 으로 간주하므로, 모든 cmd_*.go 가
// `--start-time`/`--end-time` 을 옵션 구조체에 그대로 박아 넣고
// 별도 분기를 두지 않아도 됩니다. RFC3339(예: 2026-05-08T09:00:00Z)
// 로만 파싱하고 다른 포맷은 거부 — Python 측 typer 도 동일한
// 정책이므로 parity 가 유지됩니다.
func parseTimeFlag(name, value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("--%s: %w", name, err)
	}
	return &t, nil
}

// readJSONFile loads `path` and decodes it as a generic map. The
// `report` exporters' toMap helpers accept arbitrary JSON-marshalable
// input, so feeding them this map gives the user the round-trip they
// expect (load → render).
//
// [한글] `report` 그룹의 흐름:
//   분석기 → JSON 파일 → readJSONFile → exporter → 보고서.
// generic map 으로 디코드하기 때문에 AnalysisResult 의 임의 확장
// 필드도 잃지 않고 그대로 전달되는 round-trip 이 보장됩니다.
func readJSONFile(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return payload, nil
}

// splitCommaSeparated normalises a slice of CLI values that may be a
// mix of repeated --in invocations and comma-separated strings. It
// preserves order and drops empty fragments.
//
// [한글] 사용자는 멀티-입력을 두 가지 스타일로 표현합니다.
//   • 반복 플래그        :  --in a.txt --in b.txt
//   • 콤마 구분          :  --in a.txt,b.txt
//   • 혼합               :  --in a.txt --in b.txt,c.txt
// 셸 스크립트에서는 반복 플래그가 다루기 까다롭고, IDE 의 task config
// 같은 곳에서는 콤마가 편하므로 양쪽을 모두 허용합니다. 정규화 규칙:
//   1) 입력 슬라이스를 순회.
//   2) 각 원소를 콤마로 다시 split.
//   3) TrimSpace 후 빈 문자열은 버림(연속 콤마/꼬리 콤마 무해화).
//   4) 입력 순서를 그대로 보존(분석 결과의 source_files 순서가
//      사용자 관점과 일치해야 보고서에서 혼동이 없음).
func splitCommaSeparated(values []string) []string {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		for _, item := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}
