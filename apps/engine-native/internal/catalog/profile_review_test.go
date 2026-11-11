package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
)

func TestR4ProfileChunkBoundsAndOrder(t *testing.T) {
	first := profileChunk{Offset: 0, Data: []byte(`{"some":"json`)}
	body, e := appendProfileChunk(nil, first)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = appendProfileChunk(body, first); e == nil {
		t.Fatal("duplicate/out-of-order chunk accepted")
	}
	raw, _ := json.Marshal(profileChunk{Offset: 0, Data: make([]byte, profileChunkBytes+1)})
	if _, e = decodeProfileChunk(raw); e == nil {
		t.Fatal("oversized chunk accepted")
	}
	_, e = appendProfileChunk(make([]byte, MaxProfileTransportBytes), profileChunk{Offset: MaxProfileTransportBytes, Data: []byte("x")})
	var budget *ingestion.ImportError
	if !errors.As(e, &budget) || budget.Detail == nil || budget.Detail.ReasonCode != "profile_transport_bytes" || *budget.Detail.Limit != "33554432" {
		t.Fatal(e)
	}
}
func TestR4BoundedDetectionKeepsLargeDocumentsAsHints(t *testing.T) {
	c, _, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	original, _ := Contracts.ReadFile("contracts/profile-set.json")
	var object map[string]any
	json.Unmarshal(original, &object)
	object["extensions"] = map[string]any{"synthetic_padding": string(make([]byte, 70<<10))}
	large, _ := json.Marshal(object)
	for _, tc := range []struct {
		body     []byte
		verified bool
	}{{original, true}, {large, false}} {
		set, e := c.DetectCandidates(context.Background(), ingestion.Probe{Head: tc.body[:min(len(tc.body), ingestion.MaxProbeBytes)], Extension: ".json", SizeBytes: int64(len(tc.body))})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, candidate := range set.Candidates {
			if candidate.ImporterID == "profile.set-json" {
				found = true
				if (candidate.Grade == ingestion.Verified) != tc.verified {
					t.Fatal(candidate)
				}
			}
		}
		if !found {
			t.Fatal(set)
		}
		if !tc.verified && (set.Selected != "" || set.State != "needs_selection") {
			t.Fatal(set)
		}
	}
}
