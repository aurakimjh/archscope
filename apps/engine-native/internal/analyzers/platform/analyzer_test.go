package platform

import (
	"testing"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/platform"
)

func TestAnalyzePlatformEvidence(t *testing.T) {
	result := Build([]parser.Record{
		{Kind: "kubernetes_event", Reason: "OOMKilled", Severity: "ERROR", Pod: "api-1"},
		{Kind: "pod", RestartCount: 2, Pod: "api-1"},
		{Kind: "cloud_audit", CloudProvider: "aws", Operation: "CreateAccessKey"},
	}, "platform.json", nil, Options{})
	if result.Summary["oom_killed_count"] != 1 || result.Summary["restart_count"] != 2 || result.Summary["security_event_count"] != 1 {
		t.Fatalf("summary=%+v", result.Summary)
	}
	if len(result.Metadata.Findings) < 3 {
		t.Fatalf("findings=%+v", result.Metadata.Findings)
	}
}
