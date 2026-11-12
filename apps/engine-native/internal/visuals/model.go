// Package visuals owns bounded immutable reporting views and artifact contracts.
// It consumes analyzer/query output and never parses original diagnostic inputs.
package visuals

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/common"
	"math"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const Version = "1.0.0"
const RendererVersion = "1.0.0"
const FontSHA256 = "194018e6b2b293a7964f037b25c0249ce1418bc9ab3c971060a03aa57861e252"
const Redaction = "index-mask-0.2.0"
const MaxViewBytes = 4 << 20
const MaxPNGBytes = 16 << 20
const MaxExpandedBytes = 192 << 20
const MaxBundleBytes = 96 << 20

type Query struct {
	Sort          string              `json:"sort,omitempty"`
	OmitZeroDelta *bool               `json:"omit_zero_delta,omitempty"`
	Kind          string              `json:"kind"`
	Provider      string              `json:"provider,omitempty"`
	Offset        int                 `json:"offset"`
	Limit         int                 `json:"limit"`
	Columns       []string            `json:"columns,omitempty"`
	Profile       *profilemodel.Query `json:"profile,omitempty"`
}
type Presentation struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FontSize    int    `json:"font_size"`
	RowsPerPage int    `json:"rows_per_page"`
	Transparent bool   `json:"transparent"`
	Title       string `json:"title"`
	Annotation  string `json:"annotation"`
}
type Source struct {
	SHA256  string `json:"sha256"`
	Type    string `json:"type"`
	Payload string `json:"payload_version"`
}
type Node struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Self     string  `json:"self"`
	Evidence string  `json:"evidence"`
	X        float64 `json:"x"`
	Width    float64 `json:"width"`
	Depth    int     `json:"depth"`
}
type View struct {
	ScopeLabel   string                  `json:"scope_label"`
	QualityLabel string                  `json:"quality_label"`
	TimeScope    string                  `json:"time_scope"`
	Version      string                  `json:"version"`
	Kind         string                  `json:"kind"`
	Sources      []Source                `json:"sources"`
	Query        Query                   `json:"query"`
	Columns      []string                `json:"columns"`
	Rows         [][]resultcontract.Cell `json:"rows"`
	Nodes        []Node                  `json:"nodes"`
	Total        int                     `json:"total"`
	Offset       int                     `json:"offset"`
	Unit         string                  `json:"unit"`
	Denominator  string                  `json:"denominator"`
	Population   string                  `json:"population"`
	Quality      string                  `json:"quality"`
	Warnings     []string                `json:"warnings"`
}
type Spec struct {
	Version         string       `json:"version"`
	Template        string       `json:"template"`
	TemplateVersion string       `json:"template_version"`
	Renderer        string       `json:"renderer"`
	FontSHA256      string       `json:"font_sha256"`
	Redaction       string       `json:"redaction_profile"`
	ViewSHA256      string       `json:"view_sha256"`
	Sources         []Source     `json:"sources"`
	Query           Query        `json:"query"`
	Presentation    Presentation `json:"presentation"`
	Parent          string       `json:"parent,omitempty"`
}

