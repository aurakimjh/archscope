package multithread

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// jvmTables collects the optional JVM-only tables consumed by the
// renderer. Mirrors Python's `_jvm_metadata_tables` return shape.
//
// All five slices are kept sorted in the same order Python uses so a
// downstream golden-file diff would line up:
//
//   - carrierPinning      sort key: (dump_index, thread_name)
//   - smrUnresolved       insertion order (Python iteration order)
//   - nativeMethods       sort key: (dump_index, thread_name)
//   - histogramRows       sort key: -bytes (so largest first)
//   - histogramIncomplete insertion order
type jvmTables struct {
	carrierPinning      []map[string]any
	smrUnresolved       []map[string]any
	nativeMethods       []map[string]any
	histogramRows       []map[string]any
	histogramIncomplete []map[string]any
}

func buildJVMMetadataTables(bundles []models.ThreadDumpBundle, topN int) jvmTables {
	t := jvmTables{
		carrierPinning:      []map[string]any{},
		smrUnresolved:       []map[string]any{},
		nativeMethods:       []map[string]any{},
		histogramRows:       []map[string]any{},
		histogramIncomplete: []map[string]any{},
	}

	for _, bundle := range bundles {
		for _, snapshot := range bundle.Snapshots {
			if pinning, ok := snapshot.Metadata["carrier_pinning"].(map[string]any); ok {
				t.carrierPinning = append(t.carrierPinning, map[string]any{
					"dump_index":       bundle.DumpIndex,
					"dump_label":       derefString(bundle.DumpLabel),
					"thread_name":      snapshot.ThreadName,
					"thread_id":        derefString(snapshot.ThreadID),
					"state":            string(snapshot.State),
					"candidate_method": pinning["candidate_method"],
					"top_frame":        pinning["top_frame"],
					"reason":           pinning["reason"],
				})
			}
			if nativeMethod, ok := snapshot.Metadata["native_method"].(string); ok {
				t.nativeMethods = append(t.nativeMethods, map[string]any{
					"dump_index":      bundle.DumpIndex,
					"dump_label":      derefString(bundle.DumpLabel),
					"thread_name":     snapshot.ThreadName,
					"thread_id":       derefString(snapshot.ThreadID),
					"state":           string(snapshot.State),
					"native_method":   nativeMethod,
					"stack_signature": snapshot.StackSignature(0),
				})
			}
		}

		if smr, ok := bundle.Metadata["smr"].(map[string]any); ok {
			if entries, ok := smr["unresolved"].([]any); ok {
				for _, raw := range entries {
					entry, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					t.smrUnresolved = append(t.smrUnresolved, map[string]any{
						"dump_index":   bundle.DumpIndex,
						"dump_label":   derefString(bundle.DumpLabel),
						"section_line": entry["section_line"],
						"line":         entry["line"],
						"kind":         "tagged",
					})
				}
			}
			if entries, ok := smr["addresses_unresolved"].([]any); ok {
				for _, raw := range entries {
					entry, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					if isTrue(entry["tagged_unresolved"]) {
						continue
					}
					address, _ := entry["address"].(string)
					t.smrUnresolved = append(t.smrUnresolved, map[string]any{
						"dump_index":   bundle.DumpIndex,
						"dump_label":   derefString(bundle.DumpLabel),
						"section_line": entry["section_line"],
						"line":         "address=" + address,
						"address":      entry["address"],
						"kind":         "address_unresolved",
					})
				}
			}
		}

		if histogram, ok := bundle.Metadata["class_histogram"].(map[string]any); ok {
			if classes, ok := histogram["classes"].([]any); ok {
				for _, raw := range classes {
					entry, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					t.histogramRows = append(t.histogramRows, map[string]any{
						"dump_index": bundle.DumpIndex,
						"dump_label": derefString(bundle.DumpLabel),
						"rank":       entry["rank"],
						"class_name": entry["class_name"],
						"instances":  entry["instances"],
						"bytes":      entry["bytes"],
					})
				}
			}
			if isTrue(histogram["incomplete"]) {
				t.histogramIncomplete = append(t.histogramIncomplete, map[string]any{
					"dump_index":        bundle.DumpIndex,
					"dump_label":        derefString(bundle.DumpLabel),
					"reason":            histogram["incomplete_reason"],
					"partial_tail_line": histogram["partial_tail_line"],
					"last_rank":         histogram["last_rank"],
					"total_rows":        histogram["total_rows"],
				})
			}
		}
	}

	// Sort histogram rows by bytes DESC. Python uses `-int(row["bytes"] or 0)`.
	sort.SliceStable(t.histogramRows, func(i, j int) bool {
		return asInt(t.histogramRows[i]["bytes"]) > asInt(t.histogramRows[j]["bytes"])
	})
	// Sort native methods by (dump_index ASC, thread_name ASC).
	sort.SliceStable(t.nativeMethods, func(i, j int) bool {
		di, dj := asInt(t.nativeMethods[i]["dump_index"]), asInt(t.nativeMethods[j]["dump_index"])
		if di != dj {
			return di < dj
		}
		return asString(t.nativeMethods[i]["thread_name"]) < asString(t.nativeMethods[j]["thread_name"])
	})
	// Sort carrier pinning by (dump_index ASC, thread_name ASC).
	sort.SliceStable(t.carrierPinning, func(i, j int) bool {
		di, dj := asInt(t.carrierPinning[i]["dump_index"]), asInt(t.carrierPinning[j]["dump_index"])
		if di != dj {
			return di < dj
		}
		return asString(t.carrierPinning[i]["thread_name"]) < asString(t.carrierPinning[j]["thread_name"])
	})

	t.carrierPinning = capRows(t.carrierPinning, topN)
	t.smrUnresolved = capRows(t.smrUnresolved, topN)
	t.nativeMethods = capRows(t.nativeMethods, topN)
	t.histogramRows = capRows(t.histogramRows, topN)
	t.histogramIncomplete = capRows(t.histogramIncomplete, topN)
	return t
}

func capRows(rows []map[string]any, n int) []map[string]any {
	if n <= 0 || len(rows) <= n {
		return rows
	}
	return rows[:n]
}

func isTrue(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	if i, ok := v.(int); ok {
		return i != 0
	}
	return true
}

// buildJVMMetadataFindings emits the three JVM metadata findings that
// piggyback on top of the metadata tables. Mirrors Python's
// `_jvm_metadata_findings`.
func buildJVMMetadataFindings(t jvmTables, topN int) []map[string]any {
	out := []map[string]any{}
	for _, entry := range capRows(t.carrierPinning, topN) {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "VIRTUAL_THREAD_CARRIER_PINNING",
			"message": "Thread " + formatThreadName(asString(entry["thread_name"])) +
				" contains a virtual-thread carrier/pinning marker.",
			"evidence": entry,
		})
	}
	for _, entry := range capRows(t.smrUnresolved, topN) {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "SMR_UNRESOLVED_THREAD",
			"message":  "JVM SMR diagnostics include an unresolved/zombie thread marker.",
			"evidence": entry,
		})
	}
	for _, entry := range capRows(t.histogramIncomplete, topN) {
		message := asString(entry["reason"])
		if message == "" {
			message = "Class histogram block is incomplete (likely truncated source)."
		}
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "INCOMPLETE_HISTOGRAM",
			"message":  message,
			"evidence": entry,
		})
	}
	return out
}
