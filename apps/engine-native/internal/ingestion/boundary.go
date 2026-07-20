// Package ingestion defines shared file-ingestion contracts used before a
// parser hands typed records to an analyzer.
package ingestion

import (
	"fmt"
	"strings"
)

const (
	SourceKindAccessLog        = "access_log"
	SourceKindTrace            = "trace"
	SourceKindServerLog        = "server_log"
	SourceKindObservabilityLog = "observability_log"
	SourceKindMetricsSnapshot  = "metrics_snapshot"
	SourceKindDatabaseLog      = "database_log"
	SourceKindBrokerLog        = "broker_log"
	SourceKindPlatformEvidence = "platform_evidence"
	SourceKindCloudAuditLog    = "cloud_audit_log"
	SourceKindRuntimeProfile   = "runtime_profile"
	SourceKindHTTPCapture      = "http_capture"
)

// EvidenceFamilySpec fixes the package and public boundary naming for one
// evidence family. The values are intentionally strings so future packages can
// expose the same contract without importing parser or analyzer implementations.
type EvidenceFamilySpec struct {
	Family          string `json:"family"`
	SourceKind      string `json:"source_kind"`
	ResultType      string `json:"result_type"`
	ParserPackage   string `json:"parser_package"`
	AnalyzerPackage string `json:"analyzer_package"`
	CLIGroup        string `json:"cli_group"`
	CLILeaf         string `json:"cli_leaf"`
	WailsBinding    string `json:"wails_binding"`
}

// CoreEvidenceFamilies is the Mid-Term Plus ingestion boundary. New parser and
// analyzer work should either reuse one of these specs or add another explicit
// spec before implementation reaches the CLI or Wails binding surface.
func CoreEvidenceFamilies() []EvidenceFamilySpec {
	return []EvidenceFamilySpec{
		{
			Family:          "access_and_edge_logs",
			SourceKind:      SourceKindAccessLog,
			ResultType:      "access_log",
			ParserPackage:   "internal/parsers/accesslog",
			AnalyzerPackage: "internal/analyzers/accesslog",
			CLIGroup:        "access-log",
			CLILeaf:         "analyze",
			WailsBinding:    "AnalyzeAccessLog",
		},
		{
			Family:          "external_traces",
			SourceKind:      SourceKindTrace,
			ResultType:      "trace_import",
			ParserPackage:   "internal/parsers/traceimport",
			AnalyzerPackage: "internal/analyzers/traceimport",
			CLIGroup:        "trace",
			CLILeaf:         "import",
			WailsBinding:    "AnalyzeTraceImport",
		},
		{
			Family:          "application_server_logs",
			SourceKind:      SourceKindServerLog,
			ResultType:      "server_log",
			ParserPackage:   "internal/parsers/serverlog",
			AnalyzerPackage: "internal/analyzers/serverlog",
			CLIGroup:        "server-log",
			CLILeaf:         "analyze",
			WailsBinding:    "AnalyzeServerLog",
		},
		{
			Family:          "observability_logs",
			SourceKind:      SourceKindObservabilityLog,
			ResultType:      "otel_logs",
			ParserPackage:   "internal/parsers/otellogs",
			AnalyzerPackage: "internal/analyzers/otellogs",
			CLIGroup:        "otel-logs",
			CLILeaf:         "analyze",
			WailsBinding:    "AnalyzeOtelLogs",
		},
		{
			Family:          "offline_metrics",
			SourceKind:      SourceKindMetricsSnapshot,
			ResultType:      "metrics_snapshot",
			ParserPackage:   "internal/parsers/metrics",
			AnalyzerPackage: "internal/analyzers/metrics",
			CLIGroup:        "metrics",
			CLILeaf:         "import",
			WailsBinding:    "AnalyzeMetricsSnapshot",
		},
		{
			Family:          "database_slow_query",
			SourceKind:      SourceKindDatabaseLog,
			ResultType:      "database_slow_query",
			ParserPackage:   "internal/parsers/databaselog",
			AnalyzerPackage: "internal/analyzers/databaselog",
			CLIGroup:        "database-log",
			CLILeaf:         "analyze",
			WailsBinding:    "AnalyzeDatabaseLog",
		},
		{
			Family:          "broker_logs",
			SourceKind:      SourceKindBrokerLog,
			ResultType:      "broker_log",
			ParserPackage:   "internal/parsers/brokerlog",
			AnalyzerPackage: "internal/analyzers/brokerlog",
			CLIGroup:        "broker-log",
			CLILeaf:         "analyze",
			WailsBinding:    "AnalyzeBrokerLog",
		},
		{
			Family:          "kubernetes_and_container",
			SourceKind:      SourceKindPlatformEvidence,
			ResultType:      "kubernetes_evidence",
			ParserPackage:   "internal/parsers/platform",
			AnalyzerPackage: "internal/analyzers/platform",
			CLIGroup:        "platform",
			CLILeaf:         "import",
			WailsBinding:    "AnalyzePlatformEvidence",
		},
		{
			Family:          "cloud_audit_logs",
			SourceKind:      SourceKindCloudAuditLog,
			ResultType:      "cloud_audit_log",
			ParserPackage:   "internal/parsers/cloudaudit",
			AnalyzerPackage: "internal/analyzers/cloudaudit",
			CLIGroup:        "cloud-audit",
			CLILeaf:         "import",
			WailsBinding:    "AnalyzeCloudAuditLog",
		},
		{
			Family:          "http_capture",
			SourceKind:      SourceKindHTTPCapture,
			ResultType:      "http_capture",
			ParserPackage:   "internal/parsers/httpcapture",
			AnalyzerPackage: "internal/analyzers/httpcapture",
			CLIGroup:        "http-capture",
			CLILeaf:         "analyze",
			WailsBinding:    "AnalyzeHttpCapture",
		},
		{
			Family:          "runtime_profiles",
			SourceKind:      SourceKindRuntimeProfile,
			ResultType:      "profile_evidence",
			ParserPackage:   "internal/parsers/profile",
			AnalyzerPackage: "internal/analyzers/profile",
			CLIGroup:        "profile",
			CLILeaf:         "import",
			WailsBinding:    "AnalyzeProfileEvidence",
		},
	}
}

