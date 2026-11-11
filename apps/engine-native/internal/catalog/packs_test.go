package catalog

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJobCatalogAndPackProvenanceRemainPinnedAfterDeactivation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base, registry, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	session, e := OpenPackSession(ctx, root, base)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Manager.Close()
	old, e := session.Provider("")
	if e != nil {
		t.Fatal(e)
	}
	rawSnapshot, e := session.Manager.Snapshot(ctx, "")
	if e != nil {
		t.Fatal(e)
	}
	duplicate := rawSnapshot.Mappings[0].Copy()
	duplicate.ID = "data.ambiguous"
	duplicate.Aliases = nil
	rawSnapshot.Mappings = append(rawSnapshot.Mappings, duplicate)
	old.Catalog, e = WithPacks(base, rawSnapshot)
	if e != nil {
		t.Fatal(e)
	}
	input, _ := packs.Assets.ReadFile("assets/input.jsonl")
	path := filepath.Join(t.TempDir(), "input.jsonl")
	os.WriteFile(path, input, 0600)
	service, e := analysis.New(filepath.Join(root, "jobs"), base, registry)
	if e != nil {
		t.Fatal(e)
	}
	defer service.Close()
	service.SetCatalogProvider(func(scope string) (analysis.CatalogSnapshot, error) {
		current, e := session.Provider(scope)
		if current.Revision == old.Revision {
			current.Catalog = old.Catalog
		}
		return current, e
	})
	// Auto-detect pauses for explicit confirmation; deactivation occurs while
	// the job still owns a catalog, before selection and record-schema validation.
	job, e := service.Start(analysis.Request{Path: path, Importer: "auto", Options: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	for {
		job, e = service.Get(job.ID, job.Generation)
		if e != nil {
			t.Fatal(e)
		}
		if job.State == "needs_selection" {
			break
		}
		if job.State == "failed" || job.State == "completed" {
			t.Fatalf("%+v", job)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	state, _ := session.Manager.State()
	if e = session.Manager.Deactivate(ctx, state.Revision, "application", old.Packs[0].ID); e != nil {
		t.Fatal(e)
	}
	if job.State == "needs_selection" {
		if _, e = service.Select(job.ID, job.Generation, "synthetic-metrics"); e != nil {
			t.Fatal(e)
		}
	}
	job, e = service.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatalf("%+v %v", job, e)
	}
	if len(job.Packs) != 1 || job.Packs[0].SHA256 != old.Packs[0].SHA256 {
		t.Fatal("pin changed")
	}
	job.Packs[0].SHA256 = "mutated"
	fresh, _ := service.Get(job.ID, job.Generation)
	if fresh.Packs[0].SHA256 == "mutated" {
		t.Fatal("snapshot alias")
	}
	_, result, e := service.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(result.Tables["samples"])
	if string(raw) == "null" {
		t.Fatal("missing samples")
	}
	var rows []map[string]string
	json.Unmarshal(raw, &rows)
	if len(rows) != 2 || rows[0]["value"] != "0.0125" || rows[1]["value"] != "0" {
		t.Fatal(rows)
	}
	if result.Metadata.Extra["effective_pack"] == nil {
		t.Fatal("missing provenance")
	}
	current, e := session.Provider("")
	if e != nil {
		t.Fatal(e)
	}
	if len(current.Packs) != 0 {
		t.Fatal("new provider still active")
	}
	if _, e = current.Catalog.Resolve("synthetic-metrics", nil, "test"); e == nil {
		t.Fatal("new catalog retained deactivated mapping")
	}
	if _, e = old.Catalog.Resolve("synthetic-metrics", nil, "test"); e != nil {
		t.Fatal("old catalog changed")
	}
	session.Manager.Close()
	reopened, e := OpenPackSession(ctx, root, base)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Manager.Close()
	snapshot, e := reopened.Provider("")
	if e != nil || len(snapshot.Packs) != 0 {
		t.Fatal("bootstrap overwrote owner deactivation", e)
	}
}
func TestLocalAliasConflictRequiresChoiceWithoutMutatingBase(t *testing.T) {
	base, _, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	b, _ := packs.Assets.ReadFile("assets/mapping.json")
	var m packs.Mapping
	packs.Strict(b, &m)
	m.ID = "local.metrics"
	m.Aliases = []string{"tomcat"}
	m.Pack = packs.Ref{ID: "local.preset", Version: "1.0.0", SHA256: strings.Repeat("a", 64), Scope: "user", Origin: "locally_authored"}
	snapshot := packs.Snapshot{Mappings: []packs.Mapping{m}}
	if _, e = WithPacks(base, snapshot); e == nil {
		t.Fatal("local automatic precedence")
	}
	snapshot.Choices = map[string]string{"tomcat": "server.tomcat"}
	compiled, e := WithPacks(base, snapshot)
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := compiled.Resolve("tomcat", nil, "test")
	if e != nil || resolved.Descriptor.ID != "server.tomcat" {
		t.Fatal(e)
	}
	snapshot.Choices["tomcat"] = "local.metrics"
	compiled, e = WithPacks(base, snapshot)
	if e != nil {
		t.Fatal(e)
	}
	resolved, e = compiled.Resolve("tomcat", nil, "test")
	if e != nil || resolved.Descriptor.ID != "local.metrics" {
		t.Fatal(e)
	}
	resolved, _ = base.Resolve("tomcat", nil, "test")
	if resolved.Descriptor.ID != "server.tomcat" {
		t.Fatal("base mutated")
	}
	m.ID = "server.tomcat"
	snapshot.Mappings = []packs.Mapping{m}
	if _, e = WithPacks(base, snapshot); e == nil {
		t.Fatal("canonical shadow allowed")
	}
}
