package packs

import (
	"os"
	"path/filepath"
)

func (m *Manager) Directory() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", problem("PACK_STORE_CLOSED", "pack store closed")
	}
	return m.root.Name(), nil
}
func CheckExport(root, target string) error {
	info, e := os.Stat(root)
	if e != nil {
		return e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(target))
	if e != nil {
		return e
	}
	for p := filepath.Join(parent, filepath.Base(target)); ; p = filepath.Dir(p) {
		i, e := os.Stat(p)
		if e == nil && os.SameFile(info, i) {
			return problem("PACK_EXPORT_UNSAFE", "export outside managed pack storage")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
