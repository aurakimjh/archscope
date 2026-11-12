package execution

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"regexp"
	"strings"
)

var crashHeader = regexp.MustCompile(`^#\s+(SIGSEGV|SIGBUS|SIGILL|SIGFPE|SIGABRT|EXCEPTION_ACCESS_VIOLATION)\b.*pid=([0-9]+),\s*tid=([0-9]+)`)
var jreVersion = regexp.MustCompile(`^# JRE version:.*?\(([0-9][A-Za-z0-9.+_-]*)\)`)
var moduleFrame = regexp.MustCompile(`^([CJjVv])\s+\[([^\]\s/\\+]+)(?:\+0x[0-9a-fA-F]+)?\](?:\s+([A-Za-z0-9_.$:<>()/\[\];,+~?*=&!@%-]+))?`)
var javaFrame = regexp.MustCompile(`^([JjVv])\s+([A-Za-z0-9_.$:/<>]+)(?:\([^\r\n]*\))?`)

func parseCrash(ctx context.Context, raw []byte, s *e.Set) error {
	if bytes.Contains(raw, []byte{0}) {
		return fmt.Errorf("EXECUTION_CRASH_TEXT_REQUIRED: core/binary dumps unsupported")
	}
	s.Source.Producer = "HotSpot fatal error report"
	s.Source.Version = "unknown-layout"
	c := &e.Crash{Signal: "unknown", Runtime: "HotSpot", Completeness: "partial_or_unknown", Frames: []e.Frame{}}
	s.Crash = c
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 64<<10), 1<<20)
	signature := false
	wantProblem := false
	stack := false
	for line := 1; scan.Scan(); line++ {
		if line > 500000 {
			return fmt.Errorf("EXECUTION_CRASH_LINE_BUDGET")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		text := scan.Text()
		if strings.Contains(text, "A fatal error has been detected by the Java Runtime Environment") {
			signature = true
		}
		if v := crashHeader.FindStringSubmatch(text); v != nil {
			c.Signal = v[1]
			c.PID = e.Ptr(v[2])
			c.TID = e.Ptr(v[3])
		}
		if strings.HasPrefix(text, "#  Internal Error") {
			c.Signal = "INTERNAL_ERROR"
		}
		if v := jreVersion.FindStringSubmatch(text); v != nil {
			s.Source.Version = v[1]
		}
		if text == "END." {
			c.Completeness = "end_marker_present"
		}
		if strings.HasPrefix(text, "elapsed time:") {
			parts := strings.Fields(strings.TrimPrefix(text, "elapsed time:"))
			if len(parts) > 1 && strings.HasPrefix(parts[1], "second") {
				v, err := decimal(parts[0])
				if err == nil {
					c.Elapsed = v
				}
			}
		}
		if strings.HasPrefix(text, "# Problematic frame:") {
			wantProblem = true
			continue
		}
		if strings.HasPrefix(text, "Native frames:") || strings.HasPrefix(text, "Java frames:") {
			stack = true
			continue
		}
		clean := strings.TrimSpace(strings.TrimPrefix(text, "#"))
		if wantProblem || stack {
			wantProblem = false
			if clean == "" {
				stack = false
				continue
			}
			if len(c.Frames) >= 128 {
				return fmt.Errorf("EXECUTION_CRASH_FRAME_BUDGET")
			}
			f := e.Frame{Position: fmt.Sprintf("line/%d", line)}
			if v := moduleFrame.FindStringSubmatch(clean); v != nil {
				f.Kind = v[1]
				f.Module = v[2]
				f.Symbol = v[3]
			} else if v := javaFrame.FindStringSubmatch(clean); v != nil {
				f.Kind = v[1]
				f.Symbol = v[2]
			} else {
				stack = false
				continue
			}
			if !e.Text(f.Module) || !e.Text(f.Symbol) {
				return fmt.Errorf("EXECUTION_CRASH_FRAME_BUDGET")
			}
			c.Frames = append(c.Frames, f)
		}
	}
	if err := scan.Err(); err != nil {
		return fmt.Errorf("EXECUTION_CRASH_LINE_INVALID")
	}
	if !signature {
		return fmt.Errorf("EXECUTION_HS_ERR_SIGNATURE_REQUIRED")
	}
	s.Limitations = append(s.Limitations, "hs_err_pid contains sensitive process data. Only fatal signal, IDs, runtime version, elapsed time and bounded frame symbols/modules survive; no environment, command line, memory dumps or arbitrary messages.", "Problematic/crashing frames are observations, not proof of root cause. Missing END. means partial_or_unknown; a marker is not a completeness authentication.")
	return nil
}
