package cases

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CheckExportDestination protects authoritative case data and current external
// originals even when a chosen destination uses a directory alias/hard link.
func (s *Store) CheckExportDestination(caseID, destination string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.get(caseID)
	if e != nil {
		return e
	}
	resolve := func(p string) (string, error) {
		p, e := filepath.Abs(p)
		if e != nil {
			return "", e
		}
		parent, e := filepath.EvalSymlinks(filepath.Dir(p))
		if e != nil {
			return "", e
		}
		return filepath.Join(parent, filepath.Base(p)), nil
	}
	target, e := resolve(destination)
	if e != nil {
		return e
	}
	root, e := filepath.EvalSymlinks(s.root.Name())
	if e != nil {
		return e
	}
	root, e = filepath.Abs(root)
	if e != nil {
		return e
	}
	relative, e := filepath.Rel(root, target)
	if e != nil {
		return e
	}
	if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return problem("CASE_EXPORT_DESTINATION_UNSAFE", "export outside managed case storage")
	}
	// String relativity does not canonicalize case on APFS. Compare filesystem
	// identity at every existing ancestor, including the destination itself.
	rootInfo, e := os.Stat(root)
	if e != nil {
		return e
	}
	for ancestor := target; ; ancestor = filepath.Dir(ancestor) {
		info, statErr := os.Stat(ancestor)
		if statErr == nil && os.SameFile(rootInfo, info) {
			return problem("CASE_EXPORT_DESTINATION_UNSAFE", "export outside managed case storage")
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return statErr
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	targetInfo, _ := os.Stat(target)
	if targetInfo != nil {
		entries := 0
		e = fs.WalkDir(s.root.FS(), ".", func(p string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			entries++
			if entries > 32768+4*MaxCases {
				return problem("CASE_BUDGET_EXCEEDED", "export identity scan limit")
			}
			if entry.IsDir() {
				return nil
			}
			info, e := entry.Info()
			if e != nil {
				return e
			}
			if os.SameFile(targetInfo, info) {
				return problem("CASE_EXPORT_DESTINATION_UNSAFE", "destination aliases managed case data")
			}
			return nil
		})
		if e != nil {
			return e
		}
	}
	for _, source := range m.Sources {
		if source.ExternalPath == "" {
			continue
		}
		original, e := resolve(source.ExternalPath)
		if e != nil {
			continue
		}
		equal := original == target
		if os.PathSeparator == '\\' {
			equal = strings.EqualFold(original, target)
		}
		originalInfo, e := os.Stat(source.ExternalPath)
		if equal || e == nil && targetInfo != nil && os.SameFile(originalInfo, targetInfo) {
			return problem("CASE_EXPORT_DESTINATION_UNSAFE", "external original is read-only; choose another export destination")
		}
	}
	return nil
}
