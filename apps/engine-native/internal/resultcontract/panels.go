package resultcontract

import (
	"fmt"
	"regexp"
	"sort"
)

// Panel is compiled application capability, never authority from stored JSON.
// The renderer selects a known panel protocol, independently from payload versions.
type Panel struct {
	ID      string   `json:"id"`
	Version string   `json:"version"`
	Actions []string `json:"actions"`
}

var panelToken = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func validatePanel(p *Panel) error {
	if p == nil {
		return nil
	}
	if !panelToken.MatchString(p.ID) || p.Version != "1.0.0" || len(p.Actions) < 1 || len(p.Actions) > 16 {
		return fmt.Errorf("invalid result panel capability")
	}
	seen := map[string]bool{}
	for _, action := range p.Actions {
		if !panelToken.MatchString(action) || seen[action] {
			return fmt.Errorf("invalid result panel action")
		}
		seen[action] = true
	}
	return nil
}

func clonePanel(p *Panel) *Panel {
	if p == nil {
		return nil
	}
	out := *p
	out.Actions = append([]string{}, p.Actions...)
	return &out
}

type PanelContract struct {
	Type       string   `json:"type"`
	Payload    Range    `json:"payload"`
	Extensions []string `json:"extensions"`
	Panel      *Panel   `json:"panel,omitempty"`
}

// PanelContracts is a frozen registry inventory for renderer contract checks.
// Legacy/unrecognized payloads retain generic display without a specialized panel.
func (r *Registry) PanelContracts() []PanelContract {
	out := []PanelContract{}
	for kind, descriptors := range r.types {
		for _, descriptor := range descriptors {
			keys := []string{}
			for key := range descriptor.Extensions {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			out = append(out, PanelContract{kind, descriptor.Payload, keys, clonePanel(descriptor.Panel)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].Payload.Min < out[j].Payload.Min
		}
		return out[i].Type < out[j].Type
	})
	return out
}
