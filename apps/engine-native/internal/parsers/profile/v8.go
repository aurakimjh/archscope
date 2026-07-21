package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

// parseV8 remains the small-input compatibility entry point. ParseFile uses
// parseV8StreamFile so Chrome traces are never first decoded as a full
// []RawMessage envelope and then decoded a second time.
func parseV8(data []byte, format string) (Parsed, error) {
	return parseV8Reader(bytes.NewReader(data), format, 0)
}

type v8Node struct {
	ID        int `json:"id"`
	CallFrame struct {
		FunctionName string `json:"functionName"`
		URL          string `json:"url"`
		LineNumber   int    `json:"lineNumber"`
	} `json:"callFrame"`
	Children []int `json:"children"`
	HitCount *int  `json:"hitCount,omitempty"`
}

type v8Profile struct {
	Nodes      []v8Node `json:"nodes"`
	Samples    []int    `json:"samples"`
	TimeDeltas []int64  `json:"timeDeltas"`
	StartTime  int64    `json:"startTime"`
	EndTime    int64    `json:"endTime"`
}

func profileStreamFormat(path, requested string, maxBytes int64) (string, bool, error) {
	if requested == "v8-cpuprofile" || requested == "chrome-trace-json" {
		return requested, true, nil
	}
	if requested != "auto" {
		return "", false, nil
	}
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".cpuprofile") || strings.HasSuffix(base, ".cpuprofile.gz") {
		return "v8-cpuprofile", true, nil
	}
	reader, err := openProfileInput(path, maxBytes)
	if err != nil {
		return "", false, err
	}
	defer reader.Close()
	preview := make([]byte, 64*1024)
	n, err := reader.Read(preview)
	if err != nil && err != io.EOF {
		return "", false, err
	}
	preview = preview[:n]
	if isChromeTrace(preview) {
		return "chrome-trace-json", true, nil
	}
	if isV8Profile(preview) {
		return "v8-cpuprofile", true, nil
	}
	return "", false, nil
}

func parseV8StreamFile(path, format string, maxBytes int64, maxSamples int) (Parsed, error) {
	// Trace downsampling needs the final sample count to choose deterministic
	// time-order buckets. The first pass is token streaming and retains no
	// events; the second pass retains only the selected output samples.
	if format == "chrome-trace-json" && maxSamples > 0 {
		reader, err := openProfileInput(path, maxBytes)
		if err != nil {
			return Parsed{}, err
		}
		count, err := countChromeTraceSamples(reader)
		closeErr := reader.Close()
		if err != nil {
			return Parsed{}, err
		}
		if closeErr != nil {
			return Parsed{}, closeErr
		}
		reader, err = openProfileInput(path, maxBytes)
		if err != nil {
			return Parsed{}, err
		}
		defer reader.Close()
		return parseChromeTraceReader(reader, maxSamples, count)
	}
	reader, err := openProfileInput(path, maxBytes)
	if err != nil {
		return Parsed{}, err
	}
	defer reader.Close()
	return parseV8Reader(reader, format, maxSamples)
}

func parseV8Reader(reader io.Reader, format string, maxSamples int) (Parsed, error) {
	if format == "chrome-trace-json" {
		return parseChromeTraceReader(reader, maxSamples, 0)
	}
	return parseV8ProfileReader(reader, format, maxSamples)
}

func parseV8Profile(data []byte, format string) (Parsed, error) {
	return parseV8ProfileReader(bytes.NewReader(data), format, 0)
}

func parseV8ProfileReader(reader io.Reader, format string, maxSamples int) (Parsed, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	var payload v8Profile
	if err := decoder.Decode(&payload); err != nil {
		return Parsed{}, err
	}
	return parsedV8Profile(payload, format, maxSamples)
}

