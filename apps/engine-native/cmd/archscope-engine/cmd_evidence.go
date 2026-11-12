package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "evidence", Short: "Catalog-based bounded evidence import and backend export."}
	group.PersistentFlags().StringVar(&evidencePackStore, "pack-store", "", "use an explicit persistent pack store; defaults preserve the legacy CLI catalog")
	group.PersistentFlags().StringVar(&evidencePresetScope, "preset-scope", "", "explicit user/case preset scope, otherwise application packs only")
	list := &cobra.Command{Use: "formats", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, e := catalog.Builtin()
		if e != nil {
			return e
		}
		if evidencePackStore != "" {
			session, e := selectedPackSession(cmd.Context(), evidencePackStore)
			if e != nil {
				return e
			}
			defer session.Manager.Close()
			snapshot, e := session.Provider(evidencePresetScope)
			if e != nil {
				return e
			}
			return writeJSONAny(snapshot.Catalog.List(), "-")
		}
		return writeJSONAny(c.List(), "-")
	}}
	var input, importer, options, output, exportFormat string
	var partial bool
	run := &cobra.Command{Use: "import", RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if input == "" {
			return fmt.Errorf("--in is required")
		}
		c, r, e := catalog.Builtin()
		if e != nil {
			return e
		}
		root, e := os.MkdirTemp("", "archscope-evidence-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(root)
		service, e := analysis.New(root, c, r)
		if e != nil {
			return e
		}
		defer service.Close()
		if evidencePackStore != "" {
			session, e := selectedPackSession(ctx, evidencePackStore)
			if e != nil {
				return e
			}
			defer session.Manager.Close()
			service.SetCatalogProvider(session.Provider)
		}
		job, e := service.Start(analysis.Request{PresetScope: evidencePresetScope, Path: input, Importer: importer, Options: json.RawMessage(options), AllowPartial: partial})
		if e != nil {
			return e
		}
		// CLI cannot stop for an asynchronous GUI selection. Resolve the same
		// catalog first; hints/ambiguity return a visible, actionable error.
		for {
			snapshot, e := service.Get(job.ID, job.Generation)
			if e != nil {
				return e
			}
			if snapshot.State == "needs_selection" {
				service.Cancel(job.ID, job.Generation)
				candidates, _ := json.Marshal(snapshot.Detection)
				fmt.Fprintln(cmd.ErrOrStderr(), string(candidates))
				return fmt.Errorf("IMPORT_SELECTION_REQUIRED: choose a detected --importer")
			}
			if snapshot.State == "completed" || snapshot.State == "partial" || snapshot.State == "failed" || snapshot.State == "cancelled" {
				job = snapshot
				break
			}
			select {
			case <-ctx.Done():
				service.Cancel(job.ID, job.Generation)
				return ctx.Err()
			case <-time.After(5 * time.Millisecond):
			}
		}
		if job.ResultHandle == "" {
			if job.Problem != nil {
				return fmt.Errorf("%s: %s", job.Problem.Code, job.Problem.Message)
			}
			return fmt.Errorf("import %s", job.State)
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		body, e := service.Export(job.ResultHandle, exportFormat)
		if e != nil {
			return e
		}
		if output == "" || output == "-" {
			_, e = cmd.OutOrStdout().Write(body)
			return e
		}
		return analysis.AtomicExport(output, body)
	}}
	run.Flags().StringVar(&input, "in", "", "selected local source")
	run.Flags().StringVar(&importer, "importer", "auto", "canonical ID or legacy alias; ambiguity requires explicit selection")
	run.Flags().StringVar(&options, "options", "{}", "schema-validated JSON options")
	run.Flags().StringVar(&output, "out", "-", "output path or stdout")
	run.Flags().StringVar(&exportFormat, "export-format", "json", "json, csv, html or report-pack")
	run.Flags().BoolVar(&partial, "allow-partial", false, "allow importer-defined complete-record partial output")
	var stored, destination, kind string
	reopen := &cobra.Command{Use: "export", Short: "Reopen a stored result and export its backend revision.", RunE: func(cmd *cobra.Command, args []string) error {
		if stored == "" {
			return fmt.Errorf("--in is required")
		}
		c, r, e := catalog.Builtin()
		if e != nil {
			return e
		}
		root, e := os.MkdirTemp("", "archscope-evidence-export-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(root)
		service, e := analysis.New(root, c, r)
		if e != nil {
			return e
		}
		defer service.Close()
		view, e := service.Load(cmd.Context(), stored)
		if e != nil {
			return e
		}
		body, e := service.Export(view.Handle, kind)
		if e != nil {
			return e
		}
		if destination == "-" {
			_, e = cmd.OutOrStdout().Write(body)
			return e
		}
		return analysis.AtomicExport(destination, body)
	}}
	reopen.Flags().StringVar(&stored, "in", "", "saved AnalysisResult JSON")
	reopen.Flags().StringVar(&destination, "out", "-", "output path or stdout")
	reopen.Flags().StringVar(&kind, "export-format", "json", "json, raw-json, csv, html or report-pack")
	run.Aliases = []string{"analyze"}
	group.AddCommand(list, run, reopen)
	group.AddCommand(inputAssistanceCommands()...)
	group.AddCommand(resultProviderCommands()...)
	group.AddCommand(evidenceCleanupCommand())
	rootCmd.AddCommand(group)
}
