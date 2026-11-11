package profilemodel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// FromLegacyCollapsed preserves exactly the information present in an aggregate
// stack map. It does not infer chronology, producer version, thread identity or
// an interval. Callers supply the actual source digest and declared metric.
func FromLegacyCollapsed(stacks map[string]int, sourceHash string, metric Metric) (ProfileSet, Selection, error) {
	s := ProfileSet{SchemaVersion: Version, Producer: "legacy-aggregate", Format: "collapsed", SourceHashes: []string{sourceHash}, CaptureOptions: json.RawMessage(`{}`), Frames: []Frame{}, Stacks: []Stack{}, Metrics: []Metric{metric}, Profiles: []Profile{{ID: "aggregate", Clock: "aggregate", Samples: []Sample{}}}, Quality: Quality{ClockConfidence: "unavailable", ConversionLoss: []string{"original aggregate has no chronology or process/thread identity"}}}
	frames := map[string]bool{}
	ordered := []string{}
	for stack := range stacks {
		ordered = append(ordered, stack)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		path := []string{}
		for _, name := range strings.Split(key, ";") {
			if name == "" {
				return s, Selection{}, fmt.Errorf("empty legacy frame cannot be recovered")
			}
			frame := Frame{Symbol: name, Display: name}
			frame.ID = FrameID(frame)
			if !frames[frame.ID] {
				s.Frames = append(s.Frames, frame)
				frames[frame.ID] = true
			}
			path = append(path, frame.ID)
		}
		stack := Stack{ID: StackID(path), Frames: path}
		s.Stacks = append(s.Stacks, stack)
		s.Profiles[0].Samples = append(s.Profiles[0].Samples, Sample{StackID: stack.ID, MetricID: metric.ID, Weight: strconv.Itoa(stacks[key])})
	}
	selection := Selection{ProfileIDs: []string{"aggregate"}, MetricID: metric.ID}
	return s, selection, s.Validate()
}

// CheckLegacyRange validates aggregate totals without materializing a second
// ProfileSet on the legacy large-file fast path. Legacy nonpositive entries keep
// their documented omitted semantics; new ProfileSets never use this policy.
func CheckLegacyRange(stacks map[string]int) error {
	total := 0
	limit := int(^uint(0) >> 1)
	for _, value := range stacks {
		if value > 0 {
			if total > limit-value {
				return fmt.Errorf("legacy total int range exceeded; use exact ProfileSet operations")
			}
			total += value
		}
	}
	return nil
}
