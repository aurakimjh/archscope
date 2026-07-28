// Package store implements the append-only, recoverable live-capture session
// store. Transaction bodies are separate blobs and transaction metadata is
// cursor-paged from NDJSON without changing the AnalysisResult envelope.
package store

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	DefaultMaxSessionBytes = int64(2 << 30)
	DefaultReserveBytes    = int64(512 << 20)
	DefaultFetchLimit      = 500
	MaxFetchLimit          = 2000
	transactionsFile       = "transactions.ndjson"
	offsetIndexFile        = "transactions.idx"
	manifestFile           = "manifest.json"
	blobsDir               = "blobs"
)

var (
	ErrSessionLimit = errors.New("capture session size limit reached")
	ErrDiskReserve  = errors.New("capture disk reserve exhausted")
	ErrStaleCursor  = errors.New("capture cursor does not belong to this session")
)

type Config struct {
	Root           string
	SessionID      string
	MaxBytes       int64
	ReserveBytes   int64
	BodyFraction   float64
	OverflowPolicy capture.OverflowPolicy
	FlushInterval  time.Duration
	FlushBytes     int64
	FreeBytes      func(string) (uint64, error)
}

type Counters struct {
	Persisted   uint64 `json:"persisted"`
	BodyOmitted uint64 `json:"bodyOmitted"`
	Rotated     uint64 `json:"rotated"`
}

type Finding struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type Manifest struct {
	SchemaVersion   int                    `json:"schemaVersion"`
	SessionID       string                 `json:"sessionId"`
	State           capture.SessionState   `json:"state"`
	StartedAt       time.Time              `json:"startedAt"`
	EndedAt         *time.Time             `json:"endedAt,omitempty"`
	SnapshotVersion uint64                 `json:"snapshotVersion"`
	StoreBytes      int64                  `json:"storeBytes"`
	OverflowPolicy  capture.OverflowPolicy `json:"overflowPolicy"`
	Counters        Counters               `json:"counters"`
	CaptureStats    *capture.Stats         `json:"captureStats,omitempty"`
	Findings        []Finding              `json:"findings"`
}

type StoredTransaction struct {
	Seq         uint64                    `json:"seq"`
	Transaction models.CaptureTransaction `json:"transaction"`
}

