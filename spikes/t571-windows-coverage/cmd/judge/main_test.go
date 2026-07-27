package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

func TestDispositionRequiresAppliedCAP3ForRatio(t *testing.T) {
	result := ratioCandidate(true, false)

	if ratioBearing(result) {
		t.Fatal("CAP-3 N/A must not produce a ratio-bearing disposition")
	}
	if got := disposition(result); !strings.Contains(got, "개별 endpoint 귀속만 승인") {
		t.Fatalf("disposition=%q, want individual-attribution-only", got)
	}
}

func TestDispositionAllowsHighAndMediumOnlyWithMeasuredCAP3(t *testing.T) {
	tests := []struct {
		name      string
		cap3Pass  bool
		want      string
		wantRatio bool
	}{
		{name: "high", cap3Pass: true, want: "Confidence: high", wantRatio: true},
		{name: "medium", cap3Pass: false, want: "Confidence: medium", wantRatio: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ratioCandidate(false, tt.cap3Pass)
			if got := ratioBearing(result); got != tt.wantRatio {
				t.Fatalf("ratioBearing=%v, want %v", got, tt.wantRatio)
			}
			if got := disposition(result); !strings.Contains(got, tt.want) {
				t.Fatalf("disposition=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestDispositionDiscardsAnyFalseAttribution(t *testing.T) {
	result := ratioCandidate(false, true)
	cap2 := result.Criteria["CAP-2"]
	cap2.Pass = false
	cap2.Numeric = 1
	result.Criteria["CAP-2"] = cap2

	if ratioBearing(result) {
		t.Fatal("CAP-2 failure must never be ratio-bearing")
	}
	if got := disposition(result); !strings.Contains(got, "폐기") {
		t.Fatalf("disposition=%q, want discarded", got)
	}
}

func TestRatioRequiresMeasuredCAP2(t *testing.T) {
	result := ratioCandidate(false, true)
	cap2 := result.Criteria["CAP-2"]
	cap2.Applied = false
	cap2.Pass = false
	result.Criteria["CAP-2"] = cap2

	if ratioBearing(result) {
		t.Fatal("CAP-2 N/A must never be ratio-bearing")
	}
	if got := disposition(result); !strings.Contains(got, "CAP-2 미측정") {
		t.Fatalf("disposition=%q, want CAP-2 unmeasured", got)
	}
}

func TestSummarizeCoverageUsesCounterFallbackForCAP3NA(t *testing.T) {
	fallback, outcome := summarizeCoverage([]capmodel.CapResult{ratioCandidate(true, false)})
	if !fallback {
		t.Fatal("CAP-3 N/A must enable counter fallback")
	}
	if !strings.Contains(outcome, "5개 카운터만 유지") {
		t.Fatalf("outcome=%q", outcome)
	}
}

func TestScoreTreatsEmptyWFPRelevantSetAsNotApplicable(t *testing.T) {
	gt := capmodel.GroundTruth{
		Controls: []capmodel.ControlProcess{{
			PID: 100, Image: `C:\loadgen.exe`, LocalPort: 49152,
			RemotePort: 18080, GoesViaProxy: false,
		}},
	}
	obs := capmodel.Observation{
		Candidate:             capmodel.CandidateWFP,
		Scope:                 capmodel.ScopeProcessAttribution,
		Elevated:              true,
		CPUOverheadPct:        1,
		KernelReportedDropped: -1,
	}

	got := score(capmodel.CandidateWFP, obs, gt, 0)
	for _, id := range []string{"CAP-1", "CAP-2", "CAP-4"} {
		if got.Criteria[id].Applied {
			t.Fatalf("%s Applied=true, want N/A for empty relevant observation set", id)
		}
	}
	if !strings.Contains(got.Disposition, "제품 coverage 후보에서 제거") {
		t.Fatalf("disposition=%q", got.Disposition)
	}
	if got.Criteria["CAP-6"].Applied {
		t.Fatal("CAP-6 must remain partial/unmeasured in the spike")
	}

	rows := appendixRows([]capmodel.CapResult{got})
	if len(rows) != 2 || rows[1].Status != "fixed" {
		t.Fatalf("WFP removal ledger row=%+v, want fixed conservative disposition", rows)
	}
}

func TestCandidateGroundTruthPrefersPerCandidateFile(t *testing.T) {
	dir := t.TempDir()
	fallback := capmodel.GroundTruth{
		Controls: []capmodel.ControlProcess{{PID: 1}},
	}
	candidate := capmodel.GroundTruth{
		Controls: []capmodel.ControlProcess{{PID: 2}},
	}
	name := "ground_truth_tcpowner.json"
	if err := control.WriteJSON(filepath.Join(dir, name), candidate); err != nil {
		t.Fatal(err)
	}

	got := candidateGroundTruth(dir, capmodel.CandidateTCPOwner, fallback, map[capmodel.Candidate]string{
		capmodel.CandidateTCPOwner: name,
	})
	if len(got.Controls) != 1 || got.Controls[0].PID != 2 {
		t.Fatalf("candidate ground truth=%+v", got.Controls)
	}
}

func ratioCandidate(cap3NA, cap3Pass bool) capmodel.CapResult {
	cap3 := capmodel.Verdict{ID: "CAP-3", Applied: !cap3NA, Pass: cap3Pass}
	return capmodel.CapResult{
		Candidate: capmodel.CandidateTCPOwner,
		Measured:  true,
		Criteria: map[string]capmodel.Verdict{
			"CAP-1": {ID: "CAP-1", Applied: true, Pass: true},
			"CAP-2": {ID: "CAP-2", Applied: true, Pass: true},
			"CAP-3": cap3,
			"CAP-4": {ID: "CAP-4", Applied: true, Pass: true},
		},
	}
}
