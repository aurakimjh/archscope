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
	if len(payload.Nodes) == 0 || len(payload.Samples) == 0 {
		return Parsed{}, fmt.Errorf("V8 profile requires non-empty nodes and samples")
	}
	resolver, err := newV8StackResolver(payload.Nodes, format)
	if err != nil {
		return Parsed{}, err
	}
	if len(payload.TimeDeltas) > 0 && len(payload.TimeDeltas) != len(payload.Samples) {
		return Parsed{}, fmt.Errorf("V8 timeDeltas length %d does not match samples length %d", len(payload.TimeDeltas), len(payload.Samples))
	}
	collector := newWeightedSampleCollector(len(payload.Samples), maxSamples)
	timestamp := payload.StartTime
	for i, nodeID := range payload.Samples {
		value := int64(0)
		if len(payload.TimeDeltas) > 0 && i > 0 {
			value = payload.TimeDeltas[i]
		}
		if value < 0 {
			return Parsed{}, fmt.Errorf("V8 profile contains negative time delta at sample %d", i)
		}
		stack, err := resolver.stackFor(nodeID)
		if err != nil {
			return Parsed{}, err
		}
		collector.Add(i, Sample{Stack: stack, Value: value, TimestampUS: timestamp, Runtime: "V8", Language: "JavaScript", ProfileKind: "cpu", SourceFormat: format})
		if i < len(payload.TimeDeltas) {
			timestamp += payload.TimeDeltas[i]
		}
	}
	return newV8Parsed(format, collector.Samples(), len(payload.Samples), payload.StartTime, payload.EndTime), nil
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
	start, end, timestamp := int64(0), int64(0), int64(0)
	var resolver *v8StackResolver
	collector := newWeightedSampleCollector(totalSamples, maxSamples)
	index := 0
	err := walkChromeTraceEvents(reader, func(event chromeTraceEvent) error {
		if event.Phase != "P" || len(event.Args.Data) == 0 {
			return nil
		}
		var data traceProfileData
		if err := json.Unmarshal(event.Args.Data, &data); err != nil {
			return nil
		}
		if len(data.Nodes) > 0 {
			nodes = append(nodes, data.Nodes...)
			foundNodes = true
			if start == 0 {
				start = data.StartTime
				timestamp = start
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
				timestamp = start
			}
			if data.CPUProfile.EndTime > end {
				end = data.CPUProfile.EndTime
			}
		}
		for _, chunk := range []v8Profile{data.v8Profile, data.CPUProfile} {
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
			}
			foundSamples = true
			for i, nodeID := range chunk.Samples {
				value := int64(0)
				if len(chunk.TimeDeltas) > 0 && index > 0 {
					value = chunk.TimeDeltas[i]
				}
				if value < 0 {
					return fmt.Errorf("V8 profile contains negative time delta at sample %d", index)
				}
				stack, err := resolver.stackFor(nodeID)
				if err != nil {
					return err
				}
				collector.Add(index, Sample{Stack: stack, Value: value, TimestampUS: timestamp, Runtime: "V8", Language: "JavaScript", ProfileKind: "cpu", SourceFormat: "chrome-trace-json"})
				if i < len(chunk.TimeDeltas) {
					timestamp += chunk.TimeDeltas[i]
				}
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
	return newV8Parsed("chrome-trace-json", collector.Samples(), index, start, end), nil
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

func isChromeTrace(data []byte) bool { return bytes.Contains(data, []byte(`"traceEvents"`)) }
