package jenniferprofile

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Event-line grammar per §9.1: `[NNNN][HH:MM:SS NNN][NNNN][NNNN] message`.
// The first bracket can be 4 digits or all-spaces (END line uses `[    ]`).
// gap and cpu time can be empty (`[     ]`) on continuation-style lines.
var (
	eventLineRE = regexp.MustCompile(
		`^\[(\d{4}|\s*)\]\[(\d{2}:\d{2}:\d{2}\s+\d{3})\]\[\s*([0-9,]*)\]\[\s*([0-9,]*)\]\s*(.*)$`,
	)
	elapsedTrailingRE = regexp.MustCompile(`\[\s*([0-9,]+)\s*ms\s*\]\s*$`)
	totalLineRE       = regexp.MustCompile(`^\s*TOTAL\s*\[\s*([0-9,]+)\s*\]\s*\[\s*([0-9,]+)\s*\]\s*$`)
)

// parseBody walks every line of the body region, recognising START /
// END / TOTAL plus regular event rows. Lines that don't match the
// event grammar are appended to the most recent event's DetailLines
// (per §9.3 — SQL bodies, param dumps, etc.).
func parseBody(bodyText string, profile *models.JenniferTransactionProfile) {
	if bodyText == "" {
		return
	}
	body := &profile.Body
	var current *models.JenniferProfileEvent

	for _, rawLine := range strings.Split(bodyText, "\n") {
		// Trailing dash separator (`---…---`) closes the body table.
		// Stop once we see one outside an event continuation.
		if isDashLine(rawLine) {
			current = nil
			continue
		}
		if m := totalLineRE.FindStringSubmatch(rawLine); m != nil {
			body.HasTotal = true
			gap, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
			cpu, _ := strconv.Atoi(strings.ReplaceAll(m[2], ",", ""))
			body.TotalGapMs = &gap
			body.TotalCPUMs = &cpu
			current = nil
			continue
		}
		em := eventLineRE.FindStringSubmatch(rawLine)
		if em == nil {
			// Continuation: indented detail line for the prior event.
			if current != nil && strings.TrimSpace(rawLine) != "" {
				current.DetailLines = append(current.DetailLines, strings.TrimSpace(rawLine))
			}
			continue
		}
		event := models.JenniferProfileEvent{
			EventNo:    strings.TrimSpace(em[1]),
			EventStart: strings.TrimSpace(em[2]),
			RawMessage: strings.TrimSpace(em[5]),
		}
		event.GapMs = parseCommaInt(em[3])
		event.CPUTimeMs = parseCommaInt(em[4])

		// START / END are recognised here so the body knows whether
		// the profile fully bracketed.
		upperMsg := strings.ToUpper(event.RawMessage)
		if upperMsg == "START" {
			body.HasStart = true
			body.BodyStartTime = event.EventStart
			body.Events = append(body.Events, event)
			current = &body.Events[len(body.Events)-1]
			continue
		}
		if upperMsg == "END" {
			body.HasEnd = true
			body.Events = append(body.Events, event)
			current = nil
			continue
		}
		if elapsed, ok := extractTrailingElapsed(&event); ok {
			event.ElapsedMs = &elapsed
		}
		body.Events = append(body.Events, event)
		current = &body.Events[len(body.Events)-1]
	}
}

// extractTrailingElapsed pulls `[ NNN ms ]` off the end of an event
// message and returns it as an int, leaving the message minus the
// suffix. Reports `ok=false` when no elapsed marker is present.
func extractTrailingElapsed(event *models.JenniferProfileEvent) (int, bool) {
	m := elapsedTrailingRE.FindStringSubmatchIndex(event.RawMessage)
	if m == nil {
		return 0, false
	}
	value, err := strconv.Atoi(strings.ReplaceAll(event.RawMessage[m[2]:m[3]], ",", ""))
	if err != nil {
		return 0, false
	}
	// Trim the bracket off the visible message so downstream
	// rendering doesn't double-print it.
	event.RawMessage = strings.TrimSpace(event.RawMessage[:m[0]])
	return value, true
}

// parseCommaInt accepts comma-separated digits and empty strings;
// returns 0 for empty / unparseable so the body table stays
// schema-stable.
func parseCommaInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	v, err := strconv.Atoi(strings.ReplaceAll(value, ",", ""))
	if err != nil {
		return 0
	}
	return v
}

// isDashLine reports true for `--…--` separators that bracket the
// body table.
func isDashLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	return strings.Trim(t, "-") == ""
}
