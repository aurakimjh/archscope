// Package distribution builds and preflights disconnected installations. A
// checksum manifest is an integrity inventory; delivery/signing establishes trust.
package distribution

import (
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Entry struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type Inventory struct {
	Version        string           `json:"version"`
	OS             string           `json:"os"`
	Arch           string           `json:"arch"`
	Engine         string           `json:"engine"`
	App            string           `json:"app,omitempty"`
	WebviewPath    string           `json:"webview_path,omitempty"`
	WebviewVersion string           `json:"webview_version,omitempty"`
	Files          map[string]Entry `json:"files"`
	Dependencies   []string         `json:"dependencies"`
}
type BuildOptions struct{ Out, Engine, App, WebviewRuntime, WebviewVersion string }

func digestFile(p string) (Entry, error) {
	var v Entry
	i, e := os.Lstat(p)
	if e != nil {
		return v, e
	}
	if !i.Mode().IsRegular() || i.Size() > 2<<30 {
		return v, fmt.Errorf("bounded regular file required: %s", p)
	}
	f, e := os.Open(p)
	if e != nil {
		return v, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !os.SameFile(i, info) {
		return v, fmt.Errorf("file changed during preflight")
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, 2<<30+1))
	if e != nil || n != i.Size() {
		return v, fmt.Errorf("inventory read changed")
	}
	return Entry{hex.EncodeToString(h.Sum(nil)), n}, nil
}
func copyFile(source, dest string) error {
	if _, e := digestFile(source); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
		return e
	}
	f, e := os.Open(source)
	if e != nil {
		return e
	}
	defer f.Close()
	out, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, io.LimitReader(f, 2<<30+1))
	closeErr := out.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func checkPE(p, arch string) error {
	f, e := pe.Open(p)
	if e != nil {
		return fmt.Errorf("fixed WebView PE: %w", e)
	}
	defer f.Close()
	want := map[string]uint16{"386": pe.IMAGE_FILE_MACHINE_I386, "amd64": pe.IMAGE_FILE_MACHINE_AMD64, "arm64": pe.IMAGE_FILE_MACHINE_ARM64}[arch]
	if want == 0 || f.FileHeader.Machine != want {
		return fmt.Errorf("fixed WebView architecture mismatch")
	}
	return nil
}
func runtimePreflight(root, arch string) error {
	client := filepath.Join(root, "EBWebView", map[string]string{"386": "x86", "amd64": "x64", "arm64": "arm64"}[arch], "EmbeddedBrowserWebView.dll")
	if _, e := digestFile(client); e != nil {
		return fmt.Errorf("OFFLINE_WEBVIEW_MISSING: architecture-specific EmbeddedBrowserWebView.dll")
	}
	if e := checkPE(client, arch); e != nil {
		return e
	}
	for _, n := range []string{"msedgewebview2.exe", "msedge.dll", "resources.pak", "icudtl.dat"} {
		if _, e := digestFile(filepath.Join(root, n)); e != nil {
			return fmt.Errorf("OFFLINE_WEBVIEW_MISSING: %s", n)
		}
	}
	locales, e := os.ReadDir(filepath.Join(root, "locales"))
	if e != nil || len(locales) == 0 {
		return fmt.Errorf("OFFLINE_WEBVIEW_MISSING: locales")
	}
	license := false
	entries, _ := os.ReadDir(root)
	for _, v := range entries {
		if strings.Contains(strings.ToLower(v.Name()), "license") {
			license = true
		}
	}
	if !license {
		return fmt.Errorf("OFFLINE_WEBVIEW_MISSING: license")
	}
	if e := checkPE(filepath.Join(root, "msedge.dll"), arch); e != nil {
		return e
	}
	return checkPE(filepath.Join(root, "msedgewebview2.exe"), arch)
}
func Build(ctx context.Context, o BuildOptions) (Inventory, error) {
	m := Inventory{Version: "1.0.0", OS: runtime.GOOS, Arch: runtime.GOARCH, Files: map[string]Entry{}, Dependencies: []string{"OS filesystem and clock"}}
	if o.Out == "" || o.Engine == "" {
		return m, fmt.Errorf("new --out and --engine required")
	}
	if e := checkExecutable(o.Engine); e != nil {
		return m, e
	}
	if o.App != "" {
		if e := checkExecutable(o.App); e != nil {
			return m, e
		}
	}
	if _, e := CheckFixtures(ctx); e != nil {
		return m, e
	}
	if e := os.Mkdir(o.Out, 0755); e != nil {
		return m, e
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(o.Out)
		}
	}()
	m.Engine = "archscope-engine"
	if runtime.GOOS == "windows" {
		m.Engine += ".exe"
	}
	if e := copyFile(o.Engine, filepath.Join(o.Out, m.Engine)); e != nil {
		return m, e
	}
	if o.App != "" {
		m.App = "archscope-app"
		if runtime.GOOS == "windows" {
			m.App += ".exe"
		}
		if e := copyFile(o.App, filepath.Join(o.Out, m.App)); e != nil {
			return m, e
		}
		switch runtime.GOOS {
		case "windows":
			if o.WebviewRuntime == "" || o.WebviewVersion == "" || len(o.WebviewVersion) > 64 {
				return m, fmt.Errorf("OFFLINE_WEBVIEW_MISSING: explicit complete Fixed Version runtime and version required")
			}
			if e := runtimePreflight(o.WebviewRuntime, m.Arch); e != nil {
				return m, e
			}
			m.WebviewPath = "webview2"
			if e := checkRuntimeVersion(filepath.Join(o.WebviewRuntime, "msedgewebview2.exe"), o.WebviewVersion); e != nil {
				return m, e
			}
			m.WebviewVersion = o.WebviewVersion
			e := filepath.WalkDir(o.WebviewRuntime, func(p string, d fs.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if d.IsDir() {
					return nil
				}
				rel, e := filepath.Rel(o.WebviewRuntime, p)
				if e != nil {
					return e
				}
				return copyFile(p, filepath.Join(o.Out, m.WebviewPath, rel))
			})
			if e != nil {
				return m, e
			}
		case "darwin":
			m.Dependencies = append(m.Dependencies, "OS WKWebView (not vendored)")
		default:
			return m, fmt.Errorf("OFFLINE_DESKTOP_UNQUALIFIED: use CLI on this platform")
		}
	}
	files, e := ResourceFiles()
	if e != nil {
		return m, e
	}
	for n, b := range files {
		p := filepath.Join(o.Out, "resources", n)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return m, e
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			return m, e
		}
	}
	e = filepath.WalkDir(o.Out, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(o.Out, p)
		if e != nil {
			return e
		}
		v, e := digestFile(p)
		if e == nil {
			m.Files[filepath.ToSlash(rel)] = v
		}
		return e
	})
	if e != nil {
		return m, e
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if e = os.WriteFile(filepath.Join(o.Out, "inventory.json"), append(b, '\n'), 0644); e != nil {
		return m, e
	}
	if _, e = Verify(o.Out, o.App != ""); e != nil {
		return m, e
	}
	success = true
	return m, nil
}
func Verify(root string, desktop bool) (Inventory, error) {
	var m Inventory
	info, e := os.Lstat(root)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return m, fmt.Errorf("OFFLINE_ROOT_INVALID")
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return m, e
	}
	defer r.Close()
	f, e := r.Open("inventory.json")
	if e != nil {
		return m, e
	}
	b, e := io.ReadAll(io.LimitReader(f, 8<<20+1))
	f.Close()
	if e != nil || len(b) > 8<<20 {
		return m, fmt.Errorf("inventory budget")
	}
	if e = packs.Strict(b, &m); e != nil {
		return m, e
	}
	if m.Version != "1.0.0" || m.OS != runtime.GOOS || m.Arch != runtime.GOARCH || len(m.Files) == 0 || len(m.Files) > 30000 || !validPath(m.Engine) || desktop && !validPath(m.App) {
		return m, fmt.Errorf("OFFLINE_INCOMPATIBLE: platform/application manifest")
	}
	for n, w := range m.Files {
		if !validPath(n) {
			return m, fmt.Errorf("inventory path invalid")
		}
		i, e := r.Lstat(n)
		if e != nil || !i.Mode().IsRegular() {
			return m, fmt.Errorf("OFFLINE_FILE_MISSING: %s", n)
		}
		v, e := digestRoot(r, n)
		if e != nil || v != w {
			return m, fmt.Errorf("OFFLINE_HASH_MISMATCH: %s", n)
		}
	}
	// Deny symlink directories and undeclared files too, including runtime extras.
	count := 0
	e = filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		count++
		if count > 40000 {
			return fmt.Errorf("inventory entry budget")
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("offline symlink forbidden")
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		n := filepath.ToSlash(rel)
		if n != "inventory.json" {
			if _, ok := m.Files[n]; !ok {
				return fmt.Errorf("OFFLINE_UNDECLARED_FILE: %s", n)
			}
		}
		return nil
	})
	if e != nil {
		return m, e
	}
	if _, ok := m.Files[m.Engine]; !ok {
		return m, fmt.Errorf("engine missing from inventory")
	}
	if desktop {
		if _, ok := m.Files[m.App]; !ok {
			return m, fmt.Errorf("app missing from inventory")
		}
	}
	required, e := ResourceFiles()
	if e != nil {
		return m, e
	}
	for n, w := range required {
		v, ok := m.Files["resources/"+n]
		if !ok || v.SHA256 != packs.Hash(w) || v.Bytes != int64(len(w)) {
			return m, fmt.Errorf("OFFLINE_RESOURCE_REQUIRED: %s", n)
		}
	}
	if desktop && runtime.GOOS == "windows" {
		if m.WebviewPath != "webview2" || m.WebviewVersion == "" {
			return m, fmt.Errorf("OFFLINE_WEBVIEW_MISSING")
		}
		if e = runtimePreflight(filepath.Join(root, m.WebviewPath), m.Arch); e != nil {
			return m, e
		}
		if e = checkRuntimeVersion(filepath.Join(root, m.WebviewPath, "msedgewebview2.exe"), m.WebviewVersion); e != nil {
			return m, e
		}
	}
	if e := checkExecutable(filepath.Join(root, m.Engine)); e != nil {
		return m, e
	}
	if desktop {
		if e := checkExecutable(filepath.Join(root, m.App)); e != nil {
			return m, e
		}
	}
	return m, nil
}
func (m Inventory) Names() []string {
	names := []string{}
	for n := range m.Files {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func digestRoot(root *os.Root, name string) (Entry, error) {
	var v Entry
	info, e := root.Lstat(name)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 2<<30 {
		return v, fmt.Errorf("invalid rooted inventory file")
	}
	f, e := root.Open(name)
	if e != nil {
		return v, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return v, fmt.Errorf("inventory file changed")
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, 2<<30+1))
	if e != nil || n != info.Size() {
		return v, fmt.Errorf("inventory size changed")
	}
	return Entry{hex.EncodeToString(h.Sum(nil)), n}, nil
}
