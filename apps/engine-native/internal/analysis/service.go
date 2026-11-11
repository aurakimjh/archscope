// Package analysis orchestrates catalog import jobs and immutable result revisions.
package analysis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type Request struct {
	Path         string            `json:"path"`
	Importer     string            `json:"importer"`
	Options      json.RawMessage   `json:"options"`
	Budgets      ingestion.Budgets `json:"budgets"`
	AllowPartial bool              `json:"allowPartial"`
}
type JobProblem struct {
	Code      string `json:"code"`
	Stage     string `json:"stage"`
	Retryable bool   `json:"retryable"`
	SourceID  string `json:"sourceId"`
	Message   string `json:"message"`
}
type JobSnapshot struct {
	CleanupPending bool                   `json:"cleanupPending,omitempty"`
	ID             string                 `json:"id"`
	Generation     uint64                 `json:"generation"`
	State          string                 `json:"state"`
	Source         ingestion.SourceInfo   `json:"source"`
	Detection      ingestion.DetectionSet `json:"detection"`
	Importer       string                 `json:"importer"`
	OptionsHash    string                 `json:"optionsHash,omitempty"`
	PolicyRevision string                 `json:"policyRevision"`
	Budget         ingestion.Budgets      `json:"budget"`
	ResultHandle   string                 `json:"resultHandle,omitempty"`
	Problem        *JobProblem            `json:"problem,omitempty"`
}
type CancelOutcome struct {
	Outcome string      `json:"outcome"`
	Job     JobSnapshot `json:"job"`
}
type job struct {
	mu        sync.Mutex
	snapshot  JobSnapshot
	ctx       context.Context
	cancel    context.CancelFunc
	selection chan string
	done      chan struct{}
	selected  string
}
type cleanupRef struct {
	path string
	job  *job
}
type Service struct {
	slots   chan struct{}
	pending []cleanupRef
	mu      sync.Mutex
	root    string
	catalog *ingestion.Catalog
	results *resultcontract.Registry
	policy  ingestion.Budgets
	jobs    map[string]*job
	closed  bool
	wait    sync.WaitGroup
}

