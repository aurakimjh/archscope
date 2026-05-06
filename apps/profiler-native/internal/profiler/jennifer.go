package profiler

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
)

type JenniferCsvParseResult struct {
	Root        FlameNode
	Diagnostics ParserDiagnostics
}

var jenniferColumnAliases = map[string][]string{
	"key":            {"key", "id", "node_key", "node_id"},
	"parent_key":     {"parent_key", "parent", "parent_id", "parentkey"},
	"method_name":    {"method_name", "method", "name", "frame", "methodname"},
	"ratio":          {"ratio", "percent", "percentage"},
	"sample_count":   {"sample_count", "samples", "sample", "count", "samplecount"},
	"color_category": {"color_category", "category", "colorcategory"},
}

var jenniferRequiredColumns = []string{"key", "method_name", "sample_count"}

func ParseJenniferFlamegraphCSV(path string, debugLog *DebugLog) (JenniferCsvParseResult, error) {
	source := path
	diagnostics := ParserDiagnostics{
		SourceFile:      &source,
		Format:          "jennifer_flamegraph_csv",
		SkippedByReason: map[string]int{},
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return JenniferCsvParseResult{}, err
	}

	if len(raw) == 0 {
		root := buildJenniferTree(nil, &diagnostics)
		addDiagnosticWarning(&diagnostics, 0, "EMPTY_FILE", "Jennifer CSV file is empty.", "")
		return JenniferCsvParseResult{Root: root, Diagnostics: diagnostics}, nil
	}

	rows := readJenniferRows(raw, &diagnostics, debugLog)
	root := buildJenniferTree(rows, &diagnostics)
	if diagnostics.ParsedRecords == 0 {
		addDiagnosticWarning(&diagnostics, 0, "NO_VALID_RECORDS", "No valid Jennifer CSV rows were parsed.", "")
	}
	debugLog.SetTotals(diagnostics.TotalLines, diagnostics.ParsedRecords, diagnostics.SkippedLines)
	return JenniferCsvParseResult{Root: root, Diagnostics: diagnostics}, nil
}

type jenniferRow struct {
	Key           string
	ParentKey     string
	MethodName    string
	Ratio         float64
	SampleCount   int
	ColorCategory string
}

func readJenniferRows(raw []byte, diagnostics *ParserDiagnostics, debugLog *DebugLog) map[string]*jenniferRow {
	for index, encoding := range jenniferEncodingCandidates() {
		text, ok := decodeJenniferBytes(raw, encoding)
		if !ok {
			if index < len(jenniferEncodingCandidates())-1 {
				addDiagnosticWarning(diagnostics, 0, "ENCODING_FALLBACK",
					fmt.Sprintf("Could not decode Jennifer CSV as %s; trying next encoding.", encoding), "")
			}
			continue
		}
		debugLog.SetEncodingDetected(encoding)
		checkpoint := snapshotDiagnostics(diagnostics)
		rows, parseErr := parseJenniferCSVText(text, diagnostics, debugLog)
		if parseErr != nil {
			restoreDiagnostics(diagnostics, checkpoint)
			addDiagnosticWarning(diagnostics, 0, "ENCODING_FALLBACK",
				fmt.Sprintf("Could not parse Jennifer CSV as %s: %v.", encoding, parseErr), "")
			continue
		}
		return rows
	}
	msg := "Could not decode Jennifer CSV with supported encodings: " + strings.Join(jenniferEncodingCandidates(), ", ") + "."
	addDiagnosticError(diagnostics, 0, "ENCODING_ERROR", msg, "")
	debugLog.AddParseError(0, "ENCODING_ERROR", msg, "", nil, nil)
	return nil
}

func jenniferEncodingCandidates() []string {
	return []string{"utf-8-sig", "utf-8", "cp949"}
}

func decodeJenniferBytes(raw []byte, encoding string) (string, bool) {
	switch encoding {
	case "utf-8-sig":
		bom := []byte{0xEF, 0xBB, 0xBF}
		if !bytes.HasPrefix(raw, bom) {
			return "", false
		}
		stripped := raw[len(bom):]
		if !utf8.Valid(stripped) {
			return "", false
		}
		return string(stripped), true
	case "utf-8":
		if !utf8.Valid(raw) {
			return "", false
		}
		return string(raw), true
	case "cp949":
		decoder := korean.EUCKR.NewDecoder()
		decoded, err := decoder.Bytes(raw)
		if err != nil {
			return "", false
		}
		return string(decoded), true
	}
	return "", false
}

