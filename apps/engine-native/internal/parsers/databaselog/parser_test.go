package databaselog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePostgresAndMySQLAndRedis(t *testing.T) {
	dir := t.TempDir()
	pg := filepath.Join(dir, "pg.log")
	if err := os.WriteFile(pg, []byte(`2026-05-16 10:00:00 UTC LOG: duration: 123.456 ms statement: SELECT * FROM orders WHERE id = 1001`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err := ParseFile(pg, "postgres-text", Options{})
	if err != nil || len(records) != 1 || records[0].Fingerprint != "select * from orders where id = ?" {
		t.Fatalf("postgres records=%+v err=%v", records, err)
	}
	my := filepath.Join(dir, "mysql-slow.log")
	if err := os.WriteFile(my, []byte("# Time: 260516 10:00:00\n# Query_time: 1.200 Lock_time: 0.010 Rows_sent: 1 Rows_examined: 2000\nSELECT * FROM users WHERE id=42;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err = ParseFile(my, "mysql-slow", Options{})
	if err != nil || len(records) != 1 || records[0].DurationMS != 1200 || records[0].Rows != 2000 {
		t.Fatalf("mysql records=%+v err=%v", records, err)
	}
	redis := filepath.Join(dir, "redis.slowlog")
	if err := os.WriteFile(redis, []byte("1 1778916000 12345 GET user:1001\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err = ParseFile(redis, "redis-slowlog", Options{})
	if err != nil || len(records) != 1 || records[0].Engine != "redis" {
		t.Fatalf("redis records=%+v err=%v", records, err)
	}
}

func TestParseJSONDatabaseEvidence(t *testing.T) {
	dir := t.TempDir()
	mongo := filepath.Join(dir, "mongo.json")
	if err := os.WriteFile(mongo, []byte(`[{"op":"query","ns":"shop.orders","command":"find orders by customer 1001","millis":250,"nreturned":10}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err := ParseFile(mongo, "mongodb-json", Options{})
	if err != nil || len(records) != 1 || records[0].DurationMS != 250 {
		t.Fatalf("mongo records=%+v err=%v", records, err)
	}
	plan := filepath.Join(dir, "explain.json")
	if err := os.WriteFile(plan, []byte(`{"Query":"SELECT * FROM orders WHERE id=1001","Plan":{"Node Type":"Seq Scan"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err = ParseFile(plan, "postgres-explain-json", Options{})
	if err != nil || len(records) != 1 || !records[0].Plan {
		t.Fatalf("plan records=%+v err=%v", records, err)
	}
}
