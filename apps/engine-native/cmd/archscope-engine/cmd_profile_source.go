package main

import (
	"encoding/json"
	"fmt"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"syscall"
)

func profileSourceCommands() []*cobra.Command {
	commands := []*cobra.Command{}
	for _, operation := range []string{"inspect-source", "normalize-source", "import-source"} {
		var input, output, format, producer, version, capture, metric string
		var profiles []string
		cmd := &cobra.Command{Use: operation, Args: cobra.NoArgs, Short: "Exact original-source profile normalization with explicit semantic revision and selection.", RunE: func(cmd *cobra.Command, _ []string) error {
			if input == "" {
				return fmt.Errorf("--in required")
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			set, e := parser.ParseSourceFile(ctx, input, parser.SourceOptions{Format: format, Producer: producer, ProducerVersion: version, CaptureOptions: json.RawMessage(capture)})
			if e != nil {
				return e
			}
			switch operation {
			case "inspect-source":
				return writeJSONAny(parser.Inventory(set), "-")
			case "normalize-source":
				body, e := json.MarshalIndent(set, "", "  ")
				if e != nil {
					return e
				}
				if output == "-" {
					_, e = cmd.OutOrStdout().Write(body)
					return e
				}
				root, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				return localfiles.Publish(ctx, output, append(body, '\n'), root)
			default:
				if len(profiles) == 0 || metric == "" {
					return fmt.Errorf("PROFILE_SELECTION_REQUIRED: inspect-source, then select --profile-id and --metric-id")
				}
				result, e := analyzer.BuildSet(ctx, set, profilemodel.Selection{ProfileIDs: profiles, MetricID: metric}, input)
				if e != nil {
					return e
				}
				return writeJSONResult(result, output)
			}
		}}
		cmd.Flags().StringVar(&input, "in", "", "original selected source")
		cmd.Flags().StringVar(&format, "format", "auto", "speedscope-json, pyspy-raw, pprof")
		cmd.Flags().StringVar(&producer, "producer", "", "explicit declared producer when the format has no exporter")
		cmd.Flags().StringVar(&version, "producer-version", "", "declared source tool version, not a support grant")
		cmd.Flags().StringVar(&capture, "capture-options", "{}", "complete sanitized capture context JSON; imported whitelists have no authority")
		cmd.Flags().StringVar(&output, "out", "-", "output path or stdout; normalize-source refuses existing files")
		cmd.Flags().StringSliceVar(&profiles, "profile-id", nil, "explicit profile IDs from inspect-source")
		cmd.Flags().StringVar(&metric, "metric-id", "", "explicit retained value/unit metric from inspect-source")
		commands = append(commands, cmd)
	}
	return commands
}
