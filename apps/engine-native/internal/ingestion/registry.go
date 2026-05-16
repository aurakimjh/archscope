package ingestion

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxProbeBytes = 64 * 1024

var ErrUnknownFormat = errors.New("unknown source format")

// Probe is the bounded input shown to format detectors. Detectors should not
// open or parse the full file; full parsing belongs in parser packages.
type Probe struct {
	Path      string
	Head      []byte
	SizeBytes int64
	Extension string
}

// Detection is the result of one format detector. Confidence is 0..1. A zero
// confidence detection is ignored.
type Detection struct {
	FormatID   string  `json:"format_id"`
	SourceKind string  `json:"source_kind"`
	Product    string  `json:"product,omitempty"`
	Version    string  `json:"version,omitempty"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

type DetectorFunc func(Probe) Detection

// SourceFormat describes one parser format exposed by an evidence family.
type SourceFormat struct {
	ID            string       `json:"id"`
	SourceKind    string       `json:"source_kind"`
	Product       string       `json:"product,omitempty"`
	Version       string       `json:"version,omitempty"`
	ResultType    string       `json:"result_type,omitempty"`
	Parser        string       `json:"parser,omitempty"`
	Extensions    []string     `json:"extensions,omitempty"`
	Detector      DetectorFunc `json:"-"`
	DetectorOrder int          `json:"detector_order,omitempty"`
}

type FormatRegistry struct {
	formats map[string]SourceFormat
	order   []string
}

func NewFormatRegistry(formats ...SourceFormat) (*FormatRegistry, error) {
	r := &FormatRegistry{
		formats: map[string]SourceFormat{},
		order:   []string{},
	}
	for _, format := range formats {
		if err := r.Register(format); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *FormatRegistry) Register(format SourceFormat) error {
	if strings.TrimSpace(format.ID) == "" {
		return fmt.Errorf("source format id is required")
	}
	if strings.TrimSpace(format.SourceKind) == "" {
		return fmt.Errorf("%s source kind is required", format.ID)
	}
	if _, exists := r.formats[format.ID]; exists {
		return fmt.Errorf("duplicate source format %q", format.ID)
	}
	normalized := format
	normalized.Extensions = normalizeExtensions(format.Extensions)
	r.formats[format.ID] = normalized
	r.order = append(r.order, format.ID)
	sort.SliceStable(r.order, func(i, j int) bool {
		a := r.formats[r.order[i]]
		b := r.formats[r.order[j]]
		if a.DetectorOrder != b.DetectorOrder {
			return a.DetectorOrder < b.DetectorOrder
		}
		return a.ID < b.ID
	})
	return nil
}

func (r *FormatRegistry) Formats(sourceKind string) []SourceFormat {
	out := make([]SourceFormat, 0, len(r.order))
	for _, id := range r.order {
		format := r.formats[id]
		if sourceKind == "" || format.SourceKind == sourceKind {
			out = append(out, format)
		}
	}
	return out
}

func (r *FormatRegistry) Lookup(id string) (SourceFormat, bool) {
	format, ok := r.formats[id]
	return format, ok
}

func (r *FormatRegistry) DetectFile(path string) (Detection, error) {
	probe, err := ReadProbe(path)
	if err != nil {
		return Detection{}, err
	}
	return r.Detect(probe)
}

func (r *FormatRegistry) Detect(probe Probe) (Detection, error) {
	best := Detection{}
	for _, id := range r.order {
		format := r.formats[id]
		detection := Detection{}
		if format.Detector != nil {
			detection = format.Detector(probe)
		} else if extensionMatches(probe.Extension, format.Extensions) {
			detection = Detection{Confidence: 0.25, Reason: "extension match"}
		}
		if detection.Confidence <= 0 {
			continue
		}
		if detection.FormatID == "" {
			detection.FormatID = format.ID
		}
		if detection.SourceKind == "" {
			detection.SourceKind = format.SourceKind
		}
		if detection.Product == "" {
			detection.Product = format.Product
		}
		if detection.Version == "" {
			detection.Version = format.Version
		}
		if detection.Confidence > best.Confidence {
			best = detection
		}
	}
	if best.Confidence <= 0 {
		return Detection{}, fmt.Errorf("%w for %s", ErrUnknownFormat, probe.Path)
	}
	if best.Confidence > 1 {
		best.Confidence = 1
	}
	return best, nil
}

func ReadProbe(path string) (Probe, error) {
	f, err := os.Open(path)
	if err != nil {
		return Probe{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Probe{}, err
	}
	limit := MaxProbeBytes
	if info.Size() >= 0 && info.Size() < int64(limit) {
		limit = int(info.Size())
	}
	head := make([]byte, limit)
	n, readErr := io.ReadFull(f, head)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return Probe{}, readErr
	}
	return Probe{
		Path:      path,
		Head:      head[:n],
		SizeBytes: info.Size(),
		Extension: strings.ToLower(filepath.Ext(path)),
	}, nil
}

func ContainsSignature(signature string, confidence float64, reason string) DetectorFunc {
	return func(probe Probe) Detection {
		if signature == "" {
			return Detection{}
		}
		if strings.Contains(string(probe.Head), signature) {
			return Detection{Confidence: confidence, Reason: reason}
		}
		return Detection{}
	}
}

func normalizeExtensions(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		next := strings.ToLower(strings.TrimSpace(value))
		if next == "" {
			continue
		}
		if !strings.HasPrefix(next, ".") {
			next = "." + next
		}
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		out = append(out, next)
	}
	sort.Strings(out)
	return out
}

func extensionMatches(ext string, formats []string) bool {
	ext = strings.ToLower(strings.TrimSpace(ext))
	for _, candidate := range formats {
		if ext == candidate {
			return true
		}
	}
	return false
}
