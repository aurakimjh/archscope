package ingestion_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
)

func TestC2SinkOwnsBytesAndEnforcesBudgetsAndSeal(t *testing.T) {
	c, _, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	d, _ := c.Resolve("tomcat", nil, "test")
	f, e := os.CreateTemp(t.TempDir(), "records-")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	budget := ingestion.DefaultBudgets()
	budget.Records = 1
	payload := json.RawMessage(`{"Timestamp":"2026-11-11T00:00:00Z","Severity":"INFO","Product":"fixture","Message":"test","RawLine":"synthetic"}`)
	sink := &ingestion.BoundedSink{File: f, Family: d.Descriptor.RecordFamily, Schema: d.Descriptor.RecordSchema, Catalog: c, Budget: budget}
	batch := ingestion.Batch{Family: d.Descriptor.RecordFamily, Records: []ingestion.Record{{Sequence: 1, Payload: payload}}}
	if e = sink.Write(context.Background(), batch); e != nil {
		t.Fatal(e)
	}
	payload[0] = 'x'
	batch.Records[0].Sequence = 2
	batch.Records[0].Payload = append([]byte{'{'}, payload[1:]...)
	if e = sink.Write(context.Background(), batch); e == nil {
		t.Fatal("record budget ignored")
	}
	if e = sink.Seal(); e != nil {
		t.Fatal(e)
	}
	if e = sink.Write(context.Background(), batch); e == nil {
		t.Fatal("write after parser completion accepted")
	}
	count := 0
	if e = (ingestion.RecordFile{Path: f.Name(), RecordBytes: budget.RecordBytes}).ForEach(context.Background(), func(r ingestion.Record) error {
		count++
		if r.Payload[0] != '{' {
			t.Fatal("caller changed retained bytes")
		}
		return nil
	}); e != nil || count != 1 {
		t.Fatal(count, e)
	}
}
