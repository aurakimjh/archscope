package visuals

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Page struct {
	PNG []byte
	SVG []byte
}
type Receipt struct {
	TableProtection *Protection       `json:"table_protection,omitempty"`
	Version         string            `json:"version"`
	CreatedAt       string            `json:"created_at"`
	Spec            Spec              `json:"spec"`
	Files           map[string]string `json:"files"`
	Pages           int               `json:"pages"`
	Environment     string            `json:"environment"`
	Reproducibility string            `json:"reproducibility"`
}
type Artifact struct {
	Receipt Receipt
	View    View
	Files   map[string][]byte
}
type Descriptor struct {
	TableProtection Protection `json:"table_protection"`
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Pages           int        `json:"pages"`
	Spec            Spec       `json:"spec"`
	Reproducible    bool       `json:"reproducible"`
	CreatedAt       string     `json:"created_at"`
}

func ValidatePNG(body []byte, width, height int) error {
	if len(body) > MaxPNGBytes {
		return fmt.Errorf("VISUAL_PNG_BUDGET")
	}
	config, e := png.DecodeConfig(bytes.NewReader(body))
	if e != nil {
		return fmt.Errorf("VISUAL_PNG_INVALID: %w", e)
	}
	if config.Width != width || config.Height != height || width*height > 8_000_000 {
		return fmt.Errorf("VISUAL_PNG_DIMENSION_MISMATCH")
	}
	if _, e = png.Decode(bytes.NewReader(body)); e != nil {
		return fmt.Errorf("VISUAL_PNG_INVALID: %w", e)
	}
	return nil
}

