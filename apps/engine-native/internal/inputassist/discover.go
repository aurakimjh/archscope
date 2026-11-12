// Package inputassist explains local evidence without parsing it into results,
// attaching to a process, searching siblings or running collection recipes.
package inputassist

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var slots = make(chan struct{}, 2)

type Advice struct {
	Code   string `json:"code"`
	Route  string `json:"route"`
	Recipe string `json:"recipe"`
}
type Context struct {
	Status  string   `json:"status"`
	Missing []string `json:"missing"`
}
type Report struct {
	MappingPreviews []MappingPreview               `json:"mapping_previews"`
	Version         string                         `json:"version"`
	Source          ingestion.SourceInfo           `json:"source"`
	ProbeBytes      int                            `json:"probe_bytes"`
	Detection       ingestion.DetectionSet         `json:"detection"`
	Importers       []ingestion.ImporterDescriptor `json:"importers"`
	Context         Context                        `json:"context"`
	Advice          []Advice                       `json:"advice"`
	Draft           cases.Capture                  `json:"capture_draft"`
}

func Discover(ctx context.Context, catalog *ingestion.Catalog, path, sidecar string, mappings ...packs.Mapping) (Report, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return Report{}, ingestion.Problem("DISCOVERY_BUSY", "another input inspection is running")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	report := Report{MappingPreviews: []MappingPreview{}, Version: "1.0.0", Advice: []Advice{}, Importers: catalog.List()}
	dir, e := os.MkdirTemp("", "archscope-discover-")
	if e != nil {
		return report, e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "discovery-source", ingestion.DefaultBudgets())
	if e != nil {
		return report, e
	}
	defer lease.Close()
	report.Source = lease.Info()
	reader, e := lease.Open(ctx)
	if e != nil {
		return report, e
	}
	head, e := io.ReadAll(io.LimitReader(reader, ingestion.MaxProbeBytes))
	reader.Close()
	if e != nil {
		return report, e
	}
	report.ProbeBytes = len(head)
	report.Detection, e = catalog.DetectCandidates(ctx, ingestion.Probe{Path: report.Source.Name, Head: head, SizeBytes: report.Source.Size, Extension: strings.ToLower(filepath.Ext(path))})
	if e != nil {
		return report, e
	}
	if e = previewMappings(ctx, &report, head, mappings); e != nil {
		return report, e
	}
	if len(head) >= 4 && string(head[:4]) == "FLR\x00" {
		report.Advice = append(report.Advice, Advice{"binary_jfr_requires_conversion", "jfr", "jdk.jfr"})
	} else if strings.Contains(string(head), `"recording"`) && strings.Contains(string(head), `"events"`) {
		report.Advice = append(report.Advice, Advice{"jfr_json_hint_validate_in_jfr_view", "jfr", "jdk.jfr"})
	}
	if report.Detection.State == "unsupported" && len(report.Advice) == 0 {
		report.Advice = append(report.Advice, Advice{"no_generic_importer_use_supported_format_guide", "manual", "manual.review"})
	}
	capture := cases.Capture{}
	report.Context.Status = "absent"
	if sidecar != "" {
		f, e := os.Open(sidecar)
		if e != nil {
			return report, ingestion.Problem("CAPTURE_SOURCE_UNAVAILABLE", "select a local capture sidecar")
		}
		defer f.Close()
		i, e := f.Stat()
		if e != nil || !i.Mode().IsRegular() || i.Size() > cases.MaxManifestBytes {
			return report, ingestion.Problem("CAPTURE_INVALID", "bounded regular capture sidecar required")
		}
		b, e := io.ReadAll(io.LimitReader(f, cases.MaxManifestBytes+1))
		if e != nil {
			return report, e
		}
		if len(b) > cases.MaxManifestBytes {
			return report, ingestion.Problem("CAPTURE_INVALID", "capture sidecar byte limit")
		}
		if e = wirejson.Decode(b, &capture); e != nil {
			return report, ingestion.Problem("CAPTURE_INVALID", "capture sidecar JSON invalid")
		}
		if e = capture.Validate(); e != nil {
			return report, e
		}
		matched := false
		for _, file := range capture.Files {
			matched = matched || file.SHA256 == report.Source.SHA256 && file.Size == strconv.FormatInt(report.Source.Size, 10)
		}
		if !matched {
			return report, ingestion.Problem("CAPTURE_FILE_MISMATCH", "capture sidecar does not describe the selected bytes")
		}
		report.Context.Status = "matched_declared"
	}
	// Only entities explicitly linked to these file bytes may fill identity gaps.
	contextCapture := capture
	contextCapture.Entities = nil
	selectedEntities := map[string]bool{}
	for _, file := range capture.Files {
		if file.SHA256 == report.Source.SHA256 && file.Size == strconv.FormatInt(report.Source.Size, 10) {
			for _, id := range file.EntityIDs {
				selectedEntities[id] = true
			}
		}
	}
	for _, entity := range capture.Entities {
		if selectedEntities[entity.ID] {
			contextCapture.Entities = append(contextCapture.Entities, entity)
		}
	}
	report.Context.Missing = MissingContext(contextCapture)
	var id [16]byte
	if _, e = rand.Read(id[:]); e != nil {
		return report, e
	}
	report.Draft = cases.Capture{SchemaVersion: cases.Version, ID: "capture-" + hex.EncodeToString(id[:]), Producer: "unknown", ProducerVersion: "unknown", IdentityBasis: "unknown", Files: []cases.CaptureFile{{Name: report.Source.Name, SHA256: report.Source.SHA256, Size: strconv.FormatInt(report.Source.Size, 10), EntityIDs: []string{}}}, Entities: []cases.Entity{}, Devices: []cases.Device{}, Clock: cases.Clock{Domain: "unknown"}, Options: map[string]json.RawMessage{}, Quality: map[string]cases.Claim{}, Conversions: []cases.Conversion{}, Warnings: []string{"Generated draft: file hash/size are observed; capture identity, producer and clock require explicit review. File mtime is not collection time."}}
	if e = report.Draft.Validate(); e != nil {
		report.Draft.Files[0].Name = "selected-source.bin"
		report.Draft.Warnings = append(report.Draft.Warnings, "Original filename is unsafe for a portable sidecar; review file name before use.")
		if e = report.Draft.Validate(); e != nil {
			return report, e
		}
	}
	if e = lease.Verify(ctx); e != nil {
		return report, e
	}
	return report, ctx.Err()
}
func known(c *cases.Claim) bool { return c != nil && c.Value != nil && c.Basis != "unknown" }
func MissingContext(c cases.Capture) []string {
	missing := []string{}
	if c.Producer == "" || c.Producer == "unknown" {
		missing = append(missing, "producer")
	}
	if c.ProducerVersion == "" || c.ProducerVersion == "unknown" {
		missing = append(missing, "producer_version")
	}
	if !known(c.Clock.Start) {
		missing = append(missing, "collection_start")
	}
	if !known(c.Clock.End) {
		missing = append(missing, "collection_end")
	}
	if c.Clock.Domain == "" || c.Clock.Domain == "unknown" {
		missing = append(missing, "clock_domain")
	}
	if !known(c.Clock.Timezone) {
		missing = append(missing, "timezone")
	}
	host, process := false, false
	for _, entity := range c.Entities {
		host = host || known(entity.Host)
		process = process || known(entity.PID) && known(entity.ProcessStart)
	}
	if !host {
		missing = append(missing, "host_identity")
	}
	if !process {
		missing = append(missing, "process_lifetime")
	}
	if c.IdentityBasis == "" || c.IdentityBasis == "unknown" {
		missing = append(missing, "identity_basis")
	}
	return missing
}
