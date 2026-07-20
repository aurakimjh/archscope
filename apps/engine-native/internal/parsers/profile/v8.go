package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

// parseV8 normalizes a Chrome CPU profile or the Profile chunks in a Chrome
// Performance trace. The timeDeltas[i] duration belongs to samples[i-1]; this
// is Chrome's interval encoding, not a hitCount estimate.
func parseV8(data []byte, format string) (Parsed, error) {
	if format == "chrome-trace-json" {
		return parseChromeTrace(data)
	}
	return parseV8Profile(data, format)
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

func parseV8Profile(data []byte, format string) (Parsed, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload v8Profile
	if err := decoder.Decode(&payload); err != nil {
		return Parsed{}, err
	}
	if len(payload.Nodes) == 0 || len(payload.Samples) == 0 {
		return Parsed{}, fmt.Errorf("V8 profile requires non-empty nodes and samples")
	}
	nodes := map[int]v8Node{}
	parents := map[int]int{}
	for _, node := range payload.Nodes {
		if node.ID == 0 {
			return Parsed{}, fmt.Errorf("V8 profile contains node with id 0")
		}
		if _, exists := nodes[node.ID]; exists {
			return Parsed{}, fmt.Errorf("V8 profile contains duplicate node id %d", node.ID)
		}
		nodes[node.ID] = node
		for _, child := range node.Children {
			if previous, exists := parents[child]; exists && previous != node.ID {
				return Parsed{}, fmt.Errorf("V8 node %d has multiple parents", child)
			}
			parents[child] = node.ID
		}
	}
	for child := range parents {
		if _, ok := nodes[child]; !ok {
			return Parsed{}, fmt.Errorf("V8 profile references missing child node %d", child)
		}
	}
	if len(payload.TimeDeltas) > 0 && len(payload.TimeDeltas) != len(payload.Samples) {
		return Parsed{}, fmt.Errorf("V8 timeDeltas length %d does not match samples length %d", len(payload.TimeDeltas), len(payload.Samples))
	}
	stackFor := func(id int) ([]Frame, error) {
		var reversed []Frame
		seen := map[int]bool{}
		for id != 0 {
			if seen[id] {
				return nil, fmt.Errorf("V8 profile contains a parent cycle at node %d", id)
			}
			seen[id] = true
			node, ok := nodes[id]
			if !ok {
				return nil, fmt.Errorf("V8 sample references missing node %d", id)
			}
			name := firstNonEmpty(node.CallFrame.FunctionName, "(anonymous)")
			// Script URLs are evidence, but query values frequently contain tokens.
			// Keep the stable location while applying the shared profile redaction.
			file := coreprofiler.RedactText(node.CallFrame.URL).Text
			frame := makeFrame(name, name, file, node.CallFrame.LineNumber+1, format)
			frame.Runtime, frame.Language, frame.Kind = "V8", "JavaScript", "managed"
			reversed = append(reversed, frame)
			id = parents[id]
		}
		reverseFrames(reversed)
		return reversed, nil
	}
	samples := make([]Sample, 0, len(payload.Samples))
	timestamp := payload.StartTime
	for i, nodeID := range payload.Samples {
		stack, err := stackFor(nodeID)
		if err != nil {
			return Parsed{}, err
		}
		value := int64(0)
		if len(payload.TimeDeltas) > 0 && i > 0 {
			value = payload.TimeDeltas[i]
		}
		if value < 0 {
			return Parsed{}, fmt.Errorf("V8 profile contains negative time delta at sample %d", i)
		}
		samples = append(samples, Sample{Stack: stack, Value: value, TimestampUS: timestamp, Runtime: "V8", Language: "JavaScript", ProfileKind: "cpu", SourceFormat: format})
		if i < len(payload.TimeDeltas) {
			timestamp += payload.TimeDeltas[i]
		}
	}
	return Parsed{Format: format, ValueUnit: "microseconds", Samples: samples, Metadata: map[string]any{
		"v8_sample_count": len(samples), "v8_start_time_us": payload.StartTime, "v8_end_time_us": payload.EndTime,
		"v8_duration_source": "timeDeltas", "hit_count_used_for_duration": false,
	}}, nil
}

func parseChromeTrace(data []byte) (Parsed, error) {
	var envelope struct {
		TraceEvents []json.RawMessage `json:"traceEvents"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Parsed{}, err
	}
	if len(envelope.TraceEvents) == 0 {
		return Parsed{}, fmt.Errorf("Chrome trace has no traceEvents")
	}
	type chunk struct {
		PID  int    `json:"pid"`
		TID  int    `json:"tid"`
		TS   int64  `json:"ts"`
		Name string `json:"name"`
		Args struct {
			Data json.RawMessage `json:"data"`
		} `json:"args"`
	}
	chunks := []chunk{}
	for _, raw := range envelope.TraceEvents {
		var event struct {
			Phase string `json:"ph"`
			PID   int    `json:"pid"`
			TID   int    `json:"tid"`
			TS    int64  `json:"ts"`
			Name  string `json:"name"`
			Args  struct {
				Data json.RawMessage `json:"data"`
			} `json:"args"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			continue
		}
		if event.Phase != "P" || len(event.Args.Data) == 0 {
			continue
		}
		chunks = append(chunks, chunk{event.PID, event.TID, event.TS, event.Name, event.Args})
	}
	if len(chunks) == 0 {
		return Parsed{}, fmt.Errorf("Chrome trace has no ph:P CPU profile chunks")
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].TS < chunks[j].TS })
	// Chrome stores a complete profile object in ProfileChunk data for exported
	// traces. Supporting this shape gives a deterministic first adapter; other
	// trace event modelling intentionally remains outside Phase 1.
	for _, chunk := range chunks {
		var profile v8Profile
		if err := json.Unmarshal(chunk.Args.Data, &profile); err == nil && len(profile.Nodes) > 0 {
			encoded, _ := json.Marshal(profile)
			return parseV8Profile(encoded, "chrome-trace-json")
		}
	}
	return Parsed{}, fmt.Errorf("Chrome trace ph:P chunks did not contain a complete CPU profile")
}

func isV8Profile(data []byte) bool {
	var probe struct {
		Nodes   json.RawMessage `json:"nodes"`
		Samples json.RawMessage `json:"samples"`
	}
	return json.Unmarshal(data, &probe) == nil && len(probe.Nodes) > 0 && len(probe.Samples) > 0
}

func isChromeTrace(data []byte) bool { return strings.Contains(stringPreview(data), "\"traceEvents\"") }
