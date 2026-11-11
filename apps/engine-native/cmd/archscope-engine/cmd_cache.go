package main

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

func evidenceCleanupCommand() *cobra.Command {
	var directory string
	cmd := &cobra.Command{Use: "cleanup-cache", Short: "Explicitly remove unowned evidence-cache snapshots; save/export needed results first and close the desktop app.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if directory == "" {
			root, e := cases.DefaultRoot()
			if e != nil {
				return e
			}
			directory = filepath.Join(root, "evidence-cache")
		}
		if _, e := os.Stat(directory); e != nil {
			return fmt.Errorf("select an existing evidence-cache directory: %w", e)
		}
		c, r, e := catalog.Builtin()
		if e != nil {
			return e
		}
		s, e := analysis.New(directory, c, r)
		if e != nil {
			return e
		}
		defer s.Close()
		report, e := s.CleanupEvidenceCache()
		if e != nil {
			return e
		}
		return writeJSONAny(report, "-")
	}}
	cmd.Flags().StringVar(&directory, "cache-dir", "", "evidence-cache override (distinct from case-store)")
	return cmd
}
