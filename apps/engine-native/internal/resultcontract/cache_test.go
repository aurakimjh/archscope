package resultcontract

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestR4ValidationCacheContentTamperAndOperationGates(t *testing.T) {
	var validations atomic.Int64
	registry, e := New(Descriptor{Type: "cache-test", Payload: Range{"1.0.0", "2.0.0"}, Envelope: Range{"1.0.0", "2.0.0"}, Extensions: map[string]Range{"needed": {"1.0.0", "2.0.0"}}, Required: map[string][]string{"compare": {"needed"}}, ValidatePayload: func(r models.AnalysisResult) error {
		validations.Add(1)
		if r.Summary["tampered"] != nil {
			return fmt.Errorf("tampered payload")
		}
		return nil
	}})
	if e != nil {
		t.Fatal(e)
	}
	result := models.New("cache-test", "cache")
	result.Metadata.SchemaVersion = "1.0.0"
	for _, op := range []string{"save", "display", "json", "csv", "html", "save"} {
		if v := registry.Check(result, op); !v.Allowed {
			t.Fatal(v)
		}
	}
	if validations.Load() != 1 {
		t.Fatal("payload repeatedly recomputed", validations.Load())
	}
	if v := registry.Check(result, "compare"); v.Allowed {
		t.Fatal("cache bypassed required extension")
	}
	if v := registry.Check(result, "not-an-operation"); v.Allowed {
		t.Fatal("cache activated unknown operation")
	}
	result.Summary["tampered"] = true
	for i := 0; i < 2; i++ {
		if v := registry.Check(result, "save"); v.Allowed {
			t.Fatal("changed bytes reused cached validation")
		}
	}
	if validations.Load() != 3 {
		t.Fatal("failures cached", validations.Load())
	}
	delete(result.Summary, "tampered")
	if v := registry.Check(result, "save"); !v.Allowed || validations.Load() != 3 {
		t.Fatal(v, validations.Load())
	}
	for i := 0; i < maxValidationCacheEntries+2; i++ {
		result.Summary["id"] = i
		if e = registry.Validate(result); e != nil {
			t.Fatal(e)
		}
	}
	if len(registry.validated) > maxValidationCacheEntries {
		t.Fatal("unbounded validation cache")
	}
}
