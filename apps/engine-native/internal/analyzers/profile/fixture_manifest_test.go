package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
)

type browserProfileManifest struct {
	Corpus string                  `json:"corpus"`
	Files  []browserProfileFixture `json:"files"`
}

type browserProfileFixture struct {
	File     string                        `json:"file"`
	Expected browserProfileFixtureExpected `json:"expected"`
}

type browserProfileFixtureExpected struct {
	Format           string                `json:"format"`
	ValueUnit        string                `json:"value_unit"`
	Parse            string                `json:"parse"`
	Samples          *int                  `json:"samples"`
	Diagnostics      *[]string             `json:"diagnostics"`
	DiagnosticsAnyOf []string              `json:"diagnostics_any_of"`
	ErrorContains    string                `json:"error_contains"`
	Findings         []string              `json:"findings"`
	TemporalOutputs  *bool                 `json:"temporal_outputs"`
	CPUSampleRunsTop *browserProfileRunTop `json:"cpu_sample_runs_top"`
}

type browserProfileRunTop struct {
	TopFrame   string `json:"top_frame"`
	StartUS    int64  `json:"start_us"`
	DurationUS int64  `json:"duration_us"`
}

func TestBrowserProfileFixtureManifest(t *testing.T) {
	root := browserProfileFixtureRoot(t)
	payload, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatalf("read browser profile fixture manifest: %v", err)
	}
	var manifest browserProfileManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("decode browser profile fixture manifest: %v", err)
	}
	if manifest.Corpus != "browser-profile-fixtures" || len(manifest.Files) == 0 {
		t.Fatalf("unexpected browser profile fixture manifest: corpus=%q files=%d", manifest.Corpus, len(manifest.Files))
	}

	results := map[string]map[string]any{}
	for _, fixture := range manifest.Files {
		fixture := fixture
		t.Run(fixture.File, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(fixture.File))
			parsed, diags, parseErr := parser.ParseFile(path, "auto", parser.Options{})
			codes := profileDiagnosticCodes(diags)

			switch fixture.Expected.Parse {
			case "error":
				if parseErr == nil {
					t.Fatalf("expected parse error; format=%q diagnostics=%v", parsed.Format, codes)
				}
				if fixture.Expected.ErrorContains != "" && !strings.Contains(parseErr.Error(), fixture.Expected.ErrorContains) {
					t.Fatalf("parse error %q does not contain %q", parseErr, fixture.Expected.ErrorContains)
				}
				assertAnyDiagnostic(t, codes, fixture.Expected.DiagnosticsAnyOf)
				return
			case "success_or_diagnostic":
				if parseErr != nil {
					assertAnyDiagnostic(t, codes, fixture.Expected.DiagnosticsAnyOf)
					return
				}
			case "success":
				if parseErr != nil {
					t.Fatalf("parse fixture: %v (diagnostics=%v)", parseErr, codes)
				}
			default:
				t.Fatalf("unsupported manifest parse expectation %q", fixture.Expected.Parse)
			}

			if fixture.Expected.Format != "" && parsed.Format != fixture.Expected.Format {
				t.Fatalf("format = %q, want %q", parsed.Format, fixture.Expected.Format)
			}
			if fixture.Expected.ValueUnit != "" && parsed.ValueUnit != fixture.Expected.ValueUnit {
				t.Fatalf("value_unit = %q, want %q", parsed.ValueUnit, fixture.Expected.ValueUnit)
			}
			if fixture.Expected.Samples != nil && len(parsed.Samples) != *fixture.Expected.Samples {
				t.Fatalf("samples = %d, want %d", len(parsed.Samples), *fixture.Expected.Samples)
			}
			if fixture.Expected.Diagnostics != nil {
				assertDiagnosticSet(t, codes, *fixture.Expected.Diagnostics)
			}
			assertAnyDiagnostic(t, codes, fixture.Expected.DiagnosticsAnyOf)

			result := Build(parsed, path, diags, Options{TopN: 50, ProfileKind: "cpu"})
			assertFindingCodes(t, result.Metadata.Findings, fixture.Expected.Findings)
			if fixture.Expected.TemporalOutputs != nil {
				_, hasRuns := result.Tables["cpu_sample_runs"]
				_, hasActivity := result.Series["cpu_activity"]
				if hasRuns != *fixture.Expected.TemporalOutputs || hasActivity != *fixture.Expected.TemporalOutputs {
					t.Fatalf("temporal outputs: runs=%t activity=%t, want %t", hasRuns, hasActivity, *fixture.Expected.TemporalOutputs)
				}
			}
			if fixture.Expected.CPUSampleRunsTop != nil {
				assertTopSampleRun(t, result.Tables["cpu_sample_runs"], parsed.Metadata, *fixture.Expected.CPUSampleRunsTop)
			}
			for _, finding := range result.Metadata.Findings {
				if finding["code"] == "SAMPLED_CPU_HOTSPOT" && strings.Contains(strings.ToLower(fmt.Sprint(finding["message"])), "long task") {
					t.Fatalf("sample-derived finding must not use Long Task wording: %q", finding["message"])
				}
			}
			if fixture.File == "e2e/e2e-render-hotspot.cpuprofile" || fixture.File == "e2e/e2e-chrome-trace.json" {
				results[fixture.File] = result.Summary
			}
		})
	}

	cpuprofile := results["e2e/e2e-render-hotspot.cpuprofile"]
	trace := results["e2e/e2e-chrome-trace.json"]
	if cpuprofile == nil || trace == nil || cpuprofile["total_duration_us"] != trace["total_duration_us"] {
		t.Fatalf("e2e Chrome trace/cpuprofile duration parity failed: cpuprofile=%v trace=%v", cpuprofile, trace)
	}
}

func browserProfileFixtureRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("ARCHSCOPE_BROWSER_PROFILE_FIXTURES"); root != "" {
		if _, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil {
			t.Fatalf("ARCHSCOPE_BROWSER_PROFILE_FIXTURES: %v", err)
		}
		return root
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve fixture test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "..", "..", "projects-assets", "test-data", "browser-profile-fixtures"))
	if _, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil {
		t.Skipf("shared browser profile fixtures are not checked out next to archscope: %v", err)
	}
	return root
}

func profileDiagnosticCodes(diags *diagnostics.ParserDiagnostics) []string {
	if diags == nil {
		return nil
	}
	codes := make([]string, 0, len(diags.Warnings)+len(diags.Errors))
	for _, item := range diags.Warnings {
		codes = append(codes, item.Reason)
	}
	for _, item := range diags.Errors {
		codes = append(codes, item.Reason)
	}
	sort.Strings(codes)
	return codes
}

func assertDiagnosticSet(t *testing.T, got, want []string) {
	t.Helper()
	want = append([]string(nil), want...)
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("diagnostics = %v, want %v", got, want)
	}
}

func assertAnyDiagnostic(t *testing.T, got, want []string) {
	t.Helper()
	if len(want) == 0 {
		return
	}
	for _, candidate := range want {
		for _, code := range got {
			if code == candidate {
				return
			}
		}
	}
	t.Fatalf("diagnostics = %v, want any of %v", got, want)
}

func assertFindingCodes(t *testing.T, findings []map[string]any, want []string) {
	t.Helper()
	for _, code := range want {
		found := false
		for _, finding := range findings {
			if finding["code"] == code {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing finding %q in %#v", code, findings)
		}
	}
}

func assertTopSampleRun(t *testing.T, raw any, metadata map[string]any, want browserProfileRunTop) {
	t.Helper()
	runs, ok := raw.([]map[string]any)
	if !ok || len(runs) == 0 {
		t.Fatalf("missing cpu_sample_runs: %#v", raw)
	}
	top := runs[0]
	if !strings.Contains(fmt.Sprint(top["stack"]), want.TopFrame) {
		t.Fatalf("top run stack = %q, want frame %q", top["stack"], want.TopFrame)
	}
	start, _ := top["start_us"].(int64)
	if base, ok := metadata["v8_start_time_us"].(int64); ok {
		start -= base
	}
	if start != want.StartUS {
		t.Fatalf("top run start offset = %d, want exactly %d", start, want.StartUS)
	}
	duration, _ := top["duration_us"].(int64)
	if duration != want.DurationUS {
		t.Fatalf("top run duration = %d, want exactly %d", duration, want.DurationUS)
	}
}
