package runtimeevidence

import (
	"bufio"
	"bytes"
	"fmt"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"regexp"
	"strconv"
)

var rustFrame = regexp.MustCompile(`^\s*[0-9]+:\s+(.+?)\s*$`)
var rustLocation = regexp.MustCompile(`^\s*at\s+(.+):(\d+):(\d+)\s*$`)

func (b *builder) rustBacktrace(raw []byte) error {
	if e := b.process(); e != nil {
		return e
	}
	frames := []frame{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	panicObserved := false
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if bytes.Contains([]byte(text), []byte("panicked at")) {
			panicObserved = true
		}
		if v := rustFrame.FindStringSubmatch(text); v != nil {
			frames = append(frames, frame{Symbol: v[1]})
			if len(frames) > 1024 {
				return fmt.Errorf("RUNTIME_STACK_DEPTH_BUDGET")
			}
		} else if v := rustLocation.FindStringSubmatch(text); v != nil && len(frames) > 0 {
			n, e := strconv.ParseInt(v[2], 10, 64)
			if e != nil {
				return e
			}
			frames[len(frames)-1].File = v[1]
			frames[len(frames)-1].Line = &n
		}
	}
	if e := scanner.Err(); e != nil {
		return e
	}
	if !panicObserved || len(frames) == 0 {
		return fmt.Errorf("RUNTIME_RUST_BACKTRACE_REQUIRED")
	}
	id := b.entity("runtime_thread", "panic-thread", "panic thread (native TID unobserved)", "", "none")
	return b.add(r.Observation{Entity: id, Kind: "panic_snapshot", Basis: "snapshot", State: "panicked", Time: b.options.Time, Stack: b.stack(frames), Ref: r.Ref{Position: fmt.Sprintf("lines:1-%d; panic message excluded", line)}})
}
