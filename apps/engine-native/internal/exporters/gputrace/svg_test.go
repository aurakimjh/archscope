package gputrace_test

import (
	"bytes"
	"context"
	"encoding/xml"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	x "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/gputrace"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func svgFixture(t *testing.T) g.Set {
	t.Helper()
	raw, err := os.ReadFile("../../catalog/contracts/gpu-execution-set.json")
	if err != nil {
		t.Fatal(err)
	}
	set, err := g.Decode(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestSVGWellFormedEscapingAndSourceGuards(t *testing.T) {
	ctx := context.Background()
	set := svgFixture(t)
	attack := `<script onload="x&y">alert('unsafe')</script>`
	set.Events[0].Name = attack
	set.Events[0].ID = g.EventID(set.Events[0])
	source := ""
	for _, track := range set.Tracks {
		if track.ID == set.Events[0].Track {
			source = track.Source
		}
	}
	projection, err := a.Project(ctx, set)
	if err != nil {
		t.Fatal(err)
	}
	query := a.Query{Kind: "events", Source: source, Limit: 100}
	out, err := x.SVG(ctx, set, projection, strings.Repeat("a", 64), query)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("<script")) || !bytes.Contains(out, []byte("&lt;script")) {
		t.Fatal("unescaped markup")
	}
	decoder := xml.NewDecoder(bytes.NewReader(out))
	found := false
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("invalid SVG XML", err)
		}
		switch v := token.(type) {
		case xml.StartElement:
			if !found {
				if v.Name.Local != "svg" || v.Name.Space != "http://www.w3.org/2000/svg" {
					t.Fatal(v.Name)
				}
				found = true
			}
			if v.Name.Local == "script" {
				t.Fatal("script node")
			}
			for _, attr := range v.Attr {
				if strings.HasPrefix(strings.ToLower(attr.Name.Local), "on") {
					t.Fatal("event-handler attribute")
				}
			}
		case xml.CharData:
			text.Write(v)
		}
	}
	if !found || !strings.Contains(text.String(), attack) {
		t.Fatal("escaped title no longer preserves exact text")
	}
	for _, q := range []a.Query{{Kind: "events", Limit: 100}, {Kind: "busy", Source: source, Limit: 100}, {Kind: "events", Source: source, Limit: 100, Cursor: "external"}} {
		if _, err = x.SVG(ctx, set, projection, strings.Repeat("a", 64), q); err == nil {
			t.Fatal("unsafe SVG query", q)
		}
	}
}

func TestSVGEventBudgetRefusesInsteadOfTruncating(t *testing.T) {
	ctx := context.Background()
	set := svgFixture(t)
	tracks := map[string]g.Track{}
	for _, track := range set.Tracks {
		tracks[track.ID] = track
	}
	var kernel g.Event
	for _, event := range set.Events {
		if event.Kind == "gpu_kernel" {
			kernel = event
			break
		}
	}
	source := tracks[kernel.Track].Source
	if source == "" {
		t.Fatal("fixture lacks GPU kernel")
	}
	set.Events = []g.Event{}
	for i := 0; i < 2001; i++ {
		event := kernel
		event.From = strconv.Itoa(i * 2)
		event.To = strconv.Itoa(i*2 + 1)
		event.Position = "synthetic/" + strconv.Itoa(i)
		event.Correlation = ""
		event.ID = g.EventID(event)
		set.Events = append(set.Events, event)
	}
	projection, err := a.Project(ctx, set)
	if err != nil {
		t.Fatal(err)
	}
	_, err = x.SVG(ctx, set, projection, strings.Repeat("a", 64), a.Query{Kind: "events", Source: source, Limit: 100})
	if err == nil || !strings.Contains(err.Error(), "SVG_EVENT_BUDGET") {
		t.Fatal(err)
	}
}
