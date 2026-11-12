package localconvert_test

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localconvert"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jfr"
	"testing"
)

func TestR1001ConverterAndLegacyJSONReaderShareOutputCeiling(t *testing.T) {
	if localconvert.MaxOutput != jfr.MaxDirectJSONBytes {
		t.Fatal(localconvert.MaxOutput, jfr.MaxDirectJSONBytes)
	}
}
