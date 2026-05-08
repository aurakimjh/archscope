// archscope-engine is the Go counterpart of `python -m
// archscope_engine.cli`. Each command group below mirrors the typer
// surface byte-for-byte (`gc-log` not `gclog`; `thread-dump
// analyze-multi` not `multithread`) so users, the parity gate, and
// the future demo runner (T-380) can swap engines without rewriting
// invocations.
//
// Build layout:
//   - main.go            (this file): root *cobra.Command + Execute.
//   - cmd_*.go:           one file per top-level group. Each file owns
//                         its leaves, flag bindings, and RunE handlers.
//   - helpers.go:         writeJSONResult, parseTimeFlag, readJSONFile.
//
// T-360 only refactors the CLI surface; analyzers/exporters under
// internal/ are untouched.
//
// ─────────────────────────────────────────────────────────────────────
// [한글 설명]
// archscope-engine 은 Python 의 `python -m archscope_engine.cli` 를
// Go 로 옮긴 헤드리스 CLI 바이너리입니다. T-360 작업으로 도입된
// Cobra 기반 명령 트리이며, 핵심 설계 원칙은 다음과 같습니다.
//
//   1) Python typer 표면을 글자 단위로 동일하게 미러링합니다.
//      예) typer 가 `gc-log analyze` 라면 Cobra 도 `gc-log analyze`
//          (절대 `gclog`/`gcLogAnalyze` 같은 변형 금지).
//      이유: parity gate(.github/workflows/profiler-native.yml) 가
//      두 엔진을 같은 인자로 호출해 결과(JSON)를 비교하기 때문에
//      명령/플래그 표면이 1:1 이어야 회귀를 잡을 수 있습니다.
//
//   2) 파일 분할 규칙
//      • main.go      : 루트 cobra.Command 정의 + Execute 진입점만 담당.
//      • cmd_*.go     : typer 의 top-level group 1개당 파일 1개.
//        각 파일이 자신의 리프 명령, 플래그 바인딩, RunE 핸들러를
//        모두 소유합니다(분석기/exporter 코드는 직접 두지 않음).
//      • helpers.go   : 모든 cmd_*.go 가 공통으로 쓰는 작은 유틸
//        (writeJSONResult / parseTimeFlag / readJSONFile).
//
//   3) 부수효과(import _) 등록
//      thread-dump 플러그인은 자기 자신을 `enginethreaddump.DefaultRegistry`
//      에 등록하는 init() 를 가지고 있습니다. 아래 익명 import 가 없으면
//      플러그인의 init() 이 실행되지 않아 ParseMany / ParseOne 이
//      해당 포맷을 인식하지 못합니다(레지스트리에 미등록 상태).
//
//   4) T-360 의 책임 한계
//      이 작업은 CLI surface 만 재정비합니다. internal/ 하위의 분석기,
//      파서, exporter 코드는 일절 손대지 않습니다 — 그래야 parity gate 가
//      "CLI 변경" 과 "엔진 변경" 을 분리해 회귀를 추적할 수 있습니다.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// Side-effect imports register thread-dump plugins on the default
	// registry. javajstack + gogoroutine self-register; the rest expose
	// New() factories we wire into the registry below.
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/dotnetclrstack"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/gogoroutine"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajcmdjson"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/nodejsreport"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/pythondump"
)

// rootCmd is the parent of every command group. Each cmd_*.go file
// attaches its top-level group to it via a `func init()` so adding a
// new analyzer is a single-file change.
//
// [한글] rootCmd 는 모든 분석기 명령 그룹의 부모 *cobra.Command 입니다.
// 각 cmd_*.go 파일이 init() 안에서 자기 group 을 rootCmd 에 부착하므로
// "분석기 추가 = 파일 1개 추가" 의 1-파일 변경 규칙이 유지됩니다.
// SilenceUsage/SilenceErrors 를 켜둔 이유: RunE 가 에러를 반환할 때
// cobra 가 자동으로 usage 를 또 출력하면 stderr 에 Python CLI 와
// 형태가 다른 노이즈가 섞여 parity gate 의 stderr 비교가 깨집니다.
// 따라서 에러 출력은 main() 의 단 한 줄(`archscope-engine: %v`) 로
// 통일합니다.
var rootCmd = &cobra.Command{
	Use:   "archscope-engine",
	Short: "ArchScope analysis engine CLI (Go)",
	Long: `ArchScope analysis engine CLI — the Go counterpart of
` + "`python -m archscope_engine.cli`" + `.

Every analyzer emits the same models.AnalysisResult JSON envelope as
the Python CLI; the parity gate at .github/workflows/profiler-native.yml
runs them side-by-side on every PR. Subcommand names mirror the typer
surface verbatim — see ` + "`archscope-engine <group> --help`" + ` for the
flags each leaf accepts.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// main 은 cobra 의 Execute() 결과만 단일 exit 코드로 변환합니다.
// 흐름:
//   1) rootCmd.Execute() 가 argv 를 파싱해 알맞은 RunE 를 호출.
//   2) RunE 가 nil 을 반환하면 종료 코드 0.
//   3) error 를 반환하면 stderr 에 한 줄로 출력 후 종료 코드 1.
// 이 한 줄 형식("archscope-engine: <msg>") 은 Python CLI 의
// `typer` 기본 에러 형식과 동일하게 맞춰져 있어, parity gate 가
// "엔진별 에러 메시지가 같은 라인 형태인지" 까지 verify 할 수 있습니다.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "archscope-engine: %v\n", err)
		os.Exit(1)
	}
}