var svgColor = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|hsl\([0-9]{1,3},[0-9]{1,3}%,[0-9]{1,3}%\)|none)$`)

func ValidateSVG(body []byte, spec Spec) error {
	if len(body) > 20<<20 {
		return fmt.Errorf("VISUAL_SVG_BUDGET")
	}
	d := xml.NewDecoder(bytes.NewReader(body))
	depth, elements := 0, 0
	inStyle := false
	style := ""
	root := false
	allowed := map[string]bool{"svg": true, "metadata": true, "title": true, "defs": true, "style": true, "text": true, "g": true, "rect": true, "line": true}
	attributes := map[string]bool{"xmlns": true, "width": true, "height": true, "viewBox": true, "role": true, "x": true, "y": true, "font-size": true, "fill": true, "stroke": true, "x1": true, "x2": true, "y1": true, "y2": true}
	for {
		token, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 && root {
				return fmt.Errorf("VISUAL_SVG_MULTIPLE_ROOTS")
			}
			depth++
			elements++
			if depth > 16 || elements > 5000 || !allowed[t.Name.Local] || t.Name.Space != "http://www.w3.org/2000/svg" {
				return fmt.Errorf("VISUAL_SVG_UNSAFE")
			}
			if !root {
				if t.Name.Local != "svg" {
					return fmt.Errorf("VISUAL_SVG_INVALID")
				}
				root = true
				var w, h string
				for _, a := range t.Attr {
					if a.Name.Local == "width" {
						w = a.Value
					}
					if a.Name.Local == "height" {
						h = a.Value
					}
				}
				if w != strconv.Itoa(spec.Presentation.Width) || h != strconv.Itoa(spec.Presentation.Height) {
					return fmt.Errorf("VISUAL_SVG_DIMENSION_MISMATCH")
				}
			}
			for _, a := range t.Attr {
				if !attributes[a.Name.Local] || a.Name.Space != "" {
					return fmt.Errorf("VISUAL_SVG_UNSAFE_ATTRIBUTE")
				}
				switch a.Name.Local {
				case "xmlns":
					if a.Value != "http://www.w3.org/2000/svg" {
						return fmt.Errorf("VISUAL_SVG_NAMESPACE")
					}
				case "fill", "stroke":
					if !svgColor.MatchString(a.Value) {
						return fmt.Errorf("VISUAL_SVG_UNSAFE_COLOR")
					}
				case "role":
					if a.Value != "img" {
						return fmt.Errorf("VISUAL_SVG_ROLE")
					}
				case "viewBox":
					for _, part := range strings.Fields(a.Value) {
						n, e := strconv.ParseFloat(part, 64)
						if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 10000 {
							return fmt.Errorf("VISUAL_SVG_COORDINATE")
						}
					}
				default:
					n, e := strconv.ParseFloat(a.Value, 64)
					if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 10000 {
						return fmt.Errorf("VISUAL_SVG_COORDINATE")
					}
				}
			}
			if t.Name.Local == "style" {
				if inStyle || style != "" {
					return fmt.Errorf("VISUAL_SVG_STYLE")
				}
				inStyle = true
			}
		case xml.EndElement:
			depth--
			if t.Name.Local == "style" {
				inStyle = false
			}
		case xml.CharData:
			if inStyle {
				style += string(t)
			}
		case xml.Directive, xml.ProcInst:
			return fmt.Errorf("VISUAL_SVG_DIRECTIVE")
		}
	}
	prefix := "@font-face{font-family:ReportPinned;src:url(data:font/ttf;base64,"
	suffix := ") format('truetype')}text{font-family:ReportPinned,'Noto Sans KR',sans-serif;fill:#172033}"
	if !root || depth != 0 || !strings.HasPrefix(style, prefix) || !strings.HasSuffix(style, suffix) {
		return fmt.Errorf("VISUAL_SVG_FONT_STYLE")
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(style, prefix), suffix)
	font, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || Digest(font) != spec.FontSHA256 {
		return fmt.Errorf("VISUAL_SVG_FONT_MISMATCH")
	}
	return nil
}

// validateCanonicalSVG binds supported generator output to the frozen view. It
// does not authenticate the author or prove raster/SVG pixel equivalence.
func validateCanonicalSVG(body []byte, view View, spec Spec, page int) error {
	if e := ValidateSVG(body, spec); e != nil {
		return e
	}
	if !spec.Reproducible() {
		return nil
	}
	marker := []byte("data:font/ttf;base64,")
	start := bytes.Index(body, marker)
	if start < 0 {
		return fmt.Errorf("VISUAL_SVG_FONT_STYLE")
	}
	start += len(marker)
	end := bytes.IndexByte(body[start:], ')')
	if end < 0 {
		return fmt.Errorf("VISUAL_SVG_FONT_STYLE")
	}
	font, e := base64.StdEncoding.DecodeString(string(body[start : start+end]))
	if e != nil {
		return e
	}
	expected, e := SVG(view, spec, page, font)
	if e != nil {
		return e
	}
	if !bytes.Equal(body, expected) {
		return fmt.Errorf("VISUAL_SVG_VIEW_MISMATCH: supported SVG must match deterministic frozen view output")
	}
	return nil
}

func BuildArtifact(ctx context.Context, view View, spec Spec, pages []Page, environment string, license []byte) ([]byte, error) {
	if e := spec.Validate(view, false); e != nil {
		return nil, e
	}
	if len(pages) != Pages(view, spec.Presentation) || len(pages) > 5 || len(environment) > 1024 {
		return nil, fmt.Errorf("VISUAL_RENDER_INCOMPLETE")
	}
	files := map[string][]byte{}
	for i, page := range pages {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if e := ValidatePNG(page.PNG, spec.Presentation.Width, spec.Presentation.Height); e != nil {
			return nil, e
		}
		if e := validateCanonicalSVG(page.SVG, view, spec, i); e != nil {
			return nil, e
		}
		files[fmt.Sprintf("image-%02d.png", i+1)] = page.PNG
		files[fmt.Sprintf("image-%02d.svg", i+1)] = page.SVG
	}
	files["view.json"], _ = json.Marshal(view)
	files["spec.json"], _ = json.Marshal(spec)
	files["table.tsv"] = TSV(view)
	files["table.html"] = []byte(StandaloneHTML(view, spec))
	files["font-OFL.txt"] = license
	protection := TableProtection(view)
	receipt := Receipt{TableProtection: &protection, Version: Version, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Spec: spec, Files: map[string]string{}, Pages: len(pages), Environment: environment, Reproducibility: "Integrity hashes do not authenticate authorship; supported SVG is regenerated from the frozen view; PNG/SVG pixel equivalence is not verified. Exact numeric strings and selected layout inputs are pinned. Raster byte identity is only comparable within the same fixed WebView/platform environment."}
	expanded := 0
	for name, body := range files {
		receipt.Files[name] = Digest(body)
		expanded += len(body)
	}
	if expanded > MaxExpandedBytes {
		return nil, fmt.Errorf("VISUAL_ARTIFACT_BUDGET: expanded bytes %d exceed %d", expanded, MaxExpandedBytes)
	}
	files["manifest.json"], _ = json.Marshal(receipt)
	var b bytes.Buffer
	writer := zip.NewWriter(&b)
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
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
			return nil, fmt.Errorf("VISUAL_ARTIFACT_BUDGET")
		}
	}
	if e := writer.Close(); e != nil {
		return nil, e
	}
	if b.Len() > MaxBundleBytes {
		return nil, fmt.Errorf("VISUAL_ARTIFACT_BUDGET")
	}
	return b.Bytes(), nil
}
func ReadArtifact(body []byte) (Artifact, error) {
	out := Artifact{Files: map[string][]byte{}}
	if len(body) > MaxBundleBytes {
		return out, fmt.Errorf("VISUAL_ARTIFACT_BUDGET")
	}
	archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		return out, e
	}
	if len(archive.File) < 6 || len(archive.File) > 16 {
		return out, fmt.Errorf("VISUAL_ARTIFACT_FILES")
	}
	total := uint64(0)
	allowed := regexp.MustCompile(`^(manifest|spec|view)\.json$|^table\.(tsv|html)$|^font-OFL\.txt$|^image-0[1-5]\.(png|svg)$`)
	for _, f := range archive.File {
		if !allowed.MatchString(f.Name) || out.Files[f.Name] != nil || f.Mode()&os.ModeSymlink != 0 {
			return out, fmt.Errorf("VISUAL_ARTIFACT_PATH")
		}
		total += f.UncompressedSize64
		if total > MaxExpandedBytes || f.UncompressedSize64 > 32<<20 {
			return out, fmt.Errorf("VISUAL_ARTIFACT_BUDGET")
		}
		r, e := f.Open()
		if e != nil {
			return out, e
		}
		data, e := io.ReadAll(io.LimitReader(r, 32<<20+1))
		r.Close()
		if e != nil {
			return out, e
		}
		if len(data) > 32<<20 {
			return out, fmt.Errorf("VISUAL_ARTIFACT_BUDGET")
		}
		out.Files[f.Name] = data
	}
	if e = Strict(out.Files["manifest.json"], &out.Receipt, 1<<20); e != nil {
		return out, e
	}
	if out.Receipt.Version != Version || out.Receipt.Pages < 1 || out.Receipt.Pages > 5 || len(out.Receipt.Files) != len(out.Files)-1 {
		return out, fmt.Errorf("VISUAL_ARTIFACT_MANIFEST")
	}
	for name, digest := range out.Receipt.Files {
		data, ok := out.Files[name]
		if !ok || Digest(data) != digest {
			return out, fmt.Errorf("VISUAL_ARTIFACT_HASH_MISMATCH")
		}
	}
	if e = Strict(out.Files["view.json"], &out.View, MaxViewBytes); e != nil {
		return out, e
	}
	var spec Spec
	if e = Strict(out.Files["spec.json"], &spec, 128<<10); e != nil {
		return out, e
	}
	if p := out.Receipt.TableProtection; p != nil {
		if (p.Version != "spreadsheet-text-0.2.0" && p.Version != "spreadsheet-text-0.2.1") || *p != protectionForVersion(out.View, p.Version) {
			return out, fmt.Errorf("VISUAL_TABLE_PROTECTION_MISMATCH")
		}
	}
	if Hash(spec) != Hash(out.Receipt.Spec) {
		return out, fmt.Errorf("VISUAL_ARTIFACT_SPEC_MISMATCH")
	}
	if e = spec.Validate(out.View, true); e != nil {
		return out, e
	}
	if out.Receipt.Pages != Pages(out.View, spec.Presentation) {
		return out, fmt.Errorf("VISUAL_ARTIFACT_PAGE_MISMATCH")
	}
	for i := 0; i < out.Receipt.Pages; i++ {
		if e = ValidatePNG(out.Files[fmt.Sprintf("image-%02d.png", i+1)], spec.Presentation.Width, spec.Presentation.Height); e != nil {
			return out, e
		}
		if e = validateCanonicalSVG(out.Files[fmt.Sprintf("image-%02d.svg", i+1)], out.View, spec, i); e != nil {
			return out, e
		}
	}
	// Never trust imported HTML/TSV as executable clipboard content. Consumers
	// rebuild those encodings from the bounded immutable view, without statistics.
	return out, nil
}

type Store struct {
	mu   sync.Mutex
	root *os.Root
}

func OpenStore(path string) (*Store, error) {
	if e := os.MkdirAll(path, 0700); e != nil {
		return nil, e
	}
	r, e := os.OpenRoot(path)
	if e != nil {
		return nil, e
	}
	return &Store{root: r}, nil
}
func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.root.Close() }

var artifactID = regexp.MustCompile(`^visual-[0-9a-f]{64}$`)

func (s *Store) Read(id string) ([]byte, Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}
func (s *Store) read(id string) ([]byte, Artifact, error) {
	if !artifactID.MatchString(id) {
		return nil, Artifact{}, fmt.Errorf("VISUAL_ARTIFACT_ID_INVALID")
	}
	f, e := s.root.Open(id + ".zip")
	if e != nil {
		return nil, Artifact{}, e
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, MaxBundleBytes+1))
	if e != nil {
		return nil, Artifact{}, e
	}
	if Digest(body) != strings.TrimPrefix(id, "visual-") {
		return nil, Artifact{}, fmt.Errorf("VISUAL_ARTIFACT_HASH_MISMATCH")
	}
	a, e := ReadArtifact(body)
	return body, a, e
}
func (s *Store) Save(body []byte) (Descriptor, error) {
	return s.SaveContext(context.Background(), body)
}
func (s *Store) SaveContext(ctx context.Context, body []byte) (Descriptor, error) {
	a, e := ReadArtifact(body)
	if e != nil {
		return Descriptor{}, e
	}
	if e = ctx.Err(); e != nil {
		return Descriptor{}, e
	}
	id := "visual-" + Digest(body)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, old, e := s.read(id); e == nil {
		return describe(id, old), nil
	}
	dir, e := s.root.Open(".")
	if e != nil {
		return Descriptor{}, e
	}
	entries, e := dir.ReadDir(-1)
	dir.Close()
	if e != nil {
		return Descriptor{}, e
	}
	count, total := 0, int64(0)
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".zip") {
			info, e := entry.Info()
			if e != nil {
				return Descriptor{}, e
			}
			count++
			total += info.Size()
		}
	}
	if count >= 32 || total+int64(len(body)) > 512<<20 {
		return Descriptor{}, fmt.Errorf("VISUAL_STORE_FULL: export/delete old artifacts explicitly; limit 32 artifacts / 512 MiB")
	}
	tmp := id + ".tmp"
	f, e := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return Descriptor{}, e
	}
	defer s.root.Remove(tmp)
	if _, e = f.Write(body); e != nil {
		f.Close()
		return Descriptor{}, e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return Descriptor{}, e
	}
	if e = f.Close(); e != nil {
		return Descriptor{}, e
	}
	// Content names are immutable; existing bytes are never replaced.
	if e = ctx.Err(); e != nil {
		return Descriptor{}, e
	}
	if e = s.root.Link(tmp, id+".zip"); e != nil {
		return Descriptor{}, e
	}
	return describe(id, a), nil
}
func describe(id string, a Artifact) Descriptor {
	return Descriptor{TableProtection(a.View), id, a.Receipt.Spec.Presentation.Title, a.Receipt.Pages, a.Receipt.Spec, a.Receipt.Spec.Reproducible(), a.Receipt.CreatedAt}
}
func (s *Store) List() ([]Descriptor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, e := s.root.Open(".")
	if e != nil {
		return nil, e
	}
	entries, e := f.ReadDir(-1)
	f.Close()
	if e != nil {
		return nil, e
	}
	out := []Descriptor{}
	for _, entry := range entries {
		if len(out) >= 32 {
			break
		}
		id := strings.TrimSuffix(entry.Name(), ".zip")
		if artifactID.MatchString(id) {
			_, a, e := s.read(id)
			if e != nil {
				return nil, e
			}
			out = append(out, describe(id, a))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !artifactID.MatchString(id) {
		return fmt.Errorf("VISUAL_ARTIFACT_ID_INVALID")
	}
	return s.root.Remove(id + ".zip")
}
func ReadLocalArtifact(path string) ([]byte, Artifact, error) {
	f, e := os.Open(filepath.Clean(path))
	if e != nil {
		return nil, Artifact{}, e
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, MaxBundleBytes+1))
	if e != nil {
		return nil, Artifact{}, e
	}
	a, e := ReadArtifact(body)
	return body, a, e
}
