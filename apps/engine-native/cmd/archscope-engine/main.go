// archscope-engine is the Go counterpart of `python -m
// archscope_engine.cli`. The full Cobra surface lands under T-360;
// this stub only wires the subcommands the parity gate needs (T-310 +
// T-330 access-log slice today).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
)

const usage = `archscope-engine — Go engine CLI (T-360 stub)

Usage:
  archscope-engine accesslog --in <path> [--format nginx] [--max-lines N]
                              [--start-time RFC3339] [--end-time RFC3339]
                              [--out <path>]

Subcommands ship as the matching Python tracks land. See work_status.md
T-301..T-392 for the full plan.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "accesslog":
		if err := runAccessLog(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "archscope-engine: %v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func runAccessLog(args []string) error {
	fs := flag.NewFlagSet("accesslog", flag.ExitOnError)
	in := fs.String("in", "", "path to access log (required)")
	format := fs.String("format", "nginx", "log format label")
	maxLines := fs.Int("max-lines", 0, "stop after N lines (0 = unlimited)")
	startStr := fs.String("start-time", "", "RFC3339 lower bound (inclusive)")
	endStr := fs.String("end-time", "", "RFC3339 upper bound (inclusive)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}

	opts := accesslog.Options{MaxLines: *maxLines}
	if *startStr != "" {
		t, err := time.Parse(time.RFC3339, *startStr)
		if err != nil {
			return fmt.Errorf("--start-time: %w", err)
		}
		opts.StartTime = &t
	}
	if *endStr != "" {
		t, err := time.Parse(time.RFC3339, *endStr)
		if err != nil {
			return fmt.Errorf("--end-time: %w", err)
		}
		opts.EndTime = &t
	}

	result, err := accesslog.Analyze(*in, *format, opts)
	if err != nil {
		return err
	}

	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	if *out == "-" {
		_, err := os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(*out, body, 0o644)
}