//go:build windows

package dumppolicy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	localDumpsKey  = `SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps`
	excludedAppKey = `SOFTWARE\Microsoft\Windows\Windows Error Reporting\ExcludedApplications`

	// WER_FAULT_REPORTING_FLAG_NOHEAP suppresses heap collection for reports
	// this process generates. It cannot be read back, so it is applied as
	// defense in depth and never counted as evidence.
	werFaultReportingFlagNoHeap = 0x0001

	// SEM_NOGPFAULTERRORBOX stops the OS fault dialog from launching the
	// default handler for this process.
	semNoGPFaultErrorBox = 0x0002
)

var (
	modwer                = windows.NewLazySystemDLL("wer.dll")
	procWerAddExcludedApp = modwer.NewProc("WerAddExcludedApplication")
	procWerSetFlags       = modwer.NewProc("WerSetFlags")
	modkernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procSetErrorMode      = modkernel32.NewProc("SetErrorMode")
)

// current implements the four preflight steps documented in
// SYSTEM_HTTP_CAPTURE 2.1.1. Every step that cannot conclusively establish
// suppression returns unavailable; there is no path that reports verified on a
// call whose effect was not read back.
func current() Result {
	exe, err := executableName()
	if err != nil {
		return unavailable(ReasonPolicyUnreadable,
			"the running executable name could not be determined, so no policy could be evaluated for it")
	}

	// Step 1 — would any LocalDumps policy collect a dump for this process?
	collecting, err := localDumpsCollects(exe)
	if err != nil {
		return unavailable(ReasonPolicyUnreadable,
			"the Windows Error Reporting LocalDumps policy could not be read")
	}
	if collecting {
		// This is the managed-machine case RD-135 accepts. A per-user
		// exclusion is not proven to override a machine-wide LocalDumps
		// policy, so the preflight does not claim that it does.
		return unavailable(ReasonLocalDumpsCollecting,
			"a LocalDumps policy collects a crash dump for this application and cannot be overridden without administrator rights")
	}

	// Step 2 — suppress reporting for this executable.
	applyProcessReportingFlags()
	if err := addExcludedApplication(exe); err != nil {
		return unavailable(ReasonExclusionFailed,
			"this application could not be registered as excluded from Windows Error Reporting")
	}

	// Step 3 — read the effect back. A successful call is not evidence; the
	// registered state is.
	excluded, err := excludedApplicationRegistered(exe)
	if err != nil {
		return unavailable(ReasonPolicyUnreadable,
			"the Windows Error Reporting exclusion list could not be read back")
	}
	if !excluded {
		return unavailable(ReasonExclusionUnconfirmed,
			"the Windows Error Reporting exclusion was requested but is not present in the effective policy")
	}

	// Re-read the dump policy last. Between step 1 and here the machine policy
	// could have changed, and this ordering means the verified result reflects
	// the later observation rather than the earlier one.
	collecting, err = localDumpsCollects(exe)
	if err != nil {
		return unavailable(ReasonPolicyUnreadable,
			"the Windows Error Reporting LocalDumps policy could not be re-read for confirmation")
	}
	if collecting {
		return unavailable(ReasonLocalDumpsCollecting,
			"a LocalDumps policy became active for this application while the preflight was running")
	}

	// Step 4 — bound result. This states what was established, not that the
	// process memory is universally unreachable; the exposure this does not
	// cover is documented in SYSTEM_HTTP_CAPTURE 2.1.1.
	return verified("no LocalDumps policy collects a dump for this application and it is registered as excluded from Windows Error Reporting")
}

func executableName() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	name := filepath.Base(path)
	if strings.TrimSpace(name) == "" {
		return "", os.ErrInvalid
	}
	return name, nil
}

// localDumpsCollects reports whether the LocalDumps mechanism would write a
// dump for this executable.
//
// The rule is deliberately coarse: any applicable LocalDumps key counts as
// collecting, including DumpType 1. A minidump still carries thread stacks,
// and a body buffer referenced from a stack is exactly the plaintext this gate
// exists to keep off disk. Narrowing this by DumpType would trade a provable
// property for a guess about what a given dump format happens to include.
func localDumpsCollects(exe string) (bool, error) {
	perApp, err := keyExists(localDumpsKey + `\` + exe)
	if err != nil {
		return false, err
	}
	if perApp {
		return true, nil
	}
	// The per-application subkey overrides the global one, so the global key
	// only applies when no per-application key exists.
	return keyExists(localDumpsKey)
}

func keyExists(path string) (bool, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	_ = key.Close()
	return true, nil
}

// excludedApplicationRegistered checks both scopes. The preflight writes the
// per-user entry, but a machine-wide entry satisfies the same property, and
// reading only HKCU would report unconfirmed on a machine where an
// administrator already excluded the application for everyone.
func excludedApplicationRegistered(exe string) (bool, error) {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, excludedAppKey, registry.QUERY_VALUE)
		if err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				continue
			}
			return false, err
		}
		names, err := key.ReadValueNames(0)
		closeErr := key.Close()
		if err != nil {
			return false, err
		}
		if closeErr != nil {
			return false, closeErr
		}
		for _, name := range names {
			if strings.EqualFold(name, exe) {
				return true, nil
			}
		}
	}
	return false, nil
}

func addExcludedApplication(exe string) error {
	if err := procWerAddExcludedApp.Find(); err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	// bAllUsers is false: the preflight must not require administrator rights
	// to run, and a per-user exclusion is what an unelevated capture can
	// establish for itself.
	hr, _, _ := procWerAddExcludedApp.Call(uintptr(unsafe.Pointer(name)), 0)
	if int32(hr) < 0 {
		return windows.Errno(uint32(hr))
	}
	return nil
}

// applyProcessReportingFlags is best effort by construction: neither setting
// can be read back, so a failure here never turns into a verified result and a
// success here never contributes to one.
func applyProcessReportingFlags() {
	if err := procWerSetFlags.Find(); err == nil {
		_, _, _ = procWerSetFlags.Call(uintptr(werFaultReportingFlagNoHeap))
	}
	if err := procSetErrorMode.Find(); err == nil {
		_, _, _ = procSetErrorMode.Call(uintptr(semNoGPFaultErrorBox))
	}
}
