package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"github.com/spf13/cobra"
)

func init() {
	var root, caseID, revision, requestFile, input, output, resultID, digest, token, title string
	group := &cobra.Command{Use: "case", Short: "Durable local cases, safe source intake and explicitly reviewed portable snapshots."}
	group.PersistentFlags().StringVar(&root, "store", "", "app-data case store override")
	group.PersistentFlags().StringVar(&caseID, "id", "", "case ID")
	group.PersistentFlags().StringVar(&revision, "revision", "", "expected immutable revision")
	open := func() (*cases.Store, error) {
		p := root
		if p == "" {
			r, e := cases.DefaultRoot()
			if e != nil {
				return nil, e
			}
			p = filepath.Join(r, "case-store")
		}
		return cases.Open(p)
	}
	command := func(name, help string, fn func(context.Context, *cases.Store) (any, error)) *cobra.Command {
		c := &cobra.Command{Use: name, Short: help, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			store, e := open()
			if e != nil {
				return e
			}
			defer store.Close()
			value, e := fn(cmd.Context(), store)
			if e != nil {
				return e
			}
			return writeJSONAny(value, "-")
		}}
		group.AddCommand(c)
		return c
	}
	read := func(file string, out any) error {
		var r io.ReadCloser
		if file == "-" {
			r = io.NopCloser(os.Stdin)
		} else {
			f, e := os.Open(file)
			if e != nil {
				return e
			}
			defer f.Close()
			r = f
		}
		body, e := io.ReadAll(io.LimitReader(r, cases.MaxManifestBytes+1))
		if e != nil {
			return e
		}
		if len(body) > cases.MaxManifestBytes {
			return fmt.Errorf("request byte limit")
		}
		return wirejson.Decode(body, out)
	}
	c := command("create", "Create an active case with no automatic retention expiry.", func(_ context.Context, s *cases.Store) (any, error) { return s.Create(title) })
	c.Flags().StringVar(&title, "title", "", "case title")
	command("list", "List cases, corruption diagnostics and user retention reminders.", func(_ context.Context, s *cases.Store) (any, error) { return s.List() })
	command("show", "Read the committed manifest and source availability.", func(_ context.Context, s *cases.Store) (any, error) { return s.Get(caseID) })
	c = command("edit", "Apply a revision-checked JSON edit; see case lifecycle documentation.", func(ctx context.Context, s *cases.Store) (any, error) {
		var req cases.Edit
		if e := read(requestFile, &req); e != nil {
			return nil, e
		}
		return s.Apply(ctx, req)
	})
	c.Flags().StringVar(&requestFile, "request", "", "bounded JSON request file or -")
	c = command("save-result", "Save exact AnalysisResult bytes with explicit source/capture refs.", func(_ context.Context, s *cases.Store) (any, error) {
		var req cases.SaveResult
		if e := read(requestFile, &req); e != nil {
			return nil, e
		}
		f, e := os.Open(input)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		body, e := io.ReadAll(io.LimitReader(f, cases.MaxObjectBytes+1))
		if e != nil {
			return nil, e
		}
		return s.SaveResult(req, body)
	})
	c.Flags().StringVar(&requestFile, "request", "", "SaveResult JSON file")
	c.Flags().StringVar(&input, "in", "", "saved AnalysisResult JSON")
	c = command("export-result", "Export a saved snapshot unchanged, including unknown numeric tokens.", func(_ context.Context, s *cases.Store) (any, error) {
		body, e := s.ExportSnapshot(caseID, resultID)
		if e != nil {
			return nil, e
		}
		if output == "" {
			return nil, fmt.Errorf("--out required")
		}
		if e = s.CheckExportDestination(caseID, output); e != nil {
			return nil, e
		}
		e = analysis.AtomicExport(output, body)
		return map[string]string{"output": output}, e
	})
	c.Flags().StringVar(&resultID, "result", "", "result reference ID")
	c.Flags().StringVar(&output, "out", "", "explicit destination")
	c = command("preview", "Validate ZIP/TAR/tar.gz before selecting archive entries.", func(ctx context.Context, _ *cases.Store) (any, error) { return cases.PreviewArchive(ctx, input) })
	c.Flags().StringVar(&input, "in", "", "archive")
	c = command("pack", "Export explicitly selected, sharing-reviewed sources/results.", func(ctx context.Context, s *cases.Store) (any, error) {
		var req cases.ExportOptions
		if e := read(requestFile, &req); e != nil {
			return nil, e
		}
		body, e := s.Portable(ctx, caseID, req)
		if e != nil {
			return nil, e
		}
		if output == "" {
			return nil, fmt.Errorf("--out required")
		}
		if e = s.CheckExportDestination(caseID, output); e != nil {
			return nil, e
		}
		e = analysis.AtomicExport(output, body)
		return map[string]string{"output": output}, e
	})
	c.Flags().StringVar(&requestFile, "request", "", "ExportOptions JSON file")
	c.Flags().StringVar(&output, "out", "", "portable ZIP destination")
	c = command("restore", "Validate a previewed portable case and restore under a new case ID.", func(ctx context.Context, s *cases.Store) (any, error) { return s.Restore(ctx, input, digest) })
	c.Flags().StringVar(&input, "in", "", "portable archive")
	c.Flags().StringVar(&digest, "sha256", "", "SHA256 from preview")
	command("delete-plan", "Show managed deletion scope and exclusions; no file is deleted.", func(_ context.Context, s *cases.Store) (any, error) { return s.DeletionPlan(caseID) })
	c = command("purge", "Permanently remove a logically deleted case using its current plan token.", func(_ context.Context, s *cases.Store) (any, error) {
		e := s.Purge(caseID, revision, token)
		return map[string]string{"purged": caseID}, e
	})
	c.Flags().StringVar(&token, "token", "", "current deletion-plan token")
	command("rebuild-index", "Rebuild disposable case index from authoritative manifests.", func(_ context.Context, s *cases.Store) (any, error) { return s.RebuildIndex() })
	command("cleanup-cache", "Remove only owned temporary files and rebuildable case index.", func(_ context.Context, s *cases.Store) (any, error) {
		e := s.CleanupCache()
		return map[string]bool{"cleaned": e == nil}, e
	})
	rootCmd.AddCommand(group)
}
