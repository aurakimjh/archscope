// Synthetic installed-tool stand-in, compiled only by regression tests.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	mode := filepath.Base(os.Args[0])
	if len(os.Args) > 1 && os.Args[1] == "child" {
		for {
			time.Sleep(time.Second)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("21.0.1")
		return
	}
	if len(os.Args) != 4 || os.Args[1] != "print" || os.Args[2] != "--json" || filepath.Ext(os.Args[3]) != ".jfr" {
		os.Exit(2)
	}
	switch {
	case strings.Contains(mode, "hang"):
		child := exec.Command(os.Args[0], "child")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if e := child.Start(); e != nil {
			os.Exit(3)
		}
		os.WriteFile(os.Args[0]+".pid", []byte(fmt.Sprint(child.Process.Pid)), 0600)
		for {
			time.Sleep(time.Second)
		}
	case strings.Contains(mode, "flood"):
		for i := 0; i < 70; i++ {
			fmt.Print(strings.Repeat("x", 1<<20))
		}
	case strings.Contains(mode, "stderr"):
		fmt.Fprint(os.Stderr, strings.Repeat("private-canary", 2000))
		os.Exit(13)
	case strings.Contains(mode, "fail"):
		fmt.Fprint(os.Stderr, "private-canary /secret/input token=synthetic")
		os.Exit(13)
	case strings.Contains(mode, "invalid"):
		fmt.Print(`{"not_events":[]}`)
	default:
		b, e := os.ReadFile(os.Args[3])
		if e != nil || !strings.HasPrefix(string(b), "FLR\x00") {
			os.Exit(4)
		}
		if os.Getenv("JAVA_TOOL_OPTIONS") != "" {
			os.Exit(5)
		}
		fmt.Print(`{"recording":{"events":[]}}`)
	}
}
