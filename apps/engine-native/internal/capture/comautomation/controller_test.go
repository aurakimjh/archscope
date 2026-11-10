package comautomation

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"testing"
)

func TestCOMControlPreservesScopeIdentityAndOwnSession(t *testing.T) {
	current := capture.Session{}
	calls := []automation.Call{}
	controller := New(func(ctx context.Context, call automation.Call) (any, error) {
		calls = append(calls, call)
		switch call.Command {
		case "status":
			return current, nil
		case "processes":
			return []models.ProcessInstance{{Key: models.ProcessKey{PID: 42, StartTime: "new"}, Name: "fixture.exe"}}, nil
		case "start":
			var cfg capture.Config
			json.Unmarshal(call.Params, &cfg)
			if cfg.ProcessScope != "selected" || cfg.Backend != "explicit_proxy" || len(cfg.AllowedProcesses) != 1 || cfg.AllowedProcesses[0].StartTime != "new" {
				t.Fatal(cfg)
			}
			current = capture.Session{ID: "cap-own", State: capture.StateRunning}
			return current, nil
		}
		return nil, nil
	})
	invoke := func(name string, args []any, put bool) any {
		t.Helper()
		value, err := controller.Invoke(context.Background(), name, args, put)
		if err != nil {
			t.Fatal(name, err)
		}
		return value
	}
	invoke("Backend", []any{"explicit_proxy"}, true)
	invoke("AttachProcessByName", []any{"FIXTURE.EXE"}, false)
	invoke("Start", nil, false)
	if _, err := controller.Invoke(context.Background(), "AttachAllSessions", nil, false); err == nil {
		t.Fatal("running attach silently deferred")
	}
	current = capture.Session{ID: "cap-other", State: capture.StateRunning}
	invoke("Stop", nil, false)
	var request struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(calls[len(calls)-1].Params, &request)
	if request.SessionID != "cap-own" {
		t.Fatal("stale controller targeted another session")
	}
}
func TestCOMLogSnapshotCaseAndEnumeration(t *testing.T) {
	reads := 0
	controller := New(func(ctx context.Context, call automation.Call) (any, error) {
		switch call.Command {
		case "status":
			return capture.Session{ID: "cap-fixture", State: capture.StateFinalized}, nil
		case "stats":
			return capture.Stats{SnapshotVersion: 2}, nil
		case "legacy-log":
			reads++
			return []models.CaptureTransaction{{URL: "https://example.test/fixture", Method: "GET", StatusCode: 200, State: models.TxComplete, Process: &models.ProcessInstance{Key: models.ProcessKey{PID: 42, StartTime: "fixture"}}}}, nil
		}
		return nil, nil
	})
	ctx := context.Background()
	value, err := controller.Invoke(ctx, "Log", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	log := value.(Object)
	count, err := log.Invoke(ctx, "count", nil, false)
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
	item, err := log.Invoke(ctx, "Item", []any{int64(0)}, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := item.(Object).Invoke(ctx, "url", nil, false)
	if err != nil || raw != "https://example.test/fixture" {
		t.Fatal(raw, err)
	}
	enumeration, err := log.Invoke(ctx, "_NewEnum", nil, false)
	if err != nil || len(enumeration.(Enumeration).Items) != 1 {
		t.Fatal(enumeration, err)
	}
	controller.Invoke(ctx, "Log", nil, false)
	if reads != 1 {
		t.Fatal("stable log was reloaded for every property")
	}
	if _, err = log.Invoke(ctx, "Item", []any{-1}, false); err == nil {
		t.Fatal("negative index accepted")
	}
	if _, err = controller.Invoke(ctx, "MaxLogSize", []any{-1}, true); err == nil {
		t.Fatal("negative retention accepted")
	}
}
func TestCOMLoadedClearDoesNotMutateRunningCapture(t *testing.T) {
	clears := 0
	controller := New(func(ctx context.Context, call automation.Call) (any, error) {
		if call.Command == "clear" {
			clears++
		}
		return nil, nil
	})
	ctx := context.Background()
	if _, err := controller.Invoke(ctx, "LoadLog", []any{"fixture.httpsession"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Invoke(ctx, "Clear", nil, false); err != nil || clears != 0 {
		t.Fatal("loaded clear touched active capture")
	}
	value, err := controller.Invoke(ctx, "Log", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	count, _ := value.(Object).Invoke(ctx, "Count", nil, false)
	if count != 0 {
		t.Fatal("loaded document not cleared")
	}
}
func TestCOMCacheCommandsUsePreviewedPlanOnly(t *testing.T) {
	var commands []string
	controller := New(func(ctx context.Context, call automation.Call) (any, error) {
		commands = append(commands, call.Command)
		switch call.Command {
		case "cache-plan":
			var request struct{ Kind, Scope string }
			json.Unmarshal(call.Params, &request)
			if request.Kind != "cookies" || request.Scope != "all" {
				t.Fatal(request)
			}
			return map[string]string{"id": "owned-plan"}, nil
		case "cache-delete":
			var request struct {
				PlanID string `json:"planId"`
			}
			json.Unmarshal(call.Params, &request)
			if request.PlanID != "owned-plan" {
				t.Fatal("wrong cleanup plan")
			}
		}
		return nil, nil
	})
	if _, err := controller.Invoke(context.Background(), "DeleteAllCookies", nil, false); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0] != "cache-plan" || commands[1] != "cache-delete" {
		t.Fatal(commands)
	}
}

func TestReferenceDispatchIDsEnumsAndTimeUnits(t *testing.T) {
	ids := dispatchIDs(New(nil))
	if ids["start"] != 210 || ids["visible"] != 301 || ids["attachprocessbyid"] != 304 {
		t.Fatal("controller dispatch IDs differ from reference declarations")
	}
	log := Typed{"ILog", Collection{}}
	if dispatchIDs(log)["count"] != 204 {
		t.Fatal("ILog Count uses a different ID from INameValues")
	}
	for number, format := range map[int]string{0: "xml", 1: "xml_compact", 2: "urls", 3: "xls", 4: "html", 5: "text", 6: "native", 7: "responses"} {
		actual, err := namedLogType(number)
		if err != nil || actual != format {
			t.Fatal(number, actual, err)
		}
	}
	row := entryObject(models.CaptureTransaction{StartedAt: "1970-01-01T00:00:00Z", TotalMS: 125})
	value, err := row.Invoke(context.Background(), "TimeStart", nil, false)
	if err != nil || value != comDate(25569) {
		t.Fatal("OLE DATE conversion", value, err)
	}
	value, err = row.Invoke(context.Background(), "TimeDuration", nil, false)
	if err != nil || value != int32(125) {
		t.Fatal("duration must stay integer milliseconds", value, err)
	}
	if _, err := entryObject(models.CaptureTransaction{}).Invoke(context.Background(), "TimeStart", nil, false); err == nil {
		t.Fatal("unknown date became a fabricated timestamp")
	}
}

func TestCOMSaveAndRecordedCleanupUseOwnedSource(t *testing.T) {
	current := capture.Session{ID: "cap-own", State: capture.StateFinalized}
	var saved string
	c := New(func(ctx context.Context, call automation.Call) (any, error) {
		switch call.Command {
		case "status":
			return current, nil
		case "start":
			return capture.Session{ID: "cap-own", State: capture.StateRunning}, nil
		case "save":
			var request struct {
				Source struct {
					SessionID string `json:"sessionId"`
				}
			}
			json.Unmarshal(call.Params, &request)
			saved = request.Source.SessionID
		}
		return nil, nil
	})
	if _, err := c.Invoke(context.Background(), "Start", nil, false); err != nil {
		t.Fatal(err)
	}
	current.ID = "cap-other"
	if _, err := c.Invoke(context.Background(), "SaveLog", []any{"fixture.xml", 0}, false); err != nil {
		t.Fatal(err)
	}
	if saved != "cap-own" {
		t.Fatal("save retargeted another capture")
	}
}

func TestCOMBodyCaptureIsAnExplicitRequestThroughExistingGate(t *testing.T) {
	requested := false
	c := New(func(ctx context.Context, call automation.Call) (any, error) {
		if call.Command == "start" {
			var cfg capture.Config
			json.Unmarshal(call.Params, &cfg)
			requested = cfg.CaptureBodies
			return capture.Session{ID: "cap-own"}, nil
		}
		return nil, nil
	})
	if _, err := c.Invoke(context.Background(), "CaptureBodies", []any{true}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Invoke(context.Background(), "Start", nil, false); err != nil || !requested {
		t.Fatal("body request was lost", err)
	}
	if _, err := c.Invoke(context.Background(), "CaptureBodies", []any{"true"}, true); err == nil {
		t.Fatal("ambiguous body request accepted")
	}
}

func TestDualDeclarationsPreserveInheritanceAndReturnShapes(t *testing.T) {
	base, child := interfaceMethods("IHTTPAnalyzer"), interfaceMethods("IHTTPAnalyzerStandAlone")
	if len(base) != 20 || len(child) != 40 {
		t.Fatal(len(base), len(child))
	}
	for i, m := range base {
		if child[i].Name != m.Name || child[i].Kind != m.Kind || child[i].Returns != m.Returns {
			t.Fatal("base prefix changed", i)
		}
	}
	if base[6].Name != "SaveLog" || len(base[6].Params) != 2 || base[6].Params[1].Type != "LogType" || base[6].Returns != "void" {
		t.Fatal("wrong HRESULT/BSTR/enum signature")
	}
	for name, contract := range referenceContracts.Interfaces {
		for _, m := range contract.Methods {
			if len(m.Params) > 2 {
				t.Fatal("unsupported ABI arity", name, m.Name)
			}
			if _, ok := contract.IDs[m.Name]; !ok {
				t.Fatal("missing member ID", name, m.Name)
			}
		}
	}
}

func TestSingleURLCleanupUsesPlanAndReportsActualDeletion(t *testing.T) {
	var commands []string
	c := New(func(_ context.Context, call automation.Call) (any, error) {
		commands = append(commands, call.Command)
		switch call.Command {
		case "cache-plan-url":
			var r struct{ Kind, URL string }
			json.Unmarshal(call.Params, &r)
			if r.Kind != "cookies" || r.URL != "https://example.test/path" {
				t.Fatal(r)
			}
			return map[string]string{"id": "plan"}, nil
		case "cache-delete":
			return map[string]int{"deleted": 2}, nil
		}
		return nil, nil
	})
	v, e := c.Invoke(context.Background(), "DeletePersistentCookie", []any{"https://example.test/path"}, false)
	if e != nil || v != 2 || len(commands) != 2 || commands[0] != "cache-plan-url" || commands[1] != "cache-delete" {
		t.Fatal(v, e, commands)
	}
	if _, e = c.Invoke(context.Background(), "DeleteSessionCookies", nil, false); e == nil {
		t.Fatal("persistent cleanup claimed to reset a foreign process session")
	}
}
