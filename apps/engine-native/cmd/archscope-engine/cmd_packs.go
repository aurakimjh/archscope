package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"github.com/spf13/cobra"
)

var evidencePackStore, evidencePresetScope string

func selectedPackSession(ctx context.Context, baseDir string) (*catalog.PackSession, error) {
	base, _, e := catalog.Builtin()
	if e != nil {
		return nil, e
	}
	return catalog.OpenPackDirectory(ctx, baseDir, base)
}
func readPackInput(path string, limit int) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > int64(limit) {
		return nil, fmt.Errorf("bounded regular local file required")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(b) > limit {
		return nil, fmt.Errorf("input byte limit")
	}
	return b, e
}
func init() {
	var store, scope, hash, revision, choices, input, output, signature string
	var unsigned bool
	group := &cobra.Command{Use: "packs", Short: "Offline data-only packs, explicit trust, conformance, activation and rollback."}
	group.PersistentFlags().StringVar(&store, "store", "", "pack-store override, distinct from case-store")
	open := func(ctx context.Context) (*catalog.PackSession, error) {
		p := store
		if p == "" {
			r, e := cases.DefaultRoot()
			if e != nil {
				return nil, e
			}
			p = filepath.Join(r, "packs")
		}
		return selectedPackSession(ctx, p)
	}
	command := func(name, help string, run func(context.Context, *packs.Manager) (any, error)) *cobra.Command {
		cmd := &cobra.Command{Use: name, Short: help, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			s, e := open(cmd.Context())
			if e != nil {
				return e
			}
			defer s.Manager.Close()
			out, e := run(cmd.Context(), s.Manager)
			if e != nil {
				return e
			}
			return writeJSONAny(out, "-")
		}}
		group.AddCommand(cmd)
		return cmd
	}
	command("list", "List installed versions, scopes, active revisions and explicit alias choices.", func(_ context.Context, m *packs.Manager) (any, error) { return m.State() })
	cmd := command("install", "Verify signature/compatibility/data budgets and conformance, then stage an immutable revision without activation.", func(ctx context.Context, m *packs.Manager) (any, error) {
		b, e := readPackInput(input, packs.MaxArchiveBytes)
		if e != nil {
			return nil, e
		}
		return m.Install(ctx, b, packs.InstallOptions{AllowUnsigned: unsigned, Scope: scope})
	})
	cmd.Flags().StringVar(&input, "in", "", "data-only ZIP")
	cmd.Flags().BoolVar(&unsigned, "unsigned-local", false, "explicit locally-authored preset, never a verified publisher")
	cmd.Flags().StringVar(&scope, "scope", "application", "application, user or case:case-ID; unsigned cannot be application scoped")
	for _, name := range []string{"activate", "rollback"} {
		rollback := name == "rollback"
		cmd = command(name, "Validate retained revisions/dependencies/alias choices and atomically switch new jobs; existing jobs/results stay pinned.", func(ctx context.Context, m *packs.Manager) (any, error) {
			var selection map[string]string
			if e := packs.Strict([]byte(choices), &selection); e != nil {
				return nil, e
			}
			if rollback {
				return m.Rollback(ctx, revision, scope, hash, selection)
			}
			return m.Activate(ctx, revision, scope, hash, selection)
		})
		cmd.Flags().StringVar(&revision, "revision", "", "current state revision from list")
		cmd.Flags().StringVar(&scope, "scope", "application", "explicit activation scope")
		cmd.Flags().StringVar(&hash, "sha256", "", "installed manifest SHA-256")
		cmd.Flags().StringVar(&choices, "choices", "{}", "explicit alias to canonical importer selections")
	}
	cmd = command("deactivate", "Stop using a pack for new jobs without deleting retained versions or changing results.", func(ctx context.Context, m *packs.Manager) (any, error) {
		e := m.Deactivate(ctx, revision, scope, input)
		return map[string]bool{"deactivated": e == nil}, e
	})
	cmd.Flags().StringVar(&revision, "revision", "", "current state revision")
	cmd.Flags().StringVar(&scope, "scope", "application", "scope")
	cmd.Flags().StringVar(&input, "id", "", "pack ID")
	cmd = command("register-key", "Explicit owner/admin registration of a separately reviewed public key; packs cannot register their own signers.", func(ctx context.Context, m *packs.Manager) (any, error) {
		b, e := readPackInput(input, 4096)
		if e != nil {
			return nil, e
		}
		var key packs.Key
		if e = packs.Strict(b, &key); e != nil {
			return nil, e
		}
		e = m.RegisterKey(ctx, revision, key)
		return map[string]bool{"registered": e == nil}, e
	})
	cmd.Flags().StringVar(&revision, "revision", "", "current state revision")
	cmd.Flags().StringVar(&input, "in", "", "reviewed Key JSON with public key only")
	cmd = command("update-trust", "Import an exact-byte management-signed monotonic rotation/revocation document.", func(ctx context.Context, m *packs.Manager) (any, error) {
		b, e := readPackInput(input, packs.MaxManifestBytes)
		if e != nil {
			return nil, e
		}
		sig, e := readPackInput(signature, 2048)
		if e != nil {
			return nil, e
		}
		e = m.ApplyTrustUpdate(ctx, revision, b, sig)
		return map[string]bool{"updated": e == nil}, e
	})
	cmd.Flags().StringVar(&revision, "revision", "", "current state revision")
	cmd.Flags().StringVar(&input, "in", "", "TrustUpdate JSON")
	cmd.Flags().StringVar(&signature, "signature", "", "detached signature envelope JSON")
	cmd = command("export", "Export a retained verified pack or explicitly scoped local preset unchanged.", func(ctx context.Context, m *packs.Manager) (any, error) {
		b, e := m.Export(ctx, scope, hash)
		if e != nil {
			return nil, e
		}
		if output == "" {
			return nil, fmt.Errorf("--out required")
		}
		root, e := m.Directory()
		if e != nil {
			return nil, e
		}
		if e = packs.CheckExport(root, output); e != nil {
			return nil, e
		}
		e = analysis.AtomicExport(output, b)
		return map[string]string{"output": output}, e
	})
	cmd.Flags().StringVar(&scope, "scope", "application", "scope")
	cmd.Flags().StringVar(&hash, "sha256", "", "installed manifest SHA-256")
	cmd.Flags().StringVar(&output, "out", "", "explicit export destination outside pack-store")
	rootCmd.AddCommand(group)
}
