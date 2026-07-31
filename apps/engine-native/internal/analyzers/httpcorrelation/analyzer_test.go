package httpcorrelation

import (
	"strings"
	"testing"
	"time"
)

func TestAnalyzeCorrelatesBoundedProfileJenniferAndAccessEvidence(t *testing.T) {
	httpResult := result("http_capture", map[string]any{
		"transactions": []map[string]any{
			httpRow("h1", "2026-07-30T01:00:00Z", 1200, "/users/123", 200, "req-1"),
			httpRow("h2", "2026-07-30T01:00:03Z", 200, "/health", 200, ""),
		},
	}, nil)
	profile := result("profile_evidence", map[string]any{
		"cpu_sample_runs": []map[string]any{
			{"stack": "render;JSON.parse", "start_us": int64(1_100_000), "end_us": int64(1_500_000), "duration_ms": 400.0},
		},
	}, map[string]any{"parser_metadata": map[string]any{"v8_start_time_us": int64(1_000_000)}})
	jennifer := result("jennifer_profile", map[string]any{
		"msa_edges": []map[string]any{
			{
				"guid": "g-1", "external_call_target": "https://api.example/users/123",
				"external_call_elapsed_ms": 1180.0, "adjusted_network_gap_ms": 70.0,
			},
		},
	}, nil)
	access := result("access_log", map[string]any{
		"sample_records": []map[string]any{
			{
				"timestamp": "2026-07-30T01:00:00.050Z", "method": "GET",
				"uri": "/users/999", "status": 200, "response_time_ms": 80.0, "request_id": "req-1",
			},
		},
	}, nil)

	got, err := Analyze(httpResult, profile, jennifer, access, Options{
		TopN: 10, TimeToleranceMS: 100,
		ProfileWallClockStart: "2026-07-30T01:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != ResultType || got.Metadata.SchemaVersion != SchemaVersion {
		t.Fatalf("unexpected envelope: type=%q schema=%q", got.Type, got.Metadata.SchemaVersion)
	}
	if got.Summary["causal_claims_allowed"] != false || got.Summary["store_or_file_rescanned"] != false {
		t.Fatalf("safety contract missing: %#v", got.Summary)
	}
	profileRows := got.Tables["http_profile_overlaps"].([]map[string]any)
	if len(profileRows) != 1 || profileRows[0]["alignment_grade"] != "aligned" || profileRows[0]["causal_claim_allowed"] != false {
		t.Fatalf("unexpected profile correlation: %#v", profileRows)
	}
	jenniferRows := got.Tables["jennifer_network_gap_checks"].([]map[string]any)
	if len(jenniferRows) != 1 || jenniferRows[0]["alignment_grade"] != "duration_only" ||
		jenniferRows[0]["match_basis"] != "target_host+nearest_duration" {
		t.Fatalf("unexpected Jennifer correlation: %#v", jenniferRows)
	}
	accessRows := got.Tables["access_log_matches"].([]map[string]any)
	if len(accessRows) != 1 || accessRows[0]["confidence"] != "high" || accessRows[0]["match_basis"] != "request_id" {
		t.Fatalf("unexpected access correlation: %#v", accessRows)
	}
	if accessRows[0]["alignment_grade"] != "aligned" || accessRows[0]["clock_compared"] != true ||
		accessRows[0]["timestamp_delta_ms"] != 50.0 {
		t.Fatalf("request identity and clock alignment were not evaluated independently: %#v", accessRows[0])
	}
	if got.Summary["aligned_source_count"] != 2 || got.Summary["duration_only_source_count"] != 1 {
		t.Fatalf("unexpected alignment summary: %#v", got.Summary)
	}
}

func TestAnalyzeSuppressesProfileOverlayWithoutWallClockAnchor(t *testing.T) {
	httpResult := result("http_capture", map[string]any{
		"transactions": []map[string]any{
			httpRow("h1", "2026-07-30T01:00:00Z", 100, "/api", 200, ""),
		},
	}, nil)
	profile := result("profile_evidence", map[string]any{
		"cpu_sample_runs": []map[string]any{
			{"stack": "work", "start_us": int64(100), "end_us": int64(200), "duration_ms": 0.1},
		},
	}, map[string]any{"parser_metadata": map[string]any{"v8_start_time_us": int64(0)}})

	got, err := Analyze(httpResult, profile, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rows := got.Tables["http_profile_overlaps"].([]map[string]any); len(rows) != 0 {
		t.Fatalf("incompatible clocks must not produce overlays: %#v", rows)
	}
	diagnostics := got.Tables["alignment_diagnostics"].([]sourceDiagnostic)
	if len(diagnostics) != 2 || diagnostics[1].AlignmentGrade != "none" || diagnostics[1].OverlayAllowed {
		t.Fatalf("profile clock did not fail closed: %#v", diagnostics)
	}
	if len(got.Metadata.Findings) != 1 || got.Metadata.Findings[0]["code"] != "HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE" {
		t.Fatalf("missing incompatible-clock finding: %#v", got.Metadata.Findings)
	}
}

func TestAnalyzeSuppressesProfileOverlayWithoutV8TimestampBase(t *testing.T) {
	httpResult := result("http_capture", map[string]any{
		"transactions": []map[string]any{
			httpRow("h1", "2026-07-30T01:00:00Z", 500, "/api", 200, ""),
		},
	}, nil)

	for _, testCase := range []struct {
		name     string
		metadata map[string]any
	}{
		{name: "missing", metadata: map[string]any{"parser_metadata": map[string]any{}}},
		{name: "non-numeric", metadata: map[string]any{"parser_metadata": map[string]any{"v8_start_time_us": "not-a-number"}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			profile := result("profile_evidence", map[string]any{
				"cpu_sample_runs": []map[string]any{
					{"stack": "work", "start_us": int64(100_000), "end_us": int64(300_000), "duration_ms": 200.0},
				},
			}, testCase.metadata)

			got, err := Analyze(httpResult, profile, nil, nil, Options{
				ProfileWallClockStart: "2026-07-30T01:00:00Z",
			})
			if err != nil {
				t.Fatal(err)
			}
			if rows := got.Tables["http_profile_overlaps"].([]map[string]any); len(rows) != 0 {
				t.Fatalf("invalid V8 timestamp base must not produce overlays: %#v", rows)
			}
			diagnostics := got.Tables["alignment_diagnostics"].([]sourceDiagnostic)
			if len(diagnostics) != 2 || diagnostics[1].AlignmentGrade != "none" || diagnostics[1].OverlayAllowed ||
				!strings.Contains(diagnostics[1].Reason, "v8_start_time_us") {
				t.Fatalf("invalid V8 timestamp base did not fail closed: %#v", diagnostics)
			}
			if len(got.Metadata.Findings) != 1 ||
				got.Metadata.Findings[0]["code"] != "HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE" ||
				!strings.Contains(got.Metadata.Findings[0]["message"].(string), "V8 timestamp base") {
				t.Fatalf("missing V8 timestamp-base finding: %#v", got.Metadata.Findings)
			}
		})
	}
}

func TestAnalyzeRequestIdentityDoesNotClaimUnmeasuredClockAlignment(t *testing.T) {
	for _, testCase := range []struct {
		name              string
		httpStartedAt     string
		accessTimestamp   string
		wantClockCompared bool
		wantDelta         float64
	}{
		{
			name: "large clock skew", httpStartedAt: "2026-07-30T01:00:00Z",
			accessTimestamp: "2026-07-30T07:00:00Z", wantClockCompared: true, wantDelta: 21_600_000,
		},
		{
			name: "missing client timestamp", httpStartedAt: "not-a-timestamp",
			accessTimestamp: "2026-07-30T01:00:00Z", wantClockCompared: false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			httpResult := result("http_capture", map[string]any{
				"transactions": []map[string]any{
					httpRow("h1", testCase.httpStartedAt, 100, "/api", 200, "req-1"),
				},
			}, nil)
			access := result("access_log", map[string]any{
				"sample_records": []map[string]any{
					{
						"timestamp": testCase.accessTimestamp, "method": "GET", "uri": "/api",
						"status": 200, "response_time_ms": 80.0, "request_id": "req-1",
					},
				},
			}, nil)

			got, err := Analyze(httpResult, nil, nil, access, Options{TimeToleranceMS: 100})
			if err != nil {
				t.Fatal(err)
			}
			rows := got.Tables["access_log_matches"].([]map[string]any)
			if len(rows) != 1 || rows[0]["match_basis"] != "request_id" ||
				rows[0]["alignment_grade"] != "duration_only" ||
				rows[0]["clock_compared"] != testCase.wantClockCompared {
				t.Fatalf("request identity was misreported as clock alignment: %#v", rows)
			}
			if testCase.wantClockCompared {
				if rows[0]["timestamp_delta_ms"] != testCase.wantDelta || rows[0]["timestamp_alignment_reason"] == nil {
					t.Fatalf("measured incompatible timestamp delta was not disclosed: %#v", rows[0])
				}
			} else {
				if _, exists := rows[0]["timestamp_delta_ms"]; exists ||
					rows[0]["timestamp_delta_unavailable_reason"] == nil {
					t.Fatalf("unmeasured timestamp delta must be explicitly unavailable: %#v", rows[0])
				}
			}
			diagnostics := got.Tables["alignment_diagnostics"].([]sourceDiagnostic)
			if diagnostics[1].AlignmentGrade != "duration_only" || diagnostics[1].OverlayAllowed {
				t.Fatalf("access-log source did not fail closed: %#v", diagnostics[1])
			}
			if got.Summary["aligned_source_count"] != 0 || got.Summary["duration_only_source_count"] != 1 {
				t.Fatalf("alignment summary hides the duration-only source: %#v", got.Summary)
			}
		})
	}
}

func TestAnalyzeRequestIdentityDoesNotReusePriorShapeCandidateClock(t *testing.T) {
	httpResult := result("http_capture", map[string]any{
		"transactions": []map[string]any{
			httpRow("h1", "2026-07-30T01:00:00Z", 100, "/api", 200, "req-1"),
		},
	}, nil)
	access := result("access_log", map[string]any{
		"sample_records": []map[string]any{
			{
				"timestamp": "2026-07-30T01:00:00.050Z", "method": "GET", "uri": "/api",
				"status": 200, "response_time_ms": 999.0,
			},
			{
				"timestamp": "not-a-timestamp", "method": "GET", "uri": "/api",
				"status": 200, "response_time_ms": 80.0, "request_id": "req-1",
			},
		},
	}, nil)

	got, err := Analyze(httpResult, nil, nil, access, Options{TimeToleranceMS: 100})
	if err != nil {
		t.Fatal(err)
	}
	rows := got.Tables["access_log_matches"].([]map[string]any)
	if len(rows) != 1 || rows[0]["match_basis"] != "request_id" ||
		rows[0]["server_response_ms"] != 80.0 {
		t.Fatalf("request identity did not select the expected record: %#v", rows)
	}
	if rows[0]["clock_compared"] != false || rows[0]["alignment_grade"] != "duration_only" {
		t.Fatalf("selected record inherited another candidate's clock evidence: %#v", rows[0])
	}
	if _, exists := rows[0]["timestamp_delta_ms"]; exists ||
		rows[0]["timestamp_delta_unavailable_reason"] == nil {
		t.Fatalf("selected record's unavailable clock was not preserved: %#v", rows[0])
	}
	diagnostics := got.Tables["alignment_diagnostics"].([]sourceDiagnostic)
	if diagnostics[1].AlignmentGrade != "duration_only" || diagnostics[1].OverlayAllowed {
		t.Fatalf("stale candidate clock enabled the access overlay: %#v", diagnostics[1])
	}
	if got.Summary["aligned_source_count"] != 0 || got.Summary["duration_only_source_count"] != 1 {
		t.Fatalf("stale candidate clock polluted the alignment summary: %#v", got.Summary)
	}
}

func TestAnalyzeValidatesTypesAndBoundsOutput(t *testing.T) {
	if _, err := Analyze(result("access_log", nil, nil), result("profile_evidence", nil, nil), nil, nil, Options{}); err == nil ||
		!strings.Contains(err.Error(), `want "http_capture"`) {
		t.Fatalf("expected HTTP type validation, got %v", err)
	}

	httpRows := []map[string]any{}
	accessRows := []map[string]any{}
	for index := 0; index < 5; index++ {
		id := "req-" + string(rune('a'+index))
		httpRows = append(httpRows, httpRow(id, "2026-07-30T01:00:00Z", 100, "/api", 200, id))
		accessRows = append(accessRows, map[string]any{
			"timestamp": "2026-07-30T01:00:00Z", "method": "GET", "uri": "/api",
			"status": 200, "response_time_ms": 10.0, "request_id": id,
		})
	}
	got, err := Analyze(
		result("http_capture", map[string]any{"transactions": httpRows}, nil),
		nil, nil, result("access_log", map[string]any{"sample_records": accessRows}, nil),
		Options{TopN: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if rows := got.Tables["access_log_matches"].([]map[string]any); len(rows) != 2 {
		t.Fatalf("output is not bounded: %#v", rows)
	}
	diagnostics := got.Tables["alignment_diagnostics"].([]sourceDiagnostic)
	if !diagnostics[1].OutputTruncated || diagnostics[1].CandidateRows != 5 || diagnostics[1].OutputRows != 2 {
		t.Fatalf("bounded output was not disclosed: %#v", diagnostics[1])
	}
}

func result(resultType string, tables map[string]any, extraMetadata map[string]any) map[string]any {
	if tables == nil {
		tables = map[string]any{}
	}
	metadata := map[string]any{"schema_version": "1.0.0"}
	for key, value := range extraMetadata {
		metadata[key] = value
	}
	return map[string]any{
		"type": resultType, "created_at": "2026-07-30T00:00:00Z",
		"source_files": []string{resultType + ".json"}, "tables": tables,
		"metadata": metadata,
	}
}

func httpRow(id, startedAt string, durationMS float64, path string, status int, requestID string) map[string]any {
	headers := []map[string]any{}
	if requestID != "" {
		headers = append(headers, map[string]any{"name": "X-Request-Id", "value": requestID})
	}
	start, _ := time.Parse(time.RFC3339Nano, startedAt)
	return map[string]any{
		"id": id, "started_at": startedAt,
		"ended_at": start.Add(time.Duration(durationMS * float64(time.Millisecond))).Format(time.RFC3339Nano),
		"method":   "GET", "host": "api.example", "path": path, "status": status,
		"duration_ms": durationMS, "request": map[string]any{"headers": headers},
		"timings": map[string]any{
			"importedHar": map[string]any{
				"dns":     map[string]any{"state": "known", "ms": 20.0},
				"connect": map[string]any{"state": "known", "ms": 30.0},
				"tls":     map[string]any{"state": "known", "ms": 10.0},
				"send":    map[string]any{"state": "known", "ms": 1.0},
				"receive": map[string]any{"state": "known", "ms": 2.0},
			},
		},
	}
}
