package jenniferprofile

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	totalTransactionRE = regexp.MustCompile(`Total\s+Transaction\s*:\s*([0-9,]+)`)
	txidLineRE         = regexp.MustCompile(`(?m)^TXID\s*:\s*\S+`)
	bodyHeaderLineRE   = regexp.MustCompile(`(?m)^\s*\[\s*No\.\s*\]\s*\[\s*START_TIME\s*\]\s*\[\s*GAP\s*\]\s*\[\s*CPU_T\s*\]\s*$`)
)

// parseTotalTransaction extracts the leading `Total Transaction : N`
// per §7.1. Returns (value, ok). Comma-separated digits are accepted
// (e.g. `1,234`).
func parseTotalTransaction(text string) (int, bool) {
	m := totalTransactionRE.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil {
		return 0, false
	}
	return n, true
}

// splitByTxid breaks the file into transaction blocks per §7.3:
// block N starts at the Nth `TXID :` line and ends right before the
// (N+1)th. The final block runs to EOF.
func splitByTxid(text string) []string {
	indices := txidLineRE.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return nil
	}
	blocks := make([]string, 0, len(indices))
	for i, idx := range indices {
		start := idx[0]
		var end int
		if i+1 < len(indices) {
			end = indices[i+1][0]
		} else {
			end = len(text)
		}
		blocks = append(blocks, text[start:end])
	}
	return blocks
}

// splitHeaderBody separates a single TXID block into its header
// (everything before the body table) and body (everything after,
// excluding the table-header line itself). The body-header line
// presence is what `hasBodyHeader` reports — callers use it to
// emit MISSING_BODY_HEADER when absent.
func splitHeaderBody(block string) (headerText string, bodyText string, hasBodyHeader bool) {
	loc := bodyHeaderLineRE.FindStringIndex(block)
	if loc == nil {
		return block, "", false
	}
	headerText = block[:loc[0]]
	// Skip past the `[ No.][ START_TIME ]…` line plus its trailing
	// dashes line so the body parser only sees event/START/END/TOTAL
	// rows.
	bodyText = block[loc[1]:]
	bodyText = strings.TrimLeft(bodyText, "\n")
	bodyText = trimLeadingDashLine(bodyText)
	return headerText, bodyText, true
}

// trimLeadingDashLine drops a single leading line of dashes when
// present. Jennifer wraps the body table with a `---…---` line above
// AND below the header row; the splitter already skipped the row
// itself, this strips the trailing-dash line so events start clean.
func trimLeadingDashLine(text string) string {
	nl := strings.IndexByte(text, '\n')
	if nl < 0 {
		return text
	}
	first := strings.TrimSpace(text[:nl])
	if first != "" && strings.Trim(first, "-") == "" {
		return text[nl+1:]
	}
	return text
}
