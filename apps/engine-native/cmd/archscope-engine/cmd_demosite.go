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
