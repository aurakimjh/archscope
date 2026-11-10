package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jenniferprofile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestJenniferDrilldownCLIJSONAndReportParity(t *testing.T) {
	input := filepath.Join("..", "..", "internal", "parsers", "jenniferprofile", "testdata", "sample_two_tx.txt")
	source, err := jenniferprofile.AnalyzeFile(input, jenniferprofile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	group := source.Series["guid_groups"].([]map[string]any)[0]
	guid, txid := group["guid"].(string), group["root_txid"].(string)
	expected, err := jenniferprofile.ProjectDrilldown(source, jenniferprofile.DrilldownOptions{GUID: guid, TXID: txid})
	if err != nil {
		t.Fatal(err)
	}
	binary := buildBinary(t)
	args := []string{"jennifer-profile", "analyze", "--in", input, "--guid", guid, "--txid", txid}
	raw, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	var actual models.AnalysisResult
	if err = json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	// Parsing timestamps differ across processes; selection and every numerical
	// result must still match the public analyzer projection.
	actual.CreatedAt = expected.CreatedAt
	a, _ := json.Marshal(expected)
	b, _ := json.Marshal(actual)
	var left, right any
	json.Unmarshal(a, &left)
	json.Unmarshal(b, &right)
	if !reflect.DeepEqual(left, right) {
		t.Fatal("CLI/core numerical or scope mismatch")
	}
	args[1] = "report-html"
	report, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, report)
	}
	if !strings.Contains(string(report), "Drilldown scope") || !strings.Contains(string(report), txid) {
		t.Fatal("HTML has no selected scope")
	}
	args[1] = "analyze"
	args[len(args)-1] = "missing"
	if _, err = exec.Command(binary, args...).CombinedOutput(); err == nil {
		t.Fatal("invalid scope exited successfully")
	}
}
