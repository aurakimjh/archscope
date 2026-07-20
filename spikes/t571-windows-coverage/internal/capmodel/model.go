// Package capmodel holds the shared data contract for the T-571 Windows
// proof-of-capability spike defined in docs/ko/SYSTEM_HTTP_CAPTURE.md §10.4.
//
// The whole point of the spike is that "we tried it" is not an outcome; a
// number against a threshold is. Every probe writes an Observation, and the
// judge turns Observations into CapResults and the two appendix-A ledger rows
// (Q-WIN-ETW-PAYLOAD, Q-WIN-WFP-ATTR).
package capmodel

import "time"

// Candidate is one of the three capture-scope candidates from §10.4.1.
type Candidate string

const (
	CandidateETW      Candidate = "etw-tcpip" // ETW Microsoft-Windows-Kernel-Network
	CandidateWFP      Candidate = "wfp"       // netsh wfp netevents
	CandidateNpcap    Candidate = "npcap"     // packet-level 5-tuple, no PID by design
	CandidateTCPOwner Candidate = "tcp-owner" // GetExtendedTcpTable / Get-NetTCPConnection ownership
)

// Scope mirrors CoverageEvidence.Scope in §10.1.2. It records the *kind* of
// evidence a candidate can honestly claim, so the judge never treats a
// 5-tuple observation as if it were a process attribution.
type Scope string

const (
	ScopeProcessAttribution Scope = "process_attribution" // ETW, WFP, TCP owner
	ScopeFlow5Tuple         Scope = "flow_5tuple"         // Npcap (no PID)
	ScopeAdapterCounter     Scope = "adapter_counter"     // NIC-level totals only
)

// ControlProcess is one member of the known control group (§10.4.2, CAP-1).
// loadgen spawns these and records ground truth; probes must rediscover them.
type ControlProcess struct {
	PID          int    `json:"pid"`
	Label        string `json:"label"`          // e.g. "worker-3"
	Image        string `json:"image"`          // fully-qualified exe path
	LocalPort    int    `json:"local_port"`     // ephemeral source port bound
	RemoteHost   string `json:"remote_host"`    // target listener host
	RemotePort   int    `json:"remote_port"`    // target listener port
	GoesViaProxy bool   `json:"goes_via_proxy"` // false => this is a CAP-4 bypass process
	Transactions int    `json:"transactions"`   // HTTP transactions it will drive
}

// GroundTruth is what loadgen actually did. It is the answer key the judge
// scores every probe against.
type GroundTruth struct {
	StartedAt        time.Time        `json:"started_at"`
	EndedAt          time.Time        `json:"ended_at"`
	TargetTPS        int              `json:"target_tps"`
	AchievedTPS      float64          `json:"achieved_tps"`
	ListenerAddr     string           `json:"listener_addr"`
	Controls         []ControlProcess `json:"controls"`
	TotalConnections int              `json:"total_connections"`
	TotalTxAttempted int              `json:"total_tx_attempted"`
	TotalTxOK        int              `json:"total_tx_ok"`
}

// AttributedFlow is one connection a probe observed and attributed (or failed
// to attribute) to a process.
type AttributedFlow struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	RemoteHost string `json:"remote_host"`
	PID        int    `json:"pid"`   // 0 => probe saw the flow but could not attribute a PID
	Image      string `json:"image"` // "" => unknown
	Observed   int    `json:"observed_events"`
}

// Observation is what every probe emits (one JSON file per probe run).
type Observation struct {
	Candidate Candidate        `json:"candidate"`
	Scope     Scope            `json:"scope"`
	Host      string           `json:"host"`
	OSVersion string           `json:"os_version"`
	Elevated  bool             `json:"elevated"`
	Tool      string           `json:"tool"` // exact command line used
	StartedAt time.Time        `json:"started_at"`
	EndedAt   time.Time        `json:"ended_at"`
	Flows     []AttributedFlow `json:"flows"`
	// Loss accounting (CAP-3). KernelReportedDropped is the count the kernel
	// tracing layer itself reported as lost (RealTimeLostEvents / buffers lost),
	// which is exactly the "counter로 노출 가능" requirement in §10.4.2.
	EventsDelivered       int64 `json:"events_delivered"`
	KernelReportedDropped int64 `json:"kernel_reported_dropped"`
	// CPU overhead sampled while the probe was capturing (CAP-5), percent of
	// one logical core summed across cores, i.e. Windows "% Processor Time".
	CPUOverheadPct float64 `json:"cpu_overhead_pct"`
	// Notes carries anything the probe could not encode structurally, e.g.
	// "pktmon PID column empty on this build".
	Notes []string `json:"notes,omitempty"`
	// Error is set when the probe could not run at all (tool missing, access
	// denied). A probe that could not run is NOT a pass or a fail — it is an
	// unmeasured candidate, and the judge records it as such.
	Error string `json:"error,omitempty"`
}

// CapResult is the per-candidate, per-criterion verdict.
type CapResult struct {
	Candidate Candidate          `json:"candidate"`
	Scope     Scope              `json:"scope"`
	Measured  bool               `json:"measured"` // false => probe never produced data
	Criteria  map[string]Verdict `json:"criteria"` // keyed by CAP-1..CAP-6
	// Disposition is the §10.4.3 outcome for this candidate.
	Disposition string `json:"disposition"`
}

// Verdict is one CAP-N line.
type Verdict struct {
	ID        string  `json:"id"`        // "CAP-1"
	Name      string  `json:"name"`      // "귀속 정확도"
	Threshold string  `json:"threshold"` // human-readable pass condition
	Value     string  `json:"value"`     // measured value, formatted
	Numeric   float64 `json:"numeric"`   // machine-comparable value when meaningful
	Pass      bool    `json:"pass"`
	Applied   bool    `json:"applied"` // false => not measurable for this candidate (e.g. CAP-1 for Npcap)
	Detail    string  `json:"detail,omitempty"`
}

// AppendixRow is the filled ledger row for appendix A of the design note.
// The spike's deliverable is these two rows moving from "open" to a status.
type AppendixRow struct {
	ClaimID string `json:"claim_id"` // Q-WIN-ETW-PAYLOAD / Q-WIN-WFP-ATTR
	Fact    string `json:"fact"`     // the measured fact
	Impact  string `json:"impact"`   // design impact
	Status  string `json:"status"`   // fixed | partial | open
}

// Report is the judge's full output.
type Report struct {
	GeneratedAt     time.Time     `json:"generated_at"`
	Host            string        `json:"host"`
	OSVersion       string        `json:"os_version"`
	LoopbackMode    bool          `json:"loopback_mode"` // true => control traffic was 127.x (smoke mode)
	GroundTruth     GroundTruth   `json:"ground_truth"`
	Results         []CapResult   `json:"results"`
	Findings        []string      `json:"findings,omitempty"` // cross-candidate observations (e.g. loopback invisibility)
	AppendixRows    []AppendixRow `json:"appendix_rows"`
	OverallOutcome  string        `json:"overall_outcome"`  // §10.4.3 last-row check
	CounterFallback bool          `json:"counter_fallback"` // true => absolute ratios removed, 5 counters only
}
