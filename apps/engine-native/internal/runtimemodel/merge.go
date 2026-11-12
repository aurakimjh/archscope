package runtimemodel

import (
	"context"
	"encoding/json"
	"fmt"
)

func Merge(ctx context.Context, sets []Set) (Set, error) {
	out := Empty()
	if len(sets) < 1 || len(sets) > 16 {
		return out, fmt.Errorf("RUNTIME_MERGE_BUDGET: choose 1..16 models")
	}
	seen := map[string]string{}
	add := func(kind, id string, v any, appendValue func()) error {
		raw, _ := json.Marshal(v)
		key := kind + ":" + id
		if old, ok := seen[key]; ok {
			if old != string(raw) {
				return fmt.Errorf("RUNTIME_MERGE_ID_CONFLICT: %s; preserve separate process/thread lifetimes", key)
			}
			return nil
		}
		seen[key] = string(raw)
		appendValue()
		return nil
	}
	for _, s := range sets {
		if e := s.Validate(ctx); e != nil {
			return out, e
		}
		for _, v := range s.Sources {
			if e := add("source", v.ID, v, func() { out.Sources = append(out.Sources, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Processes {
			if e := add("process", v.ID, v, func() { out.Processes = append(out.Processes, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Entities {
			if e := add("entity", v.ID, v, func() { out.Entities = append(out.Entities, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Frames {
			if e := add("frame", v.ID, v, func() { out.Frames = append(out.Frames, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Stacks {
			if e := add("stack", v.ID, v, func() { out.Stacks = append(out.Stacks, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Observations {
			if e := add("observation", v.ID, v, func() { out.Observations = append(out.Observations, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Relations {
			if e := add("relation", Hash(v), v, func() { out.Relations = append(out.Relations, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Profiles {
			if e := add("profile", Hash(v), v, func() { out.Profiles = append(out.Profiles, v) }); e != nil {
				return out, e
			}
		}
		for _, v := range s.Plans {
			if e := add("plan", v.ID, v, func() { out.Plans = append(out.Plans, v) }); e != nil {
				return out, e
			}
		}
		if e := out.Validate(ctx); e != nil {
			return out, e
		}
	}
	return out, out.Validate(ctx)
}
