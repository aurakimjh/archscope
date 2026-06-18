package jenniferprofile

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
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
