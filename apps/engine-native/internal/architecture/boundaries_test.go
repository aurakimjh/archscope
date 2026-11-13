// Package architecture checks production imports on every OS, independent of
// build tags. Source strings and test-only imports cannot grant exceptions.
package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/aurakimjh/archscope/apps/engine-native/internal/"

// Exact file/import exceptions retain existing readers/exporters. No package
// wildcard can silently authorize a new dependency or execution path.
var exceptions = map[string]string{
	"parsers/jfr/recording.go|os/exec":                                   "installed JDK discovery via LookPath only; execution belongs to guarded localconvert",
	"exporters/gputrace/svg.go|" + module + "analyzers/gputrace":         "existing frozen selected-source SVG projection; no parser access",
	"exporters/html/jennifer.go|" + module + "analyzers/jenniferprofile": "legacy report projection type; no parser access",
}

func forbidden(file, dep string) bool {
	parts := strings.Split(file, "/")
	owner := parts[0]
	domain := owner == "parsers" || owner == "analyzers" || owner == "exporters"
	model := strings.HasSuffix(owner, "model") || owner == "statistics" || owner == "resultcontract"
	if (domain || model) && (dep == "os/exec" || dep == "net" || strings.HasPrefix(dep, "net/") && dep != "net/url" && dep != "net/netip") {
		return true
	}
	if owner == "parsers" || model {
		if strings.HasPrefix(dep, module+"analyzers/") || strings.HasPrefix(dep, module+"exporters/") {
			return true
		}
	}
	if owner == "exporters" || model {
		if strings.HasPrefix(dep, module+"parsers/") || strings.HasPrefix(dep, module+"analyzers/") {
			return true
		}
	}
	// New canonical analyzers consume normalized models, never source readers.
	if owner == "analyzers" && len(parts) > 1 {
		switch parts[1] {
		case "execution", "gputrace", "metrichistory", "inference", "runtimeevidence", "systemtrace":
			return strings.HasPrefix(dep, module+"parsers/")
		}
	}
	return false
}

func TestProductionBoundaries(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(here))
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "testdata" || entry.Name() == "assets" || entry.Name() == "contracts") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			dep, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if !forbidden(rel, dep) {
				continue
			}
			key := rel + "|" + dep
			if exceptions[key] == "" {
				t.Errorf("%s: forbidden import %s", rel, dep)
				continue
			}
			used[key] = true
			if dep == "os/exec" {
				alias := "exec"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
				if alias == "." || alias == "_" {
					t.Errorf("%s: discovery import must be named", rel)
				}
				ast.Inspect(f, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == alias && sel.Sel.Name != "LookPath" {
							t.Errorf("%s: exec.%s exceeds discovery exception", rel, sel.Sel.Name)
						}
					}
					return true
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key := range exceptions {
		if !used[key] {
			t.Errorf("stale exception: %s", key)
		}
	}
}

func TestBoundaryPolicyRejectsNewPaths(t *testing.T) {
	for _, c := range []struct {
		file, dep string
		want      bool
	}{
		{"parsers/new/source.go", "net/http", true},
		{"parsers/new/source_windows.go", "os/exec", true},
		{"exporters/new/render.go", module + "parsers/new", true},
		{"parsers/new/source.go", module + "analyzers/new", true},
		{"executionmodel/model.go", module + "parsers/new", true},
		{"analyzers/execution/analyzer.go", module + "parsers/execution", true},
		{"analyzers/legacy/analyzer.go", module + "parsers/legacy", false},
		{"parsers/new/source.go", "net/url", false},
	} {
		if got := forbidden(c.file, c.dep); got != c.want {
			t.Errorf("%s -> %s: %v", c.file, c.dep, got)
		}
	}
}
