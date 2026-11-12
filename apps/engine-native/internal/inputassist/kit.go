package inputassist

import (
	"archive/zip"
	"bytes"
	"embed"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"sort"
	"time"
)

// Assets contains inert recipes and offline manuals, never an executable pack.
//
//go:embed assets/*
var Assets embed.FS

func Kit() ([]byte, error) {
	files := map[string][]byte{}
	entries, e := Assets.ReadDir("assets")
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		b, e := Assets.ReadFile("assets/" + entry.Name())
		if e != nil {
			return nil, e
		}
		files[entry.Name()] = b
	}
	schema, e := cases.Contracts.ReadFile("contracts/capture-context.schema.json")
	if e != nil {
		return nil, e
	}
	files["capture-context.schema.json"] = schema
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetModTime(time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
		header.SetMode(0600)
		w, e := z.CreateHeader(header)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(files[name]); e != nil {
			return nil, e
		}
	}
	if e = z.Close(); e != nil {
		return nil, e
	}
	return buffer.Bytes(), nil
}
