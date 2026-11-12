// Package analysis orchestrates catalog import jobs and immutable result revisions.
package analysis

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type Request struct {
	PresetScope  string            `json:"preset_scope,omitempty"`
	Path         string            `json:"path"`
	Importer     string            `json:"importer"`
	Options      json.RawMessage   `json:"options"`
	Budgets      ingestion.Budgets `json:"budgets"`
	AllowPartial bool              `json:"allowPartial"`
}
type JobProblem struct {
	Detail    *ingestion.BudgetDetail `json:"detail,omitempty"`
	Code      string                  `json:"code"`
	Stage     string                  `json:"stage"`
	Retryable bool                    `json:"retryable"`
	SourceID  string                  `json:"sourceId"`
	Message   string                  `json:"message"`
}
type JobSnapshot struct {
	Packs             []ingestion.PackPin    `json:"packs,omitempty"`
	PackStateRevision string                 `json:"pack_state_revision,omitempty"`
	PackTrustSequence string                 `json:"pack_trust_sequence,omitempty"`
	CleanupPending    bool                   `json:"cleanupPending,omitempty"`
	ID                string                 `json:"id"`
	Generation        uint64                 `json:"generation"`
	State             string                 `json:"state"`
	Source            ingestion.SourceInfo   `json:"source"`
	Detection         ingestion.DetectionSet `json:"detection"`
	Importer          string                 `json:"importer"`
	OptionsHash       string                 `json:"optionsHash,omitempty"`
	PolicyRevision    string                 `json:"policyRevision"`
	Budget            ingestion.Budgets      `json:"budget"`
	ResultHandle      string                 `json:"resultHandle,omitempty"`
	Problem           *JobProblem            `json:"problem,omitempty"`
}
type CancelOutcome struct {
	Outcome string      `json:"outcome"`
	Job     JobSnapshot `json:"job"`
}
type job struct {
	catalog   *ingestion.Catalog
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
	baselineMu       sync.Mutex
	baselineCache    []baselineCacheEntry
	projections      map[string]*projectionEntry
	projectionCharge int64
	projectionClock  uint64
	catalogProvider  CatalogProvider
	storeMu          sync.Mutex
	stored           map[string]storedRevision
	storedBytes      int64
	leases           map[string]string
	lockFile         *os.File
	ctx              context.Context
	cancel           context.CancelFunc
	closeOnce        sync.Once
	slots            chan struct{}
	pending          []cleanupRef
	mu               sync.Mutex
	root             string
	catalog          *ingestion.Catalog
	results          *resultcontract.Registry
	policy           ingestion.Budgets
	jobs             map[string]*job
	history          []string
	closed           bool
	wait             sync.WaitGroup
}

