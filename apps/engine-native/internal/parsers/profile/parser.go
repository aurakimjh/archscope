package profile

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	pprofprofile "github.com/google/pprof/profile"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

type Frame struct {
	Name     string `json:"name"`
	Function string `json:"function,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Language string `json:"language,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Native   bool   `json:"native"`
	Async    bool   `json:"async"`
}

type Sample struct {
	Stack []Frame `json:"stack"`
	// Value is an integer quantity in Parsed.ValueUnit. Browser/V8 profiles use
	// microseconds; legacy formats continue to use their native sample count.
	Value        int64             `json:"value"`
	TimestampUS  int64             `json:"timestamp_us,omitempty"`
	Thread       string            `json:"thread,omitempty"`
	Process      string            `json:"process,omitempty"`
	Runtime      string            `json:"runtime,omitempty"`
	Language     string            `json:"language,omitempty"`
	ProfileKind  string            `json:"profile_kind,omitempty"`
	SourceFormat string            `json:"source_format,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Raw          string            `json:"raw,omitempty"`
}

type Parsed struct {
	Format    string         `json:"format"`
	ValueUnit string         `json:"value_unit,omitempty"`
	Samples   []Sample       `json:"samples"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Options struct {
	MaxBytes   int64
	MaxSamples int
}

// MaxProfileBytes is the explicit Phase-1 safety boundary. Structured V8 and
// trace JSON still need in-memory graph validation, so large recordings fail
// predictably instead of exhausting the desktop process.
const MaxProfileBytes int64 = 256 * 1024 * 1024

// DefaultMaxProfileSamples bounds browser profile output even when the Wails
// or CLI caller does not supply an advanced option. The V8 decoder retains
// elapsed time as weighted buckets, rather than silently discarding it.
const DefaultMaxProfileSamples = 500_000

var (
	collapsedLineRE = regexp.MustCompile(`^(.+)\s+([0-9]+(?:e[0-9]+)?)$`)
	xdebugFnRE      = regexp.MustCompile(`^fn=(.+)$`)
)

func ParseFile(path, format string, opts Options) (Parsed, *diagnostics.ParserDiagnostics, error) {
	format = canonical(format)
	diags := diagnostics.New(format)
	diags.SetSourceFile(path)
	// Chrome/V8 JSON is the only Phase-1 profile family for which large files
	// are a normal input. Select its decoder before readProfileData so a trace
	// does not incur both a whole-file byte buffer and a RawMessage envelope.
	if streamFormat, ok, err := profileStreamFormat(path, format, opts.MaxBytes); err != nil {
		return Parsed{}, diags, err
	} else if ok {
		diags.Format = streamFormat
		if opts.MaxSamples <= 0 {
			opts.MaxSamples = DefaultMaxProfileSamples
		}
		parsed, err := parseV8StreamFile(path, streamFormat, opts.MaxBytes, opts.MaxSamples)
		if err != nil {
			diags.AddError(0, "PROFILE_PARSE_ERROR", err.Error(), "")
			return Parsed{}, diags, err
		}
		return finalizeProfileParse(parsed, diags, opts, nil), diags, nil
	}

	data, err := readProfileData(path, opts.MaxBytes)
	if err != nil {
		return Parsed{}, diags, err
	}
	if format == "auto" {
		format = detect(data, path)
		diags.Format = format
	}

	var parsed Parsed
	switch format {
	case "pprof", "pprof-gz", "pprof-pb-gz":
		parsed, err = parsePprof(data, format)
	case "async-profiler-html", "flamegraph-html":
		parsed, err = parseHTML(path, diags)
	case "collapsed", "async-profiler-collapsed", "perf-collapsed":
		parsed, err = parseCollapsed(path, format, diags)
	case "speedscope-json", "dotnet-speedscope-json":
		parsed, err = parseSpeedscope(data, format)
	case "v8-cpuprofile", "chrome-trace-json":
		parsed, err = parseV8(data, format)
	case "jfr-json":
		parsed, err = parseJFRJSON(data)
	case "stackprof-json":
		parsed, err = parseStackprof(data)
	case "pyroscope-json", "parca-json", "php-excimer-json", "php-tideways-json", "generic-profile-json":
		parsed, err = parseGenericJSON(data, format)
	case "pyspy-raw":
		parsed = parseIndentedStacks(string(data), format, "Python", "Python")
	case "rbspy-raw":
		parsed = parseIndentedStacks(string(data), format, "Ruby", "Ruby")
	case "xdebug-cachegrind":
		parsed = parseXdebug(string(data))
	case "swift-backtrace":
		parsed = parseIndentedStacks(string(data), format, "Swift", "Swift")
	case "generic-async-stack":
		parsed = parseIndentedStacks(string(data), format, "", "")
	default:
		if json.Valid(data) {
			parsed, err = parseGenericJSON(data, "generic-profile-json")
		} else {
			parsed = parseIndentedStacks(string(data), "generic-async-stack", "", "")
		}
	}
	if err != nil {
		diags.AddError(0, "PROFILE_PARSE_ERROR", err.Error(), stringPreview(data))
		return Parsed{}, diags, err
	}
	parsed.Format = firstNonEmpty(parsed.Format, format)
	if parsed.ValueUnit == "" {
		parsed.ValueUnit = "samples"
	}
	return finalizeProfileParse(parsed, diags, opts, data), diags, nil
}

func finalizeProfileParse(parsed Parsed, diags *diagnostics.ParserDiagnostics, opts Options, data []byte) Parsed {
	parsed.Format = firstNonEmpty(parsed.Format, diags.Format)
	if parsed.ValueUnit == "" {
		parsed.ValueUnit = "samples"
	}
	for i := range parsed.Samples {
		if parsed.Samples[i].SourceFormat == "" {
			parsed.Samples[i].SourceFormat = parsed.Format
		}
		normalizeSample(&parsed.Samples[i], parsed.Format, parsed.ValueUnit == "microseconds")
	}
	if parsed.Metadata != nil {
		if count, _ := parsed.Metadata["negative_delta_clamp_count"].(int); count > 0 {
			diags.AddWarning(0, "PROFILE_NEGATIVE_DELTA_CLAMPED", fmt.Sprintf("clamped %d negative V8 time delta(s) to zero", count), "", false)
		}
		if parsed.Metadata["end_time_tail_clamped"] == true {
			diags.AddWarning(0, "PROFILE_END_TIME_BEFORE_LAST_SAMPLE", "V8 endTime precedes the last normalized sample timestamp; the final sample tail was clamped to zero", "", false)
		}
		if parsed.Metadata["hit_count_only"] == true {
			diags.AddWarning(0, "PROFILE_HITCOUNT_ONLY", "profile has hitCount aggregates but no ordered samples; temporal outputs are disabled", "", false)
		}
		if count, _ := parsed.Metadata["hit_count_mismatch_nodes"].(int); count > 0 {
			diags.AddWarning(0, "PROFILE_HITCOUNT_MISMATCH", fmt.Sprintf("samples and hitCount disagree for %d V8 node(s)", count), "", false)
		}
	}
	if opts.MaxSamples > 0 && len(parsed.Samples) > opts.MaxSamples {
		original := len(parsed.Samples)
		parsed.Samples = downsampleSamples(parsed.Samples, opts.MaxSamples)
		markProfileDownsample(&parsed, diags, original)
	} else if parsed.Metadata != nil && parsed.Metadata["partial_result"] == true && parsed.Metadata["downsampled_from_samples"] != nil {
		// The V8 streaming path performed the bounded aggregation while
		// decoding, so there is no post-parse slice length to compare here.
		original, _ := parsed.Metadata["downsampled_from_samples"].(int)
		diags.AddWarning(0, "PROFILE_DOWNSAMPLED", fmt.Sprintf("profile samples downsampled from %d to %d; elapsed time is retained as bucket weights", original, len(parsed.Samples)), "", false)
	}
	diags.ParsedRecords = len(parsed.Samples)
	if diags.TotalLines == 0 && data != nil {
		diags.TotalLines = lineCount(data)
	}
	if len(parsed.Samples) == 0 {
		context := ""
		if data != nil {
			context = stringPreview(data)
		}
		diags.AddWarning(0, "NO_PROFILE_SAMPLES", "profile input did not produce stack samples", context, false)
	}
	return parsed
}

func markProfileDownsample(parsed *Parsed, diags *diagnostics.ParserDiagnostics, original int) {
	if parsed.Metadata == nil {
		parsed.Metadata = map[string]any{}
	}
	parsed.Metadata["partial_result"] = true
	parsed.Metadata["downsampled_from_samples"] = original
	parsed.Metadata["downsampled_to_samples"] = len(parsed.Samples)
	diags.AddWarning(0, "PROFILE_DOWNSAMPLED", fmt.Sprintf("profile samples downsampled from %d to %d; elapsed time is retained as bucket weights", original, len(parsed.Samples)), "", false)
}

func readProfileData(path string, maxBytes int64) ([]byte, error) {
	reader, err := openProfileInput(path, maxBytes)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

type profileReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (r profileReadCloser) Close() error {
	var first error
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// cappedProfileReader makes a gzip bomb and an over-limit plain JSON file
// fail with the same explicit safety message. It intentionally does not turn
// the profile into a []byte; JSON decoders consume it incrementally.
type cappedProfileReader struct {
	reader    io.Reader
	remaining int64
	limit     int64
}

func (r *cappedProfileReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, fmt.Errorf("decompressed profile exceeds %d MiB safety limit", r.limit/(1024*1024))
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

func openProfileInput(path string, maxBytes int64) (io.ReadCloser, error) {
	if maxBytes <= 0 {
		maxBytes = MaxProfileBytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("profile input exceeds %d MiB safety limit", maxBytes/(1024*1024))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 2)
	n, _ := io.ReadFull(f, head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}
	var reader io.Reader = f
	closers := []io.Closer{f}
	if n == 2 && head[0] == 0x1f && head[1] == 0x8b {
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		reader = gz
		closers = append(closers, gz)
	}
	return profileReadCloser{Reader: &cappedProfileReader{reader: reader, remaining: maxBytes, limit: maxBytes}, closers: closers}, nil
}

func downsampleSamples(samples []Sample, maxSamples int) []Sample {
	if maxSamples <= 0 || len(samples) <= maxSamples {
		return samples
	}
	out := make([]Sample, 0, maxSamples)
	for bucket := 0; bucket < maxSamples; bucket++ {
		start := bucket * len(samples) / maxSamples
		end := (bucket + 1) * len(samples) / maxSamples
		if start >= end {
			continue
		}
		combined := samples[start]
		combined.Value = 0
		for _, sample := range samples[start:end] {
			combined.Value += sample.Value
		}
		out = append(out, combined)
	}
	return out
}

func parsePprof(data []byte, format string) (Parsed, error) {
	prof, err := pprofprofile.Parse(bytes.NewReader(data))
	if err != nil {
		return Parsed{}, err
	}
	valueIndex := choosePprofValueIndex(prof)
	samples := make([]Sample, 0, len(prof.Sample))
	for _, sample := range prof.Sample {
		value := 1
		if valueIndex >= 0 && valueIndex < len(sample.Value) && sample.Value[valueIndex] > 0 {
			value = int(sample.Value[valueIndex])
		}
		frames := make([]Frame, 0, len(sample.Location))
		for i := len(sample.Location) - 1; i >= 0; i-- {
			loc := sample.Location[i]
			if len(loc.Line) == 0 {
				name := fmt.Sprintf("0x%x", loc.Address)
				frames = append(frames, makeFrame(name, "", "", 0, format))
				continue
			}
			for _, line := range loc.Line {
				name := ""
				file := ""
				lineNo := 0
				if line.Function != nil {
					name = firstNonEmpty(line.Function.Name, line.Function.SystemName)
					file = line.Function.Filename
				}
				if line.Line > 0 {
					lineNo = int(line.Line)
				}
				frames = append(frames, makeFrame(name, name, file, lineNo, format))
			}
		}
		labels := firstLabels(sample.Label)
		samples = append(samples, Sample{
			Stack:        frames,
			Value:        int64(value),
			Thread:       labels["thread"],
			Process:      firstNonEmpty(labels["process"], labels["pid"]),
			Runtime:      labels["runtime"],
			Language:     labels["language"],
			ProfileKind:  pprofProfileKind(prof, valueIndex),
			SourceFormat: format,
			Labels:       labels,
		})
	}
	return Parsed{Format: format, Samples: samples, Metadata: map[string]any{
		"default_sample_type": prof.DefaultSampleType,
		"sample_type":         sampleTypeName(prof, valueIndex),
	}}, nil
}

func parseHTML(path string, diags *diagnostics.ParserDiagnostics) (Parsed, error) {
	result, err := coreprofiler.ParseHtmlProfilerFile(path, nil)
	if err != nil {
		return Parsed{}, err
	}
	copyProfilerDiagnostics(diags, result.Diagnostics)
	format := "async-profiler-html"
	if result.DetectedFormat != "" {
		format = result.DetectedFormat
	}
	return Parsed{
		Format:   "async-profiler-html",
		Samples:  samplesFromCollapsed(result.Stacks, format, "JVM", "Java"),
		Metadata: map[string]any{"detected_html_format": result.DetectedFormat},
	}, nil
}

func parseCollapsed(path, format string, diags *diagnostics.ParserDiagnostics) (Parsed, error) {
	stacks, profileDiags, err := coreprofiler.ParseCollapsedFileWithOptions(path, coreprofiler.Options{})
	if err != nil {
		return Parsed{}, err
	}
	copyProfilerDiagnostics(diags, profileDiags)
	runtime, language := "", ""
	if format == "async-profiler-collapsed" || format == "collapsed" {
		runtime, language = "JVM", "Java"
	}
	if format == "perf-collapsed" {
		runtime, language = "Native", "Native"
	}
	return Parsed{Format: format, Samples: samplesFromCollapsed(stacks, format, runtime, language)}, nil
}

func parseSpeedscope(data []byte, format string) (Parsed, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return Parsed{}, err
	}
	shared, _ := payload["shared"].(map[string]any)
	frameDefs := array(shared["frames"])
	frames := make([]Frame, 0, len(frameDefs))
	for _, item := range frameDefs {
		obj, _ := item.(map[string]any)
		name := firstNonEmpty(str(obj["name"]), str(obj["function"]))
		frames = append(frames, makeFrame(name, name, str(obj["file"]), intNum(obj["line"]), format))
	}
	var samples []Sample
	for _, profileItem := range array(payload["profiles"]) {
		profileObj, _ := profileItem.(map[string]any)
		weights := array(profileObj["weights"])
		for idx, stackItem := range array(profileObj["samples"]) {
			stackIdx := intArray(stackItem)
			stack := framesByIndex(frames, stackIdx)
			value := 1
			if idx < len(weights) {
				value = maxInt(1, intNum(weights[idx]))
			}
			samples = append(samples, Sample{Stack: stack, Value: int64(value), SourceFormat: format})
		}
		if len(samples) == 0 {
			samples = append(samples, eventedSpeedscopeSamples(profileObj, frames, format)...)
		}
	}
	return Parsed{Format: format, Samples: samples}, nil
}

func eventedSpeedscopeSamples(profileObj map[string]any, frames []Frame, format string) []Sample {
	var out []Sample
	var stack []int
	for _, eventItem := range array(profileObj["events"]) {
		event, _ := eventItem.(map[string]any)
		switch strings.ToUpper(str(event["type"])) {
		case "O", "OPEN":
			stack = append(stack, intNum(event["frame"]))
		case "C", "CLOSE":
			if len(stack) > 0 {
				out = append(out, Sample{Stack: framesByIndex(frames, stack), Value: 1, SourceFormat: format})
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out
}

func parseJFRJSON(data []byte) (Parsed, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return Parsed{}, err
	}
	var samples []Sample
	walkJSON(payload, func(obj map[string]any) {
		stackObj, ok := obj["stackTrace"].(map[string]any)
		if !ok {
			return
		}
		frames := framesFromJFR(stackObj)
		if len(frames) > 0 {
			samples = append(samples, Sample{Stack: frames, Value: 1, Runtime: "JVM", Language: "Java", SourceFormat: "jfr-json"})
		}
	})
	return Parsed{Format: "jfr-json", Samples: samples}, nil
}

func framesFromJFR(stackObj map[string]any) []Frame {
	var frames []Frame
	for _, item := range array(stackObj["frames"]) {
		obj, _ := item.(map[string]any)
		method, _ := obj["method"].(map[string]any)
		typeObj, _ := method["type"].(map[string]any)
		name := firstNonEmpty(str(method["name"]), str(obj["methodName"]))
		typeName := firstNonEmpty(str(typeObj["name"]), str(method["typeName"]))
		if typeName != "" && name != "" {
			name = typeName + "." + name
		}
		frames = append(frames, makeFrame(name, name, str(obj["file"]), intNum(obj["lineNumber"]), "jfr-json"))
	}
	reverseFrames(frames)
	return frames
}

func parseStackprof(data []byte) (Parsed, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return Parsed{}, err
	}
	frameMap, _ := payload["frames"].(map[string]any)
	framesByID := map[string]Frame{}
	for id, item := range frameMap {
		obj, _ := item.(map[string]any)
		name := firstNonEmpty(str(obj["name"]), str(obj["method"]), id)
		framesByID[id] = makeFrame(name, name, str(obj["file"]), intNum(obj["line"]), "stackprof-json")
	}
	var samples []Sample
	for id, item := range frameMap {
		obj, _ := item.(map[string]any)
		parent := framesByID[id]
		edges, _ := obj["edges"].(map[string]any)
		for childID, rawCount := range edges {
			child, ok := framesByID[childID]
			if !ok {
				continue
			}
			samples = append(samples, Sample{Stack: []Frame{parent, child}, Value: int64(maxInt(1, intNum(rawCount))), Runtime: "Ruby", Language: "Ruby", SourceFormat: "stackprof-json"})
		}
		if len(edges) == 0 {
			count := firstPositive(intNum(obj["samples"]), intNum(obj["total_samples"]), intNum(obj["totalSamples"]))
			if count > 0 {
				samples = append(samples, Sample{Stack: []Frame{parent}, Value: int64(count), Runtime: "Ruby", Language: "Ruby", SourceFormat: "stackprof-json"})
			}
		}
	}
	return Parsed{Format: "stackprof-json", Samples: samples, Metadata: map[string]any{"mode": str(payload["mode"])}}, nil
}

func parseGenericJSON(data []byte, format string) (Parsed, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return Parsed{}, err
	}
	samples := genericSamples(payload, format)
	if len(samples) == 0 && (format == "pyroscope-json" || format == "parca-json") {
		samples = continuousProfileFallback(payload, format)
	}
	return Parsed{Format: format, Samples: samples}, nil
}

func genericSamples(payload any, format string) []Sample {
	var out []Sample
	walkJSON(payload, func(obj map[string]any) {
		stackValues := firstArray(obj, "stack", "frames", "trace", "stacktrace", "stackTrace", "locations")
		if len(stackValues) == 0 {
			return
		}
		frames := framesFromAny(stackValues, format)
		if len(frames) == 0 {
			return
		}
		out = append(out, Sample{
			Stack:        frames,
			Value:        int64(firstPositive(intNum(obj["value"]), intNum(obj["samples"]), intNum(obj["count"]), intNum(obj["weight"]), 1)),
			Thread:       firstNonEmpty(str(obj["thread"]), str(obj["thread_name"]), str(obj["threadName"])),
			Process:      firstNonEmpty(str(obj["process"]), str(obj["pid"])),
			Runtime:      str(obj["runtime"]),
			Language:     str(obj["language"]),
			ProfileKind:  str(obj["profile_kind"]),
			SourceFormat: format,
		})
	})
	return out
}

func continuousProfileFallback(payload any, format string) []Sample {
	var out []Sample
	walkJSON(payload, func(obj map[string]any) {
		names := stringArray(obj["names"])
		if len(names) == 0 {
			return
		}
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" || strings.EqualFold(name, "total") {
				continue
			}
			out = append(out, Sample{Stack: []Frame{makeFrame(name, name, "", 0, format)}, Value: 1, SourceFormat: format})
		}
	})
	return out
}

func parseXdebug(text string) Parsed {
	counts := map[string]int{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	current := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if match := xdebugFnRE.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}
		if current != "" && line != "" && line[0] >= '0' && line[0] <= '9' {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				counts[current] += maxInt(1, intNum(fields[len(fields)-1]))
			}
		}
	}
	return Parsed{Format: "xdebug-cachegrind", Samples: samplesFromCollapsed(counts, "xdebug-cachegrind", "PHP", "PHP")}
}

