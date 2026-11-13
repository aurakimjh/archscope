package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const rollingDirectory = "rolling"

type rollingRecord struct {
	Seq   uint64
	Bytes int64
	Refs  []string
}

func recordName(seq uint64) string { return fmt.Sprintf("%020d.json", seq) }
func bodyRefs(tx models.CaptureTransaction) ([]string, error) {
	refs := []string{}
	seen := map[string]bool{}
	for _, ref := range []string{tx.Request.BodyRef, tx.Response.BodyRef} {
		if ref == "" {
			continue
		}
		if filepath.Base(ref) != ref || strings.Contains(ref, "..") {
			return nil, fmt.Errorf("invalid retained body reference")
		}
		if !seen[ref] {
			refs = append(refs, ref)
			seen[ref] = true
		}
	}
	return refs, nil
}
func durableReplace(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), "pending-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return renameCaptureFile(name, path)
}
func (s *Store) rollingRowsLocked() ([]rollingRecord, error) {
	entries, e := os.ReadDir(filepath.Join(s.dir, rollingDirectory))
	if e != nil {
		return nil, e
	}
	if len(entries) > s.cfg.MaxRecords+1024 {
		return nil, fmt.Errorf("rolling directory exceeds retention bound")
	}
	rows := []rollingRecord{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "pending-") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil, fmt.Errorf("invalid rolling entry")
		}
		seq, e := strconv.ParseUint(strings.TrimSuffix(entry.Name(), ".json"), 10, 64)
		if e != nil || recordName(seq) != entry.Name() {
			return nil, fmt.Errorf("unknown rolling entry")
		}
		if seq < s.manifest.RetentionFloor || seq > s.manifest.SnapshotVersion {
			continue
		}
		item, size, e := readRollingItem(filepath.Join(s.dir, rollingDirectory, entry.Name()), seq)
		if e != nil {
			return nil, e
		}
		refs, e := bodyRefs(item.Transaction)
		if e != nil {
			return nil, e
		}
		rows = append(rows, rollingRecord{seq, size, refs})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })
	if len(rows) > s.cfg.MaxRecords || uint64(len(rows)) != s.manifest.RetainedRecords {
		return nil, fmt.Errorf("committed rolling row count mismatch")
	}
	return rows, nil
}
func (s *Store) appendRollingLocked(tx models.CaptureTransaction, checkpoint *CaptureCheckpoint) (StoredTransaction, error) {
	if s.rollingCleanupNeeded {
		if _, e := s.recoverRollingLocked(); e != nil {
			return StoredTransaction{}, e
		}
		s.rollingCleanupNeeded = false
	}
	next := StoredTransaction{Seq: s.manifest.SnapshotVersion + 1, Transaction: tx}
	refs, e := bodyRefs(tx)
	if e != nil {
		return StoredTransaction{}, e
	}
	for _, ref := range refs {
		if _, e = os.Stat(filepath.Join(s.dir, blobsDir, ref)); e != nil {
			return StoredTransaction{}, e
		}
	}
	encoded, e := json.Marshal(next)
	if e != nil {
		return StoredTransaction{}, e
	}
	encoded = append(encoded, '\n')
	var evicted *rollingRecord
	if len(s.rolling)-s.rollingHead >= s.cfg.MaxRecords {
		old := s.rolling[s.rollingHead]
		evicted = &old
	}
	reclaimed := int64(0)
	garbage := map[string]int64{}
	if evicted != nil {
		garbage[filepath.Join(s.dir, rollingDirectory, recordName(evicted.Seq))] = evicted.Bytes
		reclaimed += evicted.Bytes
		for _, ref := range evicted.Refs {
			shared := false
			for _, r := range refs {
				shared = shared || r == ref
			}
			if s.rollingRefs[ref] == 1 && !shared {
				info, e := os.Stat(filepath.Join(s.dir, blobsDir, ref))
				if e != nil {
					return StoredTransaction{}, e
				}
				garbage[filepath.Join(s.dir, blobsDir, ref)] = info.Size()
				reclaimed += info.Size()
			}
		}
	}
	before := s.manifest
	s.manifest.StoreBytes = max(0, s.manifest.StoreBytes-reclaimed)
	e = s.applyLimitsLocked(&next, &encoded)
	s.manifest.StoreBytes = before.StoreBytes
	if e != nil {
		return StoredTransaction{}, e
	}
	refs, e = bodyRefs(next.Transaction)
	if e != nil {
		return StoredTransaction{}, e
	}
	// Body omission can change ownership after the initial budget calculation.
	candidates := append([]string(nil), bodyRefList(tx)...)
	if evicted != nil {
		candidates = append(candidates, evicted.Refs...)
	}
	for _, ref := range candidates {
		owners := s.rollingRefs[ref]
		if evicted != nil {
			for _, r := range evicted.Refs {
				if r == ref {
					owners--
				}
			}
		}
		for _, r := range refs {
			if r == ref {
				owners++
			}
		}
		if owners == 0 {
			path := filepath.Join(s.dir, blobsDir, ref)
			if _, known := garbage[path]; !known {
				if info, err := os.Stat(path); err == nil {
					garbage[path] = info.Size()
					reclaimed += info.Size()
				}
			}
		}
	}
	path := filepath.Join(s.dir, rollingDirectory, recordName(next.Seq))
	if _, e = os.Lstat(path); !os.IsNotExist(e) {
		return StoredTransaction{}, fmt.Errorf("rolling sequence file already exists")
	}
	if e = durableReplace(path, encoded); e != nil {
		return StoredTransaction{}, e
	}
	s.manifest.SnapshotVersion = next.Seq
	s.manifest.Counters.Persisted++
	s.manifest.StoreBytes = max(0, before.StoreBytes-reclaimed) + int64(len(encoded))
	s.manifest.RetainedRecords = uint64(len(s.rolling) - s.rollingHead + 1)
	if evicted != nil {
		s.manifest.Counters.Rotated++
		s.manifest.RetainedRecords--
		s.manifest.RetentionEpoch = next.Seq
	}
	s.manifest.RetentionFloor = next.Seq
	if s.manifest.RetainedRecords > 1 {
		offset := s.rollingHead
		if evicted != nil {
			offset++
		}
		s.manifest.RetentionFloor = s.rolling[offset].Seq
	}
	if checkpoint != nil {
		stats := checkpoint.Stats
		stats.RetentionRows = s.manifest.RetentionRows
		stats.Retained = s.manifest.RetainedRecords
		stats.Evicted = s.manifest.Counters.Rotated
		stats.Persisted = s.manifest.Counters.Persisted
		stats.StoreBytes = s.manifest.StoreBytes
		stats.SnapshotVersion = next.Seq
		s.manifest.CaptureStats = &stats
		redaction := checkpoint.Redaction
		redaction.Known = true
		s.manifest.Redaction = &redaction
	}
	manifestData, e := json.Marshal(s.manifest)
	if e == nil {
		e = durableReplace(filepath.Join(s.dir, manifestFile), append(manifestData, '\n'))
	}
	if e != nil {
		s.manifest = before
		_ = os.Remove(path)
		return StoredTransaction{}, e
	}
	if s.rollingRefs == nil {
		s.rollingRefs = map[string]int{}
	}
	for _, ref := range refs {
		s.rollingRefs[ref]++
	}
	if evicted != nil {
		for _, ref := range evicted.Refs {
			s.rollingRefs[ref]--
			if s.rollingRefs[ref] == 0 {
				delete(s.rollingRefs, ref)
			}
		}
		s.rollingHead++
	}
	s.rolling = append(s.rolling, rollingRecord{next.Seq, int64(len(encoded)), refs})
	if s.rollingHead > 1024 && s.rollingHead > len(s.rolling)/2 {
		s.rolling = append([]rollingRecord(nil), s.rolling[s.rollingHead:]...)
		s.rollingHead = 0
	}
	for path, size := range garbage {
		if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
			s.rollingCleanupNeeded = true
			s.manifest.StoreBytes += size
		} else if filepath.Dir(path) == filepath.Join(s.dir, blobsDir) {
			s.bodyBytes -= size
		}
	}
	if s.rollingCleanupNeeded {
		s.manifest.Findings = appendFindingOnce(s.manifest.Findings, Finding{Code: "CAPTURE_RETENTION_CLEANUP_PENDING", Message: "committed retention; obsolete files need cleanup before the next append"})
		_ = s.writeManifestLocked()
	}
	return next, nil
}
func (s *Store) fetchRollingLocked(filter Filter, opaque string, limit int) (Page, error) {
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	if limit > MaxFetchLimit {
		limit = MaxFetchLimit
	}
	cur := cursor{SessionID: s.manifest.SessionID, Snapshot: s.manifest.SnapshotVersion}
	if opaque != "" {
		v, e := decodeCursor(opaque)
		if e != nil {
			return Page{}, e
		}
		if v.SessionID != cur.SessionID || v.Snapshot < max(s.manifest.ClearedVersion, s.manifest.RetentionEpoch) {
			return Page{}, ErrStaleCursor
		}
		if v.Snapshot > cur.Snapshot || v.LastSeq > v.Snapshot {
			return Page{}, fmt.Errorf("invalid rolling cursor")
		}
		cur = v
	}
	rows, e := s.rollingRowsLocked()
	if e != nil {
		return Page{}, e
	}
	page := Page{Items: []StoredTransaction{}, SnapshotVersion: cur.Snapshot}
	more := false
	last := uint64(0)
	for _, row := range rows {
		if row.Seq > cur.Snapshot {
			continue
		}
		item, _, e := readRollingItem(filepath.Join(s.dir, rollingDirectory, recordName(row.Seq)), row.Seq)
		if e != nil {
			return Page{}, e
		}
		if !Matches(item.Transaction, filter) {
			continue
		}
		page.Total++
		if item.Seq <= cur.LastSeq {
			continue
		}
		if len(page.Items) >= limit {
			more = true
			continue
		}
		if filter.MetadataOnly {
			item.Transaction.Request.LegacyPreview = nil
			item.Transaction.Response.LegacyPreview = nil
			item.Transaction.Request.ImagePreview = nil
			item.Transaction.Response.ImagePreview = nil
			item.Transaction.Request.BodyPreview = ""
			item.Transaction.Response.BodyPreview = ""
		}
		page.Items = append(page.Items, item)
		last = item.Seq
	}
	if more {
		page.NextCursor = encodeCursor(cursor{cur.SessionID, cur.Snapshot, last})
	}
	return page, nil
}
func (s *Store) recoverRollingLocked() (RecoveryReport, error) {
	if s.manifest.Counters.Persisted < s.manifest.Counters.Rotated || s.manifest.Counters.Persisted-s.manifest.Counters.Rotated != s.manifest.RetainedRecords {
		return RecoveryReport{}, fmt.Errorf("invalid rolling counters")
	}
	if s.cfg.MaxRecords < 1 || s.cfg.MaxRecords > 100000 {
		return RecoveryReport{}, fmt.Errorf("invalid persisted retention bound")
	}
	rows, e := s.rollingRowsLocked()
	if e != nil {
		return RecoveryReport{}, e
	}
	refs := map[string]int{}
	var bytes int64
	for _, r := range rows {
		bytes += r.Bytes
		for _, ref := range r.Refs {
			refs[ref]++
		}
	}
	entries, e := os.ReadDir(filepath.Join(s.dir, rollingDirectory))
	if e != nil {
		return RecoveryReport{}, e
	}
	kept := map[string]bool{}
	for _, r := range rows {
		kept[recordName(r.Seq)] = true
	}
	discarded := int64(0)
	for _, entry := range entries {
		if kept[entry.Name()] {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return RecoveryReport{}, e
		}
		if e = os.Remove(filepath.Join(s.dir, rollingDirectory, entry.Name())); e != nil {
			return RecoveryReport{}, e
		}
		discarded += info.Size()
	}
	bodies, e := os.ReadDir(filepath.Join(s.dir, blobsDir))
	if e != nil {
		return RecoveryReport{}, e
	}
	s.bodyBytes = 0
	for _, body := range bodies {
		if body.IsDir() || body.Type()&os.ModeSymlink != 0 {
			return RecoveryReport{}, fmt.Errorf("invalid blob entry")
		}
		info, e := body.Info()
		if e != nil {
			return RecoveryReport{}, e
		}
		if refs[body.Name()] == 0 {
			if e = os.Remove(filepath.Join(s.dir, blobsDir, body.Name())); e != nil {
				return RecoveryReport{}, e
			}
			discarded += info.Size()
		} else {
			s.bodyBytes += info.Size()
		}
	}
	for ref := range refs {
		if _, e = os.Stat(filepath.Join(s.dir, blobsDir, ref)); e != nil {
			return RecoveryReport{}, fmt.Errorf("missing retained blob: %w", e)
		}
	}
	s.rolling = rows
	s.rollingHead = 0
	s.rollingRefs = refs
	s.transactionBytes = bytes
	s.manifest.StoreBytes = bytes + s.bodyBytes
	terminal := s.manifest.State == capture.StateFinalized || s.manifest.State == capture.StateFailed
	recovered := discarded > 0 || !terminal
	if recovered {
		s.manifest.State = capture.StateRecoverable
	}
	if s.manifest.CaptureStats != nil {
		stats := *s.manifest.CaptureStats
		stats.State = s.manifest.State
		stats.StoreBytes = s.manifest.StoreBytes
		stats.RetentionRows = s.manifest.RetentionRows
		stats.Retained = s.manifest.RetainedRecords
		stats.Evicted = s.manifest.Counters.Rotated
		stats.Persisted = s.manifest.Counters.Persisted
		s.manifest.CaptureStats = &stats
	}
	if e = s.writeManifestLocked(); e != nil {
		return RecoveryReport{}, e
	}
	return RecoveryReport{Recovered: recovered, DiscardedBytes: discarded, Records: len(rows)}, nil
}