// ValidateEvidenceFamilySpecs catches boundary drift before a family reaches
// CLI or Wails bindings.
func ValidateEvidenceFamilySpecs(specs []EvidenceFamilySpec) error {
	seenFamily := map[string]struct{}{}
	seenResult := map[string]struct{}{}
	seenCLI := map[string]struct{}{}
	seenBinding := map[string]struct{}{}
	for _, spec := range specs {
		if spec.Family == "" || spec.SourceKind == "" || spec.ResultType == "" ||
			spec.ParserPackage == "" || spec.AnalyzerPackage == "" ||
			spec.CLIGroup == "" || spec.CLILeaf == "" || spec.WailsBinding == "" {
			return fmt.Errorf("evidence family spec has empty required field: %+v", spec)
		}
		for _, value := range []string{spec.Family, spec.SourceKind, spec.ResultType} {
			if strings.Contains(value, "-") {
				return fmt.Errorf("family/source/result identifiers must use snake_case: %q", value)
			}
		}
		if !strings.HasPrefix(spec.ParserPackage, "internal/parsers/") {
			return fmt.Errorf("%s parser package must live under internal/parsers", spec.Family)
		}
		if !strings.HasPrefix(spec.AnalyzerPackage, "internal/analyzers/") {
			return fmt.Errorf("%s analyzer package must live under internal/analyzers", spec.Family)
		}
		cliKey := spec.CLIGroup + " " + spec.CLILeaf
		if err := rejectDuplicate("family", spec.Family, seenFamily); err != nil {
			return err
		}
		if err := rejectDuplicate("result_type", spec.ResultType, seenResult); err != nil {
			return err
		}
		if err := rejectDuplicate("cli", cliKey, seenCLI); err != nil {
			return err
		}
		if err := rejectDuplicate("wails_binding", spec.WailsBinding, seenBinding); err != nil {
			return err
		}
	}
	return nil
}

func rejectDuplicate(label, value string, seen map[string]struct{}) error {
	if _, ok := seen[value]; ok {
		return fmt.Errorf("duplicate %s %q", label, value)
	}
	seen[value] = struct{}{}
	return nil
}
