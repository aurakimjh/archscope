package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type MigratedResult struct {
	OldHandle string `json:"old_handle"`
	Handle    string `json:"handle"`
	SHA256    string `json:"sha256"`
}
type MigrationReport struct {
	Version            string           `json:"version"`
	Results            []MigratedResult `json:"results"`
	OriginalsPreserved bool             `json:"originals_preserved"`
}

// OpenDurable imports the former OS-cache store under exclusive locks. Each
// snapshot is validated before publish; retries deduplicate already copied bytes.
// A completion marker is written last; source files are deliberately retained.
func OpenDurable(ctx context.Context, legacy, durable string, c *ingestion.Catalog, r *resultcontract.Registry) (*Service, error) {
	if legacy == durable {
		return New(durable, c, r)
	}
	dest, e := New(durable, c, r)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*Service, error) { dest.Close(); return nil, e }
	info, e := os.Lstat(legacy)
	if errors.Is(e, os.ErrNotExist) {
		return dest, nil
	}
	if e != nil {
		return fail(e)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fail(ingestion.Problem("IMPORT_MIGRATION_UNSAFE", "legacy cache root must be a real directory"))
	}
	// The marker is a completed, one-time handoff. Never repopulate cache data
	// explicitly removed after migration. Saved cases recover their own snapshots.
	marker := filepath.Join(durable, "cache-migration.json")
	if info, e := os.Lstat(marker); e == nil {
		if !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return fail(ingestion.Problem("IMPORT_MIGRATION_INVALID", "invalid migration marker; preserve legacy data and recover explicitly"))
		}
		f, e := os.Open(marker)
		if e != nil {
			return fail(e)
		}
		body, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		f.Close()
		if e != nil {
			return fail(e)
		}
		var report MigrationReport
		if e = wirejson.Decode(body, &report); e != nil {
			return fail(e)
		}
		if report.Version != "1.0.0" || !report.OriginalsPreserved || len(report.Results) > maxStoredResults {
			return fail(ingestion.Problem("IMPORT_MIGRATION_INVALID", "unsupported migration marker"))
		}
		for _, item := range report.Results {
			if !handlePattern.MatchString(item.OldHandle) || !handlePattern.MatchString(item.Handle) || len(item.SHA256) != 64 {
				return fail(ingestion.Problem("IMPORT_MIGRATION_INVALID", "invalid migration reference"))
			}
		}
		return dest, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return fail(e)
	}
	for _, p := range []string{"results", "staging"} {
		i, e := os.Lstat(filepath.Join(legacy, p))
		if e != nil {
			return fail(e)
		}
		if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return fail(ingestion.Problem("IMPORT_MIGRATION_UNSAFE", "legacy store contains a linked directory"))
		}
	}
	source, e := New(legacy, c, r)
	if e != nil {
		return fail(e)
	}
	defer source.Close()
	source.storeMu.Lock()
	keys := []string{}
	for key := range source.stored {
		keys = append(keys, key)
	}
	source.storeMu.Unlock()
	sort.Strings(keys)
	report := MigrationReport{Version: "1.0.0", Results: []MigratedResult{}, OriginalsPreserved: true}
	for _, key := range keys {
		if e = ctx.Err(); e != nil {
			return fail(e)
		}
		body, _, e := source.Read(key)
		if e != nil {
			return fail(e)
		}
		view, e := dest.LoadSnapshot(ctx, body)
		if e != nil {
			return fail(e)
		}
		source.storeMu.Lock()
		digest := source.stored[key].hash
		source.storeMu.Unlock()
		report.Results = append(report.Results, MigratedResult{key, view.Handle, digest})
	}
	body, e := json.Marshal(report)
	if e != nil {
		return fail(e)
	}
	if e = AtomicExport(marker, body); e != nil {
		return fail(e)
	}
	return dest, nil
}
