package profile

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/symbolinfo"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"io"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const SourceMaxBytes = 64 << 20
const SourceMaxModelBytes = 32 << 20

var sourceSlots = make(chan struct{}, 2)
var sourceNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func DecodeSourceOptions(body []byte) (SourceOptions, error) {
	var options SourceOptions
	if len(body) > 64<<10 {
		return options, fmt.Errorf("PROFILE_OPTIONS_INVALID")
	}
	if e := wirejson.Decode(body, &options); e != nil {
		return options, e
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	return options, d.Decode(&options)
}

type SourceOptions struct {
	Format          string          `json:"format"`
	Producer        string          `json:"producer"`
	ProducerVersion string          `json:"producer_version"`
	CaptureOptions  json.RawMessage `json:"capture_options"`
}
type SourceInventory struct {
	SourceSHA256    string                `json:"source_sha256"`
	Producer        string                `json:"producer"`
	ProducerVersion string                `json:"producer_version"`
	Format          string                `json:"format"`
	Metrics         []profilemodel.Metric `json:"metrics"`
	Profiles        []ProfileInventory    `json:"profiles"`
	DefaultMetric   string                `json:"default_metric,omitempty"`
	Warnings        []string              `json:"warnings"`
}
type ProfileInventory struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Metrics []string `json:"metrics"`
}

func Inventory(s profilemodel.ProfileSet) SourceInventory {
	out := SourceInventory{SourceSHA256: s.SourceHashes[0], Producer: s.Producer, ProducerVersion: s.ProducerVersion, Format: s.Format, Metrics: s.Metrics, Profiles: []ProfileInventory{}, Warnings: append([]string(nil), s.Quality.ConversionLoss...)}
	c, _, _ := s.SourceContract()
	for i, p := range s.Profiles {
		ids := []string{}
		for _, m := range s.Metrics {
			for _, sample := range p.Samples {
				if sample.MetricID == m.ID {
					ids = append(ids, m.ID)
					break
				}
			}
		}
		name := p.Worker
		if i < len(c.Profiles) {
			name = c.Profiles[i].Name
		}
		out.Profiles = append(out.Profiles, ProfileInventory{p.ID, name, ids})
	}
	return out
}
func ParseSourceFile(ctx context.Context, path string, options SourceOptions) (profilemodel.ProfileSet, error) {
	select {
	case sourceSlots <- struct{}{}:
		defer func() { <-sourceSlots }()
	default:
		return profilemodel.ProfileSet{}, fmt.Errorf("PROFILE_SOURCE_BUSY: two normalizations already running")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	dir, e := os.MkdirTemp("", "profile-source-")
	if e != nil {
		return profilemodel.ProfileSet{}, e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "profile-original", ingestion.DefaultBudgets())
	if e != nil {
		return profilemodel.ProfileSet{}, e
	}
	defer lease.Close()
	r, e := lease.Open(ctx)
	if e != nil {
		return profilemodel.ProfileSet{}, e
	}
	defer r.Close()
	body, e := io.ReadAll(io.LimitReader(r, SourceMaxBytes+1))
	if e != nil {
		return profilemodel.ProfileSet{}, e
	}
	s, e := ParseSourceBytes(ctx, body, options)
	if e != nil {
		return s, e
	}
	if e = lease.Verify(ctx); e != nil {
		return s, e
	}
	return s, ctx.Err()
}
func ParseSourceBytes(ctx context.Context, original []byte, options SourceOptions) (profilemodel.ProfileSet, error) {
	if len(original) > SourceMaxBytes {
		return profilemodel.ProfileSet{}, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "profile source exceeds 64 MiB")
	}
	sum := sha256.Sum256(original)
	body := original
	if bytes.HasPrefix(body, []byte{31, 139}) {
		z, e := gzip.NewReader(bytes.NewReader(body))
		if e != nil {
			return profilemodel.ProfileSet{}, e
		}
		defer z.Close()
		body, e = io.ReadAll(io.LimitReader(z, SourceMaxBytes+1))
		if e != nil {
			return profilemodel.ProfileSet{}, e
		}
		if len(body) > SourceMaxBytes {
			return profilemodel.ProfileSet{}, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "decoded profile exceeds 64 MiB")
		}
	}
	format := options.Format
	if format == "" || format == "auto" {
		if bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
			format = "speedscope-json"
		} else {
			return profilemodel.ProfileSet{}, fmt.Errorf("PROFILE_FORMAT_REQUIRED: explicitly choose pprof or pyspy-raw")
		}
	}
	switch format {
	case "pprof-gz":
		format = "pprof"
	case "speedscope":
		format = "speedscope-json"
	case "py-spy":
		format = "pyspy-raw"
	}
	if len(options.CaptureOptions) == 0 {
		options.CaptureOptions = json.RawMessage(`{}`)
	}
	var capture map[string]json.RawMessage
	if len(options.CaptureOptions) > 64<<10 || wirejson.Decode(options.CaptureOptions, &capture) != nil || capture == nil {
		return profilemodel.ProfileSet{}, fmt.Errorf("PROFILE_CAPTURE_INVALID: bounded sanitized object required")
	}
	if e := safeCapture(options.CaptureOptions); e != nil {
		return profilemodel.ProfileSet{}, e
	}
	producer := options.Producer
	if producer == "" {
		producer = "unknown"
	}
	b := sourceBuilder{ctx: ctx, set: profilemodel.ProfileSet{SchemaVersion: profilemodel.Version, Producer: producer, ProducerVersion: options.ProducerVersion, Format: format, SourceHashes: []string{hex.EncodeToString(sum[:])}, CaptureOptions: append(json.RawMessage(nil), options.CaptureOptions...), Frames: []profilemodel.Frame{}, Stacks: []profilemodel.Stack{}, Metrics: []profilemodel.Metric{}, Profiles: []profilemodel.Profile{}, Quality: profilemodel.Quality{ClockConfidence: "unavailable", ConversionLoss: []string{"producer/context identity is declared, not original-source authentication"}}, Extensions: map[string]json.RawMessage{}}, contract: profilemodel.SourceContract{Revision: profilemodel.SourceNormalizer, ProducerBasis: "unknown", Metrics: []profilemodel.SourceMetric{}, Profiles: []profilemodel.SourceProfile{}, Attributes: map[string]json.RawMessage{}}, frames: map[string]bool{}, stacks: map[string]bool{}}
	if producer != "unknown" {
		b.contract.ProducerBasis = "declared"
	}
	var e error
	switch format {
	case "speedscope-json":
		e = b.speedscope(body, options)
	case "pyspy-raw":
		e = b.collapsed(body)
	case "pprof":
		e = b.pprof(body)
	default:
		e = fmt.Errorf("PROFILE_FORMAT_UNSUPPORTED: use speedscope-json, pyspy-raw or pprof")
	}
	if e != nil {
		return b.set, e
	}
	// PID remains an entity condition even when a compiled capture policy calls it incidental.
	if pid, ok := capture["pid"]; ok && string(pid) != "null" {
		for i := range b.set.Profiles {
			b.set.Profiles[i].Process = "declared-pid:" + string(pid)
		}
	}
	raw, e := json.Marshal(b.contract)
	if e != nil {
		return b.set, e
	}
	for i, m := range b.contract.Metrics {
		b.set.Metrics[i].SourceType = m.Type
		b.set.Metrics[i].SourceUnit = m.Unit
	}
	b.set.Extensions[profilemodel.SourceExtension] = raw
	if e = b.set.Validate(); e != nil {
		return b.set, e
	}
	encoded, e := json.Marshal(b.set)
	if e != nil {
		return b.set, e
	}
	if len(encoded) > SourceMaxModelBytes {
		return b.set, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "exact profile model exceeds 32 MiB; no truncated result")
	}
	return b.set, ctx.Err()
}

