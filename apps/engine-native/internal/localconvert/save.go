package localconvert

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/netpolicy"
	"os"
)

func preflight(output string, protected ...string) error {
	if _, e := localfiles.Preflight(output, protected...); e != nil {
		return e
	}
	_, e := localfiles.Preflight(output+".conversion.json", protected...)
	return e
}

// ConvertNew rejects destination conflicts before executing any selected tool.
func ConvertNew(ctx context.Context, input, tool, output string, protected ...string) (Provenance, error) {
	if e := netpolicy.RequireOnline("converter.jfr"); e != nil {
		return Provenance{}, PublicError(e)
	}
	if e := preflight(output, protected...); e != nil {
		return Provenance{}, e
	}
	result, e := JFR(ctx, input, tool)
	if e != nil {
		return Provenance{}, PublicError(e)
	}
	defer os.Remove(result.Path)
	if e = SaveNew(ctx, result, output, protected...); e != nil {
		return Provenance{}, PublicError(e)
	}
	return result.Provenance, nil
}

// SaveNew streams exact bytes and provenance into two unused names. If the
// second publication fails, the first owned publication is removed.
func SaveNew(ctx context.Context, result Result, output string, protected ...string) error {
	if e := preflight(output, protected...); e != nil {
		return e
	}
	f, e := os.Open(result.Path)
	if e != nil {
		return fail("CONVERTER_OUTPUT_CHANGED")
	}
	defer f.Close()
	meta, e := json.MarshalIndent(result.Provenance, "", "  ")
	if e != nil {
		return e
	}
	if e = localfiles.PublishReader(ctx, output, f, result.Provenance.OutputBytes, result.Provenance.OutputSHA256, protected...); e != nil {
		return e
	}
	if e = localfiles.Publish(ctx, output+".conversion.json", append(meta, '\n'), protected...); e != nil {
		os.Remove(output)
		return e
	}
	return nil
}
