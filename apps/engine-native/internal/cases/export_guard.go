package cases

import (
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
	targetInfo, _ := os.Stat(target)
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
