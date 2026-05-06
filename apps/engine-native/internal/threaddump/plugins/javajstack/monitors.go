package javajstack

import (
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// T-219 / T-231 — Lock handle extraction.
//
// jstack reports four monitor relationships per thread block:
//
//   - locked <0x...> (a Foo)               — thread holds the monitor.
//   - waiting to lock <0x...> (a Foo)      — thread is blocked on
//     entry; analyzers correlate against another thread's locked owner.
//   - waiting on <0x...> (a Foo)           — Object.wait().
//   - parking to wait for <0x...> (a Foo)  — LockSupport.park (e.g.
//     ReentrantLock or Condition).
//
// Wait modes are surfaced verbatim so analyzer code can emit the right
// finding (lock_entry_wait drives lock-contention findings; object_wait
// is an idle wait that should NOT be treated as contention).

const (
	waitModeLockEntryWait = "lock_entry_wait"
	waitModeObjectWait    = "object_wait"
	waitModeParkingWait   = "parking_condition_wait"
	waitModeLockedOwner   = "locked_owner"
)

var (
	lockedRE = regexp.MustCompile(
		`-\s+locked\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?`,
	)
	waitingToLockRE = regexp.MustCompile(
		`-\s+waiting to lock\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?`,
	)
	waitingOnRE = regexp.MustCompile(
		`-\s+waiting on\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?`,
	)
	parkingRE = regexp.MustCompile(
		`-\s+parking to wait for\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?`,
	)
)

// matcherWithMode pairs a regex with the wait_mode label it produces.
type matcherWithMode struct {
	Pattern *regexp.Regexp
	Mode    string
}

var waitMatchers = []matcherWithMode{
	{waitingToLockRE, waitModeLockEntryWait},
	{waitingOnRE, waitModeObjectWait},
	{parkingRE, waitModeParkingWait},
}

// extractLockHandles pulls lock IDs out of a raw jstack block. Multiple
// `locked` lines may appear (re-entrant or nested locks); the first
// matching wait line wins. Lock IDs that show up as both `locked` and
// `waiting to lock` on the same thread are treated as held — the
// thread already owns the monitor and is re-entering it.
func extractLockHandles(rawBlock string) ([]models.LockHandle, *models.LockHandle) {
	holds := []models.LockHandle{}
	seenIDs := map[string]struct{}{}
	var waiting *models.LockHandle
	for _, raw := range strings.Split(rawBlock, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if waiting == nil {
			for _, m := range waitMatchers {
				match := m.Pattern.FindStringSubmatchIndex(line)
				if match == nil {
					continue
				}
				if match[0] != 0 {
					// Python uses `pattern.match(line)` which only
					// anchors at the start. Skip anything else.
					continue
				}
				groups := captureGroups(m.Pattern, line)
				handle := models.LockHandle{
					LockID:   groups["id"],
					WaitMode: m.Mode,
				}
				if cls, ok := groups["cls"]; ok && cls != "" {
					c := cls
					handle.LockClass = &c
				}
				waiting = &handle
				break
			}
		}

		if locked := lockedRE.FindStringSubmatchIndex(line); locked != nil && locked[0] == 0 {
			groups := captureGroups(lockedRE, line)
			id := groups["id"]
			if _, dup := seenIDs[id]; dup {
				continue
			}
			seenIDs[id] = struct{}{}
			handle := models.LockHandle{LockID: id, WaitMode: waitModeLockedOwner}
			if cls, ok := groups["cls"]; ok && cls != "" {
				c := cls
				handle.LockClass = &c
			}
			holds = append(holds, handle)
		}
	}

	if waiting != nil {
		if _, held := seenIDs[waiting.LockID]; held {
			// Re-entrant acquire — drop the wait so analyzers don't
			// interpret it as contention.
			waiting = nil
		}
	}
	return holds, waiting
}

// captureGroups returns a name -> match map for a regex's named
// subgroups. Returns nil when the regex did not match.
func captureGroups(re *regexp.Regexp, line string) map[string]string {
	match := re.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	out := map[string]string{}
	for i, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		out[name] = match[i]
	}
	return out
}
