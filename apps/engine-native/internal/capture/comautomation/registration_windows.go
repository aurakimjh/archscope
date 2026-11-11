//go:build windows

package comautomation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const typeLibraryID = "{02AAEB6C-A963-4D5D-9CFA-A2DBB6E5E233}"
const universalProxy = "{00020424-0000-0000-C000-000000000046}"
const dispatchProxy = "{00020420-0000-0000-C000-000000000046}"

type registryValue struct{ path, name, value string }

func metadataRoots() []string {
	paths := []string{`TypeLib\` + typeLibraryID, `Interface\` + iidLogEvents.String()}
	for name, c := range referenceContracts.Interfaces {
		if len(interfaceMethods(name)) > 0 {
			paths = append(paths, `Interface\`+c.IID)
		}
	}
	sort.Strings(paths)
	return paths
}
func metadataPreflight(root registry.Key, executable string) error {
	for _, path := range metadataRoots() {
		k, e := registry.OpenKey(root, path, registry.READ)
		if errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
			continue
		}
		if e != nil {
			return e
		}
		owner, _, e := k.GetStringValue("ArchScopeExecutable")
		k.Close()
		if e != nil || !strings.EqualFold(owner, executable) {
			return fmt.Errorf("preserving foreign COM registration %s", path)
		}
	}
	return nil
}
func metadataValues(executable, win32, win64 string) []registryValue {
	values := []registryValue{}
	for _, path := range metadataRoots() {
		values = append(values, registryValue{path, "ArchScopeExecutable", executable})
	}
	lib := `TypeLib\` + typeLibraryID + `\1.0`
	values = append(values, registryValue{lib, "", "ArchScope Automation"}, registryValue{lib + `\0\win32`, "", win32}, registryValue{lib + `\0\win64`, "", win64}, registryValue{lib + `\FLAGS`, "", "0"})
	for name, c := range referenceContracts.Interfaces {
		if len(interfaceMethods(name)) == 0 {
			continue
		}
		p := `Interface\` + c.IID
		values = append(values, registryValue{p, "", name}, registryValue{p + `\ProxyStubClsid32`, "", universalProxy}, registryValue{p + `\TypeLib`, "", typeLibraryID}, registryValue{p + `\TypeLib`, "Version", "1.0"}, registryValue{p + `\NumMethods`, "", strconv.Itoa(7 + len(interfaceMethods(name)))})
	}
	// This outgoing dispinterface has the standard IDispatch ABI; no fabricated
	// dual event declaration is inserted into the type library.
	p := `Interface\` + iidLogEvents.String()
	values = append(values, registryValue{p, "", "ILogEvents"}, registryValue{p + `\ProxyStubClsid32`, "", dispatchProxy})
	return values
}
func writeRegistryValues(root registry.Key, values []registryValue) (err error) {
	type previous struct {
		v      registryValue
		exists bool
		value  string
	}
	old := []previous{}
	created := []string{}
	defer func() {
		if err != nil {
			for i := len(old) - 1; i >= 0; i-- {
				p := old[i]
				k, e := registry.OpenKey(root, p.v.path, registry.SET_VALUE)
				if e == nil {
					if p.exists {
						k.SetStringValue(p.v.name, p.value)
					} else {
						k.DeleteValue(p.v.name)
					}
					k.Close()
				}
			}
			sort.Slice(created, func(i, j int) bool { return len(created[i]) > len(created[j]) })
			for _, p := range created {
				registry.DeleteKey(root, p)
			}
		}
	}()
	// Create each ancestor explicitly, so rollback does not leave ownership-free
	// parent keys behind after a failed first registration.
	for _, v := range values {
		parts := strings.Split(v.path, `\`)
		for i := 1; i <= len(parts); i++ {
			p := strings.Join(parts[:i], `\`)
			k, exists, e := registry.CreateKey(root, p, registry.READ|registry.WRITE)
			if e != nil {
				return e
			}
			k.Close()
			if !exists {
				created = append(created, p)
			}
		}
		k, e := registry.OpenKey(root, v.path, registry.READ|registry.WRITE)
		if e != nil {
			return e
		}
		prior, _, e := k.GetStringValue(v.name)
		if e != nil && !errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
			k.Close()
			return e
		}
		old = append(old, previous{v, e == nil, prior})
		err = k.SetStringValue(v.name, v.value)
		k.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func removeOwnedMetadata(root registry.Key, executable string) error {
	if e := metadataPreflight(root, executable); e != nil {
		return e
	}
	// Delete only the documented tree. Unknown subkeys make DeleteKey fail and
	// preserve the unexpected data, rather than recursively deleting it.
	for _, p := range metadataRoots() {
		suffixes := []string{`\ProxyStubClsid32`, `\TypeLib`, `\NumMethods`}
		if strings.HasPrefix(p, `TypeLib\`) {
			suffixes = []string{`\1.0\0\win32`, `\1.0\0\win64`, `\1.0\0`, `\1.0\FLAGS`, `\1.0`}
		}
		for _, suffix := range append(suffixes, "") {
			if e := registry.DeleteKey(root, p+suffix); e != nil && !errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
				return e
			}
		}
	}
	return nil
}

// Registration is explicit, per-user and fail-closed on existing vendor IIDs.
// Broad RegisterTypeLibForUser would overwrite those IIDs, so write only the
// preflighted owned mapping instead.
func registerWithMetadata(executable string, remove bool) error {
	if !filepath.IsAbs(executable) || !strings.EqualFold(filepath.Ext(executable), ".exe") || strings.ContainsAny(executable, "\x00\"") {
		return fmt.Errorf("choose an absolute .exe path")
	}
	if st, e := os.Stat(executable); e != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("desktop executable is unavailable")
	}
	roots := []registry.Key{}
	defer func() {
		for _, r := range roots {
			r.Close()
		}
	}()
	for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
		root, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Classes`, registry.READ|registry.WRITE|view)
		if errors.Is(e, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if e != nil {
			return e
		}
		roots = append(roots, root)
		if e = metadataPreflight(root, executable); e != nil {
			return e
		}
		if e = classPreflight(root, executable); e != nil {
			return e
		}
		if !remove {
			machine, e := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Classes`, registry.READ|view)
			if e != nil {
				return e
			}
			for _, p := range metadataRoots() {
				k, e := registry.OpenKey(machine, p, registry.READ)
				if e == nil {
					k.Close()
					machine.Close()
					return fmt.Errorf("preserving machine COM registration %s", p)
				}
				if !errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
					machine.Close()
					return e
				}
			}
			machine.Close()
		}
	}
	if len(roots) == 0 {
		return fmt.Errorf("no supported COM registry view")
	}
	previous := registeredMetadataFiles(roots)
	if remove {
		for _, root := range roots {
			if e := registerKey(root, executable, true); e != nil {
				return e
			}
			if e := removeOwnedMetadata(root, executable); e != nil {
				return e
			}
		}
		return cleanupMetadataFiles(previous, executable)
	}
	cache, e := os.UserCacheDir()
	if e != nil {
		return e
	}
	base := filepath.Join(cache, "ArchScope", "Automation")
	if e = os.MkdirAll(base, 0700); e != nil {
		return e
	}
	directory, e := os.MkdirTemp(base, "types-")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(directory, "owner.txt"), []byte(executable), 0600); e != nil {
		os.RemoveAll(directory)
		return e
	}
	published := false
	defer func() {
		if !published {
			os.RemoveAll(directory)
		}
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	e = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if e != nil && !errors.Is(e, syscall.Errno(1)) {
		return e
	}
	defer windows.CoUninitialize()
	win32 := filepath.Join(directory, "ArchScope32.tlb")
	win64 := filepath.Join(directory, "ArchScope64.tlb")
	for _, target := range []struct {
		path string
		kind uintptr
	}{{win32, 1}, {win64, 3}} {
		infos, e := buildTypeInfosFor(target.path, target.kind)
		if e != nil {
			return e
		}
		for _, p := range infos {
			releaseCOM(p)
		}
	}
	values := metadataValues(executable, win32, win64)
	for _, root := range roots {
		if e := writeRegistryValues(root, values); e != nil {
			return e
		}
		published = true
		if e := registerKey(root, executable, false); e != nil {
			return e
		}
	}
	return cleanupMetadataFiles(previous, executable)
}

func classPreflight(root registry.Key, executable string) error {
	command := `"` + executable + `" --com-server`
	for _, item := range []struct{ path, name, expected string }{{`CLSID\` + ClassID, "ArchScopeOwner", ClassID}, {`CLSID\` + ClassID + `\LocalServer32`, "", command}, {ProgID + `\CLSID`, "", ClassID}} {
		k, e := registry.OpenKey(root, item.path, registry.QUERY_VALUE)
		if errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
			continue
		}
		if e != nil {
			return e
		}
		value, _, e := k.GetStringValue(item.name)
		k.Close()
		if e != nil || !strings.EqualFold(value, item.expected) {
			return fmt.Errorf("preserving foreign class registration %s", item.path)
		}
	}
	return nil
}
func registeredMetadataFiles(roots []registry.Key) []string {
	paths := map[string]bool{}
	for _, root := range roots {
		for _, kind := range []string{"win32", "win64"} {
			k, e := registry.OpenKey(root, `TypeLib\`+typeLibraryID+`\1.0\0\`+kind, registry.QUERY_VALUE)
			if e == nil {
				p, _, e := k.GetStringValue("")
				k.Close()
				if e == nil {
					paths[p] = true
				}
			}
		}
	}
	result := []string{}
	for p := range paths {
		result = append(result, p)
	}
	return result
}
func cleanupMetadataFiles(paths []string, executable string) error {
	cache, e := os.UserCacheDir()
	if e != nil {
		return e
	}
	base := filepath.Join(cache, "ArchScope", "Automation")
	dirs := map[string]bool{}
	for _, p := range paths {
		dir := filepath.Dir(p)
		if filepath.Dir(dir) != base || !strings.HasPrefix(filepath.Base(dir), "types-") {
			continue
		}
		info, e := os.Lstat(dir)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		owner, e := os.ReadFile(filepath.Join(dir, "owner.txt"))
		if e != nil || string(owner) != executable {
			continue
		}
		if filepath.Base(p) != "ArchScope32.tlb" && filepath.Base(p) != "ArchScope64.tlb" {
			continue
		}
		if e = os.Remove(p); e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		dirs[dir] = true
	}
	for dir := range dirs {
		if e = os.Remove(filepath.Join(dir, "owner.txt")); e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if e = os.Remove(dir); e != nil {
			return e
		}
	}
	return nil
}
