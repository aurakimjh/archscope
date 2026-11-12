package runtimeevidence_test

import (
	"bytes"
	"context"
	"encoding/json"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/runtimeevidence"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"os"
	"path/filepath"
	"testing"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("../../catalog/contracts", "runtime-evidence-"+name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestPinnedProducerNormalization(t *testing.T) {
	for _, c := range []struct{ raw, options, expected string }{{"python-3.14.4.json", "python-options.json", "python-set.json"}, {"node-25.9.0.json", "node-options.json", "node-set.json"}, {"rust-1.94.1-backtrace.txt", "rust-options.json", "rust-set.json"}, {"jstack.txt", "thread-options.json", "thread-set.json"}, {"ps-l.csv", "cpu-options.json", "cpu-set.json"}, {"postgres-actual.json", "postgres-options.json", "plan-actual-set.json"}, {"postgres-estimated.json", "postgres-options.json", "plan-estimated-set.json"}} {
		t.Run(c.raw, func(t *testing.T) {
			o, e := p.DecodeOptions(read(t, c.options))
			if e != nil {
				t.Fatal(e)
			}
			s, e := p.ParseBytes(context.Background(), read(t, c.raw), o)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(s)
			var expected r.Set
			if e = json.Unmarshal(read(t, c.expected), &expected); e != nil {
				t.Fatal(e)
			}
			want, _ := json.Marshal(expected)
			if !bytes.Equal(b, want) {
				t.Fatal("normalization drift")
			}
			for _, secret := range []string{"SYNTHETIC_PANIC_BODY_NOT_RETAINED", "SYNTHETIC_SQL", "environmentVariables", "commandLine"} {
				if bytes.Contains(b, []byte(secret)) {
					t.Fatal("raw body retained", secret)
				}
			}
			if c.raw == "node-25.9.0.json" {
				for _, v := range s.Observations {
					if v.Kind == "thread_snapshot" && v.State != "unknown" {
						t.Fatal("handle inferred wait", v)
					}
				}
			}
			if c.raw == "postgres-estimated.json" && s.Plans[0].ActualRows != nil {
				t.Fatal("estimate fabricated actual")
			}
			if c.raw == "postgres-actual.json" && s.Plans[0].ActualRows == nil {
				t.Fatal("actual lost")
			}
		})
	}
}
func TestStrictBoundaryAndCancellation(t *testing.T) {
	raw := read(t, "python-set.json")
	for _, bad := range [][]byte{append([]byte(`{"unknown":1,`), raw[1:]...), []byte(`{"schema_version":"1.0.0","schema_version":"1.0.0"}`)} {
		if _, e := r.Decode(context.Background(), bad); e == nil {
			t.Fatal("unknown/duplicate accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := r.Decode(ctx, raw); e == nil {
		t.Fatal("cancel ignored")
	}
	o, _ := p.DecodeOptions(read(t, "python-options.json"))
	o.Process = nil
	if _, e := p.ParseBytes(context.Background(), read(t, "python-3.14.4.json"), o); e == nil {
		t.Fatal("process context fabricated")
	}
}

func TestProfileReferencesRequireExplicitMappingAndDoNotCopyCost(t *testing.T) {
	raw, e := os.ReadFile("../../catalog/contracts/profile-source-set.json")
	if e != nil {
		t.Fatal(e)
	}
	o := p.Options{Format: "profile-source-set-json", Clock: "unknown", Process: &r.Process{ID: "profile-process", PID: "500", Runtime: "Python"}, Entities: []r.Entity{{ID: "worker", Process: "profile-process", Kind: "worker", RuntimeID: "worker", NativeEncoding: "none", Mapping: "declared"}}, ProfileEntities: map[string]string{"profile-0": "worker", "profile-1": "worker"}}
	set, e := p.ParseBytes(context.Background(), raw, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(set.Profiles) != 2 || len(set.Observations) != 0 || len(set.Frames) != 0 {
		t.Fatal("cost or frames copied", set)
	}
	for _, ref := range set.Profiles {
		if ref.ModelSHA != p.Digest(raw) || !r.Digest(ref.OriginalSHA) {
			t.Fatal(ref)
		}
	}
	o.ProfileEntities = nil
	if _, e = p.ParseBytes(context.Background(), raw, o); e == nil {
		t.Fatal("name-based mapping invented")
	}
}
