package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func TestT629LegacyWrapperLeaseIdentityAndProjectionParity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "examples", "profiler", "sample-wall.collapsed")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	set, _, e := ParseLegacySet(context.Background(), path, "collapsed")
	if e != nil {
		t.Fatal(e)
	}
	if set.SourceHashes[0] != hex.EncodeToString(sum[:]) || len(set.Quality.ConversionLoss) == 0 {
		t.Fatal(set.Quality)
	}
	if len(set.Metrics) != 1 || set.Metrics[0].Kind != "sample_count" {
		t.Fatal("legacy samples became duration")
	}
	selection := profilemodel.Selection{MetricID: set.Metrics[0].ID}
	for _, p := range set.Profiles {
		selection.ProfileIDs = append(selection.ProfileIDs, p.ID)
		if p.Clock != "aggregate" {
			t.Fatal("invented timeline")
		}
	}
	tree, e := profilemodel.BuildTree(context.Background(), set, selection)
	if e != nil || tree.Root.Inclusive != "32629" {
		t.Fatal(tree, e)
	}
	for _, profile := range set.Profiles {
		for _, sample := range profile.Samples {
			if sample.Timestamp != nil || sample.Interval != nil {
				t.Fatal("legacy timestamp availability inferred")
			}
		}
	}
}
