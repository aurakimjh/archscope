package javajcmdjson

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// These helpers mirror the Python jstack module's private functions
// (`_frame_from_jstack`, `_normalize_proxy_frame`, `_infer_java_state`,
// `_extract_lock_handles`, `_carrier_pinning`). They live in this
// package — not in a shared one — because the Go jstack plugin
// (T-321 sibling) hasn't landed yet. When it does, hoist these to
// internal/threaddump/javacommon and dedupe.

// javaFrameRE matches `qualified.method(File.java:42)` and friends.
// Mirrors Python `_JAVA_FRAME_RE`.
var javaFrameRE = regexp.MustCompile(`^([\w$./<>]+)\(([^)]*)\)\s*$`)

// frameFromJstack parses one rendered jstack frame line into a
// StackFrame. Falls back to a function-only frame on no match — same
// shape as Python `_frame_from_jstack`.
func frameFromJstack(line string) models.StackFrame {
	text := strings.TrimSpace(line)
	lang := Language
	match := javaFrameRE.FindStringSubmatch(text)
	if match == nil {
		return models.StackFrame{Function: text, Language: &lang}
	}
	qualified := match[1]
	if idx := strings.Index(qualified, "/"); idx >= 0 {
		qualified = qualified[idx+1:]
	}
	location := match[2]

	var module *string
	function := qualified
	if dot := strings.LastIndex(qualified, "."); dot >= 0 {
		mod := qualified[:dot]
		module = &mod
		function = qualified[dot+1:]
	}

	var filePart *string
	var linePart *int
	if location != "" {
		if colon := strings.LastIndex(location, ":"); colon >= 0 {
			filePartStr := location[:colon]
			lineText := location[colon+1:]
			if n, err := strconv.Atoi(lineText); err == nil {
				linePart = &n
				if filePartStr != "" {
					filePart = &filePartStr
				}
			} else {
				whole := location
				filePart = &whole
			}
		} else {
			whole := location
			filePart = &whole
		}
	}

	return models.StackFrame{
		Function: function,
		Module:   module,
		File:     filePart,
		Line:     linePart,
		Language: &lang,
	}
}

// proxyPattern is one entry in the Python `_PROXY_PATTERNS` table.
// Replacement may be empty, in which case the match is stripped.
type proxyPattern struct {
	re          *regexp.Regexp
	replacement string
}

// proxyPatterns mirrors Python `_PROXY_PATTERNS` exactly. Order matters
// — the longest-prefix variants must come first so we don't strip a
// shared substring twice.
var proxyPatterns = []proxyPattern{
	{regexp.MustCompile(`\$\$EnhancerByCGLIB\$\$[\w$]+`), ""},
	{regexp.MustCompile(`\$\$FastClassByCGLIB\$\$[\w$]+`), ""},
	{regexp.MustCompile(`(\$\$Proxy)\d+`), "$1"},
	{regexp.MustCompile(`(GeneratedMethodAccessor)\d+`), "$1"},
	{regexp.MustCompile(`(GeneratedConstructorAccessor)\d+`), "$1"},
	{regexp.MustCompile(`(Accessor)\d+`), "$1"},
}
var doubleDotRE = regexp.MustCompile(`\.{2,}`)

func normalizeProxyText(text string) string {
	for _, p := range proxyPatterns {
		text = p.re.ReplaceAllString(text, p.replacement)
	}
	text = doubleDotRE.ReplaceAllString(text, ".")
	return strings.Trim(text, ".")
}

// normalizeProxyFrame strips CGLIB/proxy/Accessor hash suffixes from a
// Java frame so the stack signature collapses across snapshots. Returns
// the original frame untouched for non-Java frames.
func normalizeProxyFrame(frame models.StackFrame) models.StackFrame {
	if frame.Language == nil || *frame.Language != Language {
		return frame
	}
	newFunction := normalizeProxyText(frame.Function)
	var newModule *string
	if frame.Module != nil {
		m := normalizeProxyText(*frame.Module)
		newModule = &m
	}
	if newFunction == frame.Function && samePtr(frame.Module, newModule) {
		return frame
	}
	frame.Function = newFunction
	frame.Module = newModule
	return frame
}

func samePtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// inferJavaState promotes a RUNNABLE thread to NETWORK_WAIT / IO_WAIT
// when the top frame indicates the thread is parked in I/O. Mirrors
// Python `_infer_java_state` — runtime-reported non-RUNNABLE states
// always win.
var (
	networkWaitRE = regexp.MustCompile(
		`(?:` +
			`epollWait|EPoll(?:Selector)?\.wait|EPollArrayWrapper\.poll|` +
			`socketAccept|Socket\.\w*accept0|NioSocketImpl\.accept|` +
			`socketRead0|SocketInputStream\.socketRead|` +
			`NioSocketImpl\.read|SocketChannelImpl\.read|` +
			`SocketDispatcher\.read|sun\.nio\.ch\.\w+Selector\.\w*poll|` +
			`netty\.\w+EventLoop\.\w*select|netty\.channel\.nio\.NioEventLoop\.run` +
			`)`,
	)
	ioWaitRE = regexp.MustCompile(
		`(?:` +
			`FileInputStream\.read|FileInputStream\.readBytes|` +
			`FileChannelImpl\.read|FileChannelImpl\.transferFrom|` +
			`RandomAccessFile\.read|RandomAccessFile\.readBytes|` +
			`BufferedReader\.readLine|FileDispatcherImpl\.\w+|` +
			`java\.io\.FileInputStream\.read` +
			`)`,
	)
)

func inferJavaState(state models.ThreadState, frames []models.StackFrame) models.ThreadState {
	if state != models.ThreadStateRunnable || len(frames) == 0 {
		return state
	}
	top := frames[0]
	parts := make([]string, 0, 2)
	if top.Module != nil && *top.Module != "" {
		parts = append(parts, *top.Module+"."+top.Function)
	}
	parts = append(parts, top.Function)
	text := strings.Join(parts, " ")
	if networkWaitRE.MatchString(text) {
		return models.ThreadStateNetworkWait
	}
	if ioWaitRE.MatchString(text) {
		return models.ThreadStateIOWait
	}
	return state
}

// Lock extraction (text fallback). Mirrors the Python jstack module's
// `_extract_lock_handles` — the regexes operate on `raw_block` which,
// for jcmd JSON, is the compact-rendered thread object so e.g. a
// `lockedMonitors` entry rendered as `<0x...>` text is still picked up
// when the structured arrays are missing.

var (
	lockedRE = regexp.MustCompile(
		`-\s+locked\s+<(0x[0-9a-fA-F]+)>(?:\s+\(a\s+([^)]+)\))?`,
	)
	waitingToLockRE = regexp.MustCompile(
		`-\s+waiting to lock\s+<(0x[0-9a-fA-F]+)>(?:\s+\(a\s+([^)]+)\))?`,
	)
	waitingOnRE = regexp.MustCompile(
		`-\s+waiting on\s+<(0x[0-9a-fA-F]+)>(?:\s+\(a\s+([^)]+)\))?`,
	)
	parkingRE = regexp.MustCompile(
		`-\s+parking to wait for\s+<(0x[0-9a-fA-F]+)>(?:\s+\(a\s+([^)]+)\))?`,
	)
)

// waitPattern bundles a lock-wait regex with its label for the
// (regex, wait_mode) tuple loop.
type waitPattern struct {
	re       *regexp.Regexp
	waitMode string
}

var waitPatterns = []waitPattern{
	{waitingToLockRE, "lock_entry_wait"},
	{waitingOnRE, "object_wait"},
	{parkingRE, "parking_condition_wait"},
}

func extractLockHandlesText(rawBlock string) ([]models.LockHandle, *models.LockHandle) {
	holds := []models.LockHandle{}
	seenIDs := map[string]struct{}{}
	var waiting *models.LockHandle
	for _, raw := range strings.Split(rawBlock, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if waiting == nil {
			for _, p := range waitPatterns {
				if m := p.re.FindStringSubmatch(line); m != nil {
					waiting = &models.LockHandle{
						LockID:    m[1],
						LockClass: ptrIfNonEmpty(m[2]),
						WaitMode:  p.waitMode,
					}
					break
				}
			}
		}
		if m := lockedRE.FindStringSubmatch(line); m != nil {
			lockID := m[1]
			if _, dup := seenIDs[lockID]; dup {
				continue
			}
			seenIDs[lockID] = struct{}{}
			holds = append(holds, models.LockHandle{
				LockID:    lockID,
				LockClass: ptrIfNonEmpty(m[2]),
				WaitMode:  "locked_owner",
			})
		}
	}
	if waiting != nil {
		if _, owns := seenIDs[waiting.LockID]; owns {
			waiting = nil
		}
	}
	return holds, waiting
}

