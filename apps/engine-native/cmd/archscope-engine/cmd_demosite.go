// `demo-site` group — wires the manifest-driven demo runner
// (internal/demosite) into Cobra. Three leaves:
//
//   demo-site list      --manifests <dir>
//   demo-site run       --manifest <path>  --out <dir>
//   demo-site run-all   --manifests <dir>  --out <root>
//
// All three accept --mapping <path> to override
// analyzer_type_mapping.json discovery (otherwise the runner walks up
// from the manifest looking for it).
//
// JSON-only manifests; YAML support was intentionally dropped from the
// Go port (see internal/demosite/mapping.go for rationale).
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `demo-site` 명령 그룹 — T-380 으로 도입된 매니페스트 기반
// 데모 러너의 CLI 표면입니다.
//
// 데모 러너의 목적
//   examples/demo-site/ 아래의 시나리오 매니페스트(JSON)를 읽고,
//   각 시나리오의 입력 파일들을 알맞은 분석기로 통과시킨 뒤, 결과
//   JSON/HTML/PPTX 와 시나리오 인덱스 페이지를 한 번에 만들어 줍니다.
//   목표:
//     • 신기능 추가 시 "현실적인" 입력으로 회귀 검증.
//     • 발표/보고서용 시각자료를 동일 명령으로 재현 가능.
//     • Python 데모 러너와 출력물을 1:1 비교(parity).
//
// 3개 리프 명령
//   list     : 디렉터리 아래 *‌/*‌/manifest.json 을 글롭으로 모아 출력.
//   run      : 단일 매니페스트를 실행 — 결과 JSON 을 stdout 으로 emit.
//   run-all  : 디렉터리 전체 매니페스트를 순회 실행 후 최상위
//              index.html 을 생성(시나리오 인덱스의 인덱스).
//
// 분석기 매핑
//   매니페스트의 each entry 에는 `analyzer` 필드가 있고, 이 문자열을
//   실제 분석기 함수에 매핑하는 표가 analyzer_type_mapping.json 에
//   있습니다. --mapping 으로 명시하지 않으면 매니페스트 디렉터리에서
//   parent 를 거슬러 올라가며 자동 발견합니다.
//
// JSON 전용
//   Python 러너는 YAML/JSON 둘 다 지원하지만, Go 포트는 stdlib 만
//   유지하기 위해 YAML 을 의도적으로 빼고 JSON 만 받습니다. YAML 은
//   `yq -o=json input.yaml > input.json` 으로 사전 변환해서 사용.
//
// PPTX 옵션
//   --no-pptx 로 PPTX 생성을 건너뛸 수 있음. CI 에서 그래픽 라이브러리
//   미설치 환경의 빌드 시간을 줄일 때 유용합니다.
//
// 구조 메모
//   각 리프 명령을 익명 블록 `{ ... }` 으로 감싸 둔 이유:
//   클로저 캡처 변수(in/out/manifests 등)들이 서로 다른 명령 사이에
//   섞이지 않도록 스코프 격리하기 위함입니다.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/demosite"
)

const demoSiteStubMessage = "demo runner subcommand not implemented; run `archscope-engine demo-site --help` for available leaves"