func parseIndentedStacks(text, format, runtimeName, language string) Parsed {
	var samples []Sample
	var stack []Frame
	var thread string
	flush := func() {
		if len(stack) == 0 {
			return
		}
		samples = append(samples, Sample{Stack: append([]Frame(nil), stack...), Value: 1, Thread: thread, Runtime: runtimeName, Language: language, SourceFormat: format})
		stack = nil
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "python v") || strings.HasPrefix(lower, "ruby ") || strings.HasPrefix(lower, "rbspy") {
			continue
		}
		if strings.HasPrefix(lower, "process ") || strings.HasPrefix(lower, "thread ") || strings.Contains(lower, "thread id") {
			flush()
			thread = line
			continue
		}
		if isFrameLike(raw, format) {
			stack = append(stack, makeFrame(cleanFrameLine(line), cleanFrameLine(line), "", 0, format))
		}
	}
	flush()
	return Parsed{Format: format, Samples: samples}
}

func samplesFromCollapsed(stacks map[string]int, format, runtimeName, language string) []Sample {
	samples := make([]Sample, 0, len(stacks))
	for stack, count := range stacks {
		if count <= 0 {
			continue
		}
		parts := strings.Split(stack, ";")
		frames := make([]Frame, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			frames = append(frames, makeFrame(name, name, "", 0, format))
		}
		samples = append(samples, Sample{Stack: frames, Value: int64(count), Runtime: runtimeName, Language: language, SourceFormat: format})
	}
	return samples
}