func Digest(body []byte) string { s := sha256.Sum256(body); return hex.EncodeToString(s[:]) }
func Hash(value any) string     { body, _ := json.Marshal(value); return Digest(body) }
func Strict(body []byte, out any, limit int) error {
	if len(body) > limit {
		return fmt.Errorf("VISUAL_BUDGET_EXCEEDED: JSON bytes=%d limit=%d", len(body), limit)
	}
	if err := wirejson.DecodeWithLimits(body, new(any), wirejson.ResultLimits(int64(limit), 128)); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	return decoder.Decode(out)
}
func DefaultPresentation() Presentation {
	return Presentation{Width: 2400, Height: 1600, FontSize: 26, RowsPerPage: 20, Title: "Selected analysis view"}
}
func (p Presentation) Validate() error {
	if (p.Width != 1600 && p.Width != 2400 && p.Width != 3200) || p.Height < 800 || p.Height > 2400 || p.Width*p.Height > 8_000_000 || p.FontSize < 20 || p.FontSize > 48 || p.RowsPerPage < 5 || p.RowsPerPage > 25 || p.RowsPerPage*(p.FontSize*3/2+8) > p.Height-470 || len(p.Title) > 256 || len(p.Annotation) > 512 {
		return fmt.Errorf("VISUAL_PRESENTATION_INVALID: use a preset width, bounded height/font and fewer rows per page")
	}
	return nil
}
func (v View) Validate() error {
	if v.Version != Version || len(v.Sources) < 1 || len(v.Sources) > 2 || len(v.Columns) < 1 || len(v.Columns) > 8 || len(v.Rows) > 200 || len(v.Nodes) > 200 || v.Total < 0 || v.Offset < 0 || len(v.Rows)+v.Offset > v.Total || len(v.Quality) > 64<<10 || len(v.Warnings) > 32 {
		return fmt.Errorf("VISUAL_VIEW_INVALID")
	}
	switch v.Kind {
	case "provider_table", "profile_top", "profile_flamegraph", "profile_diff":
	default:
		return fmt.Errorf("VISUAL_KIND_UNSUPPORTED")
	}
	for _, source := range v.Sources {
		if len(source.SHA256) != 64 || len(source.Type) > 128 || len(source.Payload) > 32 {
			return fmt.Errorf("VISUAL_SOURCE_INVALID")
		}
		if _, e := hex.DecodeString(source.SHA256); e != nil {
			return e
		}
	}
	for _, row := range v.Rows {
		if len(row) != len(v.Columns) {
			return fmt.Errorf("VISUAL_ROW_INVALID")
		}
		for _, cell := range row {
			if len(cell.Text) > 4096 || len(cell.Unit) > 1024 || len(cell.Reason) > 1024 {
				return fmt.Errorf("VISUAL_CELL_BUDGET")
			}
		}
	}
	for _, node := range v.Nodes {
		if math.IsNaN(node.X) || math.IsNaN(node.Width) || math.IsInf(node.X, 0) || math.IsInf(node.Width, 0) || node.X < 0 || node.Width < 0 || node.X+node.Width > 1.000001 || node.Depth < 0 || node.Depth > 4096 || len(node.Name) > 1024 {
			return fmt.Errorf("VISUAL_LAYOUT_INVALID")
		}
	}
	body, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(body) > MaxViewBytes {
		return fmt.Errorf("VISUAL_BUDGET_EXCEEDED: selected view exceeds 4 MiB")
	}
	return nil
}
func NewSpec(view View, p Presentation, parent string) (Spec, error) {
	p.Title = clean(common.RedactIndexText(p.Title))
	p.Annotation = clean(common.RedactIndexText(p.Annotation))
	spec := Spec{Version: Version, Template: view.Kind, TemplateVersion: Version, Renderer: RendererVersion, FontSHA256: FontSHA256, Redaction: Redaction, ViewSHA256: Hash(view), Sources: view.Sources, Query: view.Query, Presentation: p, Parent: parent}
	return spec, spec.Validate(view, false)
}
func (s Spec) Validate(view View, historical bool) error {
	body, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if e = validateSpecSchema(body); e != nil {
		return e
	}
	if e := view.Validate(); e != nil {
		return e
	}
	if e := s.Presentation.Validate(); e != nil {
		return e
	}
	if s.Version != Version || s.ViewSHA256 != Hash(view) || s.Template != view.Kind || Hash(s.Sources) != Hash(view.Sources) || Hash(s.Query) != Hash(view.Query) || len(s.Parent) > 96 {
		return fmt.Errorf("VISUAL_SPEC_BINDING_MISMATCH")
	}
	if !historical && !s.Reproducible() {
		return fmt.Errorf("VISUAL_RENDER_DEPENDENCY_UNAVAILABLE: display saved images or explicitly create an upgraded revision")
	}
	return nil
}
func (s Spec) Reproducible() bool {
	return s.TemplateVersion == Version && s.Renderer == RendererVersion && s.FontSHA256 == FontSHA256 && s.Redaction == Redaction && s.Version == Version
}
func Pages(v View, p Presentation) int {
	if v.Kind == "profile_flamegraph" {
		return 1
	}
	return max(1, (len(v.Rows)+p.RowsPerPage-1)/p.RowsPerPage)
}
