package comautomation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func normalized(where string) []byte {
	return []byte(`{"format":"archscope.http-filter","version":1,"active":true,"where":` + where + `}`)
}
func TestNormalizedGroupsAndUnknownAreNotNegatedIntoMatches(t *testing.T) {
	where := `{"all":[{"any":[{"field":"status","op":"gte","value":400},{"field":"method","op":"eq","value":"POST"}]},{"not":{"field":"processName","op":"contains","value":"test","ignoreCase":true}}]}`
	f, e := decodeLogFilter(normalized(where))
	if e != nil {
		t.Fatal(e)
	}
	row := models.CaptureTransaction{Method: "POST", StatusCode: 200}
	if f.matches(row) {
		t.Fatal("unknown process became a match under NOT")
	}
	row.Process = &models.ProcessInstance{Name: "Worker"}
	if !f.matches(row) {
		t.Fatal("nested positive predicate failed")
	}
	row.Process.Name = "TEST runner"
	if f.matches(row) {
		t.Fatal("NOT was ignored")
	}
	for _, op := range []string{"eq", "neq", "lt", "lte", "gt", "gte"} {
		f, e = decodeLogFilter(normalized(fmt.Sprintf(`{"field":"status","op":%q,"value":400}`, op)))
		if e != nil {
			t.Fatal(e)
		}
		if f.matches(models.CaptureTransaction{}) {
			t.Fatal("unknown status matched", op)
		}
	}
}
func TestNormalizedOperatorsAndUnicode(t *testing.T) {
	for _, tc := range []struct {
		field, op, value string
		row              models.CaptureTransaction
		want             bool
	}{
		{"url", "startsWith", `"https://"`, models.CaptureTransaction{URL: "https://example.test"}, true},
		{"path", "endsWith", `"분석"`, models.CaptureTransaction{Path: "/로그/분석"}, true},
		{"host", "contains", `"AMPLE"`, models.CaptureTransaction{Host: "example.test"}, true},
		{"method", "neq", `"POST"`, models.CaptureTransaction{Method: "GET"}, true},
		{"mime", "like", `"image/%"`, models.CaptureTransaction{Response: models.HTTPMessage{ContentType: "IMAGE/PNG; charset=UTF-8"}}, true},
		{"path", "regex", `"^/한글/[0-9]+$"`, models.CaptureTransaction{Path: "/한글/123"}, true},
		{"status", "lt", `400`, models.CaptureTransaction{StatusCode: 399}, true},
		{"status", "lte", `400`, models.CaptureTransaction{StatusCode: 400}, true},
		{"responseSize", "gt", `12`, models.CaptureTransaction{Response: models.HTTPMessage{BodySize: 12}}, false},
	} {
		ignore := `true`
		if tc.field == "status" || tc.field == "responseSize" {
			ignore = "false"
		}
		f, e := decodeLogFilter(normalized(fmt.Sprintf(`{"field":%q,"op":%q,"value":%s,"ignoreCase":%s}`, tc.field, tc.op, tc.value, ignore)))
		if e != nil || f.matches(tc.row) != tc.want {
			t.Fatal(tc, e)
		}
	}
}
func TestNormalizedRejectsAmbiguousUnboundedAndUnsupportedConditions(t *testing.T) {
	valid := `{"field":"method","op":"eq","value":"GET"}`
	cases := []string{`{}`, `{"all":[]}`, `{"any":[],"not":` + valid + `}`, `{"not":` + valid + `,"field":"method"}`, `{"field":"cacheOrigin","op":"eq","value":"cache"}`, `{"field":"status","op":"eq","value":1.5}`, `{"field":"status","op":"gt","value":"400"}`, `{"field":"method","op":"eq","value":null}`, `{"field":"status","op":"eq","value":-1}`, `{"field":"method","field":"url","op":"eq","value":"GET"}`, `{"field":"url","op":"regex","value":"(?=bad)"}`, `{"field":"url","op":"eq","value":"x","unknown":true}`, `{"field":"url","op":"eq","value":"` + strings.Repeat("x", 4097) + `"}`}
	deep := valid
	for i := 0; i < 16; i++ {
		deep = `{"not":` + deep + `}`
	}
	cases = append(cases, deep, `{"all":[`+strings.TrimSuffix(strings.Repeat(valid+",", 256), ",")+`]}`)
	for _, where := range cases {
		if _, e := decodeLogFilter(normalized(where)); e == nil {
			t.Fatal("invalid condition accepted", where[:min(100, len(where))])
		}
	}
	b := append(normalized(valid), []byte(`{}`)...)
	if _, e := decodeLogFilter(b); e == nil {
		t.Fatal("trailing JSON accepted")
	}
}
