package main

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/distribution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/netpolicy"
	"github.com/spf13/cobra"
)

var offlineRoot, offlineReport string

func init() {
	rootCmd.PersistentFlags().StringVar(&offlineRoot, "offline-root", "", "verify this disconnected bundle, then deny app-owned network operations")
	rootCmd.PersistentFlags().StringVar(&offlineReport, "offline-report", "", "write app-owned offline attempt counters (separate from OS observation)")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if offlineRoot == "" {
			return nil
		}
		if _, e := distribution.Verify(offlineRoot, false); e != nil {
			return e
		}
		netpolicy.Enable()
		return nil
	}
	group := &cobra.Command{Use: "offline", Short: "Build/preflight a standalone disconnected bundle and execute release fixtures."}
	var opts distribution.BuildOptions
	build := &cobra.Command{Use: "build", RunE: func(cmd *cobra.Command, args []string) error {
		m, e := distribution.Build(cmd.Context(), opts)
		if e != nil {
			return e
		}
		return writeJSONAny(m, "-")
	}}
	build.Flags().StringVar(&opts.Out, "out", "", "new output directory")
	build.Flags().StringVar(&opts.Engine, "engine", "", "same-platform prebuilt CLI")
	build.Flags().StringVar(&opts.App, "app", "", "optional same-platform desktop executable")
	build.Flags().StringVar(&opts.WebviewRuntime, "webview-runtime", "", "complete local licensed Fixed Version WebView2 directory (Windows desktop required)")
	build.Flags().StringVar(&opts.WebviewVersion, "webview-version", "", "explicit local Fixed Version identifier")
	var checkRoot string
	verify := &cobra.Command{Use: "verify", RunE: func(cmd *cobra.Command, args []string) error {
		m, e := distribution.Verify(checkRoot, false)
		if e != nil {
			return e
		}
		return writeJSONAny(m, "-")
	}}
	verify.Flags().StringVar(&checkRoot, "root", "", "bundle directory")
	fixtures := &cobra.Command{Use: "fixtures", RunE: func(cmd *cobra.Command, args []string) error {
		ids, e := distribution.CheckFixtures(cmd.Context())
		if e != nil {
			return e
		}
		return writeJSONAny(map[string]any{"version": "1.0.0", "verified_importers": ids}, "-")
	}}
	group.AddCommand(build, verify, fixtures)
	rootCmd.AddCommand(group)
}
