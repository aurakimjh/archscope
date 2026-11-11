package main

import (
	"fmt"
	"os"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/spf13/cobra"
)

func resultProviderCommands() []*cobra.Command {
	commands := []*cobra.Command{}
	for _, operation := range []string{"describe", "query", "compare"} {
		var input, target, provider string
		var offset, limit int
		cmd := &cobra.Command{Use: operation, Short: "Read registered result capabilities without type-specific dispatch."}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return fmt.Errorf("--in is required")
			}
			c, r, e := catalog.Builtin()
			if e != nil {
				return e
			}
			root, e := os.MkdirTemp("", "archscope-provider-")
			if e != nil {
				return e
			}
			defer os.RemoveAll(root)
			service, e := analysis.New(root, c, r)
			if e != nil {
				return e
			}
			defer service.Close()
			view, e := service.Load(cmd.Context(), input)
			if e != nil {
				return e
			}
			var out any
			switch operation {
			case "describe":
				out, e = service.DescribeResult(cmd.Context(), view.Handle)
			case "query":
				out, e = service.QueryResult(cmd.Context(), view.Handle, provider, offset, limit)
			case "compare":
				if target == "" {
					return fmt.Errorf("--target is required")
				}
				other, loadErr := service.Load(cmd.Context(), target)
				if loadErr != nil {
					return loadErr
				}
				out, e = service.CompareResult(cmd.Context(), view.Handle, other.Handle, provider)
			}
			if e != nil {
				return e
			}
			return writeJSONAny(out, "-")
		}
		cmd.Flags().StringVar(&input, "in", "", "saved AnalysisResult JSON; baseline for compare")
		if operation != "describe" {
			cmd.Flags().StringVar(&provider, "provider", "generic:summary", "registered provider ID")
		}
		if operation == "query" {
			cmd.Flags().IntVar(&offset, "offset", 0, "immutable row offset")
			cmd.Flags().IntVar(&limit, "limit", 25, "bounded page size (1..100)")
		}
		if operation == "compare" {
			cmd.Flags().StringVar(&target, "target", "", "ordered target result JSON")
		}
		commands = append(commands, cmd)
	}
	return commands
}
