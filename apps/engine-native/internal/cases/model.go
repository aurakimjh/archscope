// Package cases owns durable evidence/capture references, never analysis math.
package cases

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const Version = "1.0.0"

// Operational bounds. These are not diagnostic thresholds or retention periods.
const MaxObjectBytes = 64 << 20
const MaxManifestBytes = 2 << 20
const MaxCaseBytes = 512 << 20
const MaxEntries = 128
const MaxArchiveBytes = 256 << 20

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string           { return e.Code + ": " + e.Message }
func problem(code, message string) error { return &Error{code, message} }

var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var idRE = regexp.MustCompile(`^(case|source|result|artifact|baseline|capture)-[0-9a-f]{32}$`)

func id(prefix string) (string, error) {
	var b [16]byte
	_, e := rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:]), e
}
func hash(b []byte) string { x := sha256.Sum256(b); return hex.EncodeToString(x[:]) }
func now() string          { return time.Now().UTC().Format(time.RFC3339Nano) }
func revision(s string) (uint64, error) {
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil || n == 0 || strconv.FormatUint(n, 10) != s {
		return 0, problem("CASE_REVISION_INVALID", "invalid case revision")
	}
	return n, nil
}
func validID(s, prefix string) bool { return idRE.MatchString(s) && strings.HasPrefix(s, prefix+"-") }

type Claim struct {
	Value    *string `json:"value"`
	Basis    string  `json:"basis"`
	Evidence string  `json:"evidence,omitempty"`
}
type Clock struct {
	Domain       string `json:"domain"`
	Start        *Claim `json:"start,omitempty"`
	End          *Claim `json:"end,omitempty"`
	Timezone     *Claim `json:"timezone,omitempty"`
	AnchorUTC    *Claim `json:"anchor_utc,omitempty"`
	RelativeTick *Claim `json:"relative_tick,omitempty"`
	Offset       *Claim `json:"offset,omitempty"`
	Drift        *Claim `json:"drift,omitempty"`
	Uncertainty  *Claim `json:"uncertainty,omitempty"`
}
type Entity struct {
	ID           string `json:"id"`
	Host         *Claim `json:"host,omitempty"`
	Boot         *Claim `json:"boot,omitempty"`
	ProcessStart *Claim `json:"process_start,omitempty"`
	PID          *Claim `json:"pid,omitempty"`
	TIDNamespace *Claim `json:"tid_namespace,omitempty"`
	Container    *Claim `json:"container,omitempty"`
	Pod          *Claim `json:"pod,omitempty"`
	Worker       *Claim `json:"worker,omitempty"`
}
type Device struct {
	ID           string           `json:"id"`
	Vendor       string           `json:"vendor"`
	Kind         string           `json:"kind"`
	MemoryModel  string           `json:"memory_model"`
	ParentID     string           `json:"parent_id,omitempty"`
	MappingBasis string           `json:"mapping_basis"`
	Properties   map[string]Claim `json:"properties"`
}
type CaptureFile struct {
	Name       string   `json:"name"`
	SHA256     string   `json:"sha256"`
	Size       string   `json:"size"`
	FormatHint string   `json:"format_hint,omitempty"`
	EntityIDs  []string `json:"entity_ids,omitempty"`
}
type Capture struct {
	SchemaVersion   string                     `json:"schema_version"`
	ID              string                     `json:"id"`
	Producer        string                     `json:"producer"`
	ProducerVersion string                     `json:"producer_version"`
	IdentityBasis   string                     `json:"identity_basis"`
	Files           []CaptureFile              `json:"files"`
	Entities        []Entity                   `json:"entities"`
	Devices         []Device                   `json:"devices"`
	Clock           Clock                      `json:"clock"`
	Options         map[string]json.RawMessage `json:"options"`
	Quality         map[string]Claim           `json:"quality"`
	Conversions     []Conversion               `json:"conversions"`
	Redaction       Redaction                  `json:"redaction"`
	Warnings        []string                   `json:"warnings"`
}
type Conversion struct {
	Tool             string   `json:"tool"`
	Version          string   `json:"version"`
	Location         string   `json:"location"`
	InputSHA256      string   `json:"input_sha256"`
	OutputSHA256     string   `json:"output_sha256"`
	LostCapabilities []string `json:"lost_capabilities"`
}
type Redaction struct {
	Profile            string `json:"profile"`
	Version            string `json:"version"`
	Applied            bool   `json:"applied"`
	ReviewedForSharing bool   `json:"reviewed_for_sharing"`
}
type Encoding struct {
	Name          string `json:"name"`
	Basis         string `json:"basis"`
	Override      bool   `json:"override"`
	Replacements  string `json:"replacements"`
	IndexRevision string `json:"index_revision"`
}
type Source struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SHA256       string    `json:"sha256"`
	Size         string    `json:"size"`
	Storage      string    `json:"storage"`                 // external, managed, omitted
	ExternalPath string    `json:"external_path,omitempty"` // local-only; never in a portable manifest
	CaptureID    string    `json:"capture_id,omitempty"`
	Encoding     *Encoding `json:"encoding,omitempty"`
	Redaction    Redaction `json:"redaction"`
	ParentSHA256 string    `json:"parent_sha256,omitempty"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
}
type ResultRef struct {
	ID              string            `json:"id"`
	SHA256          string            `json:"sha256"`
	Type            string            `json:"type"`
	PayloadVersion  string            `json:"payload_version"`
	SourceIDs       []string          `json:"source_ids"`
	SourceHashes    map[string]string `json:"source_hashes"`
	CaptureID       string            `json:"capture_id,omitempty"`
	Selection       json.RawMessage   `json:"selection"`
	OptionsHash     string            `json:"options_hash,omitempty"`
	Versions        map[string]string `json:"versions"`
	Stale           bool              `json:"stale"`
	SharingReviewed bool              `json:"sharing_reviewed"`
	Availability    string            `json:"availability"`
}
type Artifact struct {
	ID           string          `json:"id"`
	ResultID     string          `json:"result_id"`
	ResultSHA256 string          `json:"result_sha256"`
	Spec         json.RawMessage `json:"spec"`
	Stale        bool            `json:"stale"`
}
type Baseline struct {
	ID                string          `json:"id"`
	ResultID          string          `json:"result_id"`
	ResultSHA256      string          `json:"result_sha256"`
	CaptureID         string          `json:"capture_id,omitempty"`
	Conditions        json.RawMessage `json:"conditions"`
	OriginalAvailable bool            `json:"original_available"`
}
type Manifest struct {
	SchemaVersion string          `json:"schema_version"`
	ID            string          `json:"id"`
	Revision      string          `json:"revision"`
	State         string          `json:"state"`
	Title         string          `json:"title"`
	Symptom       string          `json:"symptom"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	RetentionDays *string         `json:"retention_days"`
	Sources       []Source        `json:"sources"`
	Results       []ResultRef     `json:"results"`
	Captures      []Capture       `json:"captures"`
	Artifacts     []Artifact      `json:"artifacts"`
	Baselines     []Baseline      `json:"baselines"`
	Workspace     json.RawMessage `json:"workspace"`
}

