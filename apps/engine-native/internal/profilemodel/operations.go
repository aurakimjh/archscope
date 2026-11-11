package profilemodel

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type PathFilter struct {
	Path  []string `json:"path"`
	Match string   `json:"match"`
}
type TreeNode struct {
	ID        string     `json:"id"`
	FrameID   string     `json:"frame_id,omitempty"`
	Name      string     `json:"name,omitempty"`
	Inclusive string     `json:"inclusive"`
	Self      string     `json:"self"`
	Share     *string    `json:"share"`
	Children  []TreeNode `json:"children"`
}
type Tree struct {
	Availability  string    `json:"availability"`
	Reason        string    `json:"reason,omitempty"`
	Root          TreeNode  `json:"root"`
	Metric        Metric    `json:"metric"`
	Selection     Selection `json:"selection"`
	Population    string    `json:"population"`
	SampleRecords int       `json:"sample_records"`
}

func MatchPath(path, pattern []string, mode string) bool {
	if len(pattern) == 0 {
		return true
	}
	if mode == "prefix" {
		if len(path) < len(pattern) {
			return false
		}
		for i := range pattern {
			if path[i] != pattern[i] {
				return false
			}
		}
		return true
	}
	for start := 0; start+len(pattern) <= len(path); start++ {
		match := true
		for j := range pattern {
			match = match && path[start+j] == pattern[j]
		}
		if match {
			return true
		}
	}
	return false
}
func (s ProfileSet) each(ctx context.Context, selection Selection, yield func(Profile, Sample, []string, *big.Rat) error) error {
	if e := s.ValidateSelection(selection); e != nil {
		return e
	}
	stacks := map[string][]string{}
	for _, stack := range s.Stacks {
		stacks[stack.ID] = stack.Frames
	}
	ids := map[string]bool{}
	for _, id := range selection.ProfileIDs {
		ids[id] = true
	}
	references := 0
	for _, p := range s.Profiles {
		if !ids[p.ID] {
			continue
		}
		for _, sample := range p.Samples {
			if e := ctx.Err(); e != nil {
				return e
			}
			if sample.MetricID != selection.MetricID {
				continue
			}
			path := stacks[sample.StackID]
			if !MatchPath(path, selection.Path, selection.Match) {
				continue
			}
			selected := true
			for _, condition := range selection.Filters {
				selected = selected && MatchPath(path, condition.Path, condition.Match)
			}
			if !selected {
				continue
			}
			references += len(path)
			if references > 1_000_000 {
				return budget("selected frame-reference budget exceeded")
			}
			value, e := Number(sample.Weight)
			if e != nil {
				return e
			}
			if e = yield(p, sample, path, value); e != nil {
				return e
			}
		}
	}
	return nil
}
func BuildTree(ctx context.Context, s ProfileSet, selection Selection) (Tree, error) {
	metric, ok := s.Metric(selection.MetricID)
	if !ok {
		return Tree{}, fmt.Errorf("unknown metric")
	}
	scale, e := Number(metric.Scale)
	if e != nil {
		return Tree{}, e
	}
	trie := NewTrie("root", "All", func() *big.Rat { return new(big.Rat) }, func(a, b *big.Rat) *big.Rat { return a.Add(a, b) }, func(parent, frame string) string { return hash("node-", []string{parent, frame}) })
	records := 0
	e = s.each(ctx, selection, func(_ Profile, _ Sample, path []string, value *big.Rat) error {
		trie.Add(path, new(big.Rat).Mul(value, scale))
		records++
		if trie.Nodes > MaxNodes {
			return budget("tree node budget exceeded")
		}
		return nil
	})
	if e != nil {
		return Tree{}, e
	}
	encodedBytes := 0
	var freeze func(*TrieNode[*big.Rat]) (TreeNode, error)
	freeze = func(node *TrieNode[*big.Rat]) (TreeNode, error) {
		if e := ctx.Err(); e != nil {
			return TreeNode{}, e
		}
		inclusive := Text(node.Value)
		if _, e := Number(inclusive); e != nil {
			return TreeNode{}, budget("result numeric budget exceeded")
		}
		out := TreeNode{ID: node.ID, FrameID: node.Name, Inclusive: inclusive, Children: []TreeNode{}}
		if node == trie.Root {
			out.FrameID = ""
			out.Name = "All"
		}
		children := make([]*TrieNode[*big.Rat], 0, len(node.Children))
		for _, child := range node.Children {
			children = append(children, child)
		}
		sort.Slice(children, func(i, j int) bool {
			cmp := children[i].Value.Cmp(children[j].Value)
			if cmp == 0 {
				return children[i].Name < children[j].Name
			}
			return cmp > 0
		})
		self := new(big.Rat).Set(node.Value)
		for _, child := range children {
			self.Sub(self, child.Value)
			next, e := freeze(child)
			if e != nil {
				return out, e
			}
			out.Children = append(out.Children, next)
		}
		out.Self = Text(self)
		if _, e := Number(out.Self); e != nil {
			return TreeNode{}, budget("self weight numeric budget exceeded")
		}

		if trie.Root.Value.Sign() != 0 {
			ratio := new(big.Rat).Quo(new(big.Rat).Set(node.Value), trie.Root.Value).RatString()
			out.Share = &ratio
		}
		if out.Share != nil {
			encodedBytes += len(*out.Share)
		}
		encodedBytes += len(out.ID) + len(out.FrameID) + len(out.Inclusive) + len(out.Self) + 256
		if encodedBytes > 8<<20 {
			return TreeNode{}, budget("tree serialization budget exceeded")
		}
		return out, nil
	}
	root, e := freeze(trie.Root)
	availability, reason := "observed", ""
	if records == 0 {
		availability = "unavailable"
		reason = "no_selected_observations"
	}
	return Tree{Root: root, Metric: metric, Selection: CopySelection(selection), Population: s.SelectionIdentity(selection), SampleRecords: records, Availability: availability, Reason: reason}, e
}