func parsedV8Profile(payload v8Profile, format string, maxSamples int) (Parsed, error) {
	if len(payload.Nodes) == 0 {
		return Parsed{}, fmt.Errorf("V8 profile requires non-empty nodes")
	}
	resolver, err := newV8StackResolver(payload.Nodes, format)
	if err != nil {
		return Parsed{}, err
	}
	if len(payload.TimeDeltas) > 0 && len(payload.TimeDeltas) != len(payload.Samples) {
		return Parsed{}, fmt.Errorf("V8 timeDeltas length %d does not match samples length %d", len(payload.TimeDeltas), len(payload.Samples))
	}
	if len(payload.Samples) == 0 {
		return parsedV8HitCounts(payload, resolver, format)
	}
	collector := newWeightedSampleCollector(len(payload.Samples), maxSamples)
	normalizer := newV8SampleNormalizer(payload.StartTime, collector)
	clamped := 0
	occurrences := map[int]int{}
	for i, nodeID := range payload.Samples {
		occurrences[nodeID]++
		delta := int64(0)
		if i < len(payload.TimeDeltas) {
			delta = payload.TimeDeltas[i]
			if delta < 0 {
				delta = 0
				clamped++
			}
		}
		stack, err := resolver.stackFor(nodeID)
		if err != nil {
			return Parsed{}, err
		}
		normalizer.Add(i, delta, Sample{Stack: stack, Runtime: "V8", Language: "JavaScript", ProfileKind: "cpu", SourceFormat: format})
	}
	end, tailClamped := normalizer.Finish(payload.EndTime)
	parsed := newV8Parsed(format, collector.Samples(), len(payload.Samples), payload.StartTime, end)
	parsed.Metadata["negative_delta_clamp_count"] = clamped
	parsed.Metadata["v8_last_sample_time_us"] = normalizer.LastTimestamp()
	parsed.Metadata["end_time_tail_us"] = normalizer.Tail()
	parsed.Metadata["end_time_tail_clamped"] = tailClamped
	mismatches := 0
	for _, node := range payload.Nodes {
		if node.HitCount != nil && occurrences[node.ID] != *node.HitCount {
			mismatches++
		}
	}
	parsed.Metadata["hit_count_mismatch_nodes"] = mismatches
	return parsed, nil
}

// v8SampleNormalizer keeps observation timestamps separate from attributed
// sample cost. CDP timeDeltas[i] advances to the observation time for sample
// i; that sample owns the interval until the next observation. The final
// sample owns the tail through endTime.
type v8SampleNormalizer struct {
	timestamp    int64
	collector    *weightedSampleCollector
	pending      *Sample
	pendingIndex int
	tail         int64
}

func newV8SampleNormalizer(start int64, collector *weightedSampleCollector) *v8SampleNormalizer {
	return &v8SampleNormalizer{timestamp: start, collector: collector, pendingIndex: -1}
}

func (n *v8SampleNormalizer) Add(index int, delta int64, sample Sample) {
	if n.pending != nil {
		n.pending.Value = delta
		n.collector.Add(n.pendingIndex, *n.pending)
	}
	n.timestamp += delta
	sample.TimestampUS = n.timestamp
	n.pending = &sample
	n.pendingIndex = index
}

func (n *v8SampleNormalizer) Finish(end int64) (int64, bool) {
	if n.pending == nil {
		return end, false
	}
	if end == 0 {
		end = n.timestamp
	}
	n.tail = end - n.timestamp
	tailClamped := n.tail < 0
	if tailClamped {
		n.tail = 0
	}
	n.pending.Value = n.tail
	n.collector.Add(n.pendingIndex, *n.pending)
	n.pending = nil
	return end, tailClamped
}

func (n *v8SampleNormalizer) LastTimestamp() int64 { return n.timestamp }
func (n *v8SampleNormalizer) Tail() int64          { return n.tail }

func parsedV8HitCounts(payload v8Profile, resolver *v8StackResolver, format string) (Parsed, error) {
	samples := make([]Sample, 0)
	total := 0
	for _, node := range payload.Nodes {
		if node.HitCount == nil || *node.HitCount <= 0 {
			continue
		}
		stack, err := resolver.stackFor(node.ID)
		if err != nil {
			return Parsed{}, err
		}
		samples = append(samples, Sample{Stack: stack, Value: int64(*node.HitCount), Runtime: "V8", Language: "JavaScript", ProfileKind: "cpu", SourceFormat: format})
		total += *node.HitCount
	}
	if len(samples) == 0 {
		return newV8Parsed(format, nil, 0, payload.StartTime, payload.EndTime), nil
	}
	return Parsed{Format: format, ValueUnit: "samples", Samples: samples, Metadata: map[string]any{
		"hit_count_only": true, "hit_count_sample_count": total, "hit_count_used_for_duration": false,
		"v8_start_time_us": payload.StartTime, "v8_end_time_us": payload.EndTime,
	}}, nil
}

type v8StackResolver struct {
	nodes   map[int]v8Node
	parents map[int]int
	format  string
	cache   map[int][]Frame
}

func newV8StackResolver(nodeList []v8Node, format string) (*v8StackResolver, error) {
	resolver := &v8StackResolver{nodes: map[int]v8Node{}, parents: map[int]int{}, format: format, cache: map[int][]Frame{}}
	for _, node := range nodeList {
		if node.ID == 0 {
			return nil, fmt.Errorf("V8 profile contains node with id 0")
		}
		if _, exists := resolver.nodes[node.ID]; exists {
			return nil, fmt.Errorf("V8 profile contains duplicate node id %d", node.ID)
		}
		resolver.nodes[node.ID] = node
		for _, child := range node.Children {
			if previous, exists := resolver.parents[child]; exists && previous != node.ID {
				return nil, fmt.Errorf("V8 node %d has multiple parents", child)
			}
			resolver.parents[child] = node.ID
		}
	}
	for child := range resolver.parents {
		if _, ok := resolver.nodes[child]; !ok {
			return nil, fmt.Errorf("V8 profile references missing child node %d", child)
		}
	}
	return resolver, nil
}

