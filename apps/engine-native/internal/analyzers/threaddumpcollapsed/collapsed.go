// Package threaddumpcollapsed ports
// archscope_engine.analyzers.thread_dump_to_collapsed (T-216 / T-338).
//
// The converter folds a slice of already-parsed ThreadDumpBundles into
// the FlameGraph "collapsed stack" format Brendan Gregg's
// flamegraph.pl consumes:
//
//	frame_root;...;frame_leaf <count>
//
// One line per unique stack across every snapshot in every bundle, with
// identical stacks aggregated by sample count.
//
// Per-language enrichment (proxy normalization, runtime state inference,
// etc.) is the upstream parser plugins' responsibility — this package
// only flattens whatever frames the snapshot already carries.
//
// The Go API is intentionally narrower than Python's: the parser /
// registry pipeline is exposed elsewhere, so we operate on
// []models.ThreadDumpBundle directly. Callers that want
// the file-IO wrapper use WriteCollapsed.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] threaddumpcollapsed — thread dump → FlameGraph collapsed
// 변환기.
//
// 출력 형식 (Brendan Gregg flamegraph.pl 표준)
//   frame_root;frame_mid;frame_leaf <count>
//
//   예) java.lang.Thread.run;com.foo.Worker.process;com.foo.Db.query 3
//
//   같은 frame 시퀀스를 가진 스레드가 N개면 한 줄로 합쳐짐.
//
// 알고리즘
//   1) 모든 bundle 의 모든 snapshot 을 순회.
//   2) IncludeThreadName == true 면 thread name 을 root frame 으로
//      prepend(스레드별 색상 구분에 유리).
//   3) frame.Render() 결과를 ";" 로 join 한 문자열을 키로 한 카운터
//      누적.
//   4) Convert 는 map[string]int 반환(parity gate 용 머신리더블),
//      WriteCollapsed 는 사람이 읽는 텍스트 파일을 작성.
//
// IncludeThreadName trade-off
//   • true 라면 같은 스택을 두 thread name 이 따로 카운트 → 색상 구분.
//   • false 라면 같은 코드 경로면 합쳐짐 → 통계가 두꺼워져 더 많은
//     호출경로를 한눈에. 큰 덤프 분석에 유용.
//
// 다언어 안전성
//   plugin 이 채운 frame.Render() 만 사용. Java/Go/Python/Node.js/.NET
//   어떤 언어든 같은 변환기로 처리.
package threaddumpcollapsed

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Options mirrors the keyword arguments of the Python converter. The
// zero value matches Python's defaults (IncludeThreadName=true) when
// constructed via DefaultOptions.
type Options struct {
	// IncludeThreadName prepends the snapshot's thread name as the
	// leftmost (root) frame so threads with otherwise identical stacks
	// stay distinguishable in the flamegraph. Mirrors Python's
	// `include_thread_name` keyword (default True).
	IncludeThreadName bool
}

// DefaultOptions returns the converter defaults: IncludeThreadName is
// true to match Python's behavior.
func DefaultOptions() Options {
	return Options{IncludeThreadName: true}
}

// Convert collapses the snapshots in `bundles` into a map of
// `stack -> count`. Stack keys take the form `frame_root;...;frame_leaf`
// with `;`/newlines stripped from each frame. Empty-stack snapshots are
// skipped (mirroring Python's `if not stack: continue`).
//
// The returned map is the analogue of Python's `Counter[str]`. Callers
// that want a deterministic iteration order should pair this with
// SortedLines.
func Convert(bundles []models.ThreadDumpBundle, opts Options) map[string]int {
	counts := make(map[string]int)
	for _, bundle := range bundles {
		for _, snapshot := range bundle.Snapshots {
			stack := collapseSnapshot(snapshot, opts.IncludeThreadName)
			if stack == "" {
				continue
			}
			counts[stack]++
		}
	}
	return counts
}

// SortedLines returns the collapsed counts as `"<stack> <count>"`
// strings, ordered like Python's `Counter.most_common()`: descending
// by count first, then ascending by stack key for stability (Python
// breaks ties by insertion order — Go maps have no insertion order, so
// lex-by-stack is the closest deterministic substitute).
func SortedLines(counts map[string]int) []string {
	type entry struct {
		stack string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for stack, count := range counts {
		entries = append(entries, entry{stack: stack, count: count})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].stack < entries[j].stack
	})
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = fmt.Sprintf("%s %d", e.stack, e.count)
	}
	return out
}

// WriteCollapsed runs Convert + SortedLines and writes the result to
// `outputPath`, creating parent directories as needed. Returns the
// number of unique stacks written (== unique lines).
func WriteCollapsed(bundles []models.ThreadDumpBundle, outputPath string, opts Options) (int, error) {
	if outputPath == "" {
		return 0, fmt.Errorf("threaddumpcollapsed: output path must be non-empty")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 0, fmt.Errorf("threaddumpcollapsed: create parent dir: %w", err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("threaddumpcollapsed: open %s: %w", outputPath, err)
	}
	defer file.Close()
	counts := Convert(bundles, opts)
	return writeLines(file, counts)
}

// writeLines emits the sorted lines to `w`. Split out so tests can
// exercise the formatting path without touching the filesystem.
func writeLines(w io.Writer, counts map[string]int) (int, error) {
	for _, line := range SortedLines(counts) {
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return 0, err
		}
	}
	return len(counts), nil
}

// collapseSnapshot mirrors Python's `_collapse_snapshot`. Frames are
// reversed (most runtime dumps print top-of-stack first; collapsed
// format wants caller-first ordering) and joined with `;`. The thread
// name, when included, becomes the leftmost root frame.
func collapseSnapshot(snapshot models.ThreadSnapshot, includeThreadName bool) string {
	frames := snapshot.StackFrames
	if len(frames) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(frames)+1)
	if includeThreadName {
		rendered = append(rendered, sanitize(snapshot.ThreadName))
	}
	for i := len(frames) - 1; i >= 0; i-- {
		rendered = append(rendered, renderFrame(frames[i]))
	}
	return strings.Join(rendered, ";")
}

// renderFrame mirrors Python's `_render_frame`: `module.function` when
// module is set, otherwise the bare function. File/line are
// intentionally omitted — the collapsed format only carries the
// callable identity. Differs from `StackFrame.Render()`, which appends
// the source location.
func renderFrame(frame models.StackFrame) string {
	text := frame.Function
	if frame.Module != nil && *frame.Module != "" {
		text = *frame.Module + "." + frame.Function
	}
	return sanitize(text)
}

// sanitize mirrors Python's `_sanitize`: replace `;` (the row
// separator) with `_`, fold newlines into spaces so a stray frame can't
// split a line, then trim surrounding whitespace.
func sanitize(text string) string {
	text = strings.ReplaceAll(text, ";", "_")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return strings.TrimSpace(text)
}