type diagnosticsCheckpoint struct {
	totalLines      int
	parsedRecords   int
	skippedLines    int
	skippedByReason map[string]int
	samples         []DiagnosticSample
	warnings        []DiagnosticSample
	errs            []DiagnosticSample
}

func snapshotDiagnostics(diagnostics *ParserDiagnostics) diagnosticsCheckpoint {
	cloned := make(map[string]int, len(diagnostics.SkippedByReason))
	for key, value := range diagnostics.SkippedByReason {
		cloned[key] = value
	}
	return diagnosticsCheckpoint{
		totalLines:      diagnostics.TotalLines,
		parsedRecords:   diagnostics.ParsedRecords,
		skippedLines:    diagnostics.SkippedLines,
		skippedByReason: cloned,
		samples:         append([]DiagnosticSample(nil), diagnostics.Samples...),
		warnings:        append([]DiagnosticSample(nil), diagnostics.Warnings...),
		errs:            append([]DiagnosticSample(nil), diagnostics.Errors...),
	}
}

func restoreDiagnostics(diagnostics *ParserDiagnostics, checkpoint diagnosticsCheckpoint) {
	diagnostics.TotalLines = checkpoint.totalLines
	diagnostics.ParsedRecords = checkpoint.parsedRecords
	diagnostics.SkippedLines = checkpoint.skippedLines
	diagnostics.SkippedByReason = checkpoint.skippedByReason
	diagnostics.Samples = checkpoint.samples
	diagnostics.Warnings = checkpoint.warnings
	diagnostics.Errors = checkpoint.errs
	diagnostics.WarningCount = len(diagnostics.Warnings)
	diagnostics.ErrorCount = len(diagnostics.Errors)
}

func parseJenniferCSVText(text string, diagnostics *ParserDiagnostics, debugLog *DebugLog) (map[string]*jenniferRow, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = false
	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]*jenniferRow{}, nil
		}
		return nil, err
	}
	columns := resolveJenniferColumns(header)
	missing := []string{}
	for _, name := range jenniferRequiredColumns {
		if _, ok := columns[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		message := fmt.Sprintf("Missing required Jennifer CSV columns: %s. Found columns: %s.",
			strings.Join(missing, ", "), strings.Join(header, ", "))
		raw := strings.Join(header, ",")
		addDiagnosticError(diagnostics, 1, "MISSING_REQUIRED_COLUMNS", message, raw)
		debugLog.AddParseError(1, "MISSING_REQUIRED_COLUMNS", message, "", map[string]string{"target": raw}, nil)
		return map[string]*jenniferRow{}, nil
	}
	rows := map[string]*jenniferRow{}
	lineNumber := 1
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		lineNumber++
		diagnostics.TotalLines++
		row, parseErr := parseJenniferRecord(record, header, columns)
		if parseErr != nil {
			raw := strings.Join(record, ",")
			addDiagnosticError(diagnostics, lineNumber, "INVALID_JENNIFER_ROW", parseErr.Error(), raw)
			debugLog.AddParseError(lineNumber, "INVALID_JENNIFER_ROW", parseErr.Error(), "", map[string]string{"target": raw}, nil)
			continue
		}
		if existing := rows[row.Key]; existing != nil {
			raw := strings.Join(record, ",")
			msg := fmt.Sprintf("Duplicate Jennifer CSV key: %s", row.Key)
			addDiagnosticError(diagnostics, lineNumber, "DUPLICATE_KEY", msg, raw)
			debugLog.AddParseError(lineNumber, "DUPLICATE_KEY", msg, "", map[string]string{"target": raw}, nil)
			continue
		}
		rows[row.Key] = row
		diagnostics.ParsedRecords++
	}
	return rows, nil
}

func resolveJenniferColumns(header []string) map[string]int {
	resolved := map[string]int{}
	lowerIndex := map[string]int{}
	for index, raw := range header {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, exists := lowerIndex[strings.ToLower(trimmed)]; !exists {
			lowerIndex[strings.ToLower(trimmed)] = index
		}
	}
	for canonical, aliases := range jenniferColumnAliases {
		for _, alias := range aliases {
			if index, ok := lowerIndex[strings.ToLower(alias)]; ok {
				resolved[canonical] = index
				break
			}
		}
	}
	return resolved
}

