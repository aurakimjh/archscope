package httpcapture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type fixtureManifest struct {
	Files []struct {
		File     string `json:"file"`
		Dialect  string `json:"dialect"`
		Expected struct {
			Parse       string   `json:"parse"`
			Dialect     string   `json:"dialect"`
			Diagnostics []string `json:"diagnostics"`
		} `json:"expected"`
	} `json:"files"`
}

func TestSharedHARFixtureManifests(t *testing.T) {
	root := sharedHARFixtures(t)
	for _, corpus := range []string{"dialects", "malformed", "adversarial"} {
		manifestPath := filepath.Join(root, corpus, "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		var manifest fixtureManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("decode %s: %v", manifestPath, err)
		}
		for _, fixture := range manifest.Files {
			fixture := fixture
			t.Run(corpus+"/"+fixture.File, func(t *testing.T) {
				path := filepath.Join(root, corpus, fixture.File)
				parsed, parseErr := ParseFile(path, Options{})
				shouldSucceed := fixture.Expected.Parse == "" || fixture.Expected.Parse == "success"
				if shouldSucceed && parseErr != nil {
					t.Fatalf("parse failed: %v", parseErr)
				}
				if !shouldSucceed {
					if parseErr == nil {
						t.Fatal("expected fixture rejection")
					}
					if len(parsed.Entries) != 0 {
						t.Fatalf("rejected fixture returned partial entries: %d", len(parsed.Entries))
					}
					return
				}
				expectedDialect := fixture.Expected.Dialect
				if expectedDialect == "" {
					expectedDialect = fixture.Dialect
				}
				if expectedDialect != "" && parsed.Dialect != expectedDialect {
					t.Fatalf("dialect: want %q, got %q", expectedDialect, parsed.Dialect)
				}
				gotDiagnostics := warningReasons(parsed)
				expectedDiagnostics := append([]string(nil), fixture.Expected.Diagnostics...)
				sort.Strings(expectedDiagnostics)
				for _, expected := range expectedDiagnostics {
					if !containsString(gotDiagnostics, expected) {
						t.Fatalf("diagnostics: required %v, got %v", expectedDiagnostics, gotDiagnostics)
					}
				}
				assertFixtureSecurity(t, fixture.File, parsed)
			})
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sharedHARFixtures(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../../../../../..", "projects-assets", "test-data", "har-fixtures"))
}

func warningReasons(parsed ParseResult) []string {
	if parsed.Diagnostics == nil {
		return []string{}
	}
	reasons := make([]string, 0, len(parsed.Diagnostics.Warnings))
	for _, warning := range parsed.Diagnostics.Warnings {
		reasons = append(reasons, warning.Reason)
	}
	sort.Strings(reasons)
	return reasons
}

func assertFixtureSecurity(t *testing.T, name string, parsed ParseResult) {
	t.Helper()
	encoded, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, secret := range []string{
		"EXAMPLE-API-KEY-0000",
		"EXAMPLE-NEW-SESSION",
		"EXAMPLE-PASSWORD-DO-NOT-USE",
		"EXAMPLE-SESSION-COOKIE",
		"FAKE-SIGNATURE-EXAMPLE",
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("normalized result leaked fixture secret %q", secret)
		}
	}
	if strings.Contains(name, "sensitive") || name == "secrets-everywhere.har" {
		if !parsed.Redaction.Applied {
			t.Fatal("sensitive fixture did not report redaction")
		}
	}
	if name == "base64-no-encoding.har" && len(parsed.Entries) > 0 && parsed.Entries[0].Response.BodyPreview != "" {
		t.Fatal("binary-looking response was exposed as text")
	}
	if name == "sizes-unavailable.har" && len(parsed.Entries) > 0 && parsed.Entries[0].Response.BodySize != -1 {
		t.Fatal("unknown response size was coerced away from -1")
	}
}