func detect(data []byte, path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	trimmed := strings.TrimSpace(stringPreview(data))
	if strings.HasSuffix(base, ".pb.gz") || strings.HasSuffix(base, ".pprof") {
		return "pprof-gz"
	}
	if strings.Contains(base, "speedscope") || strings.Contains(strings.ToLower(trimmed), "speedscope") || jsonHas(data, "shared", "profiles") {
		if strings.Contains(base, "dotnet") || strings.Contains(base, "nettrace") {
			return "dotnet-speedscope-json"
		}
		return "speedscope-json"
	}
	if isChromeTrace(data) {
		return "chrome-trace-json"
	}
	if isV8Profile(data) {
		return "v8-cpuprofile"
	}
	if strings.Contains(base, "pyroscope") || jsonHas(data, "flamebearer", "") {
		return "pyroscope-json"
	}
	if strings.Contains(base, "parca") || jsonHas(data, "stacktraces", "") {
		return "parca-json"
	}
	if strings.Contains(base, "stackprof") || jsonHas(data, "frames", "mode") {
		return "stackprof-json"
	}
	if strings.Contains(base, "excimer") {
		return "php-excimer-json"
	}
	if strings.Contains(base, "tideways") {
		return "php-tideways-json"
	}
	if strings.Contains(trimmed, "<html") || strings.Contains(trimmed, "<svg") {
		return "async-profiler-html"
	}
	if strings.Contains(trimmed, "events:") || strings.Contains(trimmed, "fn=") && strings.Contains(trimmed, "fl=") {
		return "xdebug-cachegrind"
	}
	if strings.Contains(trimmed, "Process ") && strings.Contains(trimmed, "Python v") || strings.Contains(base, "py-spy") {
		return "pyspy-raw"
	}
	if strings.Contains(base, "rbspy") {
		return "rbspy-raw"
	}
	if strings.Contains(base, "perf") || ext == ".folded" {
		return "perf-collapsed"
	}
	if collapsedLineRE.MatchString(firstNonEmptyLine(string(data))) {
		return "async-profiler-collapsed"
	}
	if ext == ".json" {
		if strings.Contains(trimmed, "stackTrace") {
			return "jfr-json"
		}
		return "generic-profile-json"
	}
	if strings.Contains(trimmed, "async") && strings.Contains(trimmed, "`") {
		return "swift-backtrace"
	}
	return "generic-async-stack"
}

