package multithread

import (
	"encoding/json"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
)

// Exercise the actual parser-to-analyzer boundary: native Go parser metadata
// carries typed map slices, whereas imported JSON metadata carries []any.
func TestJstackJVMMetadataSurvivesNativeAndJSONBundles(t *testing.T) {
	bundles, err := javajstack.New().ParseAll("testdata/jvm-signals-jstack.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 1 {
		t.Fatalf("parsed bundles = %d", len(bundles))
	}
	body, err := json.Marshal(bundles)
	if err != nil {
		t.Fatal(err)
	}
	var imported []models.ThreadDumpBundle
	if err := json.Unmarshal(body, &imported); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		bundles []models.ThreadDumpBundle
	}{
		{"native parser", bundles}, {"JSON import", imported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Analyze(tc.bundles, Options{})
			if err != nil {
				t.Fatal(err)
			}
			for table, want := range map[string]int{
				"virtual_thread_carrier_pinning": 1, "native_method_threads": 1,
				"smr_unresolved_threads": 2, "class_histogram_top_classes": 2,
			} {
				rows, ok := result.Tables[table].([]map[string]any)
				if !ok || len(rows) != want {
					t.Errorf("%s = %#v, want %d rows", table, result.Tables[table], want)
				}
			}
			if got := countFindingsWithCode(result.Metadata.Findings, "SMR_UNRESOLVED_THREAD"); got != 2 {
				t.Errorf("SMR findings = %d, want tagged and untagged address exactly once each", got)
			}
			classes := result.Tables["class_histogram_top_classes"].([]map[string]any)
			if len(classes) != 2 {
				return
			}
			if classes[0]["class_name"] != "java.lang.String" || asInt(classes[0]["bytes"]) != 2400 || asInt(classes[1]["bytes"]) != 1600 {
				t.Errorf("histogram bytes/order lost: %#v", classes)
			}
		})
	}
}