func parseJenniferRecord(record, header []string, columns map[string]int) (*jenniferRow, error) {
	get := func(canonical string) string {
		index, ok := columns[canonical]
		if !ok || index < 0 || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	key := get("key")
	if key == "" {
		return nil, fmt.Errorf("Missing required value: key")
	}
	method := get("method_name")
	if method == "" {
		return nil, fmt.Errorf("Missing required value: method_name")
	}
	sampleText := get("sample_count")
	if sampleText == "" {
		return nil, fmt.Errorf("Missing required value: sample_count")
	}
	sampleCount, err := parseJenniferSampleCount(sampleText)
	if err != nil {
		return nil, err
	}
	ratioText := get("ratio")
	ratioValue, err := parseJenniferRatio(ratioText)
	if err != nil {
		return nil, err
	}
	return &jenniferRow{
		Key:           key,
		ParentKey:     get("parent_key"),
		MethodName:    method,
		Ratio:         ratioValue,
		SampleCount:   sampleCount,
		ColorCategory: get("color_category"),
	}, nil
}

func parseJenniferSampleCount(text string) (int, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("sample_count must be numeric.")
	}
	if value != float64(int64(value)) {
		return 0, fmt.Errorf("sample_count must be a whole number.")
	}
	if value < 0 {
		return 0, fmt.Errorf("sample_count must be non-negative.")
	}
	return int(value), nil
}

func parseJenniferRatio(text string) (float64, error) {
	if text == "" {
		return 0, nil
	}
	normalized := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "%"))
	if normalized == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, fmt.Errorf("ratio must be numeric when present.")
	}
	if value < 0 {
		return 0, fmt.Errorf("ratio must be non-negative.")
	}
	return value, nil
}

func buildJenniferTree(rows map[string]*jenniferRow, diagnostics *ParserDiagnostics) FlameNode {
	if len(rows) == 0 {
		empty := FlameNode{
			ID:       "root",
			Name:     "All",
			Samples:  0,
			Ratio:    100.0,
			Children: []FlameNode{},
			Path:     []string{},
		}
		return empty
	}
	parentByKey := map[string]string{}
	for key, row := range rows {
		if row.ParentKey != "" {
			if _, ok := rows[row.ParentKey]; ok {
				parentByKey[key] = row.ParentKey
			}
		}
	}
	cycleKeys := findJenniferCycleKeys(parentByKey)
	if len(cycleKeys) > 0 {
		sortedKeys := append([]string(nil), cycleKeys...)
		sort.Strings(sortedKeys)
		message := "Cycle detected in Jennifer CSV parent_key references: " + strings.Join(sortedKeys, ", ")
		addDiagnosticError(diagnostics, 0, "PARENT_CYCLE", message, "")
		// PARENT_CYCLE is structural, not row-level — Python doesn't bump skipped_lines.
		undoSkipForReason(diagnostics, "PARENT_CYCLE")
		for _, key := range cycleKeys {
			rows[key].ParentKey = ""
		}
	}

	type jenniferNode struct {
		row      *jenniferRow
		children []*jenniferNode
		parent   *jenniferNode
	}
	nodes := map[string]*jenniferNode{}
	for key, row := range rows {
		nodes[key] = &jenniferNode{row: row}
	}
	roots := []*jenniferNode{}
	for key, node := range nodes {
		row := node.row
		parentKey := row.ParentKey
		var parent *jenniferNode
		if parentKey != "" {
			parent = nodes[parentKey]
		}
		if parent == nil {
			if parentKey != "" {
				message := fmt.Sprintf(
					"Missing Jennifer CSV parent_key '%s' for key '%s'; treating row as a root.",
					parentKey, key)
				addDiagnosticWarning(diagnostics, 0, "MISSING_PARENT", message, "")
			}
			roots = append(roots, node)
			continue
		}
		node.parent = parent
		parent.children = append(parent.children, node)
	}

	sortNodes := func(items []*jenniferNode) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].row.SampleCount == items[j].row.SampleCount {
				return items[i].row.Key < items[j].row.Key
			}
			return items[i].row.SampleCount > items[j].row.SampleCount
		})
	}
	sortNodes(roots)
	for _, node := range nodes {
		sortNodes(node.children)
	}

	hasVirtualRoot := len(roots) != 1
	var totalSamples int
	for _, root := range roots {
		totalSamples += root.row.SampleCount
	}
	denominator := totalSamples
	if !hasVirtualRoot && len(roots) == 1 {
		denominator = roots[0].row.SampleCount
	}
	if denominator <= 0 {
		denominator = 1
	}

	var freeze func(node *jenniferNode, parentID *string) FlameNode
	freeze = func(node *jenniferNode, parentID *string) FlameNode {
		out := FlameNode{
			ID:       node.row.Key,
			ParentID: parentID,
			Name:     node.row.MethodName,
			Samples:  node.row.SampleCount,
			Ratio:    ratio(node.row.SampleCount, denominator, 4),
			Children: []FlameNode{},
		}
		if node.row.ColorCategory != "" {
			category := node.row.ColorCategory
			out.Category = &category
			color := node.row.ColorCategory
			out.Color = &color
		}
		nodeID := out.ID
		for _, child := range node.children {
			out.Children = append(out.Children, freeze(child, &nodeID))
		}
		return out
	}

	var root FlameNode
	if hasVirtualRoot {
		rootID := "root"
		children := make([]FlameNode, 0, len(roots))
		for _, child := range roots {
			children = append(children, freeze(child, &rootID))
		}
		root = FlameNode{
			ID:       "root",
			Name:     "All",
			Samples:  totalSamples,
			Ratio:    100.0,
			Children: children,
			Path:     []string{},
		}
	} else {
		root = freeze(roots[0], nil)
	}

	validateInclusiveSamples(&root, diagnostics)
	assignJenniferPaths(&root, !hasVirtualRoot)
	return root
}

