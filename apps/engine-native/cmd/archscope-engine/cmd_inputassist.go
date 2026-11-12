package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/inputassist"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localconvert"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func inputAssistanceCommands() []*cobra.Command {
	var input, sidecar, output string
	discover := &cobra.Command{Use: "discover", Short: "Inspect bounded local input candidates and missing capture context without importing or executing tools.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		c, _, e := catalog.Builtin()
		if e != nil {
			return e
		}
		var mappings []packs.Mapping
		if evidencePackStore != "" {
			s, e := selectedPackSession(ctx, evidencePackStore)
			if e != nil {
				return e
			}
			defer s.Manager.Close()
			snapshot, e := s.Provider(evidencePresetScope)
			if e != nil {
				return e
			}
			c = snapshot.Catalog
			mappings = snapshot.Mappings
		}
		report, e := inputassist.Discover(ctx, c, input, sidecar, mappings...)
		if e != nil {
			return e
		}
		if output == "-" {
			return writeJSONAny(report, "-")
		}
		b, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			return e
		}
		roots, e := cliAssistanceRoots()
		if e != nil {
			return e
		}
		return localfiles.Publish(ctx, output, append(b, '\n'), roots...)
	}}
	discover.Flags().StringVar(&input, "in", "", "selected local source")
	discover.Flags().StringVar(&sidecar, "capture", "", "explicit optional capture sidecar; never auto-read adjacent files")
	discover.Flags().StringVar(&output, "out", "-", "report stdout or a new file")
	var kitOutput string
	kit := &cobra.Command{Use: "collection-kit", Short: "Export inert manual recipes, paired offline manuals and capture schema as a separate ZIP.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if kitOutput == "" {
			return fmt.Errorf("--out new ZIP required")
		}
		body, e := inputassist.Kit()
		if e != nil {
			return e
		}
		roots, e := cliAssistanceRoots()
		if e != nil {
			return e
		}
		return localfiles.Publish(cmd.Context(), kitOutput, body, roots...)
	}}
	kit.Flags().StringVar(&kitOutput, "out", "", "new manual collection ZIP")
	return []*cobra.Command{discover, kit}
}
func jfrConversionCommand() *cobra.Command {
	var input, tool, output string
	cmd := &cobra.Command{Use: "convert", Short: "Explicit installed JDK conversion with fixed argv, bounded output/time, cancellation and provenance.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if input == "" || tool == "" || output == "" {
			return fmt.Errorf("--in, --cli absolute installed tool and --out new JSON required")
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return convertCLI(ctx, input, tool, output)
	}}
	cmd.Flags().StringVar(&input, "in", "", "binary JFR source")
	cmd.Flags().StringVar(&tool, "cli", "", "explicit absolute installed JDK jfr executable")
	cmd.Flags().StringVar(&output, "out", "", "new JSON; provenance also saved as .conversion.json")
	return cmd
}
func convertCLI(ctx context.Context, input, tool, output string) error {
	roots, e := cliAssistanceRoots()
	if e != nil {
		return e
	}
	result, e := localconvert.ConvertNew(ctx, input, tool, output, roots...)
	if e != nil {
		return e
	}
	return writeJSONAny(result, "-")
}

func cliAssistanceRoots() ([]string, error) {
	root, e := cases.DefaultRoot()
	if e != nil {
		return nil, e
	}
	cache, e := os.UserCacheDir()
	if e != nil {
		return nil, e
	}
	roots := []string{root, filepath.Join(cache, "ArchScope", "evidence-imports")}
	if evidencePackStore != "" {
		roots = append(roots, evidencePackStore)
	}
	return roots, nil
}
