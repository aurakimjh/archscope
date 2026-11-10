package processlaunch

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Target holds launch-only data. Arguments are never added to capture events.
// RawArguments preserves the shortcut's command line verbatim; Args are extra
// explicit UI arguments, quoted separately by CreateProcess's caller.
type Target struct {
	Path             string
	Args             []string
	RawArguments     string
	WorkingDirectory string
}
type shortcutProperties struct{ Path, Arguments, WorkingDirectory string }

func Resolve(path string, args []string) (Target, error) {
	return resolve(path, args, readShortcut)
}
func resolve(path string, args []string, read func(string) (shortcutProperties, error)) (Target, error) {
	target := Target{Path: path, Args: append([]string(nil), args...)}
	if !filepath.IsAbs(path) {
		return Target{}, fmt.Errorf("choose an absolute executable or shortcut path")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return Target{}, fmt.Errorf("launch file is unavailable")
	}
	if strings.EqualFold(filepath.Ext(path), ".lnk") {
		if info.Size() > 16<<20 {
			return Target{}, fmt.Errorf("shortcut exceeds 16 MiB")
		}
		properties, err := read(path)
		if err != nil {
			return Target{}, err
		}
		target.Path = properties.Path
		target.RawArguments = properties.Arguments
		target.WorkingDirectory = properties.WorkingDirectory
	}
	if err := validateTarget(target); err != nil {
		return Target{}, err
	}
	return target, nil
}
func validateTarget(target Target) error {
	if !filepath.IsAbs(target.Path) {
		return fmt.Errorf("shortcut target must be an absolute executable path")
	}
	extension := strings.ToLower(filepath.Ext(target.Path))
	if extension != ".exe" && extension != ".com" {
		return fmt.Errorf("launch target must be an executable (.exe or .com)")
	}
	info, err := os.Stat(target.Path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("target executable is unavailable")
	}
	if target.WorkingDirectory != "" {
		if !filepath.IsAbs(target.WorkingDirectory) {
			return fmt.Errorf("shortcut working directory must be absolute")
		}
		info, err = os.Stat(target.WorkingDirectory)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("shortcut working directory is unavailable")
		}
	}
	if len(target.Args) > 128 {
		return fmt.Errorf("too many launch arguments")
	}
	size := 0
	for _, value := range append([]string{target.Path, target.WorkingDirectory, target.RawArguments}, target.Args...) {
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("launch values cannot contain NUL")
		}
		size += len(utf16.Encode([]rune(value))) + 1
	}
	if size >= 32767 {
		return fmt.Errorf("launch values exceed the Windows command-line limit")
	}
	return nil
}

// Validate only the fixed header; native COM interprets the actual link data.
func checkShortcutHeader(path string) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	var header [76]byte
	_, err = io.ReadFull(input, header[:])
	if err != nil || binary.LittleEndian.Uint32(header[:4]) != 76 || binary.LittleEndian.Uint32(header[4:8]) != 0x21401 || string(header[8:20]) != string([]byte{0, 0, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46}) {
		return fmt.Errorf("invalid Windows shortcut header")
	}
	if binary.LittleEndian.Uint32(header[20:24])&(0x1000|0x2000) != 0 {
		return fmt.Errorf("advertised or elevated shortcuts require unsupported shell activation")
	}
	return nil
}
