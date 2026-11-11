package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

// ParseLegacySet wraps the evidence available from existing normalizers. It is
// explicitly not a corrected producer adapter: T-634 must decode original
// fractional/multivalue/evented data before those semantics can be claimed.
func ParseLegacySet(ctx context.Context, path, format string) (profilemodel.ProfileSet, *diagnostics.ParserDiagnostics, error) {
	directory, e := os.MkdirTemp("", "profile-legacy-")
	if e != nil {
		return profilemodel.ProfileSet{}, nil, e
	}
	defer os.RemoveAll(directory)
	lease, e := ingestion.Acquire(ctx, path, directory, "legacy-profile", ingestion.DefaultBudgets())
	if e != nil {
		return profilemodel.ProfileSet{}, nil, e
	}
	defer lease.Close()
	reader, e := lease.Open(ctx)
	if e != nil {
		return profilemodel.ProfileSet{}, nil, e
	}
	defer reader.Close()
	owned, e := os.CreateTemp(directory, "normalized-*"+filepath.Ext(path))
	if e != nil {
		return profilemodel.ProfileSet{}, nil, e
	}
	_, copyErr := io.Copy(owned, reader)
	closeErr := owned.Close()
	if copyErr != nil {
		return profilemodel.ProfileSet{}, nil, copyErr
	}
	if closeErr != nil {
		return profilemodel.ProfileSet{}, nil, closeErr
	}
	parsed, diags, e := ParseFile(owned.Name(), format, Options{MaxBytes: 64 << 20, MaxSamples: profilemodel.MaxSamples + 1})
	if e != nil {
		return profilemodel.ProfileSet{}, diags, e
	}
	if parsed.Metadata["partial_result"] == true {
		return profilemodel.ProfileSet{}, diags, fmt.Errorf("legacy-normalized partial data requires explicit producer reanalysis")
	}
	if e = ctx.Err(); e != nil {
		return profilemodel.ProfileSet{}, diags, e
	}
	if e = lease.Verify(ctx); e != nil {
		return profilemodel.ProfileSet{}, diags, e
	}
	set, e := FromLegacyParsed(parsed, lease.Info().SHA256)
	return set, diags, e
}
func FromLegacyParsed(parsed Parsed, sourceHash string) (profilemodel.ProfileSet, error) {
	set := profilemodel.ProfileSet{SchemaVersion: profilemodel.Version, Producer: "legacy-normalized", ProducerVersion: "0.1.0", Format: parsed.Format, SourceHashes: []string{sourceHash}, CaptureOptions: json.RawMessage(`{}`), Frames: []profilemodel.Frame{}, Stacks: []profilemodel.Stack{}, Metrics: []profilemodel.Metric{}, Profiles: []profilemodel.Profile{}, Quality: profilemodel.Quality{ClockConfidence: "unavailable", ConversionLoss: []string{"legacy normalizer semantics retained; original producer weight/value/chronology corrections require reanalysis"}}, Extensions: map[string]json.RawMessage{}}
	metadata, e := json.Marshal(parsed.Metadata)
	if e != nil {
		return set, e
	}
	set.Extensions["legacy_normalization"] = metadata
	frameIDs, stackIDs := map[string]bool{}, map[string]bool{}
	profileIndex := map[string]int{}
	metricIDs := map[string]bool{}
	for index, sample := range parsed.Samples {
		ids := []string{}
		for _, f := range sample.Stack {
			frame := profilemodel.Frame{Symbol: f.Function, Display: f.Name, File: f.File, Line: legacyLine(f.Line), Runtime: f.Runtime, Language: f.Language, Native: f.Native, Async: f.Async}
			if frame.Symbol == "" {
				frame.Symbol = f.Name
			}
			frame.ID = profilemodel.FrameID(frame)
			if !frameIDs[frame.ID] {
				set.Frames = append(set.Frames, frame)
				frameIDs[frame.ID] = true
			}
			ids = append(ids, frame.ID)
		}
		if len(ids) == 0 {
			return set, fmt.Errorf("legacy sample lacks a recoverable stack")
		}
		stack := profilemodel.Stack{ID: profilemodel.StackID(ids), Frames: ids}
		if !stackIDs[stack.ID] {
			set.Stacks = append(set.Stacks, stack)
			stackIDs[stack.ID] = true
		}
		metric := profilemodel.Metric{ID: "legacy-count", Kind: "sample_count", Unit: "count", Scale: "1"}
		if parsed.ValueUnit == "microseconds" {
			metric = profilemodel.Metric{ID: "legacy-cpu", Kind: "cpu_time", Unit: "microseconds", Scale: "1"}
		}
		if !metricIDs[metric.ID] {
			set.Metrics = append(set.Metrics, metric)
			metricIDs[metric.ID] = true
		}
		identity, _ := json.Marshal([]string{sample.Process, sample.Thread, sample.ProfileKind, sample.SourceFormat})
		sum := sha256.Sum256(identity)
		pid := "profile-" + hex.EncodeToString(sum[:])
		pi, exists := profileIndex[pid]
		if !exists {
			pi = len(set.Profiles)
			profileIndex[pid] = pi
			set.Profiles = append(set.Profiles, profilemodel.Profile{ID: pid, Process: sample.Process, Thread: sample.Thread, Clock: "aggregate", Samples: []profilemodel.Sample{}})
		}
		labels := map[string][]string{}
		for key, value := range sample.Labels {
			labels[key] = []string{value}
		}
		if _, exists := labels["legacy.profile_kind"]; exists {
			return set, fmt.Errorf("legacy provenance label collision")
		}
		labels["legacy.profile_kind"] = []string{sample.ProfileKind}
		if _, exists := labels["legacy.timestamp_us_raw"]; exists {
			return set, fmt.Errorf("legacy timing label collision")
		}
		labels["legacy.timestamp_us_raw"] = []string{strconv.FormatInt(sample.TimestampUS, 10)}
		set.Profiles[pi].Samples = append(set.Profiles[pi].Samples, profilemodel.Sample{StackID: stack.ID, MetricID: metric.ID, Weight: strconv.FormatInt(sample.Value, 10), Labels: labels, SourceRef: fmt.Sprintf("legacy-normalized-record:%d", index+1)})
	}
	if len(set.Profiles) == 0 {
		return set, fmt.Errorf("legacy source contains no recoverable profile")
	}
	return set, set.Validate()
}

func legacyLine(value int) *int64 {
	if value == 0 {
		return nil
	}
	n := int64(value)
	return &n
}
