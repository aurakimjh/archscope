package localconvert

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	"os"
)

// SaveNew preserves JSON plus exact conversion provenance as separate new files.
// If the second publication fails, the first owned file is removed.
func SaveNew(ctx context.Context, result Result, output string, protected ...string) error {
	body, e := os.ReadFile(result.Path)
	if e != nil {
		return e
	}
	if int64(len(body)) != result.Provenance.OutputBytes {
		return fail("CONVERTER_OUTPUT_CHANGED")
	}
	digest := sha256.Sum256(body)
	if hex.EncodeToString(digest[:]) != result.Provenance.OutputSHA256 {
		return fail("CONVERTER_OUTPUT_CHANGED")
	}
	meta, e := json.MarshalIndent(result.Provenance, "", "  ")
	if e != nil {
		return e
	}
	if e = localfiles.Publish(ctx, output, body, protected...); e != nil {
		return e
	}
	if e = localfiles.Publish(ctx, output+".conversion.json", append(meta, '\n'), protected...); e != nil {
		os.Remove(output)
		return e
	}
	return nil
}