func canonical(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	format = strings.ReplaceAll(format, "_", "-")
	switch format {
	case "", "auto":
		return "auto"
	case "pprof-pb", "pprof-binary", "pprof-proto":
		return "pprof"
	case "pb-gz", "pprof.pb.gz":
		return "pprof-gz"
	case "html", "async-html":
		return "async-profiler-html"
	case "async-collapsed", "flamegraph-collapsed":
		return "async-profiler-collapsed"
	case "speedscope", "speedscope.json":
		return "speedscope-json"
	case "dotnet-trace", "nettrace", "nettrace-speedscope":
		return "dotnet-speedscope-json"
	case "py-spy", "pyspy":
		return "pyspy-raw"
	case "rbspy":
		return "rbspy-raw"
	case "stackprof":
		return "stackprof-json"
	case "xdebug":
		return "xdebug-cachegrind"
	case "cpuprofile", "v8", "v8-cpu-profile":
		return "v8-cpuprofile"
	case "chrome-trace", "chrome-trace-json", "trace-json":
		return "chrome-trace-json"
	default:
		return format
	}
}

func normalizeSample(sample *Sample, format string, allowZeroValue bool) {
	if sample.Value < 0 || (sample.Value == 0 && !allowZeroValue) {
		sample.Value = 1
	}
	for i := range sample.Stack {
		frame := &sample.Stack[i]
		if frame.Name == "" {
			frame.Name = frame.Function
		}
		if frame.Function == "" {
			frame.Function = frame.Name
		}
		if frame.Runtime == "" {
			frame.Runtime = firstNonEmpty(sample.Runtime, inferRuntime(frame.Name, frame.File, format))
		}
		if frame.Language == "" {
			frame.Language = firstNonEmpty(sample.Language, inferLanguage(frame.Name, frame.File, format))
		}
		frame.Native = frame.Native || isNativeFrame(frame.Name, frame.File, format)
		frame.Async = frame.Async || isAsyncFrame(frame.Name, format)
		if frame.Kind == "" {
			if frame.Native {
				frame.Kind = "native"
			} else {
				frame.Kind = "managed"
			}
		}
	}
	if sample.Runtime == "" && len(sample.Stack) > 0 {
		sample.Runtime = sample.Stack[len(sample.Stack)-1].Runtime
	}
	if sample.Language == "" && len(sample.Stack) > 0 {
		sample.Language = sample.Stack[len(sample.Stack)-1].Language
	}
	if sample.ProfileKind == "" {
		sample.ProfileKind = profileKindForFormat(format)
	}
}

