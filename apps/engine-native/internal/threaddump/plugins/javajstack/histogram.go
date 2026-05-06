package javajstack

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// T-230 / T-235 — Class histogram parsing + incomplete-block detection.
//
// JVM emits a leading `num #instances #bytes class name` header followed
// by ranked rows and a `Total` summary. We surface the parsed rows and
// flag truncated/incomplete blocks so analyzers can emit
// INCOMPLETE_HISTOGRAM findings.

var (
	classHistogramRowRE = regexp.MustCompile(
		`^\s*(?P<rank>\d+):\s+(?P<instances>\d+)\s+(?P<bytes>\d+)\s+(?P<class_name>.+?)\s*$`,
	)
	classHistogramTotalRE = regexp.MustCompile(
		`(?i)^\s*Total\s+(?P<instances>\d+)\s+(?P<bytes>\d+)\s*$`,
	)
	classHistogramHeaderRE = regexp.MustCompile(
		`(?i)^\s*num\s+#instances\s+#bytes\s+class\s+name`,
	)
	classHistogramPartialRowRE = regexp.MustCompile(
		`^\s*\d+:\s+\d+(?:\s+\d+)?\s*$`,
	)
)

// parseTextClassHistogram returns nil when no histogram block is
// detected. Otherwise returns a payload with classes / row_limit /
// truncated / incomplete flags. Mirrors _parse_text_class_histogram.
func parseTextClassHistogram(lines []string, rowLimit int) map[string]any {
	classes := []map[string]any{}
	totalInstances := -1
	totalBytes := -1
	totalRows := 0
	sawHeader := false
	lastRank := -1
	partialTailLine := ""

	for _, line := range lines {
		if classHistogramHeaderRE.MatchString(line) {
			sawHeader = true
			continue
		}
		if m := classHistogramRowRE.FindStringSubmatch(line); m != nil {
			totalRows++
			rank, _ := strconv.Atoi(m[classHistogramRowRE.SubexpIndex("rank")])
			lastRank = rank
			partialTailLine = ""
			if len(classes) < rowLimit {
				instances, _ := strconv.Atoi(m[classHistogramRowRE.SubexpIndex("instances")])
				bytesVal, _ := strconv.Atoi(m[classHistogramRowRE.SubexpIndex("bytes")])
				className := strings.TrimSpace(m[classHistogramRowRE.SubexpIndex("class_name")])
				classes = append(classes, map[string]any{
					"rank":       rank,
					"instances":  instances,
					"bytes":      bytesVal,
					"class_name": className,
				})
			}
			continue
		}
		if sawHeader && classHistogramPartialRowRE.MatchString(line) {
			partialTailLine = strings.TrimSpace(line)
			continue
		}
		if m := classHistogramTotalRE.FindStringSubmatch(line); m != nil {
			totalInstances, _ = strconv.Atoi(m[classHistogramTotalRE.SubexpIndex("instances")])
			totalBytes, _ = strconv.Atoi(m[classHistogramTotalRE.SubexpIndex("bytes")])
		}
	}
	if len(classes) == 0 && !sawHeader {
		return nil
	}

	incomplete := false
	incompleteReason := ""
	switch {
	case partialTailLine != "":
		incomplete = true
		incompleteReason = "Histogram ends with a partial row that is missing the class " +
			"name column — the source was likely truncated mid-write."
	case sawHeader && totalInstances < 0 && totalBytes < 0 && len(classes) > 0:
		incomplete = true
		incompleteReason = "Histogram has class rows but no `Total` summary line — the " +
			"JVM always emits one, so the source is likely truncated."
	case sawHeader && len(classes) == 0:
		incomplete = true
		incompleteReason = "Histogram header was seen but no class rows or totals were " +
			"parsed — the section is empty or its rows are malformed."
	}

	payload := map[string]any{
		"classes":     classes,
		"row_limit":   rowLimit,
		"total_rows":  totalRows,
		"truncated":   totalRows > rowLimit,
		"incomplete":  incomplete,
	}
	if incompleteReason != "" {
		payload["incomplete_reason"] = incompleteReason
	}
	if partialTailLine != "" {
		payload["partial_tail_line"] = partialTailLine
	}
	if lastRank >= 0 {
		payload["last_rank"] = lastRank
	}
	if totalInstances >= 0 {
		payload["total_instances"] = totalInstances
	}
	if totalBytes >= 0 {
		payload["total_bytes"] = totalBytes
	}
	return payload
}

// G1 heap block — JDK 8 G1 frequently embeds a {Heap before GC ...} or
// {Heap after GC ...} block at the top of jstack-style dumps.

