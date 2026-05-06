package lockcontention

import (
	"fmt"
	"strings"
)

// buildFindings ports `_build_findings`. Emits a LOCK_CONTENTION_HOTSPOT
// for each contended lock (already capped at top_n by the caller) and a
// DEADLOCK_DETECTED for each cycle.
//
// Severity codes match Python verbatim:
//   - LOCK_CONTENTION_HOTSPOT → "warning"
//   - DEADLOCK_DETECTED       → "critical"
func buildFindings(hotspots []map[string]any, deadlocks []map[string]any) []map[string]any {
	findings := make([]map[string]any, 0, len(hotspots)+len(deadlocks))

	for _, row := range hotspots {
		lockClass := "unknown class"
		if cls, ok := row["lock_class"].(string); ok && cls != "" {
			lockClass = cls
		}
		owner := "unknown"
		if name, ok := row["owner_thread"].(string); ok && name != "" {
			owner = name
		}
		message := fmt.Sprintf(
			"Lock %s (%s) has %d waiters; owner: %s.",
			asString(row["lock_id"]),
			lockClass,
			asInt(row["waiter_count"]),
			owner,
		)
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "LOCK_CONTENTION_HOTSPOT",
			"message":  message,
			"evidence": row,
		})
	}

	for _, chain := range deadlocks {
		threads, ok := chain["threads"].([]string)
		if !ok || len(threads) == 0 {
			// Defensive — matches Python's `if not isinstance(...)` guard.
			continue
		}
		// `T1 → T2 → T1` rendering; `→` is U+2192 RIGHTWARDS ARROW
		// — the same character Python emits via the literal " → ".
		joined := strings.Join(append(append([]string{}, threads...), threads[0]), " → ")
		findings = append(findings, map[string]any{
			"severity": "critical",
			"code":     "DEADLOCK_DETECTED",
			"message":  fmt.Sprintf("Deadlock cycle: %s.", joined),
			"evidence": chain,
		})
	}
	return findings
}
