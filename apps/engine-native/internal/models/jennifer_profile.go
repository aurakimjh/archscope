package models

// JenniferTransactionProfile is one TXID block of a Jennifer Profile
// Export file. Multiple profiles share the same GUID when they belong
// to the same MSA call group.
type JenniferTransactionProfile struct {
	Header JenniferProfileHeader  `json:"header"`
	Body   JenniferProfileBody    `json:"body"`
	// SourceFile is the file the profile was lifted from — useful
	// when many files are batched into one analysis.
	SourceFile string `json:"source_file,omitempty"`
	// Errors collects per-profile validation failures (MISSING_*,
	// EVENT_PARSE_ERROR, …). When non-empty the profile fails
	// FULL-Profile validation in STRICT_MODE.
	Errors []JenniferProfileIssue `json:"errors,omitempty"`
	// Warnings collects non-fatal mismatches (header vs body
	// EXTERNAL_CALL_TIME drift inside tolerance, NEGATIVE_NETWORK_GAP_ADJUSTED, …).
	Warnings []JenniferProfileIssue `json:"warnings,omitempty"`
}

// JenniferProfileHeader is the 2-column key:value section that
// precedes the body table. All numeric fields are pointers so a
// missing field round-trips as JSON null.
type JenniferProfileHeader struct {
	TXID             string  `json:"txid"`
	GUID             string  `json:"guid,omitempty"`
	Domain           string  `json:"domain,omitempty"`
	DomainID         string  `json:"domain_id,omitempty"`
	StartTime        string  `json:"start_time,omitempty"`
	CollectionTime   string  `json:"collection_time,omitempty"`
	EndTime          string  `json:"end_time,omitempty"`
	ResponseTimeMs   *int    `json:"response_time_ms,omitempty"`
	SQLTimeMs        *int    `json:"sql_time_ms,omitempty"`
	SQLCount         *int    `json:"sql_count,omitempty"`
	ExternalCallMs   *int    `json:"external_call_time_ms,omitempty"`
	FetchTimeMs      *int    `json:"fetch_time_ms,omitempty"`
	CPUTimeMs        *int    `json:"cpu_time_ms,omitempty"`
	Instance         string  `json:"instance,omitempty"`
	InstanceID       string  `json:"instance_id,omitempty"`
	Business         string  `json:"business,omitempty"`
	BusinessID       string  `json:"business_id,omitempty"`
	ClientIP         string  `json:"client_ip,omitempty"`
	ClientID         string  `json:"client_id,omitempty"`
	UserID           string  `json:"user_id,omitempty"`
	UserAgent        string  `json:"user_agent,omitempty"`
	HTTPStatusCode   *int    `json:"http_status_code,omitempty"`
	FrontAppID       string  `json:"front_app_id,omitempty"`
	FrontPageID      string  `json:"front_page_id,omitempty"`
	Error            string  `json:"error,omitempty"`
	Application      string  `json:"application"`
	// Extra catches every key:value pair we encountered but didn't
	// promote to a typed field — keeps the parser forward-compatible
	// against new Jennifer columns.
	Extra map[string]string `json:"extra,omitempty"`
}

// JenniferProfileBody holds the events between START and END plus
// the trailing TOTAL line.
type JenniferProfileBody struct {
	HasBodyHeader bool                   `json:"has_body_header"`
	HasStart      bool                   `json:"has_start"`
	HasEnd        bool                   `json:"has_end"`
	HasTotal      bool                   `json:"has_total"`
	BodyStartTime string                 `json:"body_start_time,omitempty"` // HH:MM:SS NNN of START event
	TotalGapMs    *int                   `json:"total_gap_ms,omitempty"`
	TotalCPUMs    *int                   `json:"total_cpu_ms,omitempty"`
	Events        []JenniferProfileEvent `json:"events"`
}

// JenniferEventType is the priority-ordered classifier output. The
// `event_type` JSON value is what downstream metrics aggregate on.
type JenniferEventType string

