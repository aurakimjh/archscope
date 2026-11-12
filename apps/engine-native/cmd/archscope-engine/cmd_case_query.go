package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"io"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "case-query", Short: "Revision-bound case search and original-reimported profile baseline pins."}
	var storePath, request string
	group.PersistentFlags().StringVar(&storePath, "store", "", "case store")
	group.PersistentFlags().StringVar(&request, "request", "", "bounded JSON request file")
	for _, operation := range []string{"search", "pin-baseline", "compare-baseline"} {
		command := &cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			root := storePath
			if root == "" {
				p, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				root = filepath.Join(p, "case-store")
			}
			store, e := cases.Open(root)
			if e != nil {
				return e
			}
			defer store.Close()
			f, e := os.Open(request)
			if e != nil {
				return e
			}
			defer f.Close()
			body, e := io.ReadAll(io.LimitReader(f, 64<<10+1))
			if e != nil {
				return e
			}
			var output any
			if operation == "search" {
				var q cases.SearchQuery
				if e = cases.DecodeQuery(body, &q); e != nil {
					return e
				}
				output, e = store.Search(cmd.Context(), q)
			} else {
				c, r, catalogErr := catalog.Builtin()
				if catalogErr != nil {
					return catalogErr
				}
				cache, tempErr := os.MkdirTemp("", "case-query-")
				if tempErr != nil {
					return tempErr
				}
				defer os.RemoveAll(cache)
				service, serviceErr := analysis.New(cache, c, r)
				if serviceErr != nil {
					return serviceErr
				}
				defer service.Close()
				if operation == "pin-baseline" {
					var q analysis.BaselineRequest
					if e = cases.DecodeQuery(body, &q); e != nil {
						return e
					}
					output, e = service.PinCaseBaseline(cmd.Context(), store, q)
				} else {
					var q analysis.BaselineComparisonRequest
					if e = cases.DecodeQuery(body, &q); e != nil {
						return e
					}
					output, e = service.CompareCaseBaseline(cmd.Context(), store, q)
				}
			}
			if e != nil {
				return e
			}
			return writeJSONAny(output, "-")
		}}
		group.AddCommand(command)
	}
	rootCmd.AddCommand(group)
	var input, queryJSON string
	command := &cobra.Command{Use: "profile-query", Short: "Canonical selected profile views with exact denominator and stale-cursor checks.", RunE: func(cmd *cobra.Command, _ []string) error {
		q, e := analysis.DecodeProfileQuery([]byte(queryJSON))
		if e != nil {
			return e
		}
		f, e := os.Open(input)
		if e != nil {
			return e
		}
		defer f.Close()
		raw, e := io.ReadAll(io.LimitReader(f, cases.MaxObjectBytes+1))
		if e != nil {
			return e
		}
		if len(raw) > cases.MaxObjectBytes {
			return fmt.Errorf("result byte limit")
		}
		result, e := resultcontract.DecodeWithLimits(raw, wirejson.ResultLimits(cases.MaxObjectBytes, 1024))
		if e != nil {
			return e
		}
		registry, e := catalog.ResultRegistry()
		if e != nil {
			return e
		}
		if v := registry.Check(result, "compute"); !v.Allowed {
			return fmt.Errorf("%s", v.Reason)
		}
		core, e := analyzer.ReadCore(result)
		if e != nil {
			return e
		}
		sum := sha256.Sum256(raw)
		page, e := profilemodel.QuerySet(cmd.Context(), core.Model, hex.EncodeToString(sum[:]), q)
		if e != nil {
			return e
		}
		return writeJSONAny(page, "-")
	}}
	command.Flags().StringVar(&input, "in", "", "saved result")
	command.Flags().StringVar(&queryJSON, "query", "{}", "explicit selection/view/page JSON")
	rootCmd.AddCommand(command)
}
