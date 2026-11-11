package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

func main() {
	dir, e := os.MkdirTemp("", "t632-index-")
	if e != nil {
		panic(e)
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "records.jsonl")
	start := time.Now()
	f, _ := os.Create(p)
	out := bufio.NewWriter(f)
	for i := 0; i < 100000; i++ {
		fmt.Fprintf(out, "{\"id\":%d,\"value\":\"%d\"}\n", i, i)
	}
	out.Flush()
	f.Close()
	file, _ := os.Open(p)
	scanner := bufio.NewScanner(file)
	index := map[int]string{}
	for scanner.Scan() {
		var row struct {
			ID    int
			Value string
		}
		if e = json.Unmarshal(scanner.Bytes(), &row); e != nil {
			panic(e)
		}
		index[row.ID] = row.Value
	}
	file.Close()
	build := time.Since(start)
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	start = time.Now()
	for i := 0; i < 1000; i++ {
		if index[i*97] != strconv.Itoa(i*97) {
			panic("mismatch")
		}
	}
	query := time.Since(start)
	runtime.ReadMemStats(&b)
	stat, _ := os.Stat(p)
	exe, _ := os.Executable()
	binary, _ := os.Stat(exe)
	json.NewEncoder(os.Stdout).Encode(map[string]any{"driver": "stdlib JSONL plus in-memory primary-key index", "goos": runtime.GOOS, "goarch": runtime.GOARCH, "cgo": false, "rows": 100000, "build_ms": float64(build.Microseconds()) / 1000, "lookup_1000_ms": float64(query.Microseconds()) / 1000, "lookup_allocated_bytes": b.TotalAlloc - a.TotalAlloc, "database_bytes": stat.Size(), "binary_bytes": binary.Size(), "scope": "synthetic primary-key lookup; index not persisted; no production analytical-query claim"})
}
