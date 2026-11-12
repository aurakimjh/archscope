package main

import (
	"encoding/json"
	profile "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/systemtrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/systemtrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func init() {
	group := &cobra.Command{Use: "system", Short: "File-only perf/BPF evidence; explicit source semantics, no live attach or tool execution."}
	for _, operation := range []string{"inspect-source", "normalize-source", "import-source", "project-profile"} {
		var input, output, format, capture, captureFile, metric string
		var entities []string
		cmd := &cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			publish := func(value any) error {
				body, e := json.Marshal(value)
				if e != nil {
					return e
				}
				body = append(body, '\n')
				if output == "-" {
					_, e = cmd.OutOrStdout().Write(body)
					return e
				}
				root, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				return localfiles.Publish(ctx, output, body, root)
			}
			if operation == "project-profile" {
				body, e := readMetricDocument(input, 64<<20)
				if e != nil {
					return e
				}
				result, e := resultcontract.Decode(body)
				if e != nil {
					return e
				}
				if e = a.Validate(result); e != nil {
					return e
				}
				set, e := a.ReadCore(result)
				if e != nil {
					return e
				}
				model, e := a.Project(ctx, set, metric, entities)
				if e != nil {
					return e
				}
				selection := profilemodel.Selection{MetricID: metric, ProfileIDs: []string{}}
				for _, p := range model.Profiles {
					selection.ProfileIDs = append(selection.ProfileIDs, p.ID)
				}
				projected, e := profile.BuildSet(ctx, model, selection, "system projection "+filepath.Base(input))
				if e != nil {
					return e
				}
				projected.Metadata.Extra["system_projection"] = map[string]any{"result_sha256": m.Digest(body), "source_sha256": set.SourceSHA256, "metric": metric, "entities": entities, "identity_basis": "declared"}
				return publish(projected)
			}
			raw := []byte(capture)
			if captureFile != "" {
				var e error
				raw, e = readMetricDocument(captureFile, 128<<10)
				if e != nil {
					return e
				}
			}
			var c m.Capture
			if e := m.Strict(raw, &c, 128<<10); e != nil {
				return e
			}
			set, e := p.ParseFile(ctx, input, p.Options{Format: format, Capture: c})
			if e != nil {
				return e
			}
			switch operation {
			case "inspect-source":
				return publish(a.Inventory(set))
			case "normalize-source":
				return publish(set)
			default:
				result, e := a.Build(ctx, set, filepath.Base(input))
				if e != nil {
					return e
				}
				return publish(result)
			}
		}}
		cmd.Flags().StringVar(&input, "in", "", "source or saved system result")
		cmd.Flags().StringVar(&output, "out", "-", "new output file or stdout")
		cmd.Flags().StringVar(&format, "format", "", "perf-script, events-jsonl or bpftrace-jsonl (explicit)")
		cmd.Flags().StringVar(&capture, "capture", "{}", "explicit sanitized capture contract JSON")
		cmd.Flags().StringVar(&captureFile, "capture-file", "", "capture contract JSON file, at most 128 KiB")
		cmd.Flags().StringVar(&metric, "metric-id", "", "explicit sample/period/qualified-duration projection from inventory")
		cmd.Flags().StringSliceVar(&entities, "entity-id", nil, "declared entity lifetime IDs; omitted selects all")
		group.AddCommand(cmd)
	}
	rootCmd.AddCommand(group)
}