func readRollingItem(path string, seq uint64) (StoredTransaction, int64, error) {
	var item StoredTransaction
	info, e := os.Lstat(path)
	if e != nil {
		return item, 0, e
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return item, 0, fmt.Errorf("invalid retained record size/type")
	}
	f, e := os.Open(path)
	if e != nil {
		return item, 0, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if e != nil {
		return item, 0, e
	}
	if len(b) > 16<<20 || json.Unmarshal(b, &item) != nil || item.Seq != seq {
		return item, 0, fmt.Errorf("corrupt committed rolling record")
	}
	return item, int64(len(b)), nil
}

func (s *Store) clearRollingLocked() error {
	before := s.manifest
	s.manifest.SnapshotVersion++
	s.manifest.ClearedVersion = s.manifest.SnapshotVersion
	s.manifest.RetentionEpoch = s.manifest.SnapshotVersion
	s.manifest.RetentionFloor = s.manifest.SnapshotVersion + 1
	s.manifest.RetainedRecords = 0
	s.manifest.Counters = Counters{}
	s.manifest.CaptureStats = nil
	s.manifest.Redaction = nil
	s.manifest.Findings = []Finding{}
	s.manifest.StoreBytes = 0
	b, e := json.Marshal(s.manifest)
	if e == nil {
		e = durableReplace(filepath.Join(s.dir, manifestFile), b)
	}
	if e != nil {
		s.manifest = before
		return e
	}
	// Commit the empty selection before unlinking files, so an interrupted
	// Clear cannot leave its old manifest pointing at removed records.
	s.rolling = nil
	s.rollingHead = 0
	s.rollingRefs = map[string]int{}
	s.bodyBytes = 0
	s.transactionBytes = 0
	for _, directory := range []string{rollingDirectory, blobsDir} {
		entries, err := os.ReadDir(filepath.Join(s.dir, directory))
		if err != nil {
			s.rollingCleanupNeeded = true
			continue
		}
		for _, entry := range entries {
			if err = os.Remove(filepath.Join(s.dir, directory, entry.Name())); err != nil {
				s.rollingCleanupNeeded = true
			}
		}
	}
	if s.rollingCleanupNeeded {
		s.manifest.StoreBytes = before.StoreBytes
		s.manifest.Findings = appendFindingOnce(s.manifest.Findings, Finding{Code: "CAPTURE_RETENTION_CLEANUP_PENDING", Message: "clear committed; obsolete files must be reclaimed before appending"})
		_ = s.writeManifestLocked()
	}
	return nil
}

func bodyRefList(tx models.CaptureTransaction) []string { refs, _ := bodyRefs(tx); return refs }
