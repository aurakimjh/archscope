package javajstack

import (
	"regexp"
	"strconv"
	"strings"
)

// T-228 / T-234 — Threads class SMR info parsing.
//
// JDK's "Threads class SMR info:" section embeds JavaThread* addresses.
// We parse them out and (in postProcessSMR) cross-reference with parsed
// thread tids so the multi-thread analyzer can emit
// SMR_UNRESOLVED_THREAD findings even when the JVM didn't tag a row
// with the literal "zombie" word (JDK 8/11 SMR sections frequently
// omit that).

var (
	smrHexRE    = regexp.MustCompile(`\b0x[0-9a-fA-F]{6,16}\b`)
	smrLengthRE = regexp.MustCompile(`(?i)\blength\s*=\s*(\d+)`)
)

// parseSMRDiagnostics walks the section's raw lines and surfaces a
// payload mirroring Python's _parse_smr_diagnostics. Returns nil when
// no SMR block is found.
func parseSMRDiagnostics(lines []string) map[string]any {
	unresolved := []map[string]any{}
	addresses := []map[string]any{}
	inSMRBlock := false
	remaining := 0
	sectionStart := -1
	length := -1

	for offsetZero, line := range lines {
		offset := offsetZero + 1
		lower := strings.ToLower(line)
		if strings.Contains(lower, "smr") && strings.Contains(lower, "thread") {
			inSMRBlock = true
			remaining = 80
			if sectionStart < 0 {
				sectionStart = offset
			}
		}
		if !inSMRBlock {
			continue
		}
		isUnresolvedLine := strings.Contains(lower, "unresolved") || strings.Contains(lower, "zombie")
		if isUnresolvedLine {
			unresolved = append(unresolved, map[string]any{
				"section_line": offset,
				"line":         strings.TrimSpace(line),
			})
		}
		if length < 0 {
			if m := smrLengthRE.FindStringSubmatch(line); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil {
					length = n
				}
			}
		}
		// Skip JVM bookkeeping rows for the hex-address scan.
		if strings.Contains(lower, "_java_thread_list") || strings.Contains(lower, "elements=") {
			remaining--
			if remaining <= 0 {
				inSMRBlock = false
			}
			continue
		}
		for _, m := range smrHexRE.FindAllString(line, -1) {
			addresses = append(addresses, map[string]any{
				"section_line":      offset,
				"address":           normalizeSMRAddress(m),
				"tagged_unresolved": isUnresolvedLine,
			})
		}
		remaining--
		if remaining <= 0 {
			inSMRBlock = false
		}
	}

	if len(unresolved) == 0 && len(addresses) == 0 {
		return nil
	}
	payload := map[string]any{
		"unresolved_count": len(unresolved),
		"unresolved":       unresolved,
		"addresses":        addresses,
	}
	if length >= 0 {
		payload["length"] = length
	}
	if sectionStart >= 0 {
		payload["section_start"] = sectionStart
	}
	return payload
}

// normalizeSMRAddress lower-cases and strips leading zeros so cross-
// engine address comparisons match. Mirrors Python's
// _normalize_smr_address.
func normalizeSMRAddress(value string) string {
	if !strings.HasPrefix(value, "0x") && !strings.HasPrefix(value, "0X") {
		return value
	}
	body := strings.ToLower(strings.TrimLeft(value[2:], "0"))
	if body == "" {
		body = "0"
	}
	return "0x" + body
}

// postProcessSMR cross-references SMR addresses with parsed thread
// records' tids. Returns the same map back, mutated to add
// `resolved`, `addresses_unresolved`, and counters.
//
// Mirrors Python's _post_process_smr.
func postProcessSMR(smr map[string]any, records []threadDumpRecord) map[string]any {
	if smr == nil {
		return smr
	}
	addressesAny, ok := smr["addresses"].([]map[string]any)
	if !ok || len(addressesAny) == 0 {
		return smr
	}

	tidIndex := map[string]threadDumpRecord{}
	for _, rec := range records {
		if rec.ThreadID != "" {
			tidIndex[normalizeSMRAddress(rec.ThreadID)] = rec
		}
	}

	resolved := []map[string]any{}
	unresolved := []map[string]any{}
	seen := map[string]struct{}{}
	for _, entry := range addressesAny {
		address, ok := entry["address"].(string)
		if !ok {
			continue
		}
		if _, dup := seen[address]; dup {
			// SMR sections frequently list the same address twice
			// (e.g. `=>0x...` plus the iteration block). Count once.
			continue
		}
		seen[address] = struct{}{}
		if rec, hit := tidIndex[address]; hit {
			row := map[string]any{
				"section_line": entry["section_line"],
				"address":      address,
				"thread_name":  rec.ThreadName,
			}
			if rec.ThreadID != "" {
				row["thread_id"] = rec.ThreadID
			} else {
				row["thread_id"] = nil
			}
			resolved = append(resolved, row)
		} else {
			unresolved = append(unresolved, map[string]any{
				"section_line":      entry["section_line"],
				"address":           address,
				"tagged_unresolved": entry["tagged_unresolved"],
			})
		}
	}

	smr["resolved"] = resolved
	smr["addresses_unresolved"] = unresolved
	smr["resolved_count"] = len(resolved)
	smr["addresses_unresolved_count"] = len(unresolved)
	return smr
}