type sourceBuilder struct {
	ctx            context.Context
	set            profilemodel.ProfileSet
	contract       profilemodel.SourceContract
	frames, stacks map[string]bool
	observations   int
	charge         int
	references     int
	failure        error
}

func (b *sourceBuilder) frame(f profilemodel.Frame) string {
	if b.failure != nil {
		return ""
	}
	if e := b.ctx.Err(); e != nil {
		b.failure = e
		return ""
	}
	if f.Symbol == "" {
		f.Symbol = f.Display
	}
	info := symbolinfo.Describe(f.Symbol, f.File, f.Language)
	if info.Runtime == "Rust" && info.Confidence == "encoded" && (f.Display == "" || f.Display == f.Symbol) {
		f.Display = info.Display
	}
	f.ID = profilemodel.FrameID(f)
	if !b.frames[f.ID] {
		raw, _ := json.Marshal(f)
		b.charge += len(raw)
		if b.charge > SourceMaxModelBytes || len(b.frames) >= profilemodel.MaxFrames {
			b.failure = ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "exact profile frame/model budget exceeded")
			return ""
		}
		b.frames[f.ID] = true
		b.set.Frames = append(b.set.Frames, f)
	}
	return f.ID
}
func (b *sourceBuilder) sample(p int, frames []string, metric, weight string, timestamp, interval *string, labels map[string][]string, ref string) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if b.failure != nil {
		return b.failure
	}
	b.observations++
	if b.observations > profilemodel.MaxSamples || len(frames) > profilemodel.MaxDepth || len(b.set.Frames) > profilemodel.MaxFrames {
		return ingestion.Problem("IMPORT_BUDGET_EXCEEDED", fmt.Sprintf("exact profile limit exceeded: observations=%d (limit %d), frames=%d (limit %d), depth=%d (limit %d); reduce collection duration/rate/workers or explicitly split the source; no observations were coalesced", b.observations, profilemodel.MaxSamples, len(b.set.Frames), profilemodel.MaxFrames, len(frames), profilemodel.MaxDepth))
	}
	if len(frames) == 0 {
		frames = []string{b.frame(profilemodel.Frame{Symbol: "[unattributed]", Display: "[unattributed]"})}
	}
	stack := profilemodel.Stack{ID: profilemodel.StackID(frames), Frames: append([]string(nil), frames...)}
	if !b.stacks[stack.ID] {
		b.references += len(frames)
		if b.references > 1_000_000 {
			return ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "exact profile frame references exceed one million")
		}
		raw, _ := json.Marshal(stack)
		b.charge += len(raw)
		b.stacks[stack.ID] = true
		b.set.Stacks = append(b.set.Stacks, stack)
	}
	row := profilemodel.Sample{StackID: stack.ID, MetricID: metric, Weight: weight, Timestamp: timestamp, Interval: interval, Labels: labels, SourceRef: ref}
	raw, _ := json.Marshal(row)
	b.charge += len(raw)
	if b.charge > SourceMaxModelBytes {
		return ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "exact profile model exceeds 32 MiB")
	}
	b.set.Profiles[p].Samples = append(b.set.Profiles[p].Samples, row)
	return nil
}
func number(v json.Number) (*big.Rat, error) {
	text := v.String()
	if text == "" || len(text) > 1024 || !sourceNumberPattern.MatchString(text) {
		return nil, fmt.Errorf("PROFILE_NUMBER_INVALID: number required")
	}
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		exponent, e := strconv.Atoi(text[i+1:])
		if e != nil || exponent > 128 || exponent < -128 {
			return nil, fmt.Errorf("PROFILE_NUMBER_INVALID: exponent budget")
		}
	}
	n, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("PROFILE_NUMBER_INVALID")
	}
	if _, e := profilemodel.Number(profilemodel.Text(n)); e != nil {
		return nil, e
	}
	return n, nil
}
func numberText(n *big.Rat) *string { s := profilemodel.Text(n); return &s }
func (b *sourceBuilder) collapsed(body []byte) error {
	b.contract.FormatVersion = "collapsed.v1"
	b.set.Metrics = []profilemodel.Metric{{ID: "samples", Kind: "sample_count", Unit: "count", Scale: "1"}}
	b.contract.Metrics = []profilemodel.SourceMetric{{ID: "samples", Type: "sampled", Unit: "count"}}
	b.set.Profiles = []profilemodel.Profile{{ID: "profile-0", Clock: "aggregate", Samples: []profilemodel.Sample{}}}
	b.contract.Profiles = []profilemodel.SourceProfile{{ID: "profile-0", Name: "aggregate", Type: "collapsed", Unit: "count"}}
	scan := bufio.NewScanner(bytes.NewReader(body))
	scan.Buffer(make([]byte, 4096), 1<<20)
	line := 0
	for scan.Scan() {
		line++
		text := strings.TrimSpace(scan.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		i := strings.LastIndexAny(text, " \t")
		if i <= 0 {
			return fmt.Errorf("PROFILE_COLLAPSED_INVALID: expected stack and exact weight")
		}
		weight, e := number(json.Number(strings.TrimSpace(text[i:])))
		if e != nil || weight.Sign() < 0 {
			return fmt.Errorf("PROFILE_WEIGHT_INVALID: nonnegative exact weight required")
		}
		ids := []string{}
		for _, name := range strings.Split(strings.TrimSpace(text[:i]), ";") {
			if name == "" {
				return fmt.Errorf("PROFILE_STACK_INVALID: empty frame")
			}
			ids = append(ids, b.frame(profilemodel.Frame{Symbol: name, Display: name, Runtime: "Python", Language: "Python"}))
		}
		if e = b.sample(0, ids, "samples", profilemodel.Text(weight), nil, nil, nil, fmt.Sprintf("line:%d", line)); e != nil {
			return e
		}
	}
	return scan.Err()
}

func safeCapture(raw json.RawMessage) error {
	var value any
	if e := wirejson.Decode(raw, &value); e != nil {
		return e
	}
	forbidden := map[string]bool{"password": true, "passwd": true, "secret": true, "token": true, "api_key": true, "authorization": true, "cookie": true, "headers": true, "env": true, "environment": true, "argv": true, "command_line": true}
	var walk func(any) error
	walk = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for key, item := range x {
				if forbidden[strings.ToLower(key)] {
					return fmt.Errorf("PROFILE_CAPTURE_UNSAFE: remove credential/environment/command values before import")
				}
				if e := walk(item); e != nil {
					return e
				}
			}
		case []any:
			for _, item := range x {
				if e := walk(item); e != nil {
					return e
				}
			}
		}
		return nil
	}
	return walk(value)
}
