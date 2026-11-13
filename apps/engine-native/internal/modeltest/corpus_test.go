package modeltest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	i "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	s "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"os"
	"testing"
)

func TestRetainedAdversarialCorpusHashesAndRejections(t *testing.T) {
	raw, err := os.ReadFile("testdata/adversarial/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID    string `json:"id"`
		Files []struct {
			File        string `json:"file"`
			SHA         string `json:"sha256"`
			Family      string `json:"family"`
			Expectation string `json:"expectation"`
		} `json:"files"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil || manifest.ID != "diagnostics.parser-adversarial.v1" {
		t.Fatal(err)
	}
	rejected := 0
	for _, entry := range manifest.Files {
		body, err := os.ReadFile("testdata/adversarial/" + entry.File)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != entry.SHA {
			t.Fatal("retained corpus hash", entry.File)
		}
		if entry.Expectation != "reject" {
			continue
		}
		t.Run(entry.File, func(t *testing.T) {
			ctx := context.Background()
			var err error
			switch entry.Family {
			case "execution":
				_, err = e.Decode(ctx, body)
			case "gpu":
				_, err = g.Decode(ctx, body)
			case "inference":
				_, err = i.Decode(ctx, body)
			case "llm":
				_, err = l.Decode(ctx, body)
			case "runtime":
				_, err = r.Decode(ctx, body)
			case "system":
				_, err = s.Decode(ctx, body)
			default:
				t.Fatal("unknown family")
			}
			if err == nil {
				t.Fatal("retained invalid model accepted")
			}
		})
		rejected++
	}
	if rejected != 12 {
		t.Fatal("incomplete retained negative corpus", rejected)
	}
}
