// Tests mirroring engines/python/tests/test_demo_site.py
// (TestAnalyzerTypeMapping). Same fixtures, JSON form only.
package demosite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSON(t *testing.T, path string, payload any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadMappingsFromJSON(t *testing.T) {
	tmp := t.TempDir()
	mappingFile := filepath.Join(tmp, "analyzer_type_mapping.json")
	writeJSON(t, mappingFile, map[string]any{
		"mappings": map[string]any{
			"access_log": map[string]any{
				"command":      []string{"access-log", "analyze"},
				"input_option": "--file",
			},
			"profiler_collapsed": map[string]any{
				"command":      []string{"profiler", "analyze-collapsed"},
				"input_option": "--wall",
				"format_overrides": map[string]any{
					"jennifer_csv": map[string]any{
						"command":      []string{"profiler", "analyze-jennifer-csv"},
						"input_option": "--file",
					},
				},
			},
			"reference_doc": map[string]any{
				"command":      nil,
				"input_option": nil,
				"note":         "Documentation file, not analyzed",
			},
		},
	})

	mappings, err := LoadAnalyzerTypeMappings(tmp)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := mappings["access_log"]; got.Command == nil || got.Command[0] != "access-log" || got.InputOption != "--file" {
		t.Fatalf("access_log mapping wrong: %+v", got)
	}
	if _, ok := mappings["profiler_collapsed"].FormatOverrides["jennifer_csv"]; !ok {
		t.Fatalf("missing jennifer_csv override")
	}
	if mappings["reference_doc"].Command != nil {
		t.Fatalf("reference_doc command should be nil; got %v", mappings["reference_doc"].Command)
	}
	if mappings["reference_doc"].Note != "Documentation file, not analyzed" {
		t.Fatalf("reference_doc note wrong: %q", mappings["reference_doc"].Note)
	}
}

func TestLoadMappingsRaisesOnInvalidStructure(t *testing.T) {
	tmp := t.TempDir()
	mappingFile := filepath.Join(tmp, "analyzer_type_mapping.json")
	writeJSON(t, mappingFile, map[string]any{"wrong_key": map[string]any{}})

	_, err := LoadAnalyzerTypeMappings(tmp)
	if err == nil {
		t.Fatal("expected error on invalid mapping structure")
	}
	if !strings.Contains(err.Error(), "Invalid analyzer type mapping") {
		t.Fatalf("error message mismatch: %v", err)
	}
}

func TestFindMappingWalksParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	mappingFile := filepath.Join(tmp, "analyzer_type_mapping.json")
	writeJSON(t, mappingFile, map[string]any{
		"mappings": map[string]any{
			"a": map[string]any{"command": []string{"a"}, "input_option": "--f"},
		},
	})

	nested := filepath.Join(tmp, "sub", "dir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	found, err := FindAnalyzerTypeMapping(nested)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found != mappingFile {
		t.Fatalf("found %q want %q", found, mappingFile)
	}
}

func TestFindMappingRaisesWhenNotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := FindAnalyzerTypeMapping(filepath.Join(tmp, "nonexistent"))
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestCommandForMappingUsesFormatOverride(t *testing.T) {
	override := AnalyzerTypeMapping{
		AnalyzerType: "profiler",
		Command:      []string{"profiler", "analyze-jennifer-csv"},
		InputOption:  "--file",
	}
	mapping := AnalyzerTypeMapping{
		AnalyzerType: "profiler",
		Command:      []string{"profiler", "analyze-collapsed"},
		InputOption:  "--wall",
		FormatOverrides: map[string]AnalyzerTypeMapping{
			"jennifer_csv": override,
		},
	}

	if cmd := CommandForMapping(mapping, ""); strings.Join(cmd, " ") != "profiler analyze-collapsed" {
		t.Fatalf("base command wrong: %v", cmd)
	}
	if cmd := CommandForMapping(mapping, "jennifer_csv"); strings.Join(cmd, " ") != "profiler analyze-jennifer-csv" {
		t.Fatalf("override command wrong: %v", cmd)
	}
}

func TestInputOptionForMappingUsesFormatOverride(t *testing.T) {
	override := AnalyzerTypeMapping{
		AnalyzerType: "profiler",
		Command:      []string{"profiler", "analyze-jennifer-csv"},
		InputOption:  "--file",
	}
	mapping := AnalyzerTypeMapping{
		AnalyzerType: "profiler",
		Command:      []string{"profiler", "analyze-collapsed"},
		InputOption:  "--wall",
		FormatOverrides: map[string]AnalyzerTypeMapping{
			"jennifer_csv": override,
		},
	}

	if InputOptionForMapping(mapping, "") != "--wall" {
		t.Fatalf("base input_option wrong")
	}
	if InputOptionForMapping(mapping, "jennifer_csv") != "--file" {
		t.Fatalf("override input_option wrong")
	}
}
