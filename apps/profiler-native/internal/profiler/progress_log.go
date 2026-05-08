package profiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ProgressLog is a streaming, append-mode log dedicated to long-
// running profiler analyses. It lives next to DebugLog (which is
// post-hoc / structured) but solves a different problem: when a
// 400 MB wall profile causes the desktop app to die, the user wants
// a tailable plain-text trace they can read AFTER the crash to
// understand which phase dropped them.
//
// The log records:
//
//   - one line per phase boundary (open / parse / build / freeze /
//     finalize)
//   - periodic progress ticks during long parses with byte / line /
//     unique-stack counters and a live RSS snapshot
//   - panic recoveries flushed before the goroutine unwinds
//
// Output is line-oriented and flushed (Sync) on every write so a
// hard process kill still leaves the most recent line on disk.
type ProgressLog struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	disabled bool
	started  time.Time
}

// OpenProgressLog opens (or creates, append-mode) a log file under
// `dir`. When dir is empty, the default search order is:
//
//  1. `<dir-of-current-exe>/archscope-logs/` — easiest place for the
//     user to find ("right next to the .exe I just ran"). On Windows
//     this lands in the install / dev folder; on macOS it's inside
//     the .app bundle's MacOS directory.
//  2. `<cwd>/archscope-logs/` — fallback when os.Executable() fails
//     (rare).
//  3. `<os.TempDir>/archscope-logs/` — last resort.
//
// We also print the log path to stderr the moment the file is
// created so a user with a console attached sees it before any
// crash; the same path is mirrored back to the renderer through
// AnalysisResult.Metadata.ProgressLogPath.
func OpenProgressLog(dir, source string) (*ProgressLog, error) {
	if dir == "" {
		dir = defaultProgressLogDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		// If the chosen dir refuses (read-only mount, sandbox), fall
		// through to the temp dir so the user always gets *some* log.
		dir = filepath.Join(os.TempDir(), "archscope-logs")
		if err2 := os.MkdirAll(dir, 0o755); err2 != nil {
			return nil, err2
		}
	}
	stamp := time.Now().Format("20060102-150405")
	base := filepath.Base(source)
	if base == "" || base == "." || base == "/" {
		base = "analysis"
	}
	name := fmt.Sprintf("profiler-%s-%s.log", stamp, base)
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	pl := &ProgressLog{file: f, path: path, started: time.Now()}
	pl.writeRaw(fmt.Sprintf("=== ArchScope profiler progress log\n"))
	pl.writeRaw(fmt.Sprintf("    started   : %s\n", pl.started.Format(time.RFC3339)))
	pl.writeRaw(fmt.Sprintf("    source    : %s\n", source))
	pl.writeRaw(fmt.Sprintf("    pid       : %d\n", os.Getpid()))
	pl.writeRaw(fmt.Sprintf("    go runtime: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH))
	// Announce the path on stderr so a console-attached user sees it
	// before the analysis even starts. This is cheap insurance for
	// the case where the OS kills the process before the renderer
	// gets the result back.
	fmt.Fprintf(os.Stderr, "[archscope] progress log: %s\n", path)
	return pl, nil
}

// defaultProgressLogDir picks the most discoverable location.
func defaultProgressLogDir() string {
	if exe, err := os.Executable(); err == nil {
		// `os.Executable` may return a symlink — Windows packagers
		// don't symlink so this is fine; on Linux .desktop launchers
		// it's still the real binary path.
		return filepath.Join(filepath.Dir(exe), "archscope-logs")
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "archscope-logs")
	}
	return filepath.Join(os.TempDir(), "archscope-logs")
}

// Path returns the on-disk path so callers can surface it back to
// the renderer ("look at this log when things go wrong").
func (p *ProgressLog) Path() string {
	if p == nil {
		return ""
	}
	return p.path
}

// Phase records a phase boundary. Phases are short verbs ("parse-
// start", "parse-end", "build-tree", "freeze", "finalize", "panic").
func (p *ProgressLog) Phase(name string, args ...any) {
	if p == nil || p.disabled {
		return
	}
	msg := name
	if len(args) > 0 {
		msg = fmt.Sprintf(name+" "+strings.Repeat("%v ", len(args)), args...)
	}
	p.write("PHASE " + msg)
}

// Tick writes a progress line. Phases like "parse" call this every
// N lines or every M ms — whichever is cheaper at the call site.
// The ProgressLog itself doesn't time-throttle so callers stay in
// control of cadence.
func (p *ProgressLog) Tick(format string, args ...any) {
	if p == nil || p.disabled {
		return
	}
	p.write(fmt.Sprintf(format, args...))
}

// Mem snapshots runtime stats. Cheap enough to call once per tick.
func (p *ProgressLog) Mem(label string) {
	if p == nil || p.disabled {
		return
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	p.write(fmt.Sprintf("MEM   %s alloc=%dMB sys=%dMB heap_objs=%d gc=%d",
		label,
		ms.Alloc/1024/1024,
		ms.Sys/1024/1024,
		ms.HeapObjects,
		ms.NumGC,
	))
}

// Panicf is meant to be deferred so a panic during analysis still
// flushes a final marker. Re-raises after writing.
func (p *ProgressLog) Recover(phase string) {
	if p == nil {
		return
	}
	if r := recover(); r != nil {
		buf := make([]byte, 16*1024)
		n := runtime.Stack(buf, false)
		p.write(fmt.Sprintf("PANIC %s: %v", phase, r))
		p.writeRaw(string(buf[:n]))
		p.writeRaw("\n")
		_ = p.file.Sync()
		// Re-panic so the host still sees the failure.
		panic(r)
	}
}

// Close finalizes the log with an elapsed-time footer.
func (p *ProgressLog) Close() {
	if p == nil || p.file == nil {
		return
	}
	p.write(fmt.Sprintf("=== done in %s", time.Since(p.started)))
	_ = p.file.Sync()
	_ = p.file.Close()
	p.file = nil
}

// write prepends an elapsed-since-start prefix and a newline.
// Internal — assumes msg is the human-readable body only.
func (p *ProgressLog) write(msg string) {
	if p == nil || p.file == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := time.Since(p.started).Truncate(time.Millisecond)
	line := fmt.Sprintf("[%9s] %s\n", elapsed, msg)
	if _, err := p.file.WriteString(line); err != nil {
		p.disabled = true
		return
	}
	_ = p.file.Sync()
}

func (p *ProgressLog) writeRaw(line string) {
	if p == nil || p.file == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.file.WriteString(line); err != nil {
		p.disabled = true
		return
	}
	_ = p.file.Sync()
}
