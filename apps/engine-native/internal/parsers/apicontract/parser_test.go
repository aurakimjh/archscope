package apicontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOpenAPIAndAsyncAPIJSON(t *testing.T) {
	dir := t.TempDir()
	openapi := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(openapi, []byte(`{
  "openapi": "3.0.3",
  "info": {"title": "Orders API", "version": "1.0.0"},
  "paths": {
    "/orders/{id}": {"get": {"operationId": "getOrder", "tags": ["orders"]}},
    "/orders": {"post": {"operationId": "createOrder"}}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	contract, diags, err := ParseOpenAPIFile(openapi, Options{})
	if err != nil {
		t.Fatalf("parse openapi: %v", err)
	}
	if contract.Title != "Orders API" || len(contract.Operations) != 2 || diags.ParsedRecords != 2 {
		t.Fatalf("openapi contract = %+v diagnostics=%+v", contract, diags)
	}

	asyncapi := filepath.Join(dir, "asyncapi.json")
	if err := os.WriteFile(asyncapi, []byte(`{
  "asyncapi": "2.6.0",
  "info": {"title": "Orders Events", "version": "1.0.0"},
  "channels": {
    "orders.created": {"publish": {"operationId": "publishOrderCreated", "message": {"name": "OrderCreated"}}}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	events, diags, err := ParseAsyncAPIFile(asyncapi, Options{})
	if err != nil {
		t.Fatalf("parse asyncapi: %v", err)
	}
	if events.Title != "Orders Events" || len(events.Channels) != 1 || events.Channels[0].MessageNames[0] != "OrderCreated" || diags.ParsedRecords != 1 {
		t.Fatalf("asyncapi contract = %+v diagnostics=%+v", events, diags)
	}
}