func undoSkipForReason(diagnostics *ParserDiagnostics, reason string) {
	if count, ok := diagnostics.SkippedByReason[reason]; ok && count > 0 {
		diagnostics.SkippedLines -= count
		delete(diagnostics.SkippedByReason, reason)
	}
}

func validateInclusiveSamples(root *FlameNode, diagnostics *ParserDiagnostics) {
	var walk func(node *FlameNode)
	walk = func(node *FlameNode) {
		if len(node.Children) == 0 {
			return
		}
		childTotal := 0
		for _, child := range node.Children {
			childTotal += child.Samples
		}
		if childTotal > node.Samples {
			message := fmt.Sprintf(
				"Jennifer CSV node '%s' has children totaling %d samples, above parent sample_count %d.",
				node.Name, childTotal, node.Samples)
			addDiagnosticWarning(diagnostics, 0, "CHILD_SAMPLES_EXCEED_PARENT", message, "")
		}
		for index := range node.Children {
			walk(&node.Children[index])
		}
	}
	walk(root)
}

func assignJenniferPaths(root *FlameNode, includeRootPath bool) {
	type frame struct {
		node       *FlameNode
		parentPath []string
		isRoot     bool
	}
	stack := []frame{{node: root, parentPath: nil, isRoot: true}}
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if top.isRoot && !includeRootPath {
			top.node.Path = []string{}
		} else {
			top.node.Path = append(append([]string(nil), top.parentPath...), top.node.Name)
		}
		for index := range top.node.Children {
			child := &top.node.Children[index]
			stack = append(stack, frame{node: child, parentPath: top.node.Path, isRoot: false})
		}
	}
}

func findJenniferCycleKeys(parentByKey map[string]string) []string {
	cycle := map[string]bool{}
	visited := map[string]bool{}
	for start := range parentByKey {
		if visited[start] {
			continue
		}
		pathIndex := map[string]int{}
		path := []string{}
		current := start
		for {
			if _, ok := parentByKey[current]; !ok {
				break
			}
			if index, ok := pathIndex[current]; ok {
				for _, key := range path[index:] {
					cycle[key] = true
				}
				break
			}
			if visited[current] {
				break
			}
			pathIndex[current] = len(path)
			path = append(path, current)
			parent, ok := parentByKey[current]
			if !ok {
				break
			}
			current = parent
		}
		for _, key := range path {
			visited[key] = true
		}
	}
	out := make([]string, 0, len(cycle))
	for key := range cycle {
		out = append(out, key)
	}
	return out
}
