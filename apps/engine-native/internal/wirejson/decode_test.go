package wirejson

import (
	"io"
	"strings"
	"testing"
)

type spaceReader struct{}

func (spaceReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = ' '
	}
	return len(b), nil
}
func TestReviewR01LegacyResultHasIndependentByteCeiling(t *testing.T) {
	// Large valid JSON with tiny data: do not confuse the byte ceiling with the
	// per-string/metadata limits. No large fixture is stored in the repository.
	source := func() io.Reader {
		return io.MultiReader(io.LimitReader(spaceReader{}, 65<<20), strings.NewReader(`{}`))
	}
	var value map[string]any
	if e := Read(source(), &value, LegacyResultLimits()); e != nil {
		t.Fatal("legacy result still capped at 64 MiB", e)
	}
	if e := Read(source(), &value, DefaultLimits); e == nil {
		t.Fatal("opaque byte budget was raised")
	}
}
