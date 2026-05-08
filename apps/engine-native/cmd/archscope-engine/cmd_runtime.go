// Runtime stack/log analyzers — four top-level groups (`nodejs`,
// `python-traceback`, `go-panic`, `dotnet`), all wrapping
// internal/analyzers/runtime with a different variant. Mirrors typer's
// nodejs_app / python_traceback_app / go_panic_app / dotnet_app.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] runtime 계열 4개 명령 그룹 (nodejs / python-traceback /
// go-panic / dotnet) 의 공유 정의 파일.
//
// 왜 한 파일에 모았는가?
//   네 분석기는 입력 형식만 다를 뿐 플래그 세트와 핸들러 모양이 거의
//   동일합니다(--in / --top-n / --max-lines / --strict / --out).
//   이런 "동일 모양·다른 분석기" 를 4번 복사하면 회귀 시 4곳을 모두
//   고쳐야 합니다. 이 파일은 다음 두 도구로 그 중복을 제거합니다.
//
//     1) runtimeAnalyzeFunc 타입 alias
//        path 와 runtime.Options 만 받아 AnalysisResult 를 반환하는
//        모든 분석기 진입점이 만족하는 함수 시그니처.
//
//     2) addRuntimeGroup helper
//        그룹 이름·도움말·예제·플래그 도움말과, 위 시그니처를 만족하는
//        분석기 함수 1개를 받아서 cobra group + analyze 리프 명령을
//        한 번에 만들어 rootCmd 에 부착합니다.
//
// 분석기 매핑
//   nodejs           → runtime.AnalyzeNodejsStack
//   python-traceback → runtime.AnalyzePythonTraceback
//   go-panic         → runtime.AnalyzeGoPanic
//   dotnet           → runtime.AnalyzeDotnetExceptionIIS
//                      (DOTNET 그룹은 IIS W3C access log 도 같이 처리)
//
// 새 런타임 추가 시
//   internal/analyzers/runtime 에 분석기 함수 추가 후, init() 에서
//   addRuntimeGroup 한 줄만 더하면 끝납니다.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtime"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// runtimeAnalyzeFunc abstracts the analyzer entrypoint so we can wire
// the same flag-binding helper to each variant. Every runtime analyzer
// has the same signature (path, Options) → (Result, error).
type runtimeAnalyzeFunc func(path string, opts runtime.Options) (models.AnalysisResult, error)

// addRuntimeGroup creates a `<use> analyze` group backed by `analyze`.
// The four runtime analyzers share the exact same flag set, so this
// helper avoids four near-identical bodies.
func addRuntimeGroup(use, short, long, example, inHelp string, analyze runtimeAnalyzeFunc) {
	group := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
	}

	var (
		in       string
		topN     int
		maxLines int
		strict   bool
		out      string
	)

	leaf := &cobra.Command{
		Use:     "analyze",
		Short:   short,
		Long:    long,
		Example: example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := analyze(in, runtime.Options{
				TopN:     topN,
				MaxLines: maxLines,
				Strict:   strict,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	leaf.Flags().StringVar(&in, "in", "", inHelp)
	leaf.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	leaf.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	leaf.Flags().BoolVar(&strict, "strict", false, "surface parser skips as fatal errors")
	leaf.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(leaf)
	rootCmd.AddCommand(group)
}

func init() {
	addRuntimeGroup(
		"nodejs",
		"Node.js log and stack analysis commands.",
		"Analyze Node.js error stack traces. Mirrors the typer nodejs group.",
		`  archscope-engine nodejs analyze \
    --in examples/runtime/sample-nodejs-stack.txt \
    --out result.json`,
		"path to Node.js stack/log file (required)",
		runtime.AnalyzeNodejsStack,
	)

	addRuntimeGroup(
		"python-traceback",
		"Python traceback analysis commands.",
		"Analyze Python traceback blocks. Mirrors the typer python-traceback group.",
		`  archscope-engine python-traceback analyze \
    --in examples/runtime/sample-python-traceback.txt \
    --out result.json`,
		"path to Python traceback file (required)",
		runtime.AnalyzePythonTraceback,
	)

	addRuntimeGroup(
		"go-panic",
		"Go panic and goroutine analysis commands.",
		"Analyze Go panic and goroutine dumps. Mirrors the typer go-panic group.",
		`  archscope-engine go-panic analyze \
    --in examples/runtime/sample-go-panic.txt \
    --out result.json`,
		"path to Go panic/goroutine dump (required)",
		runtime.AnalyzeGoPanic,
	)

	addRuntimeGroup(
		"dotnet",
		".NET exception and IIS log analysis commands.",
		"Analyze .NET exception stack traces and IIS W3C access logs.",
		`  archscope-engine dotnet analyze \
    --in examples/runtime/sample-dotnet-iis.txt \
    --out result.json`,
		"path to .NET exception or IIS W3C log (required)",
		runtime.AnalyzeDotnetExceptionIIS,
	)
}
