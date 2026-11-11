package serverlog

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
)

type ImportOptions struct {
	TopN     int  `json:"top_n"`
	MaxLines int  `json:"max_lines"`
	Strict   bool `json:"strict"`
}

// NewImporter binds a compiled parser to a catalog-owned descriptor. This
// initial adapter deliberately supports plain UTF-8 only; legacy textio paths
// retain their encoding fallback and aliases unchanged.
func NewImporter(format string, d ingestion.ImporterDescriptor) ingestion.ImporterDescriptor {
	d.Detector = func(ctx context.Context, p ingestion.Probe) (ingestion.Candidate, error) {
		if e := ctx.Err(); e != nil {
			return ingestion.Candidate{}, e
		}
		for _, line := range strings.Split(strings.TrimPrefix(string(p.Head), "\ufeff"), "\n") {
			if !utf8.ValidString(line) {
				continue
			}
			if record, _ := ParseLineForFormat(strings.TrimSuffix(line, "\r"), format); record != nil {
				return ingestion.Candidate{Grade: ingestion.Verified, Confidence: 0.9, Reason: "server log record structure"}, nil
			}
		}
		for _, ext := range d.Extensions {
			if p.Extension == ext {
				return ingestion.Candidate{Grade: ingestion.Hint, Confidence: 0.25, Reason: "filename extension only"}, nil
			}
		}
		return ingestion.Candidate{}, nil
	}
	d.ValidateInput = func(ctx context.Context, source ingestion.Source) error {
		f, e := source.Open(ctx)
		if e != nil {
			return e
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, ingestion.MaxProbeBytes))
		if e != nil {
			return e
		}
		c, e := d.Detector(ctx, ingestion.Probe{Head: b})
		if e != nil {
			return e
		}
		if c.Grade != ingestion.Verified {
			return ingestion.Problem("IMPORT_FORMAT_MISMATCH", "no verified server log structure")
		}
		return nil
	}
	d.ParserFactory = func(raw json.RawMessage) ingestion.Parser {
		var opts ImportOptions
		json.Unmarshal(raw, &opts)
		return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
			diags := diagnostics.New(format)
			diags.SetSourceFile(source.Info().Name)
			report := ingestion.ParseReport{Diagnostics: diags}
			f, e := source.Open(ctx)
			if e != nil {
				return report, e
			}
			defer f.Close()
			reader := bufio.NewReaderSize(f, 1<<20)
			var offset, line int64
			var sequence uint64
			for {
				if e = ctx.Err(); e != nil {
					return report, e
				}
				if opts.MaxLines > 0 && line >= int64(opts.MaxLines) {
					report.Complete = true
					report.SelectionReason = "max_lines"
					return report, nil
				}
				raw, e := reader.ReadSlice('\n')
				if e != nil && e != io.EOF {
					if ctx.Err() != nil {
						return report, ctx.Err()
					}
					if errors.Is(e, bufio.ErrBufferFull) {
						return report, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "line exceeds parser buffer")
					}
					return report, ingestion.Problem("IMPORT_IO_FAILED", "source read failed")
				}
				if len(raw) == 0 && e == io.EOF {
					break
				}
				line++
				start := offset
				offset += int64(len(raw))
				diags.TotalLines++
				text := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
				if line == 1 {
					text = strings.TrimPrefix(text, "\ufeff")
				}
				if !utf8.ValidString(text) {
					return report, ingestion.Problem("IMPORT_ENCODING_UNSUPPORTED", "new streaming adapter requires UTF-8")
				}
				if strings.TrimSpace(text) == "" {
					if e == io.EOF {
						break
					}
					continue
				}
				record, perr := ParseLineForFormat(text, format)
				if record == nil {
					diags.AddSkipped(int(line), perr.Reason, perr.Message, text)
					if opts.Strict {
						return report, ingestion.Problem("IMPORT_INPUT_INVALID", "invalid server log record")
					}
					if e == io.EOF {
						break
					}
					continue
				}
				sequence++
				payload, marshalErr := json.Marshal(record)
				if marshalErr != nil {
					return report, marshalErr
				}
				item := ingestion.Record{Sequence: sequence, Position: ingestion.Position{Byte: start, Line: line}, Payload: payload, Keys: []string{"product:" + record.Product, "component:" + record.Component, "event:" + record.EventType}}
				if writeErr := sink.Write(ctx, ingestion.Batch{Family: d.RecordFamily, Records: []ingestion.Record{item}}); writeErr != nil {
					var p *ingestion.ImportError
					if errors.As(writeErr, &p) && p.Code == "IMPORT_BUDGET_EXCEEDED" {
						report.SafeBoundary = true
						report.PartialReason = "record_budget"
					}
					return report, writeErr
				}
				diags.ParsedRecords++
				if e == io.EOF {
					break
				}
			}
			report.Complete = true
			return report, nil
		})
	}
	d.Normalize = func(ctx context.Context, r ingestion.Record) (ingestion.Record, error) { return r, ctx.Err() }
	return d
}