var (
	g1HeapTotalRE = regexp.MustCompile(
		`garbage-first heap\s+total\s+(?P<total_kb>\d+)K,\s*used\s+(?P<used_kb>\d+)K`,
	)
	g1RegionRE = regexp.MustCompile(
		`region\s+size\s+(?P<region_kb>\d+)K,\s*(?P<young>\d+)\s+young\s*\((?P<young_kb>\d+)K\),\s*(?P<survivors>\d+)\s+survivors\s*\((?P<survivors_kb>\d+)K\)`,
	)
	metaspaceRE = regexp.MustCompile(
		`^\s*Metaspace\s+used\s+(?P<used_kb>\d+)K,\s*capacity\s+(?P<capacity_kb>\d+)K,\s*committed\s+(?P<committed_kb>\d+)K,\s*reserved\s+(?P<reserved_kb>\d+)K`,
	)
)

// parseG1HeapBlock extracts the optional G1 heap snapshot. Returns nil
// when no recognisable heap block is found. Sizes are surfaced in MB so
// the UI doesn't have to convert.
func parseG1HeapBlock(lines []string) map[string]any {
	found := map[string]any{}
	inBlock := false
	blockStart := -1
	for offsetZero, line := range lines {
		offset := offsetZero + 1
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "{Heap") {
			inBlock = true
			blockStart = offset
			lower := strings.ToLower(stripped)
			if _, set := found["phase"]; !set {
				if strings.Contains(lower, "before") {
					found["phase"] = "before_gc"
				} else {
					found["phase"] = "after_gc"
				}
			}
			continue
		}
		if !inBlock {
			if strings.Contains(strings.ToLower(line), "garbage-first heap") && offset < 50 {
				inBlock = true
				blockStart = offset
			}
		}
		if !inBlock {
			continue
		}

		if m := g1HeapTotalRE.FindStringSubmatch(line); m != nil {
			totalKB, _ := strconv.Atoi(m[g1HeapTotalRE.SubexpIndex("total_kb")])
			usedKB, _ := strconv.Atoi(m[g1HeapTotalRE.SubexpIndex("used_kb")])
			found["heap_total_mb"] = roundMB(totalKB)
			found["heap_used_mb"] = roundMB(usedKB)
		}
		if m := g1RegionRE.FindStringSubmatch(line); m != nil {
			regionKB, _ := strconv.Atoi(m[g1RegionRE.SubexpIndex("region_kb")])
			young, _ := strconv.Atoi(m[g1RegionRE.SubexpIndex("young")])
			youngKB, _ := strconv.Atoi(m[g1RegionRE.SubexpIndex("young_kb")])
			survivors, _ := strconv.Atoi(m[g1RegionRE.SubexpIndex("survivors")])
			survivorsKB, _ := strconv.Atoi(m[g1RegionRE.SubexpIndex("survivors_kb")])
			found["region_size_mb"] = roundMB(regionKB)
			found["young_regions"] = young
			found["young_used_mb"] = roundMB(youngKB)
			found["survivor_regions"] = survivors
			found["survivor_used_mb"] = roundMB(survivorsKB)
		}
		if m := metaspaceRE.FindStringSubmatch(line); m != nil {
			usedKB, _ := strconv.Atoi(m[metaspaceRE.SubexpIndex("used_kb")])
			committedKB, _ := strconv.Atoi(m[metaspaceRE.SubexpIndex("committed_kb")])
			reservedKB, _ := strconv.Atoi(m[metaspaceRE.SubexpIndex("reserved_kb")])
			found["metaspace_used_mb"] = roundMB(usedKB)
			found["metaspace_committed_mb"] = roundMB(committedKB)
			found["metaspace_reserved_mb"] = roundMB(reservedKB)
		}

		if stripped == "}" || strings.HasPrefix(stripped, `"`) {
			break
		}
	}
	if _, hasHeap := found["heap_total_mb"]; !hasHeap {
		if _, hasMeta := found["metaspace_used_mb"]; !hasMeta {
			return nil
		}
	}
	if blockStart > 0 {
		found["section_start_line"] = blockStart
	}
	return found
}

// roundMB converts kilobytes to megabytes with 2-decimal-place
// precision (Python's `round(value, 2)`).
func roundMB(kb int) float64 {
	return math.Round(float64(kb)/1024.0*100) / 100
}

// sectionMetadata returns the metadata dict for one jstackSection.
// Mirrors Python's _section_metadata.
func sectionMetadata(section jstackSection, classHistogramLimit int) map[string]any {
	metadata := map[string]any{
		"start_line":                 section.StartLine,
		"end_line":                   section.EndLine,
		"class_histogram_row_limit":  classHistogramLimit,
	}
	if section.RawTimestamp != "" {
		metadata["raw_timestamp"] = section.RawTimestamp
	}
	if smr := parseSMRDiagnostics(section.RawLines); smr != nil {
		metadata["smr"] = smr
	}
	if hist := parseTextClassHistogram(section.RawLines, classHistogramLimit); hist != nil {
		metadata["class_histogram"] = hist
	}
	if heap := parseG1HeapBlock(section.RawLines); heap != nil {
		metadata["jvm_heap_block"] = heap
	}
	return metadata
}