const (
	JenniferEventStart            JenniferEventType = "PROFILE_START"
	JenniferEventEnd              JenniferEventType = "PROFILE_END"
	JenniferEventTotal            JenniferEventType = "PROFILE_TOTAL"
	JenniferEventExternalCall     JenniferEventType = "EXTERNAL_CALL"
	JenniferEventExternalCallInfo JenniferEventType = "EXTERNAL_CALL_INFO"
	JenniferEventFetch            JenniferEventType = "FETCH"
	JenniferEventTwoPCStart       JenniferEventType = "TWO_PC_XA_START"
	JenniferEventTwoPCEnd         JenniferEventType = "TWO_PC_XA_END"
	JenniferEventTwoPCPrepare     JenniferEventType = "TWO_PC_PREPARE"
	JenniferEventTwoPCCommit      JenniferEventType = "TWO_PC_COMMIT"
	JenniferEventTwoPCRollback    JenniferEventType = "TWO_PC_ROLLBACK"
	JenniferEventTwoPCUnknown     JenniferEventType = "TWO_PC_UNKNOWN"
	JenniferEventCheckQuery       JenniferEventType = "CHECK_QUERY"
	JenniferEventSQLExecute       JenniferEventType = "SQL_EXECUTE_GENERIC"
	JenniferEventSQLUpdate        JenniferEventType = "SQL_EXECUTE_UPDATE"
	JenniferEventSQLQuery         JenniferEventType = "SQL_EXECUTE_QUERY"
	JenniferEventConnAcquire      JenniferEventType = "CONNECTION_ACQUIRE"
	JenniferEventSocket           JenniferEventType = "SOCKET"
	JenniferEventMethod           JenniferEventType = "METHOD"
	// JenniferEventNetworkPrep — methods like
	// IntegrationUtil.sendToService that wrap the actual EXTERNAL_CALL.
	// We classify them so the analyzer can subtract the embedded
	// EXTERNAL_CALL elapsed and report the remainder as
	// "network preparation" (marshalling / DNS / SSL handshake / etc).
	JenniferEventNetworkPrep JenniferEventType = "NETWORK_PREP_METHOD"
	JenniferEventUnknown     JenniferEventType = "UNKNOWN"
)

// JenniferProfileEvent is one row of the body table.
type JenniferProfileEvent struct {
	EventNo       string            `json:"event_no,omitempty"`
	EventStart    string            `json:"event_start,omitempty"` // HH:MM:SS NNN
	GapMs         int               `json:"gap_ms"`
	CPUTimeMs     int               `json:"cpu_time_ms"`
	RawMessage    string            `json:"raw_message"`
	DetailLines   []string          `json:"detail_lines,omitempty"`
	EventType     JenniferEventType `json:"event_type"`
	ElapsedMs     *int              `json:"elapsed_ms,omitempty"`
	StartOffsetMs *int              `json:"start_offset_ms,omitempty"`
	EndOffsetMs   *int              `json:"end_offset_ms,omitempty"`

	// EXTERNAL_CALL extras (populated only when EventType matches).
	ExternalProtocol string `json:"external_protocol,omitempty"`
	ExternalClient   string `json:"external_client,omitempty"`
	ExternalURL      string `json:"external_url,omitempty"`

	// FETCH extras.
	CurrentFetchRows    *int `json:"current_fetch_rows,omitempty"`
	CumulativeFetchRows *int `json:"cumulative_fetch_rows,omitempty"`
}

// JenniferProfileIssue is one validation finding (error or warning).
type JenniferProfileIssue struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// JenniferMatchStatus is the per-edge matcher verdict.
type JenniferMatchStatus string

const (
	JenniferMatchOK         JenniferMatchStatus = "MATCHED"
	JenniferMatchUnmatched  JenniferMatchStatus = "UNMATCHED"
	JenniferMatchScoreLow   JenniferMatchStatus = "MATCH_SCORE_TOO_LOW"
	JenniferMatchAmbiguous  JenniferMatchStatus = "AMBIGUOUS_EXTERNAL_CALL_MATCH"
)