func makeFrame(name, function, file string, line int, format string) Frame {
	name = strings.TrimSpace(firstNonEmpty(name, function, "unknown"))
	frame := Frame{Name: name, Function: firstNonEmpty(strings.TrimSpace(function), name), File: strings.TrimSpace(file), Line: line}
	frame.Runtime = inferRuntime(frame.Name, frame.File, format)
	frame.Language = inferLanguage(frame.Name, frame.File, format)
	frame.Native = isNativeFrame(frame.Name, frame.File, format)
	frame.Async = isAsyncFrame(frame.Name, format)
	if frame.Native {
		frame.Kind = "native"
	} else {
		frame.Kind = "managed"
	}
	return frame
}

func framesFromAny(items []any, format string) []Frame {
	frames := make([]Frame, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			frames = append(frames, makeFrame(v, v, "", 0, format))
		case map[string]any:
			name := firstNonEmpty(str(v["name"]), str(v["function"]), str(v["method"]), str(v["symbol"]))
			frames = append(frames, makeFrame(name, name, str(v["file"]), intNum(firstNonNil(v["line"], v["lineno"], v["lineNumber"])), format))
		}
	}
	return frames
}

func framesByIndex(frames []Frame, indexes []int) []Frame {
	out := make([]Frame, 0, len(indexes))
	for _, index := range indexes {
		if index >= 0 && index < len(frames) {
			out = append(out, frames[index])
		}
	}
	return out
}

