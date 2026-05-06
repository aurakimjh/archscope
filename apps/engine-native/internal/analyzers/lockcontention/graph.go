package lockcontention

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// detectDeadlocks DFS-walks the waits-for graph and reports each simple
// cycle once. Mirrors `_detect_deadlocks` in the Python analyzer.
//
// Builds an edge `thread → owner_of_the_lock_it_is_waiting_on` directly
// so a 2-thread classic deadlock (T1 holds L1 wants L2, T2 holds L2
// wants L1) shows up as `T1 → T2 → T1`.
//
// Cycle detection uses three-color DFS:
//   - 0 (zero value) — unseen
//   - 1 — on the current DFS stack
//   - 2 — fully explored
//
// When DFS hits a node already colored 1, the slice from that node's
// stack index to the end of the stack is the cycle. The cycle is then
// canonicalised (rotated to its lexicographically-smallest rotation) so
// the same cycle reached from different start nodes only fires once.
func detectDeadlocks(snapshots []models.ThreadSnapshot) []map[string]any {
	holdersByLock := map[string]string{}
	for _, snap := range snapshots {
		for _, hold := range snap.LockHolds {
			// First-seen hold wins; multi-dump unions can list the same
			// lock multiple times. Either way the owner thread name is
			// canonical because lock_id is unique per JVM.
			if _, present := holdersByLock[hold.LockID]; !present {
				holdersByLock[hold.LockID] = snap.ThreadName
			}
		}
	}

	waitsFor := map[string]string{}
	for _, snap := range snapshots {
		if snap.LockWaiting == nil {
			continue
		}
		mode := snap.LockWaiting.WaitMode
		if _, isCooperative := cooperativeWaitModes[mode]; isCooperative {
			continue
		}
		owner, ok := holdersByLock[snap.LockWaiting.LockID]
		if !ok {
			continue
		}
		if owner == snap.ThreadName {
			continue
		}
		waitsFor[snap.ThreadName] = owner
	}

	color := map[string]int{}
	cycles := make([][]string, 0)
	seenCanonical := map[string]struct{}{}

	// Recursive DFS — closure captures color/cycles/seenCanonical so the
	// Go shape mirrors the Python nested-function approach 1:1. The
	// graph has out-degree 1 (each thread waits on at most one lock), so
	// recursion depth is bounded by the number of distinct waiting
	// threads in a single chain — never large enough to risk overflow.
	var dfs func(node string, stack []string)
	dfs = func(node string, stack []string) {
		color[node] = 1
		stack = append(stack, node)
		nxt, has := waitsFor[node]
		if has {
			switch color[nxt] {
			case 1:
				start := indexOf(stack, nxt)
				cycle := append([]string(nil), stack[start:]...)
				canonical := canonicalKey(cycle)
				if _, seen := seenCanonical[canonical]; !seen {
					seenCanonical[canonical] = struct{}{}
					cycles = append(cycles, cycle)
				}
			case 0:
				dfs(nxt, stack)
			}
		}
		// Implicit `stack.pop()` — we don't need to do anything because
		// `stack` is a fresh slice header each call (append'd above).
		color[node] = 2
	}

	// Sort starting nodes for deterministic ordering — mirrors Python
	// `for thread_name in sorted(waits_for.keys())`.
	starts := make([]string, 0, len(waitsFor))
	for name := range waitsFor {
		starts = append(starts, name)
	}
	sort.Strings(starts)
	for _, name := range starts {
		if color[name] == 0 {
			dfs(name, nil)
		}
	}

	out := make([]map[string]any, 0, len(cycles))
	for _, cycle := range cycles {
		edges := make([]map[string]any, 0, len(cycle))
		for index, thread := range cycle {
			target := cycle[(index+1)%len(cycle)]
			// The edge label is the lock that `thread` is waiting on
			// — easiest to pull from the original snapshot. Mirrors
			// Python's `next(... for snap in snapshots_list ...)`.
			var lockIDValue, lockClassValue any
			for i := range snapshots {
				snap := snapshots[i]
				if snap.ThreadName != thread {
					continue
				}
				if snap.LockWaiting == nil {
					continue
				}
				lockIDValue = snap.LockWaiting.LockID
				if snap.LockWaiting.LockClass != nil {
					lockClassValue = *snap.LockWaiting.LockClass
				}
				break
			}
			edges = append(edges, map[string]any{
				"from_thread": thread,
				"to_thread":   target,
				"lock_id":     lockIDValue,
				"lock_class":  lockClassValue,
			})
		}
		out = append(out, map[string]any{
			"threads": cycle,
			"edges":   edges,
		})
	}
	return out
}

// indexOf returns the first index where `stack[i] == target`. Returns
// -1 when not found — but the caller only invokes us after confirming
// `color[target] == 1`, which guarantees presence.
func indexOf(stack []string, target string) int {
	for i, v := range stack {
		if v == target {
			return i
		}
	}
	return -1
}

// canonicalKey returns the lexicographically smallest rotation of
// `cycle`, joined with NUL separators so the result is unambiguous as
// a map key. Mirrors Python `_canonical_cycle` (which returns the
// rotation as a tuple that's used directly in a set).
func canonicalKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	best := joinRotation(cycle, 0)
	for i := 1; i < len(cycle); i++ {
		candidate := joinRotation(cycle, i)
		if candidate < best {
			best = candidate
		}
	}
	return best
}

// joinRotation joins `cycle[i:] + cycle[:i]` with NUL separators.
func joinRotation(cycle []string, start int) string {
	const sep = "\x00"
	out := ""
	for i := 0; i < len(cycle); i++ {
		if i > 0 {
			out += sep
		}
		out += cycle[(start+i)%len(cycle)]
	}
	return out
}
