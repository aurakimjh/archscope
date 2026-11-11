package distribution

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed assets
var Assets embed.FS

type FixtureCase struct {
	Importer string            `json:"importer"`
	Input    string            `json:"input"`
	Options  json.RawMessage   `json:"options"`
	Summary  map[string]string `json:"summary"`
}
type FixtureManifest struct {
	Version   string            `json:"version"`
	Synthetic bool              `json:"synthetic"`
	Files     map[string]string `json:"files"`
	Cases     []FixtureCase     `json:"cases"`
}

func ResourceFiles() (map[string][]byte, error) {
	out := map[string][]byte{}
	e := fs.WalkDir(Assets, "assets", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			b, e := Assets.ReadFile(p)
			if e != nil {
				return e
			}
			out[strings.TrimPrefix(p, "assets/")] = b
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	var font struct {
		FontHash    string `json:"font_sha256"`
		FontBytes   int    `json:"font_bytes"`
		LicenseHash string `json:"license_sha256"`
	}
	if e = json.Unmarshal(out["fonts/manifest.json"], &font); e != nil || font.FontBytes != len(out["fonts/NotoSansKR.ttf"]) || font.FontHash != packs.Hash(out["fonts/NotoSansKR.ttf"]) || font.LicenseHash != packs.Hash(out["fonts/OFL.txt"]) {
		return nil, fmt.Errorf("OFFLINE_FONT_INVENTORY: original font/license hash mismatch")
	}
	var notices struct {
		Version      string `json:"version"`
		Dependencies []struct {
			Notices []struct {
				File string `json:"file"`
				Hash string `json:"sha256"`
			} `json:"notices"`
		} `json:"dependencies"`
	}
	if e = json.Unmarshal(out["licenses/manifest.json"], &notices); e != nil || notices.Version != "1.0.0" || len(notices.Dependencies) == 0 {
		return nil, fmt.Errorf("OFFLINE_LICENSE_INVENTORY: missing notices")
	}
	for _, dependency := range notices.Dependencies {
		if len(dependency.Notices) == 0 {
			return nil, fmt.Errorf("OFFLINE_LICENSE_INVENTORY: missing dependency notice")
		}
		for _, n := range dependency.Notices {
			body, ok := out["licenses/"+n.File]
			if !ok || !validPath(n.File) || packs.Hash(body) != n.Hash {
				return nil, fmt.Errorf("OFFLINE_LICENSE_INVENTORY: notice mismatch")
			}
		}
	}
	return out, nil
}
func ReadFixtures(body []byte, required []string) (FixtureManifest, map[string][]byte, error) {
	var m FixtureManifest
	want, _ := Assets.ReadFile("assets/fixtures.sha256")
	if packs.Hash(body) != strings.TrimSpace(string(want)) {
		return m, nil, fmt.Errorf("OFFLINE_FIXTURE_HASH: mandatory release fixture bundle mismatch")
	}
	z, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		return m, nil, e
	}
	files := map[string][]byte{}
	if len(z.File) > 128 {
		return m, nil, fmt.Errorf("fixture entry budget")
	}
	total := 0
	for _, f := range z.File {
		if !validPath(f.Name) || f.Mode()&os.ModeSymlink != 0 || f.UncompressedSize64 > 8<<20 || files[f.Name] != nil {
			return m, nil, fmt.Errorf("invalid fixture entry")
		}
		r, e := f.Open()
		if e != nil {
			return m, nil, e
		}
		b, e := io.ReadAll(io.LimitReader(r, 8<<20+1))
		r.Close()
		total += len(b)
		if e != nil || len(b) > 8<<20 || total > 32<<20 {
			return m, nil, fmt.Errorf("fixture budget/read failure")
		}
		files[f.Name] = b
	}
	if e = packs.Strict(files["manifest.json"], &m); e != nil {
		return m, nil, e
	}
	if m.Version != "1.0.0" || !m.Synthetic || len(files) != len(m.Files)+1 {
		return m, nil, fmt.Errorf("fixture manifest mismatch")
	}
	for n, h := range m.Files {
		if b, ok := files[n]; !ok || packs.Hash(b) != h {
			return m, nil, fmt.Errorf("fixture missing/hash mismatch: %s", n)
		}
	}
	covered := map[string]bool{}
	for _, c := range m.Cases {
		if _, ok := m.Files[c.Input]; !ok || covered[c.Importer] {
			return m, nil, fmt.Errorf("invalid fixture case")
		}
		covered[c.Importer] = true
	}
	for _, id := range required {
		if !covered[id] {
			return m, nil, fmt.Errorf("OFFLINE_FIXTURE_COVERAGE: %s", id)
		}
	}
	return m, files, nil
}
func validPath(p string) bool {
	return p != "" && fs.ValidPath(p) && path.Clean(p) == p && !strings.ContainsAny(p, "\\:\x00") && len(p) < 512
}

// CheckFixtures is a mandatory executable release gate. It uses only embedded
// standalone inputs, not a sibling asset checkout or a user's pack state.
func CheckFixtures(ctx context.Context) ([]string, error) {
	if _, e := ResourceFiles(); e != nil {
		return nil, e
	}
	root, e := os.MkdirTemp("", "archscope-conformance-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(root)
	c, r, e := catalog.Builtin()
	if e != nil {
		return nil, e
	}
	p, e := catalog.OpenPackSession(ctx, root, c)
	if e != nil {
		return nil, e
	}
	defer p.Manager.Close()
	snapshot, e := p.Provider("")
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for _, d := range snapshot.Catalog.List() {
		ids = append(ids, d.ID)
	}
	b, e := Assets.ReadFile("assets/fixtures.zip")
	if e != nil {
		return nil, e
	}
	m, files, e := ReadFixtures(b, ids)
	if e != nil {
		return nil, e
	}
	service, e := analysis.New(filepath.Join(root, "jobs"), c, r)
	if e != nil {
		return nil, e
	}
	defer service.Close()
	service.SetCatalogProvider(p.Provider)
	for i, f := range m.Cases {
		input := filepath.Join(root, fmt.Sprintf("input-%d%s", i, path.Ext(f.Input)))
		if e = os.WriteFile(input, files[f.Input], 0600); e != nil {
			return nil, e
		}
		job, e := service.Start(analysis.Request{Path: input, Importer: f.Importer, Options: f.Options})
		if e != nil {
			return nil, e
		}
		job, e = service.Wait(ctx, job.ID, job.Generation)
		if e != nil {
			return nil, e
		}
		if job.State != "completed" {
			return nil, fmt.Errorf("fixture %s: %s %+v", f.Importer, job.State, job.Problem)
		}
		_, result, e := service.Read(job.ResultHandle)
		if e != nil {
			return nil, e
		}
		for k, w := range f.Summary {
			n, e := resultcontract.ExactNumber(result.Summary[k], "json_number")
			if e != nil || n.RatString() != w {
				return nil, fmt.Errorf("fixture summary %s/%s", f.Importer, k)
			}
		}
	}
	return ids, nil
}
