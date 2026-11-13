package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAllCanonicalPayloadsHaveKnownRendererContracts(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(here), "../../cmd/archscope-app/frontend/src/state/resultPanels.ts")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	_, text, ok := strings.Cut(string(body), "/* panel-contract:start */")
	if !ok {
		t.Fatal("missing renderer inventory")
	}
	text, _, ok = strings.Cut(text, "/* panel-contract:end */")
	if !ok {
		t.Fatal("missing renderer inventory end")
	}
	var renderers map[string]struct {
		Version  string `json:"version"`
		Required string `json:"required_action"`
	}
	if err = json.Unmarshal([]byte(strings.TrimSpace(text)), &renderers); err != nil {
		t.Fatal(err)
	}
	registry, err := ResultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	// Every normalized family and every registered legacy/source/distribution
	// variant must publish a panel. New versions cannot vanish behind a TS list.
	canonical := map[string]bool{"profile_core": true, "metrics_core": true, "system_core": true, "inference_core": true, "llm_core": true, "runtime_core": true, "execution_core": true, "gpu_execution_core": true}
	seen := map[string]bool{}
	versions := 0
	for _, contract := range registry.PanelContracts() {
		needs := false
		for _, extension := range contract.Extensions {
			needs = needs || canonical[extension]
		}
		if needs && contract.Panel == nil {
			t.Fatalf("missing panel: %s %s", contract.Type, contract.Payload.Min)
		}
		if contract.Panel == nil {
			continue
		}
		versions++
		renderer, ok := renderers[contract.Panel.ID]
		if !ok || renderer.Version != contract.Panel.Version {
			t.Fatal("missing renderer version", contract)
		}
		supported := false
		for _, action := range contract.Panel.Actions {
			supported = supported || action == renderer.Required
		}
		if !supported {
			t.Fatal("missing renderer action", contract)
		}
		seen[contract.Panel.ID] = true
	}
	if len(seen) != 8 || versions < 15 {
		t.Fatal("incomplete canonical inventory", len(seen), versions)
	}
	for id := range renderers {
		if !seen[id] {
			t.Fatal("unbound renderer", id)
		}
	}
	// Verify that exposing the inventory cannot mutate application authority.
	for _, entry := range registry.PanelContracts() {
		if entry.Panel != nil {
			entry.Panel.Actions[0] = "forged"
		}
	}
	for _, entry := range registry.PanelContracts() {
		if entry.Panel != nil && entry.Panel.Actions[0] == "forged" {
			t.Fatal("mutable descriptor inventory")
		}
	}
}
