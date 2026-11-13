package profile

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
)

// cprofileFlat imports the body-free owner export, never marshal/pickle .pstats.
// Flat function self time cannot establish caller stacks or a sample timeline.
func (b *sourceBuilder) cprofileFlat(body []byte, options SourceOptions) error {
	var f struct {
		Producer  string `json:"producer"`
		Version   string `json:"version"`
		Scope     string `json:"scope"`
		Functions []struct {
			Name       string `json:"name"`
			Self       string `json:"self_seconds"`
			Cumulative string `json:"cumulative_seconds"`
			Calls      string `json:"calls"`
			Recursive  string `json:"recursive_calls"`
		} `json:"functions"`
	}
	if err := wirejson.StrictWithLimits(body, &f, wirejson.ResultLimits(SourceMaxBytes, 256)); err != nil {
		return err
	}
	if f.Producer != "CPython cProfile" || f.Version != "3.14.4" || len(f.Functions) == 0 || len(f.Functions) > profilemodel.MaxSamples {
		return fmt.Errorf("PROFILE_CPROFILE_UNQUALIFIED: require CPython cProfile 3.14.4 flat owner export")
	}
	if options.Producer != "" && options.Producer != f.Producer || options.ProducerVersion != "" && options.ProducerVersion != f.Version {
		return fmt.Errorf("PROFILE_PRODUCER_CONFLICT")
	}
	b.set.Producer, b.set.ProducerVersion = f.Producer, f.Version
	b.contract.ProducerBasis = "file_exporter"
	b.contract.FormatVersion = "cprofile-flat.v1"
	b.set.Metrics = []profilemodel.Metric{{ID: "function-self-seconds", Kind: "duration", Unit: "seconds", Scale: "1"}}
	b.contract.Metrics = []profilemodel.SourceMetric{{ID: "function-self-seconds", Type: "flat_function_self_time", Unit: "seconds"}}
	b.set.Profiles = []profilemodel.Profile{{ID: "profile-0", Worker: "flat functions", Clock: "aggregate", Samples: []profilemodel.Sample{}}}
	b.contract.Profiles = []profilemodel.SourceProfile{{ID: "profile-0", Name: "flat functions", Type: "flat", Unit: "seconds"}}
	b.set.Quality.ConversionLoss = append(b.set.Quality.ConversionLoss, "Qualified CPython cProfile 3.14.4 flat self-time export only. One-frame function entries; no caller stacks, sample count, CPU-time attribution or timeline. Cumulative seconds and call counts are non-additive labels; not separately summed. Builtin/excluded functions may be absent; total is retained functions only. Capture options remain strict; no cross-producer policy equivalence.")
	seen := map[string]bool{}
	for i, r := range f.Functions {
		if r.Name == "" || len(r.Name) > 1024 || !wirejson.Text(r.Name, 1024) || seen[r.Name] {
			return fmt.Errorf("PROFILE_CPROFILE_FUNCTION_INVALID")
		}
		seen[r.Name] = true
		self, err := number(json.Number(r.Self))
		if err != nil || self.Sign() < 0 {
			return fmt.Errorf("PROFILE_CPROFILE_TIME_INVALID")
		}
		cumulative, err := number(json.Number(r.Cumulative))
		if err != nil || cumulative.Cmp(self) < 0 {
			return fmt.Errorf("PROFILE_CPROFILE_TIME_INVALID")
		}
		calls, ok := new(big.Int).SetString(r.Calls, 10)
		if !ok || calls.Sign() < 0 || len(r.Calls) > 20 {
			return fmt.Errorf("PROFILE_CPROFILE_CALLS_INVALID")
		}
		recursive, ok := new(big.Int).SetString(r.Recursive, 10)
		if !ok || recursive.Sign() < 0 || recursive.Cmp(calls) > 0 || len(r.Recursive) > 20 {
			return fmt.Errorf("PROFILE_CPROFILE_CALLS_INVALID")
		}
		id := b.frame(profilemodel.Frame{Symbol: r.Name, Display: r.Name, Runtime: "Python", Language: "Python"})
		if err = b.sample(0, []string{id}, "function-self-seconds", profilemodel.Text(self), nil, nil, map[string][]string{"cumulative_seconds": {profilemodel.Text(cumulative)}, "calls": {r.Calls}, "recursive_calls": {r.Recursive}}, fmt.Sprintf("functions/%d", i)); err != nil {
			return err
		}
	}
	return b.failure
}
