package processlaunch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestShortcutResolutionPreservesArgumentsAndWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture.LNK")
	exe := filepath.Join(root, "한글 app.exe")
	for _, name := range []string{path, exe} {
		if err := os.WriteFile(name, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"extra space", `C:\tail\`, ""}
	raw := `--title "한글 text" --value="a b"`
	target, err := resolve(path, args, func(got string) (shortcutProperties, error) {
		if got != path {
			t.Fatal(got)
		}
		return shortcutProperties{Path: exe, Arguments: raw, WorkingDirectory: root}, nil
	})
	if err != nil || target.Path != exe || target.RawArguments != raw || target.WorkingDirectory != root || !reflect.DeepEqual(target.Args, args) {
		t.Fatalf("%+v %v", target, err)
	}
	args[0] = "changed"
	if target.Args[0] != "extra space" {
		t.Fatal("caller can mutate prepared arguments")
	}
}
func TestLaunchValidationRejectsInvalidTargetsBeforeAllocation(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "fixture.exe")
	link := filepath.Join(root, "fixture.lnk")
	os.WriteFile(exe, []byte("fixture"), 0600)
	os.WriteFile(link, []byte("fixture"), 0600)
	base := shortcutProperties{Path: exe, WorkingDirectory: root}
	for _, properties := range []shortcutProperties{{Path: "relative.exe"}, {Path: root}, {Path: link}, {Path: filepath.Join(root, "missing.exe")}, {Path: exe, WorkingDirectory: "relative"}, {Path: exe, WorkingDirectory: exe}, {Path: exe, Arguments: "bad\x00argument"}, {Path: exe, Arguments: strings.Repeat("한", 32767)}} {
		if _, err := resolve(link, nil, func(string) (shortcutProperties, error) { return properties, nil }); err == nil {
			t.Fatalf("accepted %+v", properties)
		}
	}
	if _, err := resolve(link, make([]string, 129), func(string) (shortcutProperties, error) { return base, nil }); err == nil {
		t.Fatal("too many arguments accepted")
	}
	if _, err := Resolve(exe, []string{"a\x00b"}); err == nil {
		t.Fatal("NUL accepted")
	}
	if _, err := Resolve(exe, nil); err != nil {
		t.Fatal(err)
	}
}
func TestShortcutFixedHeaderRejectsUnsupportedActivation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.lnk")
	header := make([]byte, 76)
	binary.LittleEndian.PutUint32(header, 76)
	binary.LittleEndian.PutUint32(header[4:], 0x21401)
	copy(header[8:20], []byte{0, 0, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46})
	for _, flags := range []uint32{0, 0x80, 0x1000, 0x2000} {
		binary.LittleEndian.PutUint32(header[20:], flags)
		os.WriteFile(path, header, 0600)
		err := checkShortcutHeader(path)
		if (flags&(0x1000|0x2000) != 0) != (err != nil) {
			t.Fatalf("flags=%x err=%v", flags, err)
		}
	}
	os.WriteFile(path, header[:30], 0600)
	if checkShortcutHeader(path) == nil {
		t.Fatal("truncated header accepted")
	}
	header[4] = 0
	os.WriteFile(path, header, 0600)
	if checkShortcutHeader(path) == nil {
		t.Fatal("wrong class accepted")
	}
}
