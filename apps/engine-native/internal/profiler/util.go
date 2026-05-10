// ─────────────────────────────────────────────────────────────────────
// [한글] util — profiler 패키지 공용 helper 함수 모음.
//
// 트리비얼한 helper 들이라 함수별 한글 주석 없이 헤더 블록 설명으로 갈음.
//
//   - round(value, places)    : 4/6 자리 round (math.Round 기반)
//   - ratio(part, total, p)   : 백분율 계산. total<=0 면 0.
//   - elapsedRatio            : seconds / *elapsed * 100. elapsed nil 이면 nil.
//   - splitStack(stack)       : ";" split + trim + 빈 frame drop
//   - joinPath(path)          : ";" join
//   - topCounter(counter, n)  : counter map → samples DESC 정렬 + topN cap
//   - increment(counter, k, n): counter[k] += n
//   - safePreview(value)      : trim + 최대 160자 자르기 (raw_preview용)
//   - stringPtr / floatPtr    : 포인터 helper (option/nullable JSON 표현)
//   - readAllUTF8(path)       : os.ReadFile thin wrapper
//
// 모든 round/ratio 결과의 자릿수는 Python 원본과 byte-level 동등이어야
// 한다. 임의로 places 를 바꾸면 frontend snapshot 이 깨진다.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"math"
	"os"
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
	return splitStackInto(nil, stack)
}

func splitStackInto(dst []string, stack string) []string {
	dst = dst[:0]
	start := 0
	for i := 0; i <= len(stack); i++ {
		if i < len(stack) && stack[i] != ';' {
			continue
		}
		part := stack[start:i]
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			dst = append(dst, trimmed)
		}
		start = i + 1
	}
	return dst
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

func readAllUTF8(path string) ([]byte, error) {
	return os.ReadFile(path)
}