func decodeStrict(body []byte, out any) error {
	if e := wirejson.Decode(body, new(any)); e != nil {
		return e
	}
	d := json.NewDecoder(strings.NewReader(string(body)))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(out)
}
func (m Manifest) validate() error {
	if m.SchemaVersion != Version {
		return problem("CASE_VERSION_UNSUPPORTED", "unsupported case manifest; original preserved")
	}
	if !validID(m.ID, "case") {
		return problem("CASE_INVALID", "invalid case identity")
	}
	if _, e := revision(m.Revision); e != nil {
		return e
	}
	if m.State != "active" && m.State != "archived" && m.State != "restored" && m.State != "deleted" {
		return problem("CASE_INVALID", "invalid lifecycle state")
	}
	if len(m.Title) > 4096 || len(m.Symptom) > 16<<10 || len(m.Sources) > MaxEntries || len(m.Results) > MaxEntries || len(m.Captures) > MaxEntries || len(m.Artifacts) > MaxEntries || len(m.Baselines) > MaxEntries {
		return problem("CASE_BUDGET_EXCEEDED", "manifest cardinality/string limit")
	}
	for _, t := range []string{m.CreatedAt, m.UpdatedAt} {
		if _, e := time.Parse(time.RFC3339Nano, t); e != nil {
			return problem("CASE_INVALID", "invalid case timestamp")
		}
	}
	if m.RetentionDays != nil {
		n, e := strconv.ParseUint(*m.RetentionDays, 10, 32)
		if e != nil || n == 0 || strconv.FormatUint(n, 10) != *m.RetentionDays {
			return problem("CASE_INVALID", "retention is a positive day count or null")
		}
	}
	if e := requireJSON(m.Workspace); e != nil {
		return e
	}
	seen := map[string]bool{}
	sourceIDs := map[string]bool{}
	var sourceBytes uint64
	results := map[string]string{}
	resultCaptures := map[string]string{}
	captures := map[string]bool{}
	unique := func(key, kind string) bool {
		if !validID(key, kind) || seen[key] {
			return false
		}
		seen[key] = true
		return true
	}
	for _, c := range m.Captures {
		if !unique(c.ID, "capture") {
			return problem("CASE_INVALID", "duplicate capture")
		}
		if e := c.Validate(); e != nil {
			return e
		}
		captures[c.ID] = true
	}
	for _, s := range m.Sources {
		if !unique(s.ID, "source") || !digestRE.MatchString(s.SHA256) || !validSize(s.Size) || !validSourceLabel(s.Name) || s.Status != "available" && s.Status != "missing" && s.Status != "failed" {
			return problem("CASE_INVALID", "invalid source reference")
		}
		if s.Storage != "external" && s.Storage != "managed" && s.Storage != "omitted" {
			return problem("CASE_INVALID", "invalid source storage")
		}
		if s.Storage != "external" && s.ExternalPath != "" {
			return problem("CASE_INVALID", "external path on managed source")
		}
		if s.CaptureID != "" && !captures[s.CaptureID] {
			return problem("CASE_INVALID", "unresolved capture")
		}
		if s.Storage == "external" && s.ExternalPath == "" {
			return problem("CASE_INVALID", "external source needs local path")
		}
		if s.ParentSHA256 != "" && !digestRE.MatchString(s.ParentSHA256) {
			return problem("CASE_INVALID", "invalid source parent digest")
		}
		if s.Encoding != nil {
			if encodingName(s.Encoding.Name) != s.Encoding.Name || !validSize(s.Encoding.Replacements) || !digestRE.MatchString(s.Encoding.IndexRevision) {
				return problem("CASE_INVALID", "invalid source encoding provenance")
			}
		}
		n, _ := strconv.ParseUint(s.Size, 10, 64)
		if n > MaxObjectBytes || sourceBytes+n > MaxCaseBytes {
			return problem("CASE_BUDGET_EXCEEDED", "declared source byte limit")
		}
		sourceBytes += n
		if s.Status == "failed" && s.Storage != "omitted" {
			return problem("CASE_INVALID", "failed intake must not publish a managed source")
		}
		sourceIDs[s.ID] = true
	}
	for _, r := range m.Results {
		if !unique(r.ID, "result") || !digestRE.MatchString(r.SHA256) || r.Type == "" || r.PayloadVersion == "" {
			return problem("CASE_INVALID", "invalid result reference")
		}
		if len(r.SourceIDs) > MaxEntries || len(r.SourceHashes) != len(r.SourceIDs) || len(r.Versions) > 32 {
			return problem("CASE_INVALID", "invalid source/version cardinality")
		}
		if e := requireJSON(r.Selection); e != nil {
			return e
		}
		if r.OptionsHash != "" && !digestRE.MatchString(r.OptionsHash) {
			return problem("CASE_INVALID", "invalid result option digest")
		}
		seenSources := map[string]bool{}
		for _, s := range r.SourceIDs {
			if seenSources[s] {
				return problem("CASE_INVALID", "duplicate result source")
			}
			seenSources[s] = true
			if !sourceIDs[s] || !digestRE.MatchString(r.SourceHashes[s]) {
				return problem("CASE_INVALID", "unresolved result source")
			}
		}
		if r.CaptureID != "" && !captures[r.CaptureID] {
			return problem("CASE_INVALID", "unresolved result capture")
		}
		if len(r.Type) > 256 || len(r.PayloadVersion) > 64 {
			return problem("CASE_INVALID", "result identity byte limit")
		}
		for k, v := range r.Versions {
			if len(k) > 128 || len(v) > 256 {
				return problem("CASE_INVALID", "version string byte limit")
			}
		}
		if r.Availability != "snapshot-only" && r.Availability != "originals-present-unverified" {
			return problem("CASE_INVALID", "invalid result availability")
		}
		results[r.ID] = r.SHA256
		resultCaptures[r.ID] = r.CaptureID
	}
	for _, a := range m.Artifacts {
		if e := requireJSON(a.Spec); e != nil {
			return e
		}
		if !unique(a.ID, "artifact") || results[a.ResultID] != a.ResultSHA256 || !digestRE.MatchString(a.ResultSHA256) {
			return problem("CASE_INVALID", "invalid artifact reference")
		}
	}
	for _, b := range m.Baselines {
		if e := requireJSON(b.Conditions); e != nil {
			return e
		}
		if b.CaptureID != "" && !captures[b.CaptureID] {
			return problem("CASE_INVALID", "unresolved baseline capture")
		}
		if !unique(b.ID, "baseline") || results[b.ResultID] != b.ResultSHA256 || resultCaptures[b.ResultID] != b.CaptureID {
			return problem("CASE_INVALID", "baseline must pin an immutable result")
		}
	}
	body, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if len(body) > MaxManifestBytes {
		return problem("CASE_BUDGET_EXCEEDED", "case manifest byte limit")
	}
	return nil
}
func validSize(s string) bool {
	n, e := strconv.ParseUint(s, 10, 64)
	return e == nil && strconv.FormatUint(n, 10) == s
}
func requireJSON(raw json.RawMessage) error {
	if len(raw) > 64<<10 {
		return problem("CASE_BUDGET_EXCEEDED", "context/selection byte limit")
	}
	if len(raw) == 0 {
		return nil
	}
	return wirejson.Decode(raw, new(any))
}
func currentRevision(n uint64) string { return strconv.FormatUint(n, 10) }