func New(root string, c *ingestion.Catalog, r *resultcontract.Registry) (*Service, error) {
	if c == nil || r == nil {
		return nil, fmt.Errorf("catalog and result registry required")
	}
	for _, name := range []string{"staging", "results"} {
		if e := os.MkdirAll(filepath.Join(root, name), 0700); e != nil {
			return nil, e
		}
	}
	return &Service{slots: make(chan struct{}, 4), root: root, catalog: c, results: r, policy: ingestion.DefaultBudgets(), jobs: map[string]*job{}}, nil
}
func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return prefix + hex.EncodeToString(b[:]), nil
}
func terminal(state string) bool {
	return state == "completed" || state == "partial" || state == "failed" || state == "cancelled"
}
func (s *Service) ListImporters() []ingestion.ImporterDescriptor { return s.catalog.List() }
func (s *Service) Start(req Request) (JobSnapshot, error) {
	if len(req.Options) > ingestion.MaxOptionBytes {
		return JobSnapshot{}, ingestion.Problem("IMPORT_OPTIONS_INVALID", "options exceed 64 KiB")
	}
	budget, e := req.Budgets.Within(s.policy)
	if e != nil {
		return JobSnapshot{}, e
	}
	id, e := randomID("job-")
	if e != nil {
		return JobSnapshot{}, e
	}
	req.Options = append(json.RawMessage(nil), req.Options...)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(budget.TimeoutMS)*time.Millisecond)
	j := &job{ctx: ctx, cancel: cancel, selection: make(chan string, 1), done: make(chan struct{}), snapshot: JobSnapshot{ID: id, Generation: 1, State: "queued", PolicyRevision: "import-policy-v1", Budget: budget, Source: ingestion.SourceInfo{ID: id + "-source"}, Detection: ingestion.DetectionSet{Candidates: []ingestion.Candidate{}}}}
	s.mu.Lock()
	if s.closed || len(s.jobs) >= 128 {
		s.mu.Unlock()
		cancel()
		return JobSnapshot{}, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "service job capacity reached")
	}
	s.jobs[id] = j
	s.wait.Add(1)
	s.mu.Unlock()
	initial := j.snapshot
	go func() { defer s.wait.Done(); defer close(j.done); defer cancel(); s.run(j, req) }()
	return initial, nil
}
func (s *Service) find(id string, generation uint64) (*job, error) {
	s.mu.Lock()
	j := s.jobs[id]
	s.mu.Unlock()
	if j == nil || generation != 1 {
		return nil, ingestion.Problem("IMPORT_JOB_STALE", "unknown job or stale generation")
	}
	return j, nil
}
func (j *job) copy() JobSnapshot {
	b, _ := json.Marshal(j.snapshot)
	var out JobSnapshot
	json.Unmarshal(b, &out)
	return out
}
func (s *Service) Get(id string, g uint64) (JobSnapshot, error) {
	j, e := s.find(id, g)
	if e != nil {
		return JobSnapshot{}, e
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.copy(), nil
}
func (s *Service) Select(id string, g uint64, importer string) (JobSnapshot, error) {
	j, e := s.find(id, g)
	if e != nil {
		return JobSnapshot{}, e
	}
	canonical, ok := s.catalog.Canonical(importer)
	if !ok {
		return JobSnapshot{}, ingestion.Problem("IMPORT_FORMAT_UNSUPPORTED", "unknown importer")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.selected == canonical {
		return j.copy(), nil
	}
	if j.snapshot.State != "needs_selection" || j.selected != "" {
		return JobSnapshot{}, ingestion.Problem("IMPORT_JOB_STALE", "selection requires a waiting job")
	}
	candidate := false
	for _, c := range j.snapshot.Detection.Candidates {
		candidate = candidate || c.ImporterID == canonical
	}
	if !candidate {
		return JobSnapshot{}, ingestion.Problem("IMPORT_FORMAT_MISMATCH", "select one of the detected candidates")
	}
	j.selected = canonical
	j.selection <- canonical
	return j.copy(), nil
}
func (s *Service) Cancel(id string, g uint64) (CancelOutcome, error) {
	j, e := s.find(id, g)
	if e != nil {
		return CancelOutcome{}, e
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.snapshot.State == "cancelled" {
		return CancelOutcome{"cancelled", j.copy()}, nil
	}
	if terminal(j.snapshot.State) {
		return CancelOutcome{"already_terminal", j.copy()}, nil
	}
	j.cancel()
	return CancelOutcome{"cancellation_requested", j.copy()}, nil
}
func (s *Service) Wait(ctx context.Context, id string, g uint64) (JobSnapshot, error) {
	j, e := s.find(id, g)
	if e != nil {
		return JobSnapshot{}, e
	}
	select {
	case <-ctx.Done():
		return JobSnapshot{}, ctx.Err()
	case <-j.done:
		return s.Get(id, g)
	}
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	for _, j := range s.jobs {
		j.cancel()
	}
	s.mu.Unlock()
	s.wait.Wait()
	s.mu.Lock()
	pending := s.pending
	s.mu.Unlock()
	for _, entry := range pending {
		if os.RemoveAll(entry.path) == nil {
			entry.job.mu.Lock()
			entry.job.snapshot.CleanupPending = false
			if entry.job.snapshot.Problem != nil && entry.job.snapshot.Problem.Code == "IMPORT_CLEANUP_PENDING" {
				entry.job.snapshot.Problem = nil
			}
			entry.job.mu.Unlock()
		}
	}
}
func (j *job) stage(state string) { j.mu.Lock(); j.snapshot.State = state; j.mu.Unlock() }
func (s *Service) run(j *job, req Request) {
	var failure error
	defer func() {
		if recover() != nil {
			failure = ingestion.Problem("IMPORT_FAILED", "compiled importer failed")
		}
		if failure == nil {
			return
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		if errors.Is(j.ctx.Err(), context.Canceled) {
			j.snapshot.State = "cancelled"
			j.snapshot.Problem = nil
			return
		}
		code := "IMPORT_FAILED"
		var problem *ingestion.ImportError
		if errors.As(failure, &problem) {
			code = problem.Code
		}
		if errors.Is(j.ctx.Err(), context.DeadlineExceeded) {
			code = "IMPORT_BUDGET_EXCEEDED"
		}
		stage := j.snapshot.State
		j.snapshot.State = "failed"
		j.snapshot.Problem = &JobProblem{code, stage, false, j.snapshot.Source.ID, "Import stopped; no result was published."}
	}()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-j.ctx.Done():
		failure = j.ctx.Err()
		return
	}
	directory, e := os.MkdirTemp(filepath.Join(s.root, "staging"), j.snapshot.ID+"-")
	if e != nil {
		failure = e
		return
	}
	defer s.cleanup(directory, j)
	j.stage("probing")
	lease, e := ingestion.Acquire(j.ctx, req.Path, directory, j.snapshot.Source.ID, j.snapshot.Budget)
	if e != nil {
		failure = e
		return
	}
	defer lease.Close()
	j.mu.Lock()
	j.snapshot.Source = lease.Info()
	j.mu.Unlock()
	f, e := lease.Open(j.ctx)
	if e != nil {
		failure = e
		return
	}
	head, e := io.ReadAll(io.LimitReader(f, int64(j.snapshot.Budget.ProbeBytes)))
	f.Close()
	if e != nil {
		failure = e
		return
	}
	detection, e := s.catalog.DetectCandidates(j.ctx, ingestion.Probe{Path: lease.Info().Name, Head: head, SizeBytes: lease.Info().Size, Extension: strings.ToLower(filepath.Ext(lease.Info().Name))})
	if e != nil {
		failure = e
		return
	}
	j.mu.Lock()
	j.snapshot.Detection = detection
	j.mu.Unlock()
	selected := strings.ToLower(strings.TrimSpace(req.Importer))
	if selected == "" || selected == "auto" {
		selected = detection.Selected
		if selected == "" {
			if detection.State == "unsupported" {
				failure = ingestion.Problem("IMPORT_FORMAT_UNSUPPORTED", "source not recognized")
				return
			}
			j.stage("needs_selection")
			select {
			case selected = <-j.selection:
			case <-j.ctx.Done():
				failure = j.ctx.Err()
				return
			}
		}
	}
	j.stage("validating")
	resolved, e := s.catalog.Resolve(selected, req.Options, j.snapshot.PolicyRevision)
	if e != nil {
		failure = e
		return
	}
	d := resolved.Descriptor
	verifiedOther, verifiedSelected := false, false
	for _, c := range detection.Candidates {
		if c.Grade == ingestion.Verified {
			if c.ImporterID == d.ID {
				verifiedSelected = true
			} else {
				verifiedOther = true
			}
		}
		if c.ImporterID == d.ID && c.ProducerVersion != "" {
			known := false
			for _, v := range d.VerifiedVersions {
				known = known || v == c.ProducerVersion
			}
			if !known {
				failure = ingestion.Problem("IMPORT_VERSION_UNSUPPORTED", "producer version is outside verified range")
				return
			}
		}
	}
	if verifiedOther && !verifiedSelected {
		failure = ingestion.Problem("IMPORT_FORMAT_MISMATCH", "selected importer conflicts with verified signature")
		return
	}
	if d.ConverterRequirement != "" {
		failure = ingestion.Problem("IMPORT_FORMAT_UNSUPPORTED", "approved local conversion is required")
		return
	}
	if d.NeedsSeek {
		failure = ingestion.Problem("IMPORT_FORMAT_UNSUPPORTED", "seek adapter is not available in this streaming path")
		return
	}
	if e = d.ValidateInput(j.ctx, lease); e != nil {
		failure = ingestion.Problem("IMPORT_FORMAT_MISMATCH", "selected source fails structural validation")
		return
	}
	j.mu.Lock()
	j.snapshot.Importer = d.ID
	j.snapshot.OptionsHash = resolved.OptionHash
	j.selected = d.ID
	j.mu.Unlock()
	recordPath := filepath.Join(directory, "normalized.ndjson")
	records, e := os.OpenFile(recordPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		failure = e
		return
	}
	defer records.Close()
	sink := &ingestion.BoundedSink{File: records, Family: d.NormalizerOutput, Schema: d.RecordSchema, Catalog: s.catalog, Budget: j.snapshot.Budget, TempUsed: lease.Info().Size}
	normalizer := ingestion.NormalizeSink{Family: d.RecordFamily, Output: d.NormalizerOutput, Normalize: d.Normalize, Next: sink}
	j.stage("parsing")
	parser := d.ParserFactory(append(json.RawMessage(nil), resolved.Options...))
	if parser == nil {
		failure = ingestion.Problem("IMPORT_REGISTRY_INVALID", "nil parser factory")
		return
	}
	report, parseErr := parser.Parse(j.ctx, lease, normalizer)
	if e = sink.Seal(); e != nil {
		failure = e
		return
	}
	partial := !report.Complete || parseErr != nil
	if partial {
		var problem *ingestion.ImportError
		safeError := parseErr == nil || (errors.As(parseErr, &problem) && problem.Code == "IMPORT_BUDGET_EXCEEDED")
		if !safeError || !req.AllowPartial || !d.SupportsPartial || !report.SafeBoundary || sink.Count == 0 || report.PartialReason == "" {
			failure = parseErr
			if failure == nil {
				failure = ingestion.Problem("IMPORT_PARTIAL_UNSAFE", "partial output is not permitted")
			}
			return
		}
	}
	if e = j.ctx.Err(); e != nil {
		failure = e
		return
	}
	if e = lease.Verify(j.ctx); e != nil {
		failure = e
		return
	}
	j.stage("normalizing")
	if e = records.Sync(); e != nil {
		failure = e
		return
	}
	j.stage("analyzing")
	result, e := resolved.Analyzer.Analyze(j.ctx, ingestion.RecordFile{Path: recordPath, RecordBytes: j.snapshot.Budget.RecordBytes}, lease.Info(), resolved.Options, report.Diagnostics)
	if e != nil {
		failure = e
		return
	}
	if result.Type != resolved.Analyzer.ResultType {
		failure = ingestion.Problem("IMPORT_RESULT_INVALID", "analyzer result type mismatch")
		return
	}
	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["import_provenance"] = map[string]any{"source_id": lease.Info().ID, "source_sha256": lease.Info().SHA256, "importer": d.ID, "adapter_version": d.AdapterVersion, "options_hash": resolved.OptionHash, "option_canonicalization": "options-v1", "policy_revision": resolved.PolicyRevision, "records": sink.Count, "partial": partial}
	if partial {
		result.Metadata.Extra["import_coverage"] = map[string]any{"partial": true, "reason": report.PartialReason, "complete_records": sink.Count}
	}
	j.stage("validating_result")
	body, e := json.Marshal(result)
	if e != nil {
		failure = e
		return
	}
	if int64(len(body)) > j.snapshot.Budget.DecodedBytes || int64(len(body))+lease.Info().Size+sink.Bytes > j.snapshot.Budget.TempBytes {
		failure = ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "result exceeds temp budget")
		return
	}
	decoded, e := resultcontract.Decode(body)
	if e != nil {
		failure = ingestion.Problem("IMPORT_RESULT_INVALID", "result envelope is invalid")
		return
	}
	if e = s.catalog.Validate(d.ResultSchema, body); e != nil {
		failure = ingestion.Problem("IMPORT_RESULT_INVALID", "result schema mismatch")
		return
	}
	if verdict := s.results.Check(decoded, "save"); !verdict.Allowed {
		failure = ingestion.Problem("IMPORT_RESULT_INVALID", verdict.Reason)
		return
	}
	j.stage("committing")
	handle, e := randomID("result-")
	if e != nil {
		failure = e
		return
	}
	staged := filepath.Join(directory, "result.json")
	if e = writeDurable(staged, body); e != nil {
		failure = e
		return
	}
	// Cancel and publish serialize on the same mutex. Cancellation after rename
	// observes an already-terminal job; earlier cancellation never publishes.
	j.mu.Lock()
	defer j.mu.Unlock()
	if e = j.ctx.Err(); e != nil {
		failure = e
		return
	}
	if e = os.Rename(staged, filepath.Join(s.root, "results", handle+".json")); e != nil {
		failure = e
		return
	}
	j.snapshot.ResultHandle = handle
	j.snapshot.State = "completed"
	if partial {
		j.snapshot.State = "partial"
	}
}
func writeDurable(path string, b []byte) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	return errors.Join(e, f.Close())
}

var handlePattern = regexp.MustCompile(`^result-[0-9a-f]{32}$`)

func (s *Service) Read(handle string) ([]byte, models.AnalysisResult, error) {
	if !handlePattern.MatchString(handle) {
		return nil, models.AnalysisResult{}, fmt.Errorf("invalid result handle")
	}
	f, e := os.Open(filepath.Join(s.root, "results", handle+".json"))
	if e != nil {
		return nil, models.AnalysisResult{}, e
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, s.policy.DecodedBytes+1))
	if e != nil {
		return nil, models.AnalysisResult{}, e
	}
	if int64(len(body)) > s.policy.DecodedBytes {
		return nil, models.AnalysisResult{}, fmt.Errorf("stored result exceeds read budget")
	}
	r, e := resultcontract.Decode(body)
	return body, r, e
}

func (s *Service) cleanup(path string, j *job) {
	if os.RemoveAll(path) == nil {
		return
	}
	j.mu.Lock()
	j.snapshot.CleanupPending = true
	if j.snapshot.State == "completed" || j.snapshot.State == "partial" {
		j.snapshot.Problem = &JobProblem{Code: "IMPORT_CLEANUP_PENDING", Stage: "cleanup", Retryable: true, SourceID: j.snapshot.Source.ID, Message: "Result published; owned temporary cleanup will retry on shutdown."}
	}
	j.mu.Unlock()
	s.mu.Lock()
	s.pending = append(s.pending, cleanupRef{path, j})
	s.mu.Unlock()
}
