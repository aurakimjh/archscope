package runtimeevidence

import (
	"encoding/json"
	"fmt"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
)

type plan struct {
	NodeType  string            `json:"Node Type"`
	Estimated json.Number       `json:"Plan Rows"`
	Actual    json.Number       `json:"Actual Rows"`
	Loops     json.Number       `json:"Actual Loops"`
	Total     json.Number       `json:"Actual Total Time"`
	Children  []json.RawMessage `json:"Plans"`
}

func (b *builder) postgres(raw []byte) error {
	var roots []struct {
		Plan json.RawMessage `json:"Plan"`
	}
	if e := decode(raw, &roots); e != nil {
		return e
	}
	if len(roots) == 0 || len(roots) > 128 {
		return fmt.Errorf("RUNTIME_PLAN_ROOTS_INVALID")
	}
	if b.source.Producer == "" {
		b.source.Producer = "PostgreSQL EXPLAIN JSON"
	}
	type item struct {
		raw          json.RawMessage
		path, parent string
	}
	queue := []item{}
	for i, root := range roots {
		queue = append(queue, item{root.Plan, fmt.Sprintf("/%d/Plan", i), ""})
	}
	for len(queue) > 0 {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		current := queue[0]
		queue = queue[1:]
		var v plan
		if e := decode(current.raw, &v); e != nil {
			return e
		}
		if v.NodeType == "" {
			return fmt.Errorf("RUNTIME_PLAN_NODE_TYPE_REQUIRED")
		}
		id := "plan-" + r.Hash([]string{b.source.SHA, current.path})
		node := r.PlanNode{ID: id, Parent: current.parent, NodeType: v.NodeType, Ref: r.Ref{Source: "source", Position: current.path}}
		for _, field := range []struct {
			n json.Number
			p **string
		}{{v.Estimated, &node.EstimatedRows}, {v.Actual, &node.ActualRows}, {v.Loops, &node.Loops}, {v.Total, &node.TotalMS}} {
			value, e := number(field.n)
			if e != nil {
				return e
			}
			*field.p = value
		}
		b.set.Plans = append(b.set.Plans, node)
		if len(b.set.Plans) > 20000 || len(queue)+len(v.Children) > 20000 {
			return fmt.Errorf("RUNTIME_PLAN_NODE_BUDGET")
		}
		for i, child := range v.Children {
			queue = append(queue, item{child, fmt.Sprintf("%s/Plans/%d", current.path, i), id})
		}
	}
	return nil
}
func (b *builder) profile(raw []byte) error {
	if e := b.process(); e != nil {
		return e
	}
	set, e := p.Decode(raw)
	if e != nil {
		return e
	}
	if e = b.version(set.Producer, set.ProducerVersion); e != nil {
		return e
	}
	if len(set.SourceHashes) != 1 {
		return fmt.Errorf("RUNTIME_PROFILE_ORIGINAL_HASH_AMBIGUOUS")
	}
	for _, profile := range set.Profiles {
		id, ok := b.options.ProfileEntities[profile.ID]
		if !ok {
			continue
		}
		if b.entities[id].ID == "" {
			return fmt.Errorf("RUNTIME_PROFILE_ENTITY_MISSING")
		}
		b.set.Profiles = append(b.set.Profiles, r.ProfileRef{Entity: id, ProfileID: profile.ID, OriginalSHA: set.SourceHashes[0], ModelSHA: b.source.SHA, Clock: profile.Clock, Ref: r.Ref{Source: "source", Position: "/profiles/" + profile.ID}})
	}
	if len(b.set.Profiles) == 0 {
		return fmt.Errorf("RUNTIME_PROFILE_MAPPING_REQUIRED: explicitly map profile IDs to scoped entities; no name join")
	}
	return nil
}