func (r *v8StackResolver) stackFor(id int) ([]Frame, error) {
	originalID := id
	if cached, ok := r.cache[originalID]; ok {
		return cached, nil
	}
	var reversed []Frame
	seen := map[int]bool{}
	for id != 0 {
		if seen[id] {
			return nil, fmt.Errorf("V8 profile contains a parent cycle at node %d", id)
		}
		seen[id] = true
		node, ok := r.nodes[id]
		if !ok {
			return nil, fmt.Errorf("V8 sample references missing node %d", id)
		}
		name := firstNonEmpty(node.CallFrame.FunctionName, "(anonymous)")
		file := coreprofiler.RedactText(node.CallFrame.URL).Text
		frame := makeFrame(name, name, file, node.CallFrame.LineNumber+1, r.format)
		frame.Runtime, frame.Language, frame.Kind = "V8", "JavaScript", "managed"
		reversed = append(reversed, frame)
		id = r.parents[id]
	}
	reverseFrames(reversed)
	r.cache[originalID] = reversed
	return reversed, nil
}

// weightedSampleCollector preserves all elapsed time in a bucket. The first
// sample gives each bucket its stable time/stack anchor, while Value is the
// sum of every skipped interval. It is deliberately not a count-scaled
// reservoir: profile duration must remain exact after downsampling.
type weightedSampleCollector struct {
	total, limit, bucket int
	bounded              bool
	samples              []Sample
}

func newWeightedSampleCollector(total, limit int) *weightedSampleCollector {
	if limit <= 0 || total <= limit || total <= 0 {
		return &weightedSampleCollector{total: total, limit: limit, bucket: -1, samples: make([]Sample, 0)}
	}
	return &weightedSampleCollector{total: total, limit: limit, bounded: true, bucket: -1, samples: make([]Sample, 0, limit)}
}

func (c *weightedSampleCollector) Add(index int, sample Sample) {
	if !c.bounded {
		c.samples = append(c.samples, sample)
		return
	}
	bucket := index * c.limit / c.total
	if bucket != c.bucket {
		c.bucket = bucket
		c.samples = append(c.samples, sample)
		return
	}
	c.samples[len(c.samples)-1].Value += sample.Value
}

func (c *weightedSampleCollector) Samples() []Sample { return c.samples }

func newV8Parsed(format string, samples []Sample, original int, start, end int64) Parsed {
	metadata := map[string]any{
		"v8_sample_count": original, "v8_start_time_us": start, "v8_end_time_us": end,
		"v8_duration_source": "timeDeltas", "hit_count_used_for_duration": false,
	}
	if len(samples) < original {
		metadata["partial_result"] = true
		metadata["downsampled_from_samples"] = original
		metadata["downsampled_to_samples"] = len(samples)
		metadata["downsampling"] = "deterministic_time_weighted_buckets"
	}
	return Parsed{Format: format, ValueUnit: "microseconds", Samples: samples, Metadata: metadata}
}

type chromeTraceEvent struct {
	Phase string `json:"ph"`
	Args  struct {
		Data json.RawMessage `json:"data"`
	} `json:"args"`
}

type traceProfileData struct {
	v8Profile
	CPUProfile v8Profile `json:"cpuProfile"`
}

func countChromeTraceSamples(reader io.Reader) (int, error) {
	count := 0
	err := walkChromeTraceEvents(reader, func(event chromeTraceEvent) error {
		if event.Phase != "P" || len(event.Args.Data) == 0 {
			return nil
		}
		var data traceProfileData
		if err := json.Unmarshal(event.Args.Data, &data); err != nil {
			return nil // unrelated ph:P records are not CPU profile chunks
		}
		count += len(data.Samples) + len(data.CPUProfile.Samples)
		return nil
	})
	return count, err
}

func parseChromeTrace(data []byte) (Parsed, error) {
	return parseChromeTraceReader(bytes.NewReader(data), 0, 0)
}