type Filter struct {
	Host       string `json:"host,omitempty"`
	Method     string `json:"method,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	ProcessID  int32  `json:"processId,omitempty"`
	ErrorsOnly bool   `json:"errorsOnly,omitempty"`
}

type Page struct {
	Items           []StoredTransaction `json:"items"`
	NextCursor      string              `json:"nextCursor,omitempty"`
	SnapshotVersion uint64              `json:"snapshotVersion"`
	Total           int64               `json:"total"`
}

type RecoveryReport struct {
	Recovered      bool  `json:"recovered"`
	DiscardedBytes int64 `json:"discardedBytes"`
	Records        int   `json:"records"`
}

type cursor struct {
	SessionID string `json:"s"`
	Snapshot  uint64 `json:"v"`
	LastSeq   uint64 `json:"q"`
}

type Store struct {
	mu               sync.RWMutex
	cfg              Config
	dir              string
	file             *os.File
	writer           *bufio.Writer
	index            *os.File
	indexWriter      *bufio.Writer
	manifest         Manifest
	lastFlush        time.Time
	pending          int64
	transactionBytes int64
	bodyBytes        int64
	closed           bool
}

func New(cfg Config) (*Store, error) {
	normalizeConfig(&cfg)
	if strings.TrimSpace(cfg.SessionID) == "" ||
		filepath.Base(cfg.SessionID) != cfg.SessionID ||
		strings.Contains(cfg.SessionID, "..") {
		return nil, errors.New("valid session ID is required")
	}
	if err := os.MkdirAll(cfg.Root, 0o700); err != nil {
		return nil, err
	}
	dir := filepath.Join(cfg.Root, cfg.SessionID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(dir, blobsDir), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, transactionsFile), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	index, err := os.OpenFile(filepath.Join(dir, offsetIndexFile), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		f.Close()
		return nil, err
	}
	now := time.Now().UTC()
	s := &Store{
		cfg: cfg, dir: dir, file: f, writer: bufio.NewWriterSize(f, 64<<10),
		index: index, indexWriter: bufio.NewWriterSize(index, 64<<10), lastFlush: now,
		manifest: Manifest{
			SchemaVersion: capture.LiveSchemaVersion, SessionID: cfg.SessionID,
			State: capture.StateCreated, StartedAt: now, OverflowPolicy: cfg.OverflowPolicy,
			Findings: []Finding{},
		},
	}
	if err := s.writeManifestLocked(); err != nil {
		f.Close()
		index.Close()
		return nil, err
	}
	return s, nil
}

func Open(dir string) (*Store, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode capture manifest: %w", err)
	}
	cfg := Config{Root: filepath.Dir(dir), SessionID: manifest.SessionID, OverflowPolicy: manifest.OverflowPolicy}
	normalizeConfig(&cfg)
	f, err := os.OpenFile(filepath.Join(dir, transactionsFile), os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	index, err := os.OpenFile(filepath.Join(dir, offsetIndexFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		f.Close()
		return nil, err
	}
	s := &Store{
		cfg: cfg, dir: dir, file: f, writer: bufio.NewWriterSize(f, 64<<10),
		index: index, indexWriter: bufio.NewWriterSize(index, 64<<10),
		manifest: manifest, lastFlush: time.Now().UTC(),
	}
	if _, err := s.Recover(); err != nil {
		f.Close()
		index.Close()
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		index.Close()
		return nil, err
	}
	if _, err := index.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		index.Close()
		return nil, err
	}
	return s, nil
}

// OpenReadOnly opens a capture store without recovery or lifecycle writes.
// Callers must tolerate an incomplete tail themselves or, as acceptance
// evidence does, restrict use to a stopped session.
func OpenReadOnly(dir string) (*Store, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode capture manifest: %w", err)
	}
	cfg := Config{Root: filepath.Dir(dir), SessionID: manifest.SessionID, OverflowPolicy: manifest.OverflowPolicy}
	normalizeConfig(&cfg)
	f, err := os.Open(filepath.Join(dir, transactionsFile))
	if err != nil {
		return nil, err
	}
	index, err := os.Open(filepath.Join(dir, offsetIndexFile))
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Store{
		cfg: cfg, dir: dir, file: f, writer: bufio.NewWriterSize(f, 64<<10),
		index: index, indexWriter: bufio.NewWriterSize(index, 64<<10),
		manifest: manifest, lastFlush: time.Now().UTC(),
	}, nil
}

func normalizeConfig(cfg *Config) {
	if cfg.Root == "" {
		cfg.Root = filepath.Join(os.TempDir(), "archscope-captures")
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultMaxSessionBytes
	}
	if cfg.ReserveBytes == 0 {
		cfg.ReserveBytes = DefaultReserveBytes
	}
	if cfg.ReserveBytes < 0 {
		cfg.ReserveBytes = 0
	}
	if cfg.BodyFraction <= 0 || cfg.BodyFraction > 0.9 {
		cfg.BodyFraction = 0.7
	}
	if cfg.OverflowPolicy == "" {
		cfg.OverflowPolicy = capture.OverflowStop
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.FlushBytes <= 0 {
		cfg.FlushBytes = 4 << 20
	}
	if cfg.FreeBytes == nil {
		cfg.FreeBytes = diskFreeBytes
	}
}

func (s *Store) Path() string { return s.dir }

func (s *Store) SetState(state capture.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manifest.State = state
	return s.writeManifestLocked()
}

func (s *Store) SetCaptureStats(stats capture.Stats) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := stats
	s.manifest.CaptureStats = &next
	return s.writeManifestLocked()
}

func (s *Store) AddFinding(finding Finding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manifest.Findings = appendFindingOnce(s.manifest.Findings, finding)
	return s.writeManifestLocked()
}

func (s *Store) Append(tx models.CaptureTransaction) (StoredTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return StoredTransaction{}, errors.New("capture store is closed")
	}
	next := StoredTransaction{Seq: s.manifest.SnapshotVersion + 1, Transaction: tx}
	encoded, err := json.Marshal(next)
	if err != nil {
		return StoredTransaction{}, err
	}
	encoded = append(encoded, '\n')
	if err := s.applyLimitsLocked(&next, &encoded); err != nil {
		return StoredTransaction{}, err
	}
	n, err := s.writer.Write(encoded)
	if err != nil {
		return StoredTransaction{}, err
	}
	var encodedOffset [8]byte
	binary.LittleEndian.PutUint64(encodedOffset[:], uint64(s.transactionBytes))
	if _, err := s.indexWriter.Write(encodedOffset[:]); err != nil {
		return StoredTransaction{}, err
	}
	s.pending += int64(n)
	s.transactionBytes += int64(n)
	s.manifest.StoreBytes += int64(n)
	s.manifest.SnapshotVersion = next.Seq
	s.manifest.Counters.Persisted++
	if s.pending >= s.cfg.FlushBytes || time.Since(s.lastFlush) >= s.cfg.FlushInterval {
		if err := s.flushLocked(); err != nil {
			return StoredTransaction{}, err
		}
	}
	return next, nil
}

func (s *Store) applyLimitsLocked(next *StoredTransaction, encoded *[]byte) error {
	if s.cfg.ReserveBytes > 0 {
		free, err := s.cfg.FreeBytes(s.dir)
		if err == nil && free < uint64(s.cfg.ReserveBytes) {
			return ErrDiskReserve
		}
	}
	if s.manifest.StoreBytes+int64(len(*encoded)) <= s.cfg.MaxBytes {
		return nil
	}
	switch s.cfg.OverflowPolicy {
	case capture.OverflowBodyOnly:
		if omitBodies(&next.Transaction) {
			s.manifest.Counters.BodyOmitted++
			s.manifest.Findings = appendFindingOnce(s.manifest.Findings, Finding{
				Code: "CAPTURE_BODY_OMITTED", Message: "session limit activated explicit body_only degradation",
			})
			reencoded, err := json.Marshal(next)
			if err != nil {
				return err
			}
			*encoded = append(reencoded, '\n')
			if s.manifest.StoreBytes+int64(len(*encoded)) <= s.cfg.MaxBytes {
				return nil
			}
		}
		return ErrSessionLimit
	case capture.OverflowRotate:
		return errors.New("rotate policy requires segmented retention and is unavailable in schema v1")
	default:
		return ErrSessionLimit
	}
}

func omitBodies(tx *models.CaptureTransaction) bool {
	changed := false
	for _, msg := range []*models.HTTPMessage{&tx.Request, &tx.Response} {
		if msg.BodyPreview != "" || msg.BodyRef != "" || msg.BodyStorage == "inline" || msg.BodyStorage == "blob" {
			msg.BodyPreview = ""
			msg.BodyRef = ""
			msg.BodyStorage = "omitted"
			changed = true
		}
	}
	return changed
}

func (s *Store) PutBody(data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("capture store is closed")
	}
	if s.cfg.ReserveBytes > 0 {
		free, err := s.cfg.FreeBytes(s.dir)
		if err == nil && free < uint64(s.cfg.ReserveBytes) {
			return "", ErrDiskReserve
		}
	}
	if int64(len(data))+s.bodyBytes > int64(float64(s.cfg.MaxBytes)*s.cfg.BodyFraction) {
		s.manifest.Counters.BodyOmitted++
		return "", ErrSessionLimit
	}
	if int64(len(data))+s.manifest.StoreBytes > s.cfg.MaxBytes {
		s.manifest.Counters.BodyOmitted++
		return "", ErrSessionLimit
	}
	ref := fmt.Sprintf("%016x.blob", s.bodyBytes+1)
	path := filepath.Join(s.dir, blobsDir, ref)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	s.bodyBytes += int64(len(data))
	s.manifest.StoreBytes += int64(len(data))
	if err := s.writeManifestLocked(); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *Store) Body(ref string) (io.ReadCloser, error) {
	if filepath.Base(ref) != ref || strings.Contains(ref, "..") {
		return nil, errors.New("invalid body reference")
	}
	return os.Open(filepath.Join(s.dir, blobsDir, ref))
}

func (s *Store) Fetch(filter Filter, opaque string, limit int) (Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	if limit > MaxFetchLimit {
		limit = MaxFetchLimit
	}
	cur := cursor{SessionID: s.manifest.SessionID, Snapshot: s.manifest.SnapshotVersion}
	if opaque != "" {
		decoded, err := decodeCursor(opaque)
		if err != nil {
			return Page{}, err
		}
		if decoded.SessionID != s.manifest.SessionID {
			return Page{}, ErrStaleCursor
		}
		if decoded.Snapshot > s.manifest.SnapshotVersion || decoded.LastSeq > decoded.Snapshot {
			return Page{}, errors.New("invalid capture cursor")
		}
		cur = decoded
	}
	if err := s.writer.Flush(); err != nil {
		return Page{}, err
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return Page{}, err
	}
	scanner := bufio.NewScanner(s.file)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	items := make([]StoredTransaction, 0, limit)
	var total int64
	var last uint64
	more := false
	for scanner.Scan() {
		var item StoredTransaction
		if json.Unmarshal(scanner.Bytes(), &item) != nil || item.Seq > cur.Snapshot || !matches(item.Transaction, filter) {
			continue
		}
		total++
		if item.Seq <= cur.LastSeq {
			continue
		}
		if len(items) >= limit {
			more = true
			continue
		}
		items = append(items, item)
		last = item.Seq
	}
	if err := scanner.Err(); err != nil {
		return Page{}, err
	}
	page := Page{Items: items, SnapshotVersion: cur.Snapshot, Total: total}
	if more && last > 0 {
		page.NextCursor = encodeCursor(cursor{SessionID: cur.SessionID, Snapshot: cur.Snapshot, LastSeq: last})
	}
	return page, nil
}

func matches(tx models.CaptureTransaction, f Filter) bool {
	if f.Host != "" && !strings.EqualFold(tx.Host, f.Host) {
		return false
	}
	if f.Method != "" && !strings.EqualFold(tx.Method, f.Method) {
		return false
	}
	if f.StatusCode != 0 && tx.StatusCode != f.StatusCode {
		return false
	}
	if f.ProcessID != 0 && (tx.Process == nil || tx.Process.Key.PID != f.ProcessID) {
		return false
	}
	if f.ErrorsOnly && tx.StatusCode < 400 && tx.State != models.TxFailed && tx.State != models.TxAborted {
		return false
	}
	return true
}

func (s *Store) Meta() Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manifest
}

func (s *Store) LoadAll() ([]models.CaptureTransaction, error) {
	page, err := s.Fetch(Filter{}, "", MaxFetchLimit)
	if err != nil {
		return nil, err
	}
	out := make([]models.CaptureTransaction, 0, page.Total)
	for {
		for _, item := range page.Items {
			out = append(out, item.Transaction)
		}
		if page.NextCursor == "" {
			break
		}
		page, err = s.Fetch(Filter{}, page.NextCursor, MaxFetchLimit)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Close releases an opened store without changing its lifecycle state. It is
// intended for read-only access to a finalized or recovered session.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := s.writer.Flush(); err != nil {
		return err
	}
	if err := s.indexWriter.Flush(); err != nil {
		return err
	}
	s.closed = true
	return errors.Join(s.file.Close(), s.index.Close())
}

func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
	if err := s.writer.Flush(); err != nil {
		return err
	}
	if err := s.indexWriter.Flush(); err != nil {
		return err
	}
	if err := s.file.Sync(); err != nil {
		return err
	}
	if err := s.index.Sync(); err != nil {
		return err
	}
	s.pending = 0
	s.lastFlush = time.Now().UTC()
	return s.writeManifestLocked()
}

func (s *Store) Finalize(state capture.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := s.flushLocked(); err != nil {
		return err
	}
	now := time.Now().UTC()
	s.manifest.State = state
	s.manifest.EndedAt = &now
	if err := s.writeManifestLocked(); err != nil {
		return err
	}
	s.closed = true
	return errors.Join(s.file.Close(), s.index.Close())
}

func (s *Store) Recover() (RecoveryReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writer.Flush(); err != nil {
		return RecoveryReport{}, err
	}
	if err := s.indexWriter.Flush(); err != nil {
		return RecoveryReport{}, err
	}
	stat, err := s.file.Stat()
	if err != nil {
		return RecoveryReport{}, err
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return RecoveryReport{}, err
	}
	if err := s.index.Truncate(0); err != nil {
		return RecoveryReport{}, err
	}
	if _, err := s.index.Seek(0, io.SeekStart); err != nil {
		return RecoveryReport{}, err
	}
	s.indexWriter.Reset(s.index)
	var validEnd int64
	var records int
	var lastSeq uint64
	var offset int64
	scanner := bufio.NewScanner(s.file)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		nextOffset := offset + int64(len(line)) + 1
		if nextOffset > stat.Size() {
			break
		}
		var item StoredTransaction
		if len(line) == 0 || json.Unmarshal(line, &item) != nil {
			break
		}
		var encodedOffset [8]byte
		binary.LittleEndian.PutUint64(encodedOffset[:], uint64(offset))
		if _, err := s.indexWriter.Write(encodedOffset[:]); err != nil {
			return RecoveryReport{}, err
		}
		records++
		lastSeq = item.Seq
		validEnd = nextOffset
		offset = nextOffset
	}
	if err := scanner.Err(); err != nil {
		return RecoveryReport{}, err
	}
	if err := s.indexWriter.Flush(); err != nil {
		return RecoveryReport{}, err
	}
	discarded := stat.Size() - validEnd
	if discarded > 0 {
		if err := s.file.Truncate(validEnd); err != nil {
			return RecoveryReport{}, err
		}
		s.manifest.Findings = append(s.manifest.Findings, Finding{
			Code: "CAPTURE_RECOVERED_TRUNCATED_TAIL", Message: "discarded an incomplete NDJSON tail",
			Evidence: map[string]any{"discarded_bytes": discarded},
		})
	}
	if _, err := s.file.Seek(validEnd, io.SeekStart); err != nil {
		return RecoveryReport{}, err
	}
	s.writer.Reset(s.file)
	if _, err := s.index.Seek(0, io.SeekEnd); err != nil {
		return RecoveryReport{}, err
	}
	s.indexWriter.Reset(s.index)
	bodyBytes, err := directoryBytes(filepath.Join(s.dir, blobsDir))
	if err != nil {
		return RecoveryReport{}, err
	}
	s.transactionBytes = validEnd
	s.bodyBytes = bodyBytes
	s.manifest.StoreBytes = validEnd + bodyBytes
	s.manifest.SnapshotVersion = lastSeq
	s.manifest.Counters.Persisted = uint64(records)
	terminal := s.manifest.State == capture.StateFinalized || s.manifest.State == capture.StateFailed
	recovered := discarded > 0 || !terminal
	if recovered {
		s.manifest.State = capture.StateRecoverable
	}
	if err := s.writeManifestLocked(); err != nil {
		return RecoveryReport{}, err
	}
	return RecoveryReport{Recovered: recovered, DiscardedBytes: discarded, Records: records}, nil
}

func directoryBytes(path string) (int64, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		total += info.Size()
	}
	return total, nil
}

func (s *Store) writeManifestLocked() error {
	data, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, manifestFile+".tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, manifestFile))
}

func appendFindingOnce(items []Finding, next Finding) []Finding {
	for _, item := range items {
		if item.Code == next.Code {
			return items
		}
	}
	return append(items, next)
}

func encodeCursor(c cursor) string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(value string) (cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor{}, errors.New("invalid capture cursor")
	}
	var cur cursor
	if err := json.Unmarshal(data, &cur); err != nil || cur.SessionID == "" {
		return cursor{}, errors.New("invalid capture cursor")
	}
	return cur, nil
}

func SortedSessionDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	out := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := os.Stat(filepath.Join(root, entry.Name(), manifestFile)); err == nil {
				out = append(out, filepath.Join(root, entry.Name()))
			}
		}
	}
	sort.Strings(out)
	return out, nil
}