// extractJSONLocks combines structured arrays (`lockedMonitors`,
// `lockedSynchronizers`, `lockedOwnableSynchronizers`) plus
// (`lockInfo` / `blockedOn` / `waitingOn`) with a text-mode fallback
// over the compact-rendered raw text. Mirrors Python
// `_extract_json_locks` byte-for-byte.
func extractJSONLocks(thread map[string]any, rawText string) ([]models.LockHandle, *models.LockHandle) {
	holds := []models.LockHandle{}
	seen := map[string]struct{}{}
	for _, key := range []string{"lockedMonitors", "lockedSynchronizers", "lockedOwnableSynchronizers"} {
		for _, item := range asList(thread[key]) {
			if lock := lockFromJSON(item, "locked_owner"); lock != nil {
				if _, dup := seen[lock.LockID]; dup {
					continue
				}
				seen[lock.LockID] = struct{}{}
				holds = append(holds, *lock)
			}
		}
	}
	var waiting *models.LockHandle
	for _, key := range []string{"lockInfo", "blockedOn", "waitingOn"} {
		if value, ok := thread[key]; ok && value != nil {
			if lock := lockFromJSON(value, "lock_entry_wait"); lock != nil {
				waiting = lock
				break
			}
		}
	}
	if waiting == nil {
		textHolds, textWaiting := extractLockHandlesText(rawText)
		for _, hold := range textHolds {
			if _, dup := seen[hold.LockID]; dup {
				continue
			}
			seen[hold.LockID] = struct{}{}
			holds = append(holds, hold)
		}
		waiting = textWaiting
	}
	if waiting != nil {
		if _, owns := seen[waiting.LockID]; owns {
			waiting = nil
		}
	}
	return holds, waiting
}

// lockFromJSON maps one JSON lock object (`{"className": ..., "identityHashCode": ...}`)
// to a LockHandle. Returns nil for malformed / empty entries. Mirrors
// Python `_lock_from_json`, including the decimal-id -> hex coercion
// (`identityHashCode` is sometimes emitted as a positive int).
func lockFromJSON(item any, waitMode string) *models.LockHandle {
	dict, ok := item.(map[string]any)
	if !ok {
		return nil
	}
	lockClass := firstString(dict, "className", "class", "type")
	rawID := firstString(dict, "identityHashCode", "id", "address", "lockId")
	if rawID == "" && lockClass == "" {
		return nil
	}
	lockID := rawID
	if lockID == "" {
		lockID = lockClass
	}
	if lockID == "" {
		lockID = "unknown-lock"
	}
	if rawID != "" && isAllDigits(rawID) {
		if n, err := strconv.ParseInt(rawID, 10, 64); err == nil {
			lockID = "0x" + strconv.FormatInt(n, 16)
		}
	}
	return &models.LockHandle{
		LockID:    lockID,
		LockClass: ptrIfNonEmpty(lockClass),
		WaitMode:  waitMode,
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ptrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// asList wraps a value in a one-element list when it isn't already a
// list (mirrors Python `_as_list`). Nil -> empty slice.
func asList(value any) []any {
	if value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{value}
}

// isVirtualJSONThread mirrors Python `_is_virtual_json_thread`. The
// `virtual: true` flag is the JDK 21+ canonical marker; the lowercase
// substring fallback catches older preview builds and JFR exports.
func isVirtualJSONThread(thread map[string]any, rawText string) bool {
	if v, ok := thread["virtual"].(bool); ok && v {
		return true
	}
	lowered := strings.ToLower(rawText)
	return strings.Contains(lowered, "virtualthread") || strings.Contains(lowered, "virtual thread")
}

// nativeMethodFromFrames returns the rendered text of the first frame
// containing "Native Method" — mirrors Python `_native_method_from_frames`.
func nativeMethodFromFrames(frames []models.StackFrame) string {
	for _, frame := range frames {
		rendered := frame.Render()
		if strings.Contains(rendered, "Native Method") {
			return rendered
		}
	}
	return ""
}

// carrierPinning mirrors Python `_carrier_pinning`. Activated when the
// raw text mentions `virtual` plus either `carrier` or `pinn`. Returns
// nil when no marker is present so the caller skips the metadata key.
func carrierPinning(rawText string, frames []models.StackFrame) map[string]any {
	lowered := strings.ToLower(rawText)
	if !strings.Contains(lowered, "virtual") {
		return nil
	}
	if !strings.Contains(lowered, "carrier") && !strings.Contains(lowered, "pinn") {
		return nil
	}
	var topFrame string
	if len(frames) > 0 {
		topFrame = frames[0].Render()
	}
	candidate := firstNonJDKFrame(frames)
	if candidate == "" {
		candidate = topFrame
	}
	return map[string]any{
		"top_frame":        topFrame,
		"candidate_method": candidate,
		"reason":           "virtual_thread_carrier_or_pinning_marker",
	}
}

func firstNonJDKFrame(frames []models.StackFrame) string {
	prefixes := []string{"java.", "javax.", "jdk.", "sun.", "com.sun.", "java.base."}
	for _, frame := range frames {
		rendered := frame.Render()
		jdk := false
		for _, p := range prefixes {
			if strings.HasPrefix(rendered, p) {
				jdk = true
				break
			}
		}
		if !jdk {
			return rendered
		}
	}
	return ""
}
