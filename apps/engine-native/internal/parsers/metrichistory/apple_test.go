package metrichistory_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
)

func appleFixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../../catalog/contracts/apple-ollama-capture.json")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestAppleActualCaptureScopeAvailabilityAndCommonResult(t *testing.T) {
	raw := appleFixture(t)
	set := parse(t, raw, p.Options{})
	if len(set.Series) != 14 || len(set.Points) != 42 {
		t.Fatal(len(set.Series), len(set.Points))
	}
	ids := map[string]string{}
	for _, s := range set.Series {
		ids[s.ID] = s.Name
		if s.Kind != "gauge" || !bytes.Contains(s.Resource, []byte(`"memory_model":"unified_shared"`)) || bytes.Contains(s.Resource, []byte("device_uuid")) {
			t.Fatal(s)
		}
	}
	missing := 0
	for _, point := range set.Points {
		if point.Special == "no_recorded_value" {
			missing++
			if point.Value != nil {
				t.Fatal("missing became zero")
			}
		}
		if ids[point.SeriesID] == "ollama.model.gpu_residency_estimate" && *point.Value != "1799817788" {
			t.Fatal(point)
		}
	}
	if missing != 9 {
		t.Fatal(missing)
	}
	rows, e := m.Intervals(ctx, set)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Rate != nil || r.Delta != nil {
			t.Fatal("gauge became counter", r)
		}
	}
	result, e := a.Build(ctx, set, selected(set), "capture.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(result); e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(result)
	var restored = result
	if e = json.Unmarshal(encoded, &restored); e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(restored); e != nil {
		t.Fatal(e)
	}
	var zipped bytes.Buffer
	z := gzip.NewWriter(&zipped)
	z.Write(raw)
	z.Close()
	g := parse(t, zipped.Bytes(), p.Options{Format: "apple-ollama-capture-json"})
	sum := sha256.Sum256(zipped.Bytes())
	if g.Sources[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("original digest")
	}
}
func TestAppleExactIdentityBoundsAndRefusals(t *testing.T) {
	raw := appleFixture(t)
	large := bytes.ReplaceAll(raw, []byte("1799817788"), []byte("9007199254740993"))
	set := parse(t, large, p.Options{})
	count := 0
	for _, p := range set.Points {
		if p.Value != nil && *p.Value == "9007199254740993" {
			count++
		}
	}
	if count != 6 {
		t.Fatal("integer rounded", count)
	}
	for name, edit := range map[string]func(map[string]any){
		"version":  func(c map[string]any) { c["ollama_version"] = "0.40.17" },
		"identity": func(c map[string]any) { c["boot_id"] = "" },
		"unknown":  func(c map[string]any) { c["prompt"] = "BODY_MUST_NOT_BE_ECHOED" },
		"missing":  func(c map[string]any) { delete(c["snapshots"].([]any)[0].(map[string]any), "power_watts") },
		"fake-utilization": func(c map[string]any) {
			c["snapshots"].([]any)[0].(map[string]any)["gpu_utilization"] = map[string]any{"value": "100"}
		},
		"time": func(c map[string]any) { c["snapshots"].([]any)[0].(map[string]any)["capture_end_ns"] = "0" },
		"swap": func(c map[string]any) { c["snapshots"].([]any)[0].(map[string]any)["swap_used_bytes"] = "1" },
		"negative": func(c map[string]any) {
			c["snapshots"].([]any)[0].(map[string]any)["models"].([]any)[0].(map[string]any)["size_vram"] = -1
		},
		"duplicate-model": func(c map[string]any) {
			s := c["snapshots"].([]any)[0].(map[string]any)
			s["models"] = append(s["models"].([]any), s["models"].([]any)[0])
		},
		"clock-overlap": func(c map[string]any) {
			s := c["snapshots"].([]any)
			s[1].(map[string]any)["capture_start_ns"] = s[0].(map[string]any)["capture_start_ns"]
		},
		"budget": func(c map[string]any) {
			s := c["snapshots"].([]any)
			for len(s) <= 256 {
				s = append(s, s[0])
			}
			c["snapshots"] = s
		},
	} {
		t.Run(name, func(t *testing.T) {
			var c map[string]any
			json.Unmarshal(raw, &c)
			edit(c)
			b, _ := json.Marshal(c)
			if _, err := p.ParseBytes(ctx, b, "capture.json", p.Options{}); err == nil {
				t.Fatal("invalid capture accepted")
			}
		})
	}
	if _, e := p.ParseBytes(ctx, raw, "capture.json", p.Options{Producer: "nvidia"}); e == nil {
		t.Fatal("identity override")
	}
	duplicate := bytes.Replace(raw, []byte(`"collector_version": "1.0.0",`), []byte(`"collector_version":"1.0.0","collector_version":"1.0.0",`), 1)
	if _, e := p.ParseBytes(ctx, duplicate, "capture.json", p.Options{}); e == nil {
		t.Fatal("duplicate keys")
	}
	if _, e := p.ParseBytes(ctx, []byte(strings.Repeat(" ", 4<<20+1)), "capture.json", p.Options{Format: "apple-ollama-capture-json"}); e == nil {
		t.Fatal("budget ignored")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := p.ParseBytes(cancelled, raw, "capture.json", p.Options{}); e == nil {
		t.Fatal("cancel ignored")
	}
}

func TestAppleMirroredManifestAndNormalization(t *testing.T) {
	raw, err := os.ReadFile("../../catalog/contracts/apple-ollama-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []struct {
			File string `json:"file"`
			SHA  string `json:"sha256"`
		} `json:"files"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Files {
		body, err := os.ReadFile("../../catalog/contracts/apple-ollama-" + entry.File)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != entry.SHA {
			t.Fatal(entry.File)
		}
	}
	set, err := p.ParseBytes(ctx, appleFixture(t), "apple-ollama-capture.json", p.Options{})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("../../catalog/contracts/apple-ollama-metric-model.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := m.Decode(expected)
	if err != nil {
		t.Fatal(err)
	}
	// The immutable fixture retains original semantics; current parsing records its revision.
	want.Normalizer = m.ObservationNormalizer
	got, _ := json.Marshal(set)
	canonical, _ := json.Marshal(want)
	if !bytes.Equal(got, canonical) {
		t.Fatal("normalization drift")
	}
}

func TestR20ActualSelectionCoverageAndLegacyProjection(t *testing.T) {
	for _, c := range []struct{ name, matched, excluded string }{{"apple-multi", "1", "1"}, {"apple-unmatched", "0", "2"}, {"apple-empty", "0", "0"}} {
		raw, e := os.ReadFile("../../catalog/contracts/apple-ollama-" + c.name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		set := parse(t, raw, p.Options{})
		seen := map[string]string{}
		names := map[string]string{}
		for _, s := range set.Series {
			names[s.ID] = s.Name
		}
		for _, v := range set.Points {
			if v.Value != nil {
				seen[names[v.SeriesID]] = *v.Value
			}
		}
		if seen["ollama.selection.excluded_model_count"] != c.excluded || seen["ollama.selection.selected_model_match_count"] != c.matched {
			t.Fatal(c.name, seen)
		}
		encoded, _ := json.Marshal(set)
		if bytes.Contains(encoded, []byte("nomic-embed-text")) {
			t.Fatal("excluded identity retained")
		}
		if !strings.Contains(strings.Join(set.Warnings, " "), "not loaded") {
			t.Fatal("coverage warning absent")
		}
		var bad map[string]any
		json.Unmarshal(raw, &bad)
		snap := bad["snapshots"].([]any)[0].(map[string]any)
		delete(snap, "excluded_model_count")
		b, _ := json.Marshal(bad)
		if _, err := p.ParseBytes(ctx, b, "coverage.json", p.Options{}); err == nil {
			t.Fatal("missing coverage accepted")
		}
	}
	raw := appleFixture(t)
	set := parse(t, raw, p.Options{})
	before, _ := json.Marshal(set)
	warnings := p.InspectionWarnings(set)
	after, _ := json.Marshal(set)
	if !bytes.Equal(before, after) || !strings.Contains(strings.Join(warnings, " "), "Legacy Apple") {
		t.Fatal("legacy mutated or no coverage notice")
	}
}
