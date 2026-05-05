package profiler

import (
	"math"
	"sort"
	"strings"
)

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

func ratio(part, total int, places int) float64 {
	if total <= 0 {
		return 0
	}
	return round(float64(part)/float64(total)*100, places)
}

func elapsedRatio(seconds float64, elapsed *float64, places int) *float64 {
	if elapsed == nil || *elapsed <= 0 {
		return nil
	}
	value := round(seconds/(*elapsed)*100, places)
	return &value
}

func splitStack(stack string) []string {
	parts := strings.Split(stack, ";")
	frames := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			frames = append(frames, trimmed)
		}
	}
	return frames
}

func joinPath(path []string) string {
	return strings.Join(path, ";")
}

func topCounter(counter map[string]int, limit int) []TopItem {
	keys := make([]string, 0, len(counter))
	for key := range counter {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counter[keys[i]] == counter[keys[j]] {
			return keys[i] < keys[j]
		}
		return counter[keys[i]] > counter[keys[j]]
	})
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	out := make([]TopItem, 0, len(keys))
	for _, key := range keys {
		out = append(out, TopItem{Name: key, Samples: counter[key]})
	}
	return out
}

func increment(counter map[string]int, key string, samples int) {
	counter[key] += samples
}

func safePreview(value string) string {
	const max = 160
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func stringPtr(value string) *string {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