func copyProfilerDiagnostics(dst *diagnostics.ParserDiagnostics, src coreprofiler.ParserDiagnostics) {
	dst.TotalLines = src.TotalLines
	dst.ParsedRecords = src.ParsedRecords
	dst.SkippedLines = src.SkippedLines
	if src.Format != "" {
		dst.Format = src.Format
	}
	for reason, count := range src.SkippedByReason {
		dst.SkippedByReason[reason] += count
	}
	for _, sample := range src.Samples {
		if len(dst.Samples) < diagnostics.MaxDiagnosticSamples {
			dst.Samples = append(dst.Samples, diagnostics.Sample{
				LineNumber: sample.LineNumber,
				Reason:     sample.Reason,
				Message:    sample.Message,
				RawPreview: sample.RawPreview,
			})
		}
	}
	dst.WarningCount = src.WarningCount
	dst.ErrorCount = src.ErrorCount
	for _, sample := range src.Warnings {
		if len(dst.Warnings) < diagnostics.MaxDiagnosticSamples {
			dst.Warnings = append(dst.Warnings, diagnostics.Sample{LineNumber: sample.LineNumber, Reason: sample.Reason, Message: sample.Message, RawPreview: sample.RawPreview})
		}
	}
	for _, sample := range src.Errors {
		if len(dst.Errors) < diagnostics.MaxDiagnosticSamples {
			dst.Errors = append(dst.Errors, diagnostics.Sample{LineNumber: sample.LineNumber, Reason: sample.Reason, Message: sample.Message, RawPreview: sample.RawPreview})
		}
	}
}

