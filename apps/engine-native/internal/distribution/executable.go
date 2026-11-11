package distribution

import (
	"debug/elf"
	"debug/macho"
	"fmt"
	"runtime"
)

func checkExecutable(p string) error {
	switch runtime.GOOS {
	case "windows":
		return checkPE(p, runtime.GOARCH)
	case "darwin":
		f, e := macho.Open(p)
		if e != nil {
			return e
		}
		defer f.Close()
		want := map[string]macho.Cpu{"amd64": macho.CpuAmd64, "arm64": macho.CpuArm64}[runtime.GOARCH]
		if f.Cpu != want || f.Type != macho.TypeExec {
			return fmt.Errorf("offline Mach-O platform mismatch")
		}
	case "linux":
		f, e := elf.Open(p)
		if e != nil {
			return e
		}
		defer f.Close()
		want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64, "386": elf.EM_386}[runtime.GOARCH]
		if f.Machine != want {
			return fmt.Errorf("offline ELF platform mismatch")
		}
	default:
		return fmt.Errorf("offline executable platform unqualified")
	}
	return nil
}
