package aiinterpretation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var evidenceRefRE = regexp.MustCompile(`^[a-z][a-z0-9_]*:[a-z][a-z0-9_]*:[A-Za-z0-9_.-]+$`)

type EvidenceRef struct {
	Source string
	Entity string
	ID     string
}

type EvidenceItem struct {
	Ref        string         `json:"evidence_ref"`
	Text       string         `json:"text"`
	SourceFile string         `json:"source_file,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type EvidenceRegistry struct {
	items map[string]EvidenceItem
}

func NewEvidenceRegistry() *EvidenceRegistry {
	return &EvidenceRegistry{items: map[string]EvidenceItem{}}
}

func ParseEvidenceRef(value string) (EvidenceRef, bool) {
	value = strings.TrimSpace(value)
	if !evidenceRefRE.MatchString(value) {
		return EvidenceRef{}, false
	}
	parts := strings.SplitN(value, ":", 3)
	return EvidenceRef{Source: parts[0], Entity: parts[1], ID: parts[2]}, true
}

func (r *EvidenceRegistry) Add(item EvidenceItem) error {
	if r == nil {
		return fmt.Errorf("evidence registry is nil")
	}
	if _, ok := ParseEvidenceRef(item.Ref); !ok {
		return fmt.Errorf("invalid evidence_ref grammar: %s", item.Ref)
	}
	if item.Text == "" {
		item.Text = item.Ref
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	r.items[item.Ref] = item
	return nil
}

func (r *EvidenceRegistry) Get(ref string) (EvidenceItem, bool) {
	if r == nil {
		return EvidenceItem{}, false
	}
	item, ok := r.items[ref]
	return item, ok
}

func (r *EvidenceRegistry) Items() []EvidenceItem {
	if r == nil {
		return nil
	}
	items := make([]EvidenceItem, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Ref < items[j].Ref })
	return items
}

func (r *EvidenceRegistry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.items)
}

func BuildEvidenceRegistry(result any) *EvidenceRegistry {
	registry := NewEvidenceRegistry()
	collectEvidence(result, registry, "")
	return registry
}

func collectEvidence(value any, registry *EvidenceRegistry, inheritedFile string) {
	switch v := value.(type) {
	case map[string]any:
		sourceFile := inheritedFile
		if raw, ok := v["source_file"].(string); ok && raw != "" {
			sourceFile = raw
		}
		if raw, ok := v["evidence_ref"].(string); ok && raw != "" {
			text := evidenceText(v, raw)
			_ = registry.Add(EvidenceItem{
				Ref:        raw,
				Text:       text,
				SourceFile: sourceFile,
				Metadata:   shallowMetadata(v),
			})
		}
		for _, child := range v {
			collectEvidence(child, registry, sourceFile)
		}
	case []any:
		for _, child := range v {
			collectEvidence(child, registry, inheritedFile)
		}
	case []map[string]any:
		for _, child := range v {
			collectEvidence(child, registry, inheritedFile)
		}
	}
}

func evidenceText(row map[string]any, fallback string) string {
	for _, key := range []string{"message", "summary", "text", "description", "event_type", "thread", "signature"} {
		if raw, ok := row[key].(string); ok && strings.TrimSpace(raw) != "" {
			return raw
		}
	}
	return fallback
}

func shallowMetadata(row map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range row {
		switch v.(type) {
		case string, float64, int, int64, bool, nil:
			out[k] = v
		}
	}
	return out
}