func choosePprofValueIndex(prof *pprofprofile.Profile) int {
	if len(prof.SampleType) == 0 {
		return -1
	}
	if prof.DefaultSampleType != "" {
		for i, sampleType := range prof.SampleType {
			if sampleType.Type == prof.DefaultSampleType {
				return i
			}
		}
	}
	preferred := []string{"samples", "sample", "cpu", "wall", "duration", "contentions", "delay", "alloc_objects", "inuse_objects"}
	for _, want := range preferred {
		for i, sampleType := range prof.SampleType {
			if strings.EqualFold(sampleType.Type, want) {
				return i
			}
		}
	}
	return 0
}

func pprofProfileKind(prof *pprofprofile.Profile, index int) string {
	name := strings.ToLower(sampleTypeName(prof, index))
	switch {
	case strings.Contains(name, "cpu"):
		return "cpu"
	case strings.Contains(name, "wall"), strings.Contains(name, "duration"):
		return "wall"
	case strings.Contains(name, "alloc"), strings.Contains(name, "heap"), strings.Contains(name, "space"):
		return "memory"
	case strings.Contains(name, "mutex"), strings.Contains(name, "block"), strings.Contains(name, "delay"):
		return "lock"
	default:
		return "samples"
	}
}

func sampleTypeName(prof *pprofprofile.Profile, index int) string {
	if index >= 0 && index < len(prof.SampleType) {
		return prof.SampleType[index].Type
	}
	return ""
}

