package inputassist

import (
	"bytes"
	"context"
	"errors"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type MappingPreview struct {
	Importer        string `json:"importer"`
	PackSHA256      string `json:"pack_sha256"`
	Status          string `json:"status"`
	Code            string `json:"code,omitempty"`
	Metric          string `json:"metric,omitempty"`
	SourceValue     string `json:"source_value,omitempty"`
	NormalizedValue string `json:"normalized_value,omitempty"`
	Unit            string `json:"unit,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Escaped         bool   `json:"escaped"`
}

func previewMappings(ctx context.Context, report *Report, head []byte, mappings []packs.Mapping) error {
	candidates := map[string]bool{}
	for _, candidate := range report.Detection.Candidates {
		candidates[candidate.ImporterID] = true
	}
	for _, mapping := range mappings {
		if e := ctx.Err(); e != nil {
			return e
		}
		if !candidates[mapping.ID] || len(report.MappingPreviews) >= 4 {
			continue
		}
		preview := MappingPreview{Importer: mapping.ID, PackSHA256: mapping.Pack.SHA256, Status: "unavailable"}
		row, _, lineComplete := bytes.Cut(head, []byte{'\n'})
		if !lineComplete && int64(len(head)) < report.Source.Size {
			preview.Code = "PREVIEW_ROW_EXCEEDS_PROBE"
			report.MappingPreviews = append(report.MappingPreviews, preview)
			continue
		}
		record, source, e := mapping.Preview(bytes.TrimSpace(row))
		if e != nil {
			preview.Code = "PREVIEW_INPUT_INVALID"
			var issue *packs.Error
			if errors.As(e, &issue) {
				preview.Code = issue.Code
			}
		} else {
			preview.Status = "sample_only"
			preview.Metric = record.Metric
			preview.SourceValue = source
			preview.NormalizedValue = record.Value
			preview.Unit = record.Unit
			preview.Kind = record.Kind
			for _, field := range []*string{&preview.Metric, &preview.SourceValue, &preview.NormalizedValue, &preview.Unit, &preview.Kind} {
				text, truncated, escaped := resultcontract.DisplayText(*field)
				*field = text
				preview.Escaped = preview.Escaped || truncated || escaped
			}
		}
		report.MappingPreviews = append(report.MappingPreviews, preview)
	}
	return nil
}
