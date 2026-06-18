package jenniferprofile

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	jenniferparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jenniferprofile"
)

// service() wraps controller() which wraps SQL. Self-time of the servlet
// dispatch frame must exclude the controller (and its SQL), so only the
// 30ms bracketing the controller is charged to ServletDispatch.
func servletDispatchProfile() models.JenniferTransactionProfile {
	return profileOf("svc",
		ev(models.JenniferEventServletDispatch, "jakarta.servlet.http.HttpServlet.service()", 0, 100),
		ev(models.JenniferEventMethod, "com.app.Controller.handle()", 20, 70),
		ev(models.JenniferEventSQLExecute, "select * from t", 40, 40),
	)
}

func TestServletDispatchSelfTime(t *testing.T) {
	p := servletDispatchProfile()
	m := AggregateBody(&p)
	if m.ServletDispatchCount != 1 {
		t.Fatalf("ServletDispatchCount = %d, want 1", m.ServletDispatchCount)
	}
	if m.ServletDispatchCumMs != 30 {
		t.Fatalf("ServletDispatchCumMs = %d, want 30 (100 - controller 70)", m.ServletDispatchCumMs)
	}
	if m.SQLExecuteCumMs != 40 {
		t.Fatalf("SQLExecuteCumMs = %d, want 40 (counted independently)", m.SQLExecuteCumMs)
	}
}

func TestServletDispatchExcludedFromHotspots(t *testing.T) {
	p := servletDispatchProfile()
	hs := MethodHotspots(p, 0)
	if _, ok := findHotspot(hs, "jakarta.servlet.http.HttpServlet.service()"); ok {
		t.Fatalf("servlet dispatch frame must not be ranked as a method hotspot")
	}
	if _, ok := findHotspot(hs, "com.app.Controller.handle()"); !ok {
		t.Fatalf("controller business method should still be ranked")
	}
}

func TestServletDispatchProjectedToResult(t *testing.T) {
	p := servletDispatchProfile()
	p.Header.GUID = "guid-svc"
	p.Header.ResponseTimeMs = ip(100)

	res := Build([]jenniferparser.FileResult{{
		SourceFile: "profile.txt",
		Profiles:   []models.JenniferTransactionProfile{p},
	}}, Options{})

	if got := res.Summary["servlet_dispatch_cum_ms"].(int); got != 30 {
		t.Fatalf("summary servlet_dispatch_cum_ms = %d, want 30", got)
	}
	if got := res.Summary["servlet_dispatch_count"].(int); got != 1 {
		t.Fatalf("summary servlet_dispatch_count = %d, want 1", got)
	}

	profileRows := res.Tables["profiles"].([]map[string]any)
	if len(profileRows) != 1 {
		t.Fatalf("profile rows = %d, want 1", len(profileRows))
	}
	bodyMetrics := profileRows[0]["body_metrics"].(map[string]any)
	if got := bodyMetrics["servlet_dispatch_cum_ms"].(int); got != 30 {
		t.Fatalf("profile servlet_dispatch_cum_ms = %d, want 30", got)
	}
	if got := bodyMetrics["servlet_dispatch_count"].(int); got != 1 {
		t.Fatalf("profile servlet_dispatch_count = %d, want 1", got)
	}

	groups := res.Series["guid_groups"].([]map[string]any)
	if len(groups) != 1 {
		t.Fatalf("guid groups = %d, want 1", len(groups))
	}
	metrics := groups[0]["metrics"].(map[string]any)
	breakdown := metrics["response_time_breakdown"].(map[string]any)
	if got := breakdown["servlet_dispatch_ms"].(int); got != 30 {
		t.Fatalf("breakdown servlet_dispatch_ms = %d, want 30", got)
	}
	if got := breakdown["method_time_ms"].(int); got != 30 {
		t.Fatalf("breakdown method_time_ms = %d, want 30", got)
	}
}