func firstLabels(labels map[string][]string) map[string]string {
	out := map[string]string{}
	for key, values := range labels {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}

func profileKindForFormat(format string) string {
	lower := strings.ToLower(format)
	switch {
	case strings.Contains(lower, "cpu"), strings.Contains(lower, "pprof"), strings.Contains(lower, "perf"):
		return "cpu"
	case strings.Contains(lower, "lock"), strings.Contains(lower, "mutex"):
		return "lock"
	default:
		return "wall"
	}
}

func inferRuntime(name, file, format string) string {
	lower := strings.ToLower(name + " " + file + " " + format)
	switch {
	case strings.Contains(lower, "python"), strings.Contains(lower, ".py"), strings.Contains(lower, "pyspy"):
		return "Python"
	case strings.Contains(lower, "ruby"), strings.Contains(lower, ".rb"), strings.Contains(lower, "rbspy"), strings.Contains(lower, "stackprof"):
		return "Ruby"
	case strings.Contains(lower, "php"), strings.Contains(lower, "xdebug"), strings.Contains(lower, "tideways"), strings.Contains(lower, "excimer"):
		return "PHP"
	case strings.Contains(lower, "dotnet"), strings.Contains(lower, ".net"), strings.Contains(lower, "system."), strings.Contains(lower, "microsoft."):
		return ".NET"
	case strings.Contains(lower, "swift"):
		return "Swift"
	case strings.Contains(lower, ".go"), strings.HasPrefix(lower, "runtime."), strings.Contains(lower, "goroutine"):
		return "Go"
	case strings.Contains(lower, "jfr"), strings.Contains(lower, "java"), strings.Contains(lower, "jvm"):
		return "JVM"
	case strings.Contains(lower, "rust"), strings.Contains(name, "::"):
		return "Rust"
	case strings.Contains(lower, "perf"), strings.Contains(lower, "native"):
		return "Native"
	default:
		return "Application"
	}
}

func inferLanguage(name, file, format string) string {
	runtimeName := inferRuntime(name, file, format)
	switch runtimeName {
	case "JVM":
		return "Java"
	case ".NET":
		return ".NET"
	default:
		return runtimeName
	}
}

func isNativeFrame(name, file, format string) bool {
	lower := strings.ToLower(name + " " + file + " " + format)
	if strings.Contains(lower, "native") || strings.Contains(lower, "perf") || strings.Contains(lower, ".so") || strings.Contains(lower, ".dylib") || strings.Contains(lower, ".dll") {
		return true
	}
	return strings.HasPrefix(name, "[") || strings.HasPrefix(name, "0x")
}

func isAsyncFrame(name, format string) bool {
	lower := strings.ToLower(name + " " + format)
	return strings.Contains(lower, "async") || strings.Contains(lower, "await") || strings.Contains(lower, "continuation") || strings.Contains(lower, "tokio")
}

func cleanFrameLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#0123456789 ")
	line = strings.TrimSpace(strings.TrimPrefix(line, "at "))
	return line
}

func isFrameLike(raw, format string) bool {
	if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
		return true
	}
	line := strings.TrimSpace(raw)
	return strings.Contains(line, " at ") || strings.Contains(line, "::") || strings.Contains(line, ".") || strings.Contains(format, "swift")
}

func reverseFrames(frames []Frame) {
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}
}

func walkJSON(value any, visit func(map[string]any)) {
	switch v := value.(type) {
	case map[string]any:
		visit(v)
		for _, child := range v {
			walkJSON(child, visit)
		}
	case []any:
		for _, child := range v {
			walkJSON(child, visit)
		}
	}
}

func firstArray(obj map[string]any, keys ...string) []any {
	for _, key := range keys {
		if values := array(obj[key]); len(values) > 0 {
			return values
		}
	}
	return nil
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func intArray(v any) []int {
	values := array(v)
	out := make([]int, 0, len(values))
	for _, item := range values {
		out = append(out, intNum(item))
	}
	return out
}

func stringArray(v any) []string {
	values := array(v)
	out := make([]string, 0, len(values))
	for _, item := range values {
		if s := str(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intNum(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(n))
		return i
	default:
		return 0
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringPreview(data []byte) string {
	if len(data) > diagnostics.RawPreviewLimit {
		data = data[:diagnostics.RawPreviewLimit]
	}
	return string(data)
}

func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	return bytes.Count(data, []byte("\n")) + 1
}

func firstNonEmptyLine(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line
		}
	}
	return ""
}

func jsonHas(data []byte, keys ...string) bool {
	if !json.Valid(data) {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := payload[key]; !ok {
			return false
		}
	}
	return true
}
