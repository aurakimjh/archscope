// Package automation defines explicit desktop launch/control commands. The
// desktop remains visible; this protocol does not impersonate the legacy COM API.
package automation

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Capture                                   bool
	Process                                   string
	PIDs                                      []int32
	Scope                                     string
	Duration                                  time.Duration
	Destination, Format, OpenPath, Descriptor string
	Help                                      bool
}

const Usage = `ArchScope desktop
  --capture [--process ABS_EXE | --pid PID ... | --scope all_sessions|current_session]
  [--duration 30s] [--destination ABS_PATH --format session|har|xml|xml_compact|urls|text|html|xls|responses]
  --open ABS_SESSION
  --automation-descriptor ABS_PATH   Enable authenticated loopback automation
Legacy /p, /d, /t(minutes), /f are parsed. /f NATIVE (.halog) and .lnk are unavailable.
`

func ParseOptions(args []string) (Options, error) {
	out := Options{Format: "session", Scope: "all_sessions"}
	seen := map[string]bool{}
	legacy := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		key, value, has := strings.Cut(arg, "=")
		if strings.HasPrefix(arg, "/") && len(arg) >= 2 && strings.ContainsRune("pdtf", rune(strings.ToLower(arg)[1])) {
			legacy = true
			key = strings.ToLower(arg[:2])
			value = strings.TrimLeft(arg[2:], "=:")
			has = value != ""
		}
		key = strings.ToLower(key)
		if key == "--help" || key == "-h" {
			out.Help = true
			continue
		}
		if key == "--capture" {
			if has {
				return out, fmt.Errorf("--capture has no value")
			}
			out.Capture = true
			continue
		}
		switch key {
		case "--process", "--pid", "--scope", "--duration", "--destination", "--format", "--open", "--automation-descriptor", "/p", "/d", "/t", "/f":
		default:
			return out, fmt.Errorf("unknown desktop option %q", key)
		}
		if !has {
			i++
			if i >= len(args) {
				return out, fmt.Errorf("%s requires a value", key)
			}
			value = args[i]
		}
		value = strings.Trim(value, "\"")
		if value == "" {
			return out, fmt.Errorf("%s requires a nonempty value", key)
		}
		canonical := map[string]string{"/p": "--process", "/d": "--destination", "/t": "--duration", "/f": "--format"}[key]
		if canonical == "" {
			canonical = key
		}
		if seen[canonical] && canonical != "--pid" {
			return out, fmt.Errorf("duplicate option %s", canonical)
		}
		seen[canonical] = true
		switch canonical {
		case "--process":
			out.Process = value
			out.Capture = true
		case "--pid":
			n, e := strconv.ParseInt(value, 10, 32)
			if e != nil || n <= 0 {
				return out, fmt.Errorf("PID must be positive")
			}
			out.PIDs = append(out.PIDs, int32(n))
			out.Capture = true
		case "--scope":
			out.Scope = value
		case "--duration":
			var e error
			if key == "/t" {
				n, err := strconv.ParseFloat(value, 64)
				if err != nil || n <= 0 || n > 1440 {
					return out, fmt.Errorf("/t must be 0–1440 minutes")
				}
				out.Duration = time.Duration(n * float64(time.Minute))
			} else {
				out.Duration, e = time.ParseDuration(value)
				if e != nil {
					return out, fmt.Errorf("invalid duration")
				}
			}
			if out.Duration <= 0 || out.Duration > 24*time.Hour {
				return out, fmt.Errorf("duration must be between 1ns and 24h")
			}
		case "--destination":
			out.Destination = value
			if key == "/d" {
				out.Capture = true
			}
		case "--format":
			format := strings.ToLower(value)
			aliases := map[string]string{"full": "xml", "compact": "xml_compact", "url": "urls", "contenttodir": "responses"}
			if mapped := aliases[format]; mapped != "" {
				format = mapped
			}
			if format == "native" {
				return out, fmt.Errorf("legacy NATIVE .halog format is unverified; choose --format session or HAR")
			}
			out.Format = format
		case "--open":
			out.OpenPath = value
		case "--automation-descriptor":
			out.Descriptor = value
		}
	}
	if out.Help {
		return out, nil
	}
	if out.Process != "" && len(out.PIDs) > 0 {
		return out, fmt.Errorf("choose process launch or running PIDs")
	}
	if out.Process != "" && strings.EqualFold(filepath.Ext(out.Process), ".lnk") {
		return out, fmt.Errorf("Windows shortcut resolution is not implemented; choose the executable")
	}
	if len(out.PIDs) > 1024 {
		return out, fmt.Errorf("too many PIDs")
	}
	unique := map[int32]bool{}
	for _, pid := range out.PIDs {
		if unique[pid] {
			return out, fmt.Errorf("duplicate PID")
		}
		unique[pid] = true
	}
	if out.Scope != "all_sessions" && out.Scope != "current_session" {
		return out, fmt.Errorf("unknown scope")
	}
	if seen["--scope"] && (out.Process != "" || len(out.PIDs) > 0) {
		return out, fmt.Errorf("scope cannot be combined with a process target")
	}
	if out.OpenPath != "" && out.Capture {
		return out, fmt.Errorf("open and capture are mutually exclusive")
	}
	for _, path := range []string{out.Process, out.Destination, out.OpenPath, out.Descriptor} {
		if path != "" && !filepath.IsAbs(path) {
			return out, fmt.Errorf("desktop file paths must be absolute")
		}
	}
	if !out.Capture && (out.Destination != "" || seen["--duration"] || seen["--format"] || seen["--scope"]) {
		return out, fmt.Errorf("capture options require --capture")
	}
	switch out.Format {
	case "session", "har", "xml", "xml_compact", "urls", "text", "html", "xls", "responses":
	default:
		return out, fmt.Errorf("unsupported export format")
	}
	if strings.EqualFold(filepath.Ext(out.Destination), ".halog") {
		return out, fmt.Errorf(".halog is reserved for verified legacy files; choose another extension")
	}
	if legacy && out.Capture && out.Duration == 0 {
		out.Duration = 30 * time.Minute
	}
	if out.Destination != "" && out.Duration == 0 {
		return out, fmt.Errorf("automatic destination requires --duration")
	}
	return out, nil
}
