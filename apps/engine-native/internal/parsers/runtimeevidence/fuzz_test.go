package runtimeevidence_test

import (
	"context"
	"encoding/json"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/runtimeevidence"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"os"
	"testing"
	"time"
)

func FuzzRuntimeThreadText(f *testing.F) {
	encoded, err := os.ReadFile("../../catalog/contracts/runtime-evidence-thread-options.json")
	if err != nil {
		f.Fatal(err)
	}
	java, err := p.DecodeOptions(encoded)
	if err != nil {
		f.Fatal(err)
	}
	python := java
	python.ThreadFormat = "python_faulthandler"
	python.Producer = "CPython"
	python.Version = "3.14.4"
	process := *java.Process
	process.Runtime = "CPython"
	python.Process = &process
	python.Entities = nil
	options := []p.Options{java, python}
	for i, name := range []string{"jstack.txt", "faulthandler-3.14.4.txt"} {
		raw, err := os.ReadFile("../../catalog/contracts/runtime-evidence-" + name)
		if err != nil {
			f.Fatal(err)
		}
		if _, err := p.ParseBytes(context.Background(), raw, options[i]); err != nil {
			f.Fatal("valid seed options", err)
		}
		f.Add(uint8(i), raw)
	}
	f.Add(uint8(0), []byte("\xff\x00\n"))
	f.Add(uint8(1), []byte("Current thread 0x0:\n"))
	f.Fuzz(func(t *testing.T, kind uint8, raw []byte) {
		if len(raw) > 64<<10 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		set, err := p.ParseBytes(ctx, raw, options[int(kind)%len(options)])
		if err != nil {
			return
		}
		if err = set.Validate(context.Background()); err != nil {
			t.Fatal("parser published invalid model", err)
		}
		body, err := json.Marshal(set)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.Decode(context.Background(), body); err != nil {
			t.Fatal("canonical readback", err)
		}
	})
}