type DiffRow struct {
	ID         string  `json:"id"`
	FrameID    string  `json:"frame_id,omitempty"`
	Before     string  `json:"before"`
	After      string  `json:"after"`
	Delta      string  `json:"delta"`
	ShareDelta *string `json:"share_delta"`
	Percent    *string `json:"percent"`
	Reason     string  `json:"reason,omitempty"`
}
type Difference struct {
	Metric     Metric    `json:"metric"`
	Population string    `json:"population"`
	Rows       []DiffRow `json:"rows"`
}

func Compare(ctx context.Context, a ProfileSet, as Selection, b ProfileSet, bs Selection) (Difference, error) {
	left, e := BuildTree(ctx, a, as)
	if e != nil {
		return Difference{}, e
	}
	right, e := BuildTree(ctx, b, bs)
	if e != nil {
		return Difference{}, e
	}
	if left.SampleRecords == 0 || right.SampleRecords == 0 {
		return Difference{}, fmt.Errorf("comparison unavailable: no selected observations")
	}
	if left.Metric != right.Metric {
		return Difference{}, fmt.Errorf("measurement kind/unit/scale/sign mismatch")
	}
	if left.Population != right.Population {
		return Difference{}, fmt.Errorf("population/source/selection/capture conditions mismatch")
	}
	before, after := map[string]TreeNode{}, map[string]TreeNode{}
	var walk func(TreeNode, map[string]TreeNode)
	walk = func(n TreeNode, m map[string]TreeNode) {
		m[n.ID] = n
		for _, child := range n.Children {
			walk(child, m)
		}
	}
	walk(left.Root, before)
	walk(right.Root, after)
	ids := map[string]bool{}
	for id := range before {
		ids[id] = true
	}
	for id := range after {
		ids[id] = true
	}
	ordered := []string{}
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	out := Difference{Metric: left.Metric, Population: left.Population, Rows: []DiffRow{}}
	for _, id := range ordered {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		l, lok := before[id]
		r, rok := after[id]
		av, bv := new(big.Rat), new(big.Rat)
		if lok {
			av, _ = Number(l.Inclusive)
		}
		if rok {
			bv, _ = Number(r.Inclusive)
		}
		delta := new(big.Rat).Sub(bv, av)
		row := DiffRow{ID: id, FrameID: l.FrameID, Before: Text(av), After: Text(bv), Delta: Text(delta)}
		if !lok {
			row.FrameID = r.FrameID
		}
		if av.Sign() == 0 {
			row.Reason = "zero_baseline"
		} else {
			percentage := new(big.Rat).Mul(new(big.Rat).Quo(delta, av), big.NewRat(100, 1)).RatString()
			row.Percent = &percentage
		}
		if l.Share != nil && r.Share != nil {
			lv, _ := new(big.Rat).SetString(*l.Share)
			rv, _ := new(big.Rat).SetString(*r.Share)
			share := new(big.Rat).Sub(rv, lv).RatString()
			row.ShareDelta = &share
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

type Stage struct {
	Index int  `json:"index"`
	Tree  Tree `json:"tree"`
}

func Drilldown(ctx context.Context, s ProfileSet, selection Selection, filters []PathFilter) ([]Stage, error) {
	if len(filters) > 64 {
		return nil, budget("filter count budget exceeded")
	}
	out := []Stage{}
	bytes := 0
	for index := 0; index <= len(filters); index++ {
		next := selection
		next.Filters = append(append([]PathFilter(nil), selection.Filters...), filters[:index]...)
		tree, e := BuildTree(ctx, s, next)
		if e != nil {
			return nil, e
		}
		body, e := json.Marshal(tree)
		if e != nil {
			return nil, e
		}
		bytes += len(body)
		if bytes > 32<<20 {
			return nil, budget("drilldown response byte budget exceeded")
		}
		out = append(out, Stage{index, tree})
	}
	return out, nil
}

type Projection struct {
	Stacks               map[string]int    `json:"stacks"`
	Metric               Metric            `json:"metric"`
	Selection            Selection         `json:"selection"`
	TickScale            string            `json:"tick_scale"`
	AggregatedDimensions []string          `json:"aggregated_dimensions"`
	FrameNames           map[string]string `json:"frame_names"`
}

func LegacyStackProjection(ctx context.Context, s ProfileSet, selection Selection, tickScale string, stableKeys bool) (Projection, error) {
	tick, e := Number(tickScale)
	if e != nil || tick.Sign() <= 0 {
		return Projection{}, fmt.Errorf("explicit positive tick scale required")
	}
	metric, ok := s.Metric(selection.MetricID)
	if !ok || metric.Signed {
		return Projection{}, fmt.Errorf("signed/unknown metric cannot use legacy stack projection")
	}
	scale, e := Number(metric.Scale)
	if e != nil {
		return Projection{}, e
	}
	names := map[string]string{}
	idsByName := map[string]string{}
	for _, frame := range s.Frames {
		names[frame.ID] = frame.Display
	}
	values := map[string]*big.Rat{}
	observations := 0
	e = s.each(ctx, selection, func(_ Profile, _ Sample, path []string, value *big.Rat) error {
		observations++
		keys := append([]string(nil), path...)
		if !stableKeys {
			for i, id := range keys {
				name := names[id]
				if strings.Contains(name, ";") || strings.TrimSpace(name) != name || name == "" {
					return fmt.Errorf("legacy frame name is not lossless")
				}
				if prior, exists := idsByName[name]; exists && prior != id {
					return fmt.Errorf("display name collision requires stable frame keys")
				}
				idsByName[name] = id
				keys[i] = name
			}
		}
		key := strings.Join(keys, ";")
		v := new(big.Rat).Quo(new(big.Rat).Mul(value, scale), tick)
		if !v.IsInt() || v.Sign() < 0 {
			return fmt.Errorf("legacy integer projection would lose precision/sign")
		}
		if values[key] == nil {
			values[key] = new(big.Rat)
		}
		values[key].Add(values[key], v)
		return nil
	})
	if e != nil {
		return Projection{}, e
	}
	if observations == 0 {
		return Projection{}, fmt.Errorf("legacy projection unavailable: no selected observations")
	}
	out := Projection{Stacks: map[string]int{}, Metric: metric, Selection: CopySelection(selection), TickScale: tickScale, AggregatedDimensions: []string{"profile", "process", "thread", "worker", "timestamp", "labels"}, FrameNames: names}
	maxInt := new(big.Int).SetUint64(uint64(^uint(0) >> 1))
	total := new(big.Int)
	for key, value := range values {
		if value.Num().Cmp(maxInt) > 0 {
			return Projection{}, fmt.Errorf("legacy target int range exceeded")
		}
		total.Add(total, value.Num())
		if total.Cmp(maxInt) > 0 {
			return Projection{}, fmt.Errorf("legacy total int range exceeded")
		}
		out.Stacks[key] = int(value.Num().Int64())
	}
	return out, nil
}
