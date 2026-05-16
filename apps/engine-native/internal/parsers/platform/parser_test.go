package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseKubernetesAndCloudEvidence(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.json")
	if err := os.WriteFile(events, []byte(`{"items":[{"type":"Warning","reason":"OOMKilled","message":"Container killed","lastTimestamp":"2026-05-16T10:00:00Z","involvedObject":{"namespace":"shop","name":"api-1"}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err := ParseFile(events, "auto", Options{})
	if err != nil || len(records) != 1 || records[0].Reason != "OOMKilled" {
		t.Fatalf("k8s records=%+v err=%v", records, err)
	}
	cloud := filepath.Join(dir, "cloudtrail.json")
	if err := os.WriteFile(cloud, []byte(`{"Records":[{"eventTime":"2026-05-16T10:00:00Z","eventSource":"iam.amazonaws.com","eventName":"CreateAccessKey","userIdentity":{"arn":"arn:aws:iam::123:user/demo"}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err = ParseFile(cloud, "auto", Options{})
	if err != nil || len(records) != 1 || records[0].CloudProvider != "aws" {
		t.Fatalf("cloud records=%+v err=%v", records, err)
	}
}
