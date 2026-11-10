package comautomation

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func syntheticFilter(column int32, op byte, value string) []byte {
	var criteria, container bytes.Buffer
	long := func(b *bytes.Buffer, n int32) { binary.Write(b, binary.LittleEndian, n) }
	short := func(b *bytes.Buffer, s string) { b.WriteByte(byte(len(s))); b.WriteString(s) }
	long(&criteria, -1)
	criteria.WriteByte(0)
	long(&criteria, 1)
	criteria.Write([]byte{0, op})
	long(&criteria, int32(len(value)))
	criteria.WriteString(value)
	long(&criteria, column)
	criteria.WriteByte(6)
	short(&criteria, value)
	container.WriteString("TPF0")
	short(&container, "TFilterWrapper")
	short(&container, "")
	short(&container, "Filter.Active")
	container.WriteByte(9)
	short(&container, "Filter.Criteria")
	container.WriteByte(10)
	long(&container, int32(criteria.Len()))
	container.Write(criteria.Bytes())
	container.Write([]byte{0, 0})
	return container.Bytes()
}
func TestFilterMatchesActualValueAndRejectsUnknownData(t *testing.T) {
	for _, tc := range []struct {
		col   int32
		op    byte
		value string
		row   models.CaptureTransaction
		want  bool
	}{
		{3, 0, "POST", models.CaptureTransaction{Method: "POST"}, true},
		{3, 0, "POST", models.CaptureTransaction{Method: "GET"}, false},
		{4, 6, "2__", models.CaptureTransaction{StatusCode: 204}, true},
		{4, 6, "2__", models.CaptureTransaction{StatusCode: 404}, false},
		{6, 7, "image/%", models.CaptureTransaction{Response: models.HTTPMessage{ContentType: "text/html; charset=UTF-8"}}, true},
	} {
		f, e := decodeLogFilter(syntheticFilter(tc.col, tc.op, tc.value))
		if e != nil || f.matches(tc.row) != tc.want {
			t.Fatal(tc, e)
		}
	}
	good := syntheticFilter(3, 0, "POST")
	for i := range good {
		if _, e := decodeLogFilter(good[:i]); e == nil {
			t.Fatalf("accepted truncation at %d", i)
		}
	}
	for _, b := range [][]byte{append(append([]byte{}, good...), 1), syntheticFilter(99, 0, "GET"), syntheticFilter(3, 99, "GET"), syntheticFilter(4, 1, "(Cache)")} {
		if _, e := decodeLogFilter(b); e == nil {
			t.Fatal("unsupported filter accepted")
		}
	}
}
func TestControllerFilterAndLatestRowLimitApplyToLogAndSave(t *testing.T) {
	rows := []models.CaptureTransaction{{ID: "one", Method: "POST"}, {ID: "two", Method: "GET"}, {ID: "three", Method: "POST"}}
	var saved struct {
		Scope string
		IDs   []string
	}
	c := New(func(_ context.Context, call automation.Call) (any, error) {
		switch call.Command {
		case "status":
			return capture.Session{ID: "own", State: capture.StateFinalized}, nil
		case "stats":
			return capture.Stats{SnapshotVersion: 3}, nil
		case "legacy-log":
			return rows, nil
		case "save":
			json.Unmarshal(call.Params, &saved)
		}
		return nil, nil
	})
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.flt")
	os.WriteFile(path, syntheticFilter(3, 0, "POST"), 0600)
	if _, e := c.Invoke(ctx, "SetFilter", []any{path}, false); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Invoke(ctx, "MaxLogSize", []any{2}, true); e != nil {
		t.Fatal(e)
	}
	v, e := c.Invoke(ctx, "Log", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	log := v.(Object)
	if n, _ := log.Invoke(ctx, "Count", nil, false); n != 2 {
		t.Fatal(n)
	}
	if n, _ := log.Invoke(ctx, "FilteredCount", nil, false); n != 1 {
		t.Fatal(n)
	}
	v, _ = log.Invoke(ctx, "Item", []any{0}, false)
	if filtered, _ := v.(Object).Invoke(ctx, "IsFiltered", nil, false); filtered != true {
		t.Fatal(filtered)
	}
	if _, e = c.Invoke(ctx, "SaveLog", []any{"test.xml"}, false); e != nil {
		t.Fatal(e)
	}
	if saved.Scope != "filtered" || len(saved.IDs) != 2 || saved.IDs[0] != "two" || saved.IDs[1] != "three" {
		t.Fatal(saved)
	}
	if _, e = c.Invoke(ctx, "MaxLogSize", []any{0}, true); e != nil {
		t.Fatal(e)
	}
	v, e = c.Invoke(ctx, "Log", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	if n, _ := v.(Object).Invoke(ctx, "Count", nil, false); n != 2 {
		t.Fatal("evicted row reappeared", n)
	}
	if _, e = c.Invoke(ctx, "ClearFilter", nil, false); e != nil {
		t.Fatal(e)
	}
	if v, _ = c.Invoke(ctx, "IsFiltered", nil, false); v != false {
		t.Fatal(v)
	}
}

func TestHiddenCaptureOnlyResumesItsOwnAutomaticPause(t *testing.T) {
	current := capture.Session{ID: "own", State: capture.StateRunning}
	paused, resumed := 0, 0
	c := New(func(_ context.Context, call automation.Call) (any, error) {
		switch call.Command {
		case "start":
			return current, nil
		case "status":
			return current, nil
		case "visible":
			var r struct{ Value bool }
			json.Unmarshal(call.Params, &r)
			return r.Value, nil
		case "pause":
			paused++
			current.Paused = true
		case "resume":
			resumed++
			current.Paused = false
		}
		return nil, nil
	})
	ctx := context.Background()
	for _, call := range []struct {
		n   string
		a   []any
		put bool
	}{{"Start", nil, false}, {"StillCaptureDataWhenHide", []any{false}, true}, {"Visible", []any{false}, true}, {"Visible", []any{true}, true}} {
		if _, e := c.Invoke(ctx, call.n, call.a, call.put); e != nil {
			t.Fatal(e)
		}
	}
	if paused != 1 || resumed != 1 {
		t.Fatal(paused, resumed)
	}
	current.Paused = true
	c.Invoke(ctx, "Visible", []any{false}, true)
	c.Invoke(ctx, "Visible", []any{true}, true)
	if resumed != 1 {
		t.Fatal("manual pause was resumed")
	}
	current.ID = "replacement"
	current.Paused = false
	c.Invoke(ctx, "Visible", []any{false}, true)
	if paused != 1 {
		t.Fatal("replacement session paused")
	}
}
func TestReferencePresetFiltersWhenProvided(t *testing.T) {
	root := os.Getenv("ARCHSCOPE_REFERENCE_FILTER_DIR")
	if root == "" {
		t.Skip("optional read-only original preset validation")
	}
	paths, e := filepath.Glob(filepath.Join(root, "*.flt"))
	if e != nil || len(paths) != 17 {
		t.Fatal(len(paths), e)
	}
	for _, path := range paths {
		_, e := readLogFilter(path)
		if filepath.Base(path) == "00.Result is not cache.flt" {
			if e == nil {
				t.Fatal("missing cache metadata claimed")
			}
		} else if e != nil {
			t.Fatal(filepath.Base(path), e)
		}
	}
}
