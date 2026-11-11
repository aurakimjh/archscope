package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	dir, e := os.MkdirTemp("", "t632-sqlite-")
	if e != nil {
		panic(e)
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "index.db")
	start := time.Now()
	db, e := sql.Open("sqlite", p)
	if e != nil {
		panic(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec("CREATE TABLE records (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); e != nil {
		panic(e)
	}
	tx, _ := db.Begin()
	stmt, _ := tx.Prepare("INSERT INTO records(id,value) VALUES (?,?)")
	for i := 0; i < 100000; i++ {
		if _, e = stmt.Exec(i, fmt.Sprint(i)); e != nil {
			panic(e)
		}
	}
	stmt.Close()
	if e = tx.Commit(); e != nil {
		panic(e)
	}
	build := time.Since(start)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start = time.Now()
	for i := 0; i < 1000; i++ {
		var value string
		if e = db.QueryRow("SELECT value FROM records WHERE id=?", i*97).Scan(&value); e != nil {
			panic(e)
		}
	}
	query := time.Since(start)
	runtime.ReadMemStats(&after)
	stat, _ := os.Stat(p)
	exe, _ := os.Executable()
	binary, _ := os.Stat(exe)
	json.NewEncoder(os.Stdout).Encode(map[string]any{"driver": "modernc.org/sqlite@v1.53.0", "goos": runtime.GOOS, "goarch": runtime.GOARCH, "cgo": false, "rows": 100000, "build_ms": float64(build.Microseconds()) / 1000, "lookup_1000_ms": float64(query.Microseconds()) / 1000, "lookup_allocated_bytes": after.TotalAlloc - before.TotalAlloc, "database_bytes": stat.Size(), "binary_bytes": binary.Size(), "scope": "synthetic primary-key lookup prototype; no production integration or analytical-query benchmark"})
}
