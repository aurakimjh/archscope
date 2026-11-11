package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	profileparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "profile-set", Short: "Exact operations on saved ProfileSet evidence (import with evidence import)."}
	for _, operation := range []string{"tree", "diff", "drilldown", "pprof"} {
		operation := operation
		var input, after, output, tick, filters string
		command := &cobra.Command{Use: operation, RunE: func(cmd *cobra.Command, _ []string) error {
			if input == "" {
				return fmt.Errorf("--in saved profile_evidence is required")
			}
			directory, e := os.MkdirTemp("", "profile-core-")
			if e != nil {
				return e
			}
			defer os.RemoveAll(directory)
			c, r, e := catalog.Builtin()
			if e != nil {
				return e
			}
			service, e := analysis.New(directory, c, r)
			if e != nil {
				return e
			}
			defer service.Close()
			view, e := service.Load(cmd.Context(), input)
			if e != nil {
				return e
			}
			var body []byte
			switch operation {
			case "tree":
				value, err := service.ProfileTree(cmd.Context(), view.Handle)
				if err != nil {
					return err
				}
				body, e = json.Marshal(value)
			case "diff":
				if after == "" {
					return fmt.Errorf("--after is required")
				}
				target, err := service.Load(cmd.Context(), after)
				if err != nil {
					return err
				}
				value, err := service.ProfileCompare(cmd.Context(), view.Handle, target.Handle)
				if err != nil {
					return err
				}
				body, e = json.Marshal(value)
			case "drilldown":
				selection, decodeErr := profilemodel.DecodeFilters([]byte(filters))
				if decodeErr != nil {
					return decodeErr
				}

				value, err := service.ProfileDrilldown(cmd.Context(), view.Handle, selection)
				if err != nil {
					return err
				}
				body, e = json.Marshal(value)
			case "pprof":
				body, e = service.ProfilePprof(cmd.Context(), view.Handle, tick)
			}
			if e != nil {
				return e
			}
			if output == "-" {
				_, e = cmd.OutOrStdout().Write(body)
				return e
			}
			return analysis.AtomicExport(output, body)
		}}
		command.Flags().StringVar(&input, "in", "", "saved ProfileSet result")
		command.Flags().StringVar(&after, "after", "", "saved comparison result")
		command.Flags().StringVar(&output, "out", "-", "output or stdout")
		command.Flags().StringVar(&tick, "tick-scale", "", "exact original-unit amount per integer pprof tick")
		command.Flags().StringVar(&filters, "filters", "[]", "ordered frame-ID path filters as JSON")
		group.AddCommand(command)
	}
	var legacyInput, legacyFormat, legacyOutput string
	normalize := &cobra.Command{Use: "normalize-legacy", Short: "Wrap existing normalized records; producer corrections are not applied.", RunE: func(cmd *cobra.Command, _ []string) error {
		if legacyInput == "" || legacyFormat == "" {
			return fmt.Errorf("--in and explicit --format are required")
		}
		set, _, e := profileparser.ParseLegacySet(cmd.Context(), legacyInput, legacyFormat)
		if e != nil {
			return e
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "Legacy-normalized semantics retained; corrected producer adapters remain separate.")
		body, e := json.Marshal(set)
		if e != nil {
			return e
		}
		if legacyOutput == "-" {
			_, e = cmd.OutOrStdout().Write(body)
			return e
		}
		return analysis.AtomicExport(legacyOutput, body)
	}}
	normalize.Flags().StringVar(&legacyInput, "in", "", "selected legacy source")
	normalize.Flags().StringVar(&legacyFormat, "format", "", "explicit existing normalizer format")
	normalize.Flags().StringVar(&legacyOutput, "out", "-", "normalized model JSON or stdout")
	group.AddCommand(normalize)

	rootCmd.AddCommand(group)
}