func parseChromeTraceReader(reader io.Reader, maxSamples, totalSamples int) (Parsed, error) {
	if totalSamples == 0 {
		// The non-file compatibility path has no pre-pass. It is still event
		// streaming; without a total there is simply no bounded bucket target.
		maxSamples = 0
	}
	var nodes []v8Node
	foundNodes := false
	foundSamples := false
	start, end := int64(0), int64(0)
	var resolver *v8StackResolver
	collector := newWeightedSampleCollector(totalSamples, maxSamples)
	var normalizer *v8SampleNormalizer
	index := 0
	clamped := 0
	err := walkChromeTraceEvents(reader, func(event chromeTraceEvent) error {
		if event.Phase != "P" || len(event.Args.Data) == 0 {
			return nil
		}
		var data traceProfileData
		if err := json.Unmarshal(event.Args.Data, &data); err != nil {
			return nil
		}
		if start == 0 && data.StartTime != 0 {
			start = data.StartTime
		}
		if data.EndTime > end {
			end = data.EndTime
		}
		if len(data.Nodes) > 0 {
			nodes = append(nodes, data.Nodes...)
			foundNodes = true
			if start == 0 {
				start = data.StartTime
			}
			if data.EndTime > end {
				end = data.EndTime
			}
		}
		if len(data.CPUProfile.Nodes) > 0 {
			nodes = append(nodes, data.CPUProfile.Nodes...)
			foundNodes = true
			if start == 0 {
				start = data.CPUProfile.StartTime
			}
			if data.CPUProfile.EndTime > end {
				end = data.CPUProfile.EndTime
			}
		}
		cpuProfile := data.CPUProfile
		if len(cpuProfile.Samples) > 0 && len(cpuProfile.TimeDeltas) == 0 && len(data.TimeDeltas) > 0 {
			cpuProfile.TimeDeltas = data.TimeDeltas
		}
		for _, chunk := range []v8Profile{data.v8Profile, cpuProfile} {
			if len(chunk.Samples) == 0 {
				continue
			}
			if len(chunk.TimeDeltas) > 0 && len(chunk.TimeDeltas) != len(chunk.Samples) {
				return fmt.Errorf("V8 timeDeltas length %d does not match samples length %d", len(chunk.TimeDeltas), len(chunk.Samples))
			}
			if resolver == nil {
				if !foundNodes {
					return fmt.Errorf("Chrome trace has CPU samples before a Profile node graph")
				}
				var err error
				resolver, err = newV8StackResolver(nodes, "chrome-trace-json")
				if err != nil {
					return err
				}
				normalizer = newV8SampleNormalizer(start, collector)
			}
			foundSamples = true
			for i, nodeID := range chunk.Samples {
				delta := int64(0)
				if len(chunk.TimeDeltas) > 0 {
					delta = chunk.TimeDeltas[i]
					if delta < 0 {
						delta = 0
						clamped++
					}
				}
				stack, err := resolver.stackFor(nodeID)
				if err != nil {
					return err
				}
				normalizer.Add(index, delta, Sample{Stack: stack, Runtime: "V8", Language: "JavaScript", ProfileKind: "cpu", SourceFormat: "chrome-trace-json"})
				index++
			}
		}
		return nil
	})
	if err != nil {
		return Parsed{}, err
	}
	if !foundNodes || !foundSamples {
		return Parsed{}, fmt.Errorf("Chrome trace has no ph:P CPU profile chunks")
	}
	end, tailClamped := normalizer.Finish(end)
	parsed := newV8Parsed("chrome-trace-json", collector.Samples(), index, start, end)
	parsed.Metadata["v8_last_sample_time_us"] = normalizer.LastTimestamp()
	parsed.Metadata["end_time_tail_us"] = normalizer.Tail()
	parsed.Metadata["end_time_tail_clamped"] = tailClamped
	parsed.Metadata["negative_delta_clamp_count"] = clamped
	return parsed, nil
}

func walkChromeTraceEvents(reader io.Reader, visit func(chromeTraceEvent) error) error {
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('['):
		for decoder.More() {
			var event chromeTraceEvent
			if err := decoder.Decode(&event); err != nil {
				return err
			}
			if err := visit(event); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case json.Delim('{'):
		found := false
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, _ := keyToken.(string)
			if key != "traceEvents" {
				var discard json.RawMessage
				if err := decoder.Decode(&discard); err != nil {
					return err
				}
				continue
			}
			found = true
			if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
				if err != nil {
					return err
				}
				return fmt.Errorf("Chrome trace traceEvents must be an array")
			}
			for decoder.More() {
				var event chromeTraceEvent
				if err := decoder.Decode(&event); err != nil {
					return err
				}
				if err := visit(event); err != nil {
					return err
				}
			}
			if _, err := decoder.Token(); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("Chrome trace has no traceEvents")
		}
		return nil
	default:
		return fmt.Errorf("Chrome trace must be an array or object with traceEvents")
	}
}

func isV8Profile(data []byte) bool {
	return bytes.Contains(data, []byte(`"nodes"`)) && bytes.Contains(data, []byte(`"samples"`))
}

func isChromeTrace(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if bytes.Contains(trimmed, []byte(`"traceEvents"`)) {
		return true
	}
	return len(trimmed) > 0 && trimmed[0] == '[' && bytes.Contains(trimmed, []byte(`"ph"`)) &&
		(bytes.Contains(trimmed, []byte(`"Profile"`)) || bytes.Contains(trimmed, []byte(`"cpuProfile"`)))
}