// JenniferExternalCallEdge is one Caller-Callee pairing inside a
// GUID group. Per spec §15.3.
type JenniferExternalCallEdge struct {
	GUID                  string              `json:"guid"`
	CallerTXID            string              `json:"caller_txid"`
	CallerApplication     string              `json:"caller_application"`
	ExternalCallSequence  int                 `json:"external_call_sequence"`
	ExternalCallURL       string              `json:"external_call_url"`
	ExternalCallElapsedMs int                 `json:"external_call_elapsed_ms"`
	CalleeTXID            string              `json:"callee_txid,omitempty"`
	CalleeApplication     string              `json:"callee_application,omitempty"`
	CalleeResponseTimeMs  *int                `json:"callee_response_time_ms,omitempty"`
	RawNetworkGapMs       *int                `json:"raw_network_gap_ms,omitempty"`
	AdjustedNetworkGapMs  *int                `json:"adjusted_network_gap_ms,omitempty"`
	MatchStatus           JenniferMatchStatus `json:"match_status"`
	MatchScore            int                 `json:"match_score,omitempty"`
	Warnings              []string            `json:"warnings,omitempty"`
	// CallerEventStartMs / CalleeBodyStartMs are ms-since-midnight
	// timestamps lifted from the caller's EXTERNAL_CALL event row
	// and the callee's body START row respectively. They feed the
	// MSA topology / Gantt timeline UI on the renderer side; absent
	// (zero) when the source profile didn't carry the timestamp.
	CallerEventStartMs int `json:"caller_event_start_ms,omitempty"`
	CalleeBodyStartMs  int `json:"callee_body_start_ms,omitempty"`
}

// JenniferGuidGroup is the §13 MSA transaction group: every profile
// sharing the same GUID, the matched/unmatched external calls, the
// inferred root profile and the call graph topology.
type JenniferGuidGroup struct {
	GUID               string                     `json:"guid"`
	ProfileTXIDs       []string                   `json:"profile_txids"`
	ProfileCount       int                        `json:"profile_count"`
	RootTXID           string                     `json:"root_txid,omitempty"`
	RootApplication    string                     `json:"root_application,omitempty"`
	RootResponseTimeMs *int                       `json:"root_response_time_ms,omitempty"`
	Edges              []JenniferExternalCallEdge `json:"edges"`
	MatchedEdgeCount   int                        `json:"matched_edge_count"`
	UnmatchedEdgeCount int                        `json:"unmatched_edge_count"`
	// CallGraph is a flat list of (caller_txid → callee_txid) tuples
	// with the edge metric. Used by the renderer to draw the DAG.
	CallGraph []JenniferCallGraphEdge `json:"call_graph"`
	// Metrics rolls up the §20.1 GuidTimelineMetrics envelope.
	Metrics JenniferGuidMetrics `json:"metrics"`
	// ValidationStatus per §10 — when any profile in the group
	// failed FULL validation we mark the group GROUP_FAILED.
	ValidationStatus string                 `json:"validation_status"`
	Errors           []JenniferProfileIssue `json:"errors,omitempty"`
	Warnings         []JenniferProfileIssue `json:"warnings,omitempty"`
}

// JenniferCallGraphEdge is one directed edge in the per-GUID call
// graph (node = profile TXID, edge = external call).
type JenniferCallGraphEdge struct {
	CallerTXID            string `json:"caller_txid"`
	CallerApplication     string `json:"caller_application"`
	CalleeTXID            string `json:"callee_txid"`
	CalleeApplication     string `json:"callee_application"`
	ExternalCallElapsedMs int    `json:"external_call_elapsed_ms"`
	CalleeResponseTimeMs  int    `json:"callee_response_time_ms"`
	NetworkGapMs          int    `json:"network_gap_ms"`
	CallerEventStartMs    int    `json:"caller_event_start_ms,omitempty"`
	CalleeBodyStartMs     int    `json:"callee_body_start_ms,omitempty"`
}

// JenniferTimelineSignature is the §19 cross-instance fingerprint
// of an MSA call structure. GUID is per-request; signature is
// stable across requests so the same business call pattern groups
// together for statistics.
type JenniferTimelineSignature struct {
	Version           string `json:"signature_version"`
	Hash              string `json:"signature_hash"`
	Canonical         string `json:"canonical_signature"`
	RootApplication   string `json:"root_application,omitempty"`
	EdgeCount         int    `json:"edge_count"`
}

