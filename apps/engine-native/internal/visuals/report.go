package visuals

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"path"
	"sort"
	"strings"
)

// ReportPack adds verified already-rendered assets to an existing result pack.
// It performs no diagnostic parsing, statistics or image regeneration.
func ReportPack(ctx context.Context, base []byte, artifacts map[string][]byte) ([]byte, error) {
	if len(artifacts) == 0 || len(artifacts) > 4 {
		return nil, fmt.Errorf("VISUAL_REPORT_REQUIRES_1_TO_4_ARTIFACTS")
	}
	files := map[string][]byte{}
	if len(base) > 0 {
		if len(base) > MaxBundleBytes {
			return nil, fmt.Errorf("VISUAL_REPORT_BUDGET")
		}
		archive, e := zip.NewReader(bytes.NewReader(base), int64(len(base)))
		if e != nil {
			return nil, e
		}
		for _, f := range archive.File {
			if path.Clean(f.Name) != f.Name || strings.HasPrefix(f.Name, "/") || strings.Contains(f.Name, "..") || f.UncompressedSize64 > 64<<20 {
				return nil, fmt.Errorf("VISUAL_REPORT_BASE_INVALID")
			}
			r, e := f.Open()
			if e != nil {
				return nil, e
			}
			body, e := io.ReadAll(io.LimitReader(r, 64<<20+1))
			r.Close()
			if e != nil {
				return nil, e
			}
			name := f.Name
			if name == "index.html" {
				name = "analysis.html"
			}
			files[name] = body
		}
	}
	var index strings.Builder
	index.WriteString(`<!doctype html><html lang="en"><meta charset="utf-8"><title>Rendered visual report</title><body style="font-family:Arial,sans-serif;background:white;color:#172033"><h1>Rendered visual report</h1><p>Historical image bytes. See each manifest, spec and frozen view for exact provenance and limitations.</p>`)
	ids := []string{}
	for id := range artifacts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if !artifactID.MatchString(id) || Digest(artifacts[id]) != strings.TrimPrefix(id, "visual-") {
			return nil, fmt.Errorf("VISUAL_REPORT_ID_MISMATCH")
		}
		a, e := ReadArtifact(artifacts[id])
		if e != nil {
			return nil, e
		}
		prefix := "visuals/" + id + "/"
		for name, body := range a.Files {
			files[prefix+name] = body
		}
		index.WriteString(`<section><h2>` + html.EscapeString(a.Receipt.Spec.Presentation.Title) + `</h2><p>Figure ` + id + ` · view ` + a.Receipt.Spec.ViewSHA256 + `</p><p>` + html.EscapeString(a.View.Quality+" · "+strings.Join(a.View.Warnings, "; ")) + `</p><p><a href="` + prefix + `manifest.json">Manifest</a> · <a href="` + prefix + `table.html">Table</a></p>`)
		for page := 1; page <= a.Receipt.Pages; page++ {
			index.WriteString(fmt.Sprintf(`<img style="width:100%%;max-width:1600px;display:block" alt="Figure %s image %d" src="%simage-%02d.png">`, id, page, prefix, page))
		}
		index.WriteString(`</section>`)
		// HTML is reconstructed from safe frozen strings, never copied from imports.
		files[prefix+"table.html"] = []byte(StandaloneHTML(a.View, a.Receipt.Spec))
		files[prefix+"table.tsv"] = TSV(a.View)
	}
	index.WriteString(`</body></html>`)
	files["visuals/index.html"] = []byte(index.String())
	files["index.html"] = []byte(`<!doctype html><html><meta charset="utf-8"><title>Report pack</title><body><h1>Report pack</h1><a href="visuals/index.html">Rendered visual artifacts</a><p><a href="analysis.html">Analysis result appendix (when included)</a></p></body></html>`)
	names := []string{}
	total := 0
	for name, body := range files {
		names = append(names, name)
		total += len(body)
	}
	if total > MaxExpandedBytes {
		return nil, fmt.Errorf("VISUAL_REPORT_BUDGET: select fewer artifacts/results")
	}
	sort.Strings(names)
	var b bytes.Buffer
	writer := zip.NewWriter(&b)
	for _, name := range names {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		w, e := writer.Create(name)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(files[name]); e != nil {
			return nil, e
		}
		if b.Len() > MaxBundleBytes {
			return nil, fmt.Errorf("VISUAL_REPORT_BUDGET")
		}
	}
	if e := writer.Close(); e != nil {
		return nil, e
	}
	if b.Len() > MaxBundleBytes {
		return nil, fmt.Errorf("VISUAL_REPORT_BUDGET")
	}
	return b.Bytes(), nil
}
