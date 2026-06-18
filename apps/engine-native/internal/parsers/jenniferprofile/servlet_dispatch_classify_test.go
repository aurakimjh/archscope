package jenniferprofile

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestServletDispatchPatternsWithDefaults(t *testing.T) {
	got := ServletDispatchPatternsWithDefaults(nil)
	for _, want := range []string{
		"jakarta.servlet.http.httpservlet.service",
		"javax.servlet.http.httpservlet.service",
	} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("default servlet dispatch pattern missing: %s", want)
		}
	}
}

func TestClassifyServletDispatchDefault(t *testing.T) {
	p := &models.JenniferTransactionProfile{
		Body: models.JenniferProfileBody{Events: []models.JenniferProfileEvent{
			{RawMessage: "jakarta.servlet.http.HttpServlet.service(HttpServletRequest, HttpServletResponse)"},
		}},
	}
	classifyEventsWithOptions(p, Options{})
	if got := p.Body.Events[0].EventType; got != models.JenniferEventServletDispatch {
		t.Fatalf("default: got %s, want SERVLET_DISPATCH_METHOD", got)
	}
}

func TestClassifyServletDispatchCustomPattern(t *testing.T) {
	p := &models.JenniferTransactionProfile{
		Body: models.JenniferProfileBody{Events: []models.JenniferProfileEvent{
			{RawMessage: "com.app.MyDispatcher.entry(args)"},
		}},
	}
	classifyEventsWithOptions(p, Options{ServletDispatchPatterns: []string{"mydispatcher.entry"}})
	if got := p.Body.Events[0].EventType; got != models.JenniferEventServletDispatch {
		t.Fatalf("custom: got %s, want SERVLET_DISPATCH_METHOD", got)
	}
}
