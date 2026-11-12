package profilemodel

import (
	"context"
	"encoding/json"
	"strings"
)

type ProfileEntity struct {
	ID      string `json:"id"`
	Process string `json:"process"`
	Thread  string `json:"thread"`
	Worker  string `json:"worker"`
}
type EntityMapping struct {
	Baseline ProfileEntity `json:"baseline"`
	Target   ProfileEntity `json:"target"`
	Reason   string        `json:"reason"`
}

func entity(p Profile) ProfileEntity { return ProfileEntity{p.ID, p.Process, p.Thread, p.Worker} }

// CompareMapped accepts a complete, one-to-one, explicitly declared mapping.
// It changes only entity aliases in a private comparison copy. Clock, quality,
// original capture/metric and selection gates remain authoritative. Original
// source/model/policy references always describe the unmodified inputs.
func CompareMapped(ctx context.Context, a ProfileSet, as Selection, b ProfileSet, bs Selection, mapping []EntityMapping) (Difference, error) {
	if len(mapping) == 0 {
		return Compare(ctx, a, as, b, bs)
	}
	refuse := func() (Difference, error) {
		return Difference{}, comparisonRefusal("entity_mapping_invalid", "complete current one-to-one selected entity tuples and a reason are required")
	}
	if len(mapping) != len(as.ProfileIDs) || len(mapping) != len(bs.ProfileIDs) || len(mapping) > 1024 {
		return refuse()
	}
	selected := func(ids []string, key string) bool {
		for _, v := range ids {
			if v == key {
				return true
			}
		}
		return false
	}
	left, right := map[string]Profile{}, map[string]Profile{}
	for _, p := range a.Profiles {
		left[p.ID] = p
	}
	for _, p := range b.Profiles {
		right[p.ID] = p
	}
	seenA, seenB := map[string]bool{}, map[string]bool{}
	aliases := map[string]Profile{}
	for _, m := range mapping {
		l, lok := left[m.Baseline.ID]
		r, rok := right[m.Target.ID]
		if !lok || !rok || !selected(as.ProfileIDs, l.ID) || !selected(bs.ProfileIDs, r.ID) || seenA[l.ID] || seenB[r.ID] || entity(l) != m.Baseline || entity(r) != m.Target || len(strings.TrimSpace(m.Reason)) == 0 || len(m.Reason) > 512 {
			return refuse()
		}
		seenA[l.ID], seenB[r.ID] = true, true
		aliases[r.ID] = l
	}
	raw, _ := json.Marshal(b)
	copy, e := Decode(raw)
	if e != nil {
		return Difference{}, e
	}
	next := CopySelection(bs)
	contract, present, e := copy.SourceContract()
	if e != nil {
		return Difference{}, e
	}
	for i, p := range copy.Profiles {
		if l, ok := aliases[p.ID]; ok {
			copy.Profiles[i].ID = l.ID
			copy.Profiles[i].Process = l.Process
			copy.Profiles[i].Thread = l.Thread
			copy.Profiles[i].Worker = l.Worker
			if present {
				contract.Profiles[i].ID = l.ID
				for j, profile := range a.Profiles {
					if profile.ID == l.ID {
						ac, _, _ := a.SourceContract()
						if len(ac.Profiles) > j {
							contract.Profiles[i].Name = ac.Profiles[j].Name
						}
					}
				}
			}
		}
	}
	for i, id := range next.ProfileIDs {
		next.ProfileIDs[i] = aliases[id].ID
	}
	if present {
		copy.Extensions[SourceExtension], _ = json.Marshal(contract)
		if copy.Format == "speedscope-json" {
			var capture map[string]json.RawMessage
			json.Unmarshal(copy.CaptureOptions, &capture)
			capture["source_profile_bounds"], _ = json.Marshal(contract.Profiles)
			copy.CaptureOptions, _ = json.Marshal(capture)
		}
	}
	d, e := Compare(ctx, a, as, copy, next)
	if e != nil {
		return d, e
	}
	d.Target.ModelSHA256 = hash("", b)
	d.Target.Population = b.SelectionIdentity(bs)
	d.Target.CapturePolicy = b.CapturePolicy(bs.MetricID)
	d.Target.SourceHashes = append([]string(nil), b.SourceHashes...)
	d.IncidentalDifferences = incidentalDifferences(a.CapturePolicy(as.MetricID), b.CapturePolicy(bs.MetricID))
	d.Warnings = append(d.Warnings, "entity_mapping_declared")
	return d, nil
}