func init() {
	group := &cobra.Command{
		Use:   "demo-site",
		Short: "Demo-site manifest runner commands.",
		Long: `Run JSON manifest-driven demo scenarios end-to-end. Each manifest
describes one or more access-log / GC / thread-dump / OTel inputs and
the expected analyzer output bundle (JSON + HTML + PPTX). Outputs are
written under <out>/<data_source>/<scenario>/, with an index.html
linking the per-analyzer reports.

JSON manifests only: the Python runner accepts YAML or JSON, but the
Go port is JSON-only to stay stdlib-only. Convert YAML manifests with
` + "`yq -o=json input.yaml > manifest.json`" + ` before invoking.`,
	}

	// ── demo-site list ────────────────────────────────────────────────
	{
		var manifests string
		cmd := &cobra.Command{
			Use:   "list",
			Short: "List demo manifests under --manifests as one path per line.",
			Long: `Discover manifests under --manifests by globbing
` + "`*/*/manifest.json`" + `. If --manifests is itself a manifest file,
echoes that single path. Sorted lexically.`,
			Example: `  archscope-engine demo-site list --manifests examples/demo-site`,
			RunE: func(c *cobra.Command, _ []string) error {
				if manifests == "" {
					return fmt.Errorf("--manifests is required")
				}
				paths, err := demosite.DiscoverDemoManifests(manifests)
				if err != nil {
					return err
				}
				for _, p := range paths {
					fmt.Fprintln(c.OutOrStdout(), p)
				}
				return nil
			},
		}
		cmd.Flags().StringVar(&manifests, "manifests", "", "manifest root (file or directory) (required)")
		group.AddCommand(cmd)
	}

	// ── demo-site run ─────────────────────────────────────────────────
	{
		var (
			manifest string
			out      string
			baseline string
			mapping  string
			noPPTX   bool
		)
		cmd := &cobra.Command{
			Use:   "run",
			Short: "Run a single demo manifest end-to-end.",
			Long: `Read --manifest, run each ` + "`files[]`" + ` entry through the matching
analyzer, and write JSON / HTML / (optionally) PPTX outputs into
<out>/<data_source>/<scenario>/, plus a per-scenario index.html.`,
			Example: `  archscope-engine demo-site run \
    --manifest examples/demo-site/synthetic/access-log/manifest.json \
    --out build/demo-site`,
			RunE: func(c *cobra.Command, _ []string) error {
				if manifest == "" || out == "" {
					return fmt.Errorf("--manifest and --out are required")
				}
				absManifest, err := filepath.Abs(manifest)
				if err != nil {
					return err
				}
				absOut, err := filepath.Abs(out)
				if err != nil {
					return err
				}
				if _, err := os.Stat(absManifest); err != nil {
					return fmt.Errorf("manifest: %w", err)
				}
				result, err := demosite.RunDemoSiteManifest(demosite.RunOptions{
					ManifestPath:         absManifest,
					OutputRoot:           absOut,
					BaselineManifestPath: baseline,
					WritePPTX:            !noPPTX,
					MappingPath:          mapping,
				})
				if err != nil {
					return err
				}
				return writeJSONAny(result, "-")
			},
		}
		cmd.Flags().StringVar(&manifest, "manifest", "", "manifest JSON path (required)")
		cmd.Flags().StringVar(&out, "out", "", "output root directory (required)")
		cmd.Flags().StringVar(&baseline, "baseline", "", "optional baseline manifest path for diff reports")
		cmd.Flags().StringVar(&mapping, "mapping", "", "optional analyzer_type_mapping.json path (default: walk up from manifest)")
		cmd.Flags().BoolVar(&noPPTX, "no-pptx", false, "skip PPTX generation")
		group.AddCommand(cmd)
	}

	// ── demo-site run-all ────────────────────────────────────────────
	{
		var (
			manifests string
			out       string
			mapping   string
			noPPTX    bool
		)
		cmd := &cobra.Command{
			Use:   "run-all",
			Short: "Discover + run every manifest under --manifests, writing a top-level index.html.",
			Long: `Walk --manifests for ` + "`*/*/manifest.json`" + ` files and run each one.
After all scenarios complete, emit a top-level index.html under --out
linking each scenario's per-scenario index.html.`,
			Example: `  archscope-engine demo-site run-all \
    --manifests examples/demo-site --out build/demo-site`,
			RunE: func(c *cobra.Command, _ []string) error {
				if manifests == "" || out == "" {
					return fmt.Errorf("--manifests and --out are required")
				}
				absManifests, err := filepath.Abs(manifests)
				if err != nil {
					return err
				}
				absOut, err := filepath.Abs(out)
				if err != nil {
					return err
				}
				paths, err := demosite.DiscoverDemoManifests(absManifests)
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					return fmt.Errorf("no manifests found under %s", absManifests)
				}
				var runs []demosite.DemoScenarioRun
				for _, p := range paths {
					run, err := demosite.RunDemoSiteManifest(demosite.RunOptions{
						ManifestPath: p,
						OutputRoot:   absOut,
						WritePPTX:    !noPPTX,
						MappingPath:  mapping,
					})
					if err != nil {
						return fmt.Errorf("run %s: %w", p, err)
					}
					runs = append(runs, run)
				}
				topIndex := filepath.Join(absOut, "index.html")
				if err := demosite.WriteTopLevelIndex(runs, topIndex); err != nil {
					return err
				}
				fmt.Fprintf(c.OutOrStdout(), "wrote top-level index: %s\n", topIndex)
				return nil
			},
		}
		cmd.Flags().StringVar(&manifests, "manifests", "", "manifest root directory (required)")
		cmd.Flags().StringVar(&out, "out", "", "output root directory (required)")
		cmd.Flags().StringVar(&mapping, "mapping", "", "optional analyzer_type_mapping.json path")
		cmd.Flags().BoolVar(&noPPTX, "no-pptx", false, "skip PPTX generation")
		group.AddCommand(cmd)
	}

	// Catch-all for anything we haven't wired (kept so old scripted
	// invocations still get a friendly hint instead of "unknown
	// command", per the Python CLI's stub message).
	group.AddCommand(&cobra.Command{
		Use:    "stub",
		Hidden: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return fmt.Errorf("%s", demoSiteStubMessage)
		},
	})

	rootCmd.AddCommand(group)
}
