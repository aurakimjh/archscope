package visuals_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/distribution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	v "github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
)

func sample(t *testing.T) (v.View, v.Spec, []byte, []byte) {
	t.Helper()
	view := v.View{Version: v.Version, Kind: "provider_table", Sources: []v.Source{{SHA256: strings.Repeat("a", 64), Type: "synthetic", Payload: "0.1.0"}}, Query: v.Query{Kind: "provider_table", Provider: "synthetic", Offset: 0, Limit: 25}, ScopeLabel: "synthetic selected rows; declared identity", QualityLabel: "observed / synthetic; lost/clock not assessed", TimeScope: "synthetic window [1,2)", Columns: []string{"이름", "정확한 값"}, Rows: [][]resultcontract.Cell{{{Text: "<script>literal</script>"}, {Text: "9007199254740993"}}, {{Text: "=HYPERLINK(\"synthetic\")"}, {Text: "1/3"}}}, Nodes: []v.Node{}, Total: 2, Unit: "seconds", Denominator: "0.35", Quality: "observed / synthetic", Warnings: []string{"not a causal inference"}}
	p := v.DefaultPresentation()
	p.Width = 1600
	p.Height = 1000
	p.FontSize = 20
	p.RowsPerPage = 5
	p.Title = "합성 보고서"
	spec, e := v.NewSpec(view, p, "")
	if e != nil {
		t.Fatal(e)
	}
	font, e := distribution.Assets.ReadFile("assets/fonts/NotoSansKR.ttf")
	if e != nil {
		t.Fatal(e)
	}
	svg, e := v.SVG(view, spec, 0, font)
	if e != nil {
		t.Fatal(e)
	}
	picture := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	draw.Draw(picture, picture.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	var pngBody bytes.Buffer
	if e = png.Encode(&pngBody, picture); e != nil {
		t.Fatal(e)
	}
	return view, spec, svg, pngBody.Bytes()
}
func artifact(t *testing.T) ([]byte, v.View, v.Spec) {
	t.Helper()
	view, spec, svg, png := sample(t)
	license, _ := distribution.Assets.ReadFile("assets/fonts/OFL.txt")
	body, e := v.BuildArtifact(context.Background(), view, spec, []v.Page{{PNG: png, SVG: svg}}, "test: synthetic PNG format only; no native renderer claim", license)
	if e != nil {
		t.Fatal(e)
	}
	return body, view, spec
}
func rewrite(t *testing.T, body []byte, modify func(map[string][]byte)) []byte {
	t.Helper()
	r, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		t.Fatal(e)
	}
	files := map[string][]byte{}
	for _, f := range r.File {
		in, _ := f.Open()
		files[f.Name], _ = io.ReadAll(in)
		in.Close()
	}
	modify(files)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range files {
		f, _ := w.Create(name)
		f.Write(data)
	}
	w.Close()
	return b.Bytes()
}
func TestPinnedViewArtifactHashSafetyAndHistory(t *testing.T) {
	body, view, spec := artifact(t)
	a, e := v.ReadArtifact(body)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(a.Files["image-01.svg"], []byte("&lt;script&gt;literal")) || strings.Contains(v.TableHTML(view, spec), "<script>literal") {
		t.Fatal("unsafe report text")
	}
	tsv := string(v.TSV(view))
	if !strings.Contains(tsv, "'9007199254740993") || !strings.Contains(tsv, "'=HYPERLINK") || !strings.Contains(tsv, "'1/3") {
		t.Fatal("spreadsheet text protection", tsv)
	}
	if !strings.Contains(v.TableHTML(view, spec), "&#39;=HYPERLINK") {
		t.Fatal("HTML formula text protection missing")
	}
	corrupted := rewrite(t, body, func(files map[string][]byte) { files["view.json"] = []byte(`{}`) })
	if _, e = v.ReadArtifact(corrupted); e == nil {
		t.Fatal("modified view accepted")
	}
	historical := rewrite(t, body, func(files map[string][]byte) {
		var receipt v.Receipt
		json.Unmarshal(files["manifest.json"], &receipt)
		receipt.Spec.TemplateVersion = "0.9.0"
		files["spec.json"], _ = json.Marshal(receipt.Spec)
		receipt.Files["spec.json"] = v.Digest(files["spec.json"])
		files["manifest.json"], _ = json.Marshal(receipt)
	})
	old, e := v.ReadArtifact(historical)
	if e != nil {
		t.Fatal(e)
	}
	if old.Receipt.Spec.Reproducible() || old.Receipt.Spec.Validate(old.View, false) == nil || !bytes.Equal(old.Files["image-01.png"], a.Files["image-01.png"]) {
		t.Fatal("old dependency rewrote/authorized snapshot")
	}
	store, e := v.OpenStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	saved, e := store.Save(body)
	if e != nil {
		t.Fatal(e)
	}
	again, e := store.Save(body)
	if e != nil || again.ID != saved.ID {
		t.Fatal("same bytes duplicated", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = store.SaveContext(ctx, historical); e == nil {
		t.Fatal("canceled save published")
	}
	got, _, e := store.Read(saved.ID)
	if e != nil || !bytes.Equal(got, body) {
		t.Fatal("snapshot bytes changed", e)
	}
	pack, e := v.ReportPack(context.Background(), nil, map[string][]byte{saved.ID: body})
	if e != nil {
		t.Fatal(e)
	}
	if len(pack) == 0 {
		t.Fatal("empty visual report")
	}
	if _, _, e = store.Read("../../escape"); e == nil {
		t.Fatal("untrusted artifact path")
	}
}
func TestVisualSVGExternalReferencesAndDimensionRefusals(t *testing.T) {
	_, spec, svg, png := sample(t)
	for _, bad := range [][]byte{bytes.Replace(svg, []byte("<defs>"), []byte(`<script>bad</script><defs>`), 1), bytes.Replace(svg, []byte(`<rect width=`), []byte(`<rect onload="bad" width=`), 1), bytes.Replace(svg, []byte(`width="1600"`), []byte(`width="3200"`), 1), append(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)...)} {
		if v.ValidateSVG(bad, spec) == nil {
			t.Fatal("unsafe SVG accepted")
		}
	}
	if v.ValidatePNG(png, 3200, 1000) == nil {
		t.Fatal("wrong dimensions accepted")
	}
	spec.Presentation.Width = 100000
	if spec.Presentation.Validate() == nil {
		t.Fatal("unbounded canvas accepted")
	}
}
func TestVisualFixtureForRealBrowserRaster(t *testing.T) {
	directory := os.Getenv("ARCHSCOPE_VISUAL_EVIDENCE_DIR")
	if directory == "" {
		t.Skip("fixture emission only when explicitly requested")
	}
	view, spec, svg, _ := sample(t)
	if e := os.MkdirAll(directory, 0700); e != nil {
		t.Fatal(e)
	}
	for name, value := range map[string]any{"view.json": view, "spec.json": spec, "render-input.json": map[string]any{"version": v.RendererVersion, "svg": []string{string(svg)}, "width": spec.Presentation.Width, "height": spec.Presentation.Height, "font_url": "/offline-assets/fonts/NotoSansKR.ttf", "font_sha256": v.FontSHA256}} {
		body, _ := json.Marshal(value)
		if e := os.WriteFile(filepath.Join(directory, name), body, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(directory, "image.svg"), svg, 0600); e != nil {
		t.Fatal(e)
	}
	// Additional synthetic templates exercise layout and split-image selection.
	font, _ := distribution.Assets.ReadFile("assets/fonts/NotoSansKR.ttf")
	cases := map[string]v.View{}
	long := view
	long.Rows = nil
	long.Total = 12
	for i := 0; i < 12; i++ {
		long.Rows = append(long.Rows, []resultcontract.Cell{{Text: fmt.Sprintf("합성 행 %02d 긴 이름은 표에 보존", i+1)}, {Text: fmt.Sprintf("%d/3", i+1)}})
	}
	cases["split"] = long
	flame := view
	flame.Kind = "profile_flamegraph"
	flame.Query.Kind = flame.Kind
	flame.Columns = []string{"Frame", "Inclusive", "Self"}
	flame.Rows = [][]resultcontract.Cell{{{Text: "전체"}, {Text: "10"}, {Text: "0"}}, {{Text: "분석"}, {Text: "6"}, {Text: "2"}}, {{Text: "IO"}, {Text: "4"}, {Text: "4"}}}
	flame.Total = 3
	flame.Nodes = []v.Node{{Name: "전체", Value: "10", Self: "0", X: 0, Width: 1, Depth: 0}, {Name: "분석", Value: "6", Self: "2", X: 0, Width: .6, Depth: 1}, {Name: "IO", Value: "4", Self: "4", X: .6, Width: .4, Depth: 1}}
	cases["flame"] = flame
	diff := view
	diff.Kind = "profile_diff"
	diff.Query.Kind = diff.Kind
	diff.Sources = append(append([]v.Source(nil), view.Sources...), v.Source{SHA256: strings.Repeat("b", 64), Type: "synthetic", Payload: "0.1.0"})
	diff.Columns = []string{"Frame", "Before", "After", "Delta", "Unit", "Reference"}
	diff.Rows = [][]resultcontract.Cell{{{Text: "감소"}, {Text: "10"}, {Text: "4"}, {Text: "-6"}, {Text: "seconds"}, {Text: "frame-a"}}, {{Text: "증가"}, {Text: "2"}, {Text: "5"}, {Text: "3"}, {Text: "seconds"}, {Text: "frame-b"}}}
	cases["diff"] = diff
	for name, selected := range cases {
		p := spec.Presentation
		p.Title = "합성 " + name
		p.Transparent = name == "flame"
		s, err := v.NewSpec(selected, p, "")
		if err != nil {
			t.Fatal(err)
		}
		svgs := []string{}
		for page := 0; page < v.Pages(selected, p); page++ {
			data, err := v.SVG(selected, s, page, font)
			if err != nil {
				t.Fatal(err)
			}
			svgs = append(svgs, string(data))
		}
		body, _ := json.Marshal(map[string]any{"version": v.RendererVersion, "svg": svgs, "width": p.Width, "height": p.Height, "font_url": "/offline-assets/fonts/NotoSansKR.ttf", "font_sha256": v.FontSHA256})
		if err := os.WriteFile(filepath.Join(directory, name+"-input.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Optional browser-generated PNG is validated and preserved with the same spec.
	if pngBody, e := os.ReadFile(filepath.Join(directory, "browser.png")); e == nil {
		license, _ := distribution.Assets.ReadFile("assets/fonts/OFL.txt")
		body, e := v.BuildArtifact(context.Background(), view, spec, []v.Page{{PNG: pngBody, SVG: svg}}, "Chromium browser renderer harness; native WebView not observed", license)
		if e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(directory, "browser-artifact.zip"), body, 0600)
	}
}

func TestVisualDeepSelectedPageKeepsVisibleVerticalOrigin(t *testing.T) {
	view, _, _, _ := sample(t)
	view.Kind = "profile_flamegraph"
	view.Query.Kind = view.Kind
	view.Nodes = []v.Node{{Name: "deep selected", Value: "1", Self: "1", X: 0, Width: 1, Depth: 500}}
	p := v.DefaultPresentation()
	spec, e := v.NewSpec(view, p, "")
	if e != nil {
		t.Fatal(e)
	}
	font, _ := distribution.Assets.ReadFile("assets/fonts/NotoSansKR.ttf")
	svg, e := v.SVG(view, spec, 0, font)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(svg, []byte(`y="205"`)) || !bytes.Contains(svg, []byte("Depths 501–501 of 501")) {
		t.Fatal("deep query page rendered outside fixed canvas")
	}
}

func TestR1404SpecRejectsTooManyImages(t *testing.T) {
	view, spec, _, _ := sample(t)
	row := view.Rows[0]
	view.Rows = nil
	for i := 0; i < 26; i++ {
		view.Rows = append(view.Rows, row)
	}
	view.Total = 26
	if _, e := v.NewSpec(view, spec.Presentation, ""); e == nil || !strings.Contains(e.Error(), "required_images=6") {
		t.Fatal("late-only page refusal", e)
	}
	view.Rows = view.Rows[:25]
	view.Total = 25
	s, e := v.NewSpec(view, spec.Presentation, "")
	if e != nil || v.Pages(view, s.Presentation) != 5 {
		t.Fatal(e)
	}
}

func TestR1405RehashedSVGStillBindsToView(t *testing.T) {
	view, spec, _, png := sample(t)
	font, e := distribution.Assets.ReadFile("assets/fonts/NotoSansKR.ttf")
	if e != nil {
		t.Fatal(e)
	}
	svg, e := v.SVG(view, spec, 0, font)
	if e != nil {
		t.Fatal(e)
	}
	original, e := v.BuildArtifact(context.Background(), view, spec, []v.Page{{PNG: png, SVG: svg}}, "synthetic", v.FontLicense)
	if e != nil {
		t.Fatal(e)
	}
	changed := rewrite(t, original, func(files map[string][]byte) {
		files["image-01.svg"] = bytes.Replace(files["image-01.svg"], []byte("#ffffff"), []byte("#000000"), 1)
		var receipt v.Receipt
		json.Unmarshal(files["manifest.json"], &receipt)
		receipt.Files["image-01.svg"] = v.Digest(files["image-01.svg"])
		files["manifest.json"], _ = json.Marshal(receipt)
	})
	if _, e = v.ReadArtifact(changed); e == nil || !strings.Contains(e.Error(), "VISUAL_SVG_VIEW_MISMATCH") {
		t.Fatal("rehashed SVG accepted", e)
	}
}