// JenniferMetricStats is one metric's distribution stats, computed
// across every GUID group sharing a signature hash. Values are
// integer ms unless the metric is unitless (count, ratio).
type JenniferMetricStats struct {
	Count  int     `json:"count"`
	Min    float64 `json:"min"`
	Avg    float64 `json:"avg"`
	P50    float64 `json:"p50"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	Max    float64 `json:"max"`
	Stddev float64 `json:"stddev"`
}

// JenniferEdgeStats is the §21.3 per-edge stats inside a signature.
// `OccurrenceIndex` is 1-based and disambiguates same caller→callee
// edges that fire multiple times in the same call structure.
type JenniferEdgeStats struct {
	CallerApplication      string                `json:"caller_application"`
	CalleeApplication      string                `json:"callee_application"`
	OccurrenceIndex        int                   `json:"occurrence_index"`
	ExternalCallElapsedStats JenniferMetricStats `json:"external_call_elapsed_ms"`
	CalleeResponseTimeStats  JenniferMetricStats `json:"callee_response_time_ms"`
	RawNetworkGapStats       JenniferMetricStats `json:"raw_network_gap_ms"`
	AdjustedNetworkGapStats  JenniferMetricStats `json:"adjusted_network_gap_ms"`
}

// JenniferSignatureStats aggregates every GUID-group metric for one
// signature. Per-metric stats live on `Metrics`; per-edge stats on
// `Edges`. Sample count = number of GUID groups that fold into this
// signature.
type JenniferSignatureStats struct {
	Signature       JenniferTimelineSignature           `json:"signature"`
	SampleCount     int                                 `json:"sample_count"`
	GUIDs           []string                            `json:"guids"`
	Metrics         map[string]JenniferMetricStats      `json:"metrics"`
	Edges           []JenniferEdgeStats                 `json:"edges"`
}

// JenniferExecutionMode is the §16.5 verdict for one profile's
// EXTERNAL_CALL set: do calls run back-to-back (SEQUENTIAL), all
// overlap (PARALLEL), or some overlap and some don't (MIXED)?
type JenniferExecutionMode string

const (
	JenniferExecNone       JenniferExecutionMode = "NONE"
	JenniferExecSequential JenniferExecutionMode = "SEQUENTIAL"
	JenniferExecParallel   JenniferExecutionMode = "PARALLEL"
	JenniferExecMixed      JenniferExecutionMode = "MIXED"
)

// JenniferProfileParallelism is the §16.7 per-profile parallelism
// envelope. Cumulative-vs-wall-clock split is the entire point of
// MVP4 — we MUST keep these distinct so root-cause math stays sane.
type JenniferProfileParallelism struct {
	TXID                       string                `json:"txid"`
	ExternalCallCount          int                   `json:"external_call_count"`
	ExternalCallCumulativeMs   int                   `json:"external_call_cumulative_ms"`
	ExternalCallWallTimeMs     int                   `json:"external_call_wall_time_ms"`
	ParallelismRatio           float64               `json:"parallelism_ratio"`
	MaxExternalCallConcurrency int                   `json:"max_external_call_concurrency"`
	ExecutionMode              JenniferExecutionMode `json:"execution_mode"`
}

// JenniferGuidMetrics is the §20.1 single-instance summary per GUID.
// MVP4 adds wall-clock + parallelism columns alongside the existing
// cumulative ones.
type JenniferGuidMetrics struct {
	GUID                            string `json:"guid"`
	RootApplication                 string `json:"root_application,omitempty"`
	RootResponseTimeMs              *int   `json:"root_response_time_ms,omitempty"`
	ProfileCount                    int    `json:"profile_count"`
	MatchedExternalCallCount        int    `json:"matched_external_call_count"`
	UnmatchedExternalCallCount      int    `json:"unmatched_external_call_count"`
	TotalExternalCallCumulativeMs   int    `json:"total_external_call_cumulative_ms"`
	TotalExternalCallWallTimeMs     int    `json:"total_external_call_wall_time_ms"`
	TotalNetworkGapCumulativeMs     int    `json:"total_network_gap_cumulative_ms"`
	TotalSqlExecuteMs               int    `json:"total_sql_execute_ms"`
	TotalCheckQueryMs               int    `json:"total_check_query_ms"`
	TotalTwoPcMs                    int    `json:"total_two_pc_ms"`
	TotalFetchMs                    int    `json:"total_fetch_ms"`
	TotalFetchRows                  int    `json:"total_fetch_rows"`
	TotalConnectionAcquireMs        int    `json:"total_connection_acquire_ms"`
	MaxExternalCallConcurrency      int    `json:"max_external_call_concurrency"`
	ExternalCallParallelismRatio    float64 `json:"external_call_parallelism_ratio"`
	GroupExecutionMode              JenniferExecutionMode `json:"group_execution_mode"`
	ProfileParallelism              []JenniferProfileParallelism `json:"profile_parallelism,omitempty"`
	// ResponseTimeBreakdown decomposes root_response_time_ms into the
	// categories the user wants to see. This is the "where did the
	// time go" view that drives improvement decisions.
	ResponseTimeBreakdown JenniferResponseTimeBreakdown `json:"response_time_breakdown"`
}

// JenniferResponseTimeBreakdown decomposes the root profile's wall-
// clock response time into the categories the user reasons about
// when targeting an optimisation. The math:
//
//	method_time = root_response - (sql + check_query + 2pc + fetch
//	            + network_call + network_prep + connection_acquire)
//
// network_call uses adjusted_network_gap (the time spent in the
// wire / waiting between caller and callee) rather than the raw
// external_call_elapsed, because external_call_elapsed already
// embeds the callee's own response time which gets captured in
// the callee's profile (and thus in this group's sum).
//
// method_time can go negative if the callee response time wasn't
// captured (unmatched edges) or if there's overlap from missing
// data; the renderer clamps to 0 and shows a warning.
type JenniferResponseTimeBreakdown struct {
	RootResponseTimeMs    int     `json:"root_response_time_ms"`
	SQLExecuteMs          int     `json:"sql_execute_ms"`
	CheckQueryMs          int     `json:"check_query_ms"`
	TwoPCMs               int     `json:"two_pc_ms"`
	FetchMs               int     `json:"fetch_ms"`
	NetworkCallMs         int     `json:"network_call_ms"`
	NetworkPrepMs         int     `json:"network_prep_ms"`
	ConnectionAcquireMs   int     `json:"connection_acquire_ms"`
	MethodTimeMs          int     `json:"method_time_ms"`
	MethodTimeRatio       float64 `json:"method_time_ratio"`
	Coverage              float64 `json:"coverage"`
	NegativeMethodTime    bool    `json:"negative_method_time,omitempty"`
}

// JenniferBodyMetrics is the aggregated cost ledger emitted per
// transaction profile. Cumulative-only for MVP1; wall-clock /
// parallelism comes in MVP4.
type JenniferBodyMetrics struct {
	SQLExecuteCumMs       int `json:"sql_execute_cum_ms"`
	SQLExecuteCount       int `json:"sql_execute_count"`
	CheckQueryCumMs       int `json:"check_query_cum_ms"`
	CheckQueryCount       int `json:"check_query_count"`
	TwoPCCumMs            int `json:"two_pc_cum_ms"`
	TwoPCCount            int `json:"two_pc_count"`
	FetchCumMs            int `json:"fetch_cum_ms"`
	FetchCount            int `json:"fetch_count"`
	FetchTotalRows        int `json:"fetch_total_rows"`
	ExternalCallCumMs     int `json:"external_call_cum_ms"`
	ExternalCallCount     int `json:"external_call_count"`
	ConnectionAcquireCumMs int `json:"connection_acquire_cum_ms"`
	ConnectionAcquireCount int `json:"connection_acquire_count"`
	// NetworkPrepMethodCumMs is the raw sum of method elapsed for
	// frames that match the configured network-prep patterns
	// (default: IntegrationUtil.sendToService). NetworkPrepCumMs is
	// the derived remainder: NetworkPrepMethodCumMs minus the
	// matched-call ExternalCallCumMs (clamped to 0). The latter is
	// what users see as "external call preparation time".
	NetworkPrepMethodCumMs int `json:"network_prep_method_cum_ms"`
	NetworkPrepMethodCount int `json:"network_prep_method_count"`
	NetworkPrepCumMs       int `json:"network_prep_cum_ms"`
}