func New(root string, c *ingestion.Catalog, r *resultcontract.Registry) (*Service, error) {
	if c == nil || r == nil {
		return nil, fmt.Errorf("catalog and result registry required")
	}
	for _, kind := range c.ResultTypes() {
		if !r.HasType(kind) {
			return nil, ingestion.Problem("IMPORT_REGISTRY_INVALID", "analyzer output has no registered result descriptor")
		}
	}
	for _, output := range c.OutputContracts() {
		if !r.SupportsPayload(output.Type, output.PayloadVersion) {
			return nil, ingestion.Problem("IMPORT_REGISTRY_INVALID", "result schema payload version has no compatible reader")
		}
	}
	for _, name := range []string{"staging", "results"} {
		if e := os.MkdirAll(filepath.Join(root, name), 0700); e != nil {
			return nil, e
		}
	}
	lockFile, e := os.OpenFile(filepath.Join(root, ".store.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = lockStore(lockFile); e != nil {
		lockFile.Close()
		return nil, ingestion.Problem("IMPORT_STORE_BUSY", "evidence store is already in use; close the desktop app or other cache owner before CLI maintenance")
	}
	// Reopened snapshots are session-only copies of durable saved case bytes.
	// No caller lease can survive the exclusive kernel lock across restarts.
	if e = os.RemoveAll(filepath.Join(root, "reopened")); e == nil {
		e = os.Mkdir(filepath.Join(root, "reopened"), 0700)
	}
	if e != nil {
		lockFile.Close()
		return nil, e
	}
	// Exclusive ownership makes crash-left staging safe to collect.
	entries, e := os.ReadDir(filepath.Join(root, "staging"))
	if e == nil {
		for _, entry := range entries {
			if e = os.RemoveAll(filepath.Join(root, "staging", entry.Name())); e != nil {
				break
			}
		}
	}
	if e != nil {
		lockFile.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := &Service{slots: make(chan struct{}, 4), root: root, catalog: c, results: r, policy: ingestion.DefaultBudgets(), jobs: map[string]*job{}, stored: map[string]storedRevision{}, leases: map[string]string{}, lockFile: lockFile, ctx: ctx, cancel: cancel}
	if e = service.initStore(); e != nil {
		cancel()
		lockFile.Close()
		return nil, e
	}
	return service, nil
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
	s.mu.Lock()
	provider := s.catalogProvider
	s.mu.Unlock()
	catalog := s.catalog
	pin := CatalogSnapshot{Catalog: catalog}
	if provider != nil {
		pin, e = provider(req.PresetScope)
		if e != nil {
			return JobSnapshot{}, e
		}
		if pin.Catalog == nil {
			return JobSnapshot{}, ingestion.Problem("IMPORT_REGISTRY_INVALID", "pack catalog snapshot is missing")
		}
		catalog = pin.Catalog
		pin.Packs = append([]ingestion.PackPin(nil), pin.Packs...)
	}
	id, e := randomID("job-")
	if e != nil {
		return JobSnapshot{}, e
	}
	req.Options = append(json.RawMessage(nil), req.Options...)
	ctx, cancelCause := context.WithCancelCause(context.Background())
	cancel := func() { cancelCause(context.Canceled) }
	queueTimer := time.AfterFunc(time.Duration(budget.QueueTimeoutMS)*time.Millisecond, func() { cancelCause(context.DeadlineExceeded) })
	j := &job{catalog: catalog, ctx: ctx, cancel: cancel, selection: make(chan string, 1), done: make(chan struct{}), snapshot: JobSnapshot{Packs: pin.Packs, PackStateRevision: pin.Revision, PackTrustSequence: pin.TrustSequence, ID: id, Generation: 1, State: "queued", PolicyRevision: "import-policy-v1", Budget: budget, Source: ingestion.SourceInfo{ID: id + "-source"}, Detection: ingestion.DetectionSet{Candidates: []ingestion.Candidate{}}}}
	s.mu.Lock()
	// Reclaim only finished history. Active/waiting jobs and immutable result
	// revisions are never removed to make room for another job.
	for len(s.jobs) >= 128 {
		evicted := false
		for i, old := range s.history {
			prior := s.jobs[old]
			select {
			case <-prior.done:
				delete(s.jobs, old)
				s.history = append(s.history[:i], s.history[i+1:]...)
				evicted = true
			default:
			}
			if evicted {
				break
			}
		}
		if !evicted {
			break
		}
	}
	if s.closed || len(s.jobs) >= 128 {
		s.mu.Unlock()
		cancel()
		queueTimer.Stop()
		return JobSnapshot{}, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "service job capacity reached")
	}
	s.jobs[id] = j
	s.history = append(s.history, id)
	s.wait.Add(1)
	s.mu.Unlock()
	initial := j.copy()
	go func() {
		defer s.wait.Done()
		defer close(j.done)
		defer cancel()
		s.run(j, req, queueTimer, cancelCause)
	}()
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
	canonical, ok := j.catalog.Canonical(importer)
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
		j.mu.Lock()
		defer j.mu.Unlock()
		return j.copy(), nil
	}
}
func (s *Service) Close() { s.closeOnce.Do(s.close) }
func (s *Service) close() {
	s.cancel()
	s.mu.Lock()
	s.closed = true
	for _, j := range s.jobs {
		j.cancel()
	}
	s.mu.Unlock()
	s.wait.Wait()
	s.baselineMu.Lock()
	s.baselineCache = nil
	s.baselineMu.Unlock()
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
	s.storeMu.Lock()
	s.projections = nil
	s.projectionCharge = 0
	_ = s.lockFile.Close()
	s.storeMu.Unlock()
}
func (j *job) stage(state string) { j.mu.Lock(); j.snapshot.State = state; j.mu.Unlock() }
func (s *Service) run(j *job, req Request, queueTimer *time.Timer, cancelCause context.CancelCauseFunc) {
	defer queueTimer.Stop()
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
		if errors.Is(context.Cause(j.ctx), context.Canceled) {
			j.snapshot.State = "cancelled"
			j.snapshot.Problem = nil
			return
		}
		code := "IMPORT_FAILED"
		var problem *ingestion.ImportError
		if errors.As(failure, &problem) {
			code = problem.Code
		}
		var pathError *os.PathError
		if errors.As(failure, &pathError) || errors.Is(failure, io.ErrUnexpectedEOF) {
			code = "IMPORT_IO_FAILED"
		}
		if errors.Is(context.Cause(j.ctx), context.DeadlineExceeded) {
			code = "IMPORT_BUDGET_EXCEEDED"
			if j.snapshot.State == "queued" {
				code = "IMPORT_QUEUE_TIMEOUT"
			}
		}
		stage := j.snapshot.State
		j.snapshot.State = "failed"
		j.snapshot.Problem = &JobProblem{Code: code, Stage: stage, Retryable: retryable(code), SourceID: j.snapshot.Source.ID, Message: "Import stopped; no result was published."}
		if problem != nil && problem.Detail != nil && context.Cause(j.ctx) == nil {
			j.snapshot.Problem.Detail = problem.Detail
			j.snapshot.Problem.Message = problem.Message
		}
	}()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-j.ctx.Done():
		failure = j.ctx.Err()
		return
	}
	queueTimer.Stop()
	if e := j.ctx.Err(); e != nil {
		failure = e
		return
	}
	runtimeTimer := time.AfterFunc(time.Duration(j.snapshot.Budget.TimeoutMS)*time.Millisecond, func() { cancelCause(context.DeadlineExceeded) })
	defer runtimeTimer.Stop()
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
	detection, e := j.catalog.DetectCandidates(j.ctx, ingestion.Probe{Path: lease.Info().Name, Head: head, SizeBytes: lease.Info().Size, Extension: strings.ToLower(filepath.Ext(lease.Info().Name))})
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
			if detection.State == "detection_failed" {
				failure = ingestion.Problem("IMPORT_DETECTOR_INVALID", "no usable detection result")
				return
			}
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
	resolved, e := j.catalog.Resolve(selected, req.Options, j.snapshot.PolicyRevision)
	if e != nil {
		failure = e
		return
	}
	d := resolved.Descriptor
	if d.Pack != nil {
		identity, _ := json.Marshal(struct {
			Options string
			Pack    *ingestion.PackPin
		}{resolved.OptionHash, d.Pack})
		digest := sha256.Sum256(identity)
		resolved.OptionHash = hex.EncodeToString(digest[:])
	}

	for _, diagnostic := range detection.Diagnostics {
		if diagnostic.ImporterID == d.ID {
			failure = ingestion.Problem(diagnostic.Code, "selected detector failed")
			return
		}
	}
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
		var problem *ingestion.ImportError
		if errors.As(e, &problem) || j.ctx.Err() != nil {
			failure = e
		} else {
			failure = ingestion.Problem("IMPORT_IO_FAILED", "source validation read failed")
		}
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
	sink := &ingestion.BoundedSink{File: records, Family: d.NormalizerOutput, Schema: d.RecordSchema, Catalog: j.catalog, Budget: j.snapshot.Budget, TempUsed: lease.Info().Size}
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
	if len(j.snapshot.Packs) > 0 {
		result.Metadata.Extra["pack_snapshot"] = map[string]any{"state_revision": j.snapshot.PackStateRevision, "trust_sequence": j.snapshot.PackTrustSequence, "packs": j.snapshot.Packs}
	}
	if d.Pack != nil {
		result.Metadata.Extra["effective_pack"] = d.Pack
	}
	result.Metadata.Extra["import_provenance"] = map[string]any{"source_id": lease.Info().ID, "source_sha256": lease.Info().SHA256, "importer": d.ID, "adapter_version": d.AdapterVersion, "options_hash": resolved.OptionHash, "option_canonicalization": "options-v1", "policy_revision": resolved.PolicyRevision, "records": sink.Count, "partial": partial}
	if resolved.Analyzer.BindSelection != nil {
		if e = resolved.Analyzer.BindSelection(&result, lease.Info(), ingestion.AnalysisSelection{ImporterID: d.ID, AdapterVersion: d.AdapterVersion, OptionsHash: resolved.OptionHash, Records: sink.Count, Partial: partial, PartialReason: report.PartialReason, SelectionReason: report.SelectionReason}); e != nil {
			failure = ingestion.Problem("IMPORT_RESULT_INVALID", "analysis selection binding failed")
			return
		}
	}
	if report.SelectionReason != "" {
		result.Metadata.Extra["import_coverage"] = map[string]any{"partial": false, "selection": report.SelectionReason, "complete_records": sink.Count}
	}
	if partial {
		result.Metadata.Extra["import_coverage"] = map[string]any{"partial": true, "reason": report.PartialReason, "complete_records": sink.Count}
	}
	j.stage("validating_result")
	body, e := json.Marshal(result)
	if e != nil {
		failure = e
		return
	}
	if int64(len(body)) > j.snapshot.Budget.DecodedBytes {
		failure = ingestion.BudgetProblem("result_bytes", "result_bytes", int64(len(body)), j.snapshot.Budget.DecodedBytes)
		return
	}
	if int64(len(body))+lease.Info().Size+sink.Bytes > j.snapshot.Budget.TempBytes {
		failure = ingestion.BudgetProblem("temporary_bytes", "temporary_bytes", int64(len(body))+lease.Info().Size+sink.Bytes, j.snapshot.Budget.TempBytes)
		return
	}
	decoded, e := resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(s.policy.DecodedBytes, s.policy.StackDepth))
	if e != nil {
		failure = ingestion.Problem("IMPORT_RESULT_INVALID", "result envelope is invalid")
		return
	}
	if e = j.catalog.Validate(d.ResultSchema, body); e != nil {
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
	if handle, e = s.publish(j.ctx, staged, handle, body); e != nil {
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
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if _, ok := s.stored[handle]; !ok {
		return nil, models.AnalysisResult{}, ingestion.Problem("IMPORT_RESULT_STALE", "reopen a saved case or result file")
	}
	f, e := os.Open(s.resultPathLocked(handle))
	if e != nil {
		if os.IsNotExist(e) {
			e = ingestion.Problem("IMPORT_RESULT_STALE", "result is no longer available; reopen the saved file")
		}
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
	sum := sha256.Sum256(body)
	rev, ok := s.stored[handle]
	if !ok || rev.hash != hex.EncodeToString(sum[:]) {
		return nil, models.AnalysisResult{}, ingestion.Problem("IMPORT_RESULT_CORRUPT", "stored revision changed; reopen the saved case snapshot")
	}
	r, e := resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(s.policy.DecodedBytes, s.policy.StackDepth))
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

// A retry may succeed for transient I/O, detector scheduling, queue pressure or
// cleanup. Invalid contracts/input and exhausted input budgets need a changed
// request, so repeating them is not advertised as retryable.
func retryable(code string) bool {
	switch code {
	case "IMPORT_IO_FAILED", "IMPORT_DETECTOR_TIMEOUT", "IMPORT_QUEUE_TIMEOUT", "IMPORT_CLEANUP_PENDING":
		return true
	}
	return false
}
