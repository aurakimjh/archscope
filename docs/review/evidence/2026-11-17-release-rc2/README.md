# v0.4.0-rc.2 release receipt

**PUBLISHED_AND_VERIFIED — 2026-11-17**:
[GitHub prerelease](https://github.com/aurakimjh/archscope/releases/tag/v0.4.0-rc.2).
[Publication identity](publication-identity.json) binds annotated tag
`2db81164cd2b04b87d2d9e7c5c184ff991144c82` to source
`9dc0c2390a7602125af5f53401ad3b3f70112824`. The tag is not moved by this
post-publication receipt. RC1 tag/evidence stay unchanged; latest stable remains
v0.3.5-24. Application0.4.0-rc.2, Windows0.4.0.2, macOS0.4.0/build4002,
and Linux0.4.0~rc.2-1 are verified.

## Validation

- [Local validation](local-validation.json): full Go/race131passing packages and
  four without tests, vet, frontend state/build,41portable Node receipt/probe
  cases each on Mac and disconnected Docker,20tagged publisher race repeats,
  actual signed-bootstrap RC2 activation, resource/version checks and an arm64
  app with exact frontend build identity. Wails regenerated an unused Windows
  icon in the owned Mac build checkout; only that generated change was restored.
- [Hosted validation](hosted-validation.json): all19jobs across CI, Engine Native,
  Windows GUI x64/x86, Windows COM x64/x86 and three-platform packaging preflight
  pass on the exact candidate. [Three-OS race logs](hosted-race-logs.json) verify
  131packages per full race step, with complete job-log digests. Native publisher
  unit tests pass on Windowsamd64/386; changed full UI runner acceptance remains.
- [Offline summary](offline-summary.json):70disconnected CLI/manual-helper
  operations,0target Internet syscalls,2failed-connect positive controls.
  This is not a Windows/desktop outbound claim. Raw models remain private.
- [Preparation history](preparation-history.json) retains the failed d74b7953
  baseline and cancelled a8940361 preparation attempts, superseded by the final
  exact-candidate checks. No failed/cancelled run is relabeled PASS.

The preceding native386 publisher test failed on Win32 sharing violation32
while final-file rename completed. RC2 retries Windows EBUSY within the original
bounded reader deadline. The Go test tolerates only missing/transient Win32
sharing errors and requires eventual exact complete bytes. Permanent busy files
still time out, exited children fail, and EACCES/EPERM fail immediately. libuv's
[official mapping](https://raw.githubusercontent.com/libuv/libuv/v1.x/src/win/error.c)
identifies sharing/lock violations as EBUSY. Original23-case remediation receipts
are historical and unchanged. [Initial preparation](local-preparation.json)
records the then-running local full race; the final result is in local-validation.

## Published assets

[Preflight assets](preflight-assets.json) were independently checked before
making the tag. The [tag-triggered publication run](publication-run.json) then
passed all four jobs. [Published metadata](published-release.json) confirms a
public prerelease, not a draft. All11assets were downloaded and independently
verified: [artifact receipt](published-assets.json), [SHA256SUMS](SHA256SUMS).
All11GitHub asset digests and all10checksum entries match. ZIP CRCs pass.

[macOS verification](published-macos.json) checks arm64, short0.4.0/build4002,
DMG checksum, identical DMG/ZIP binaries and deep/strict ad-hoc signatures on
both. This is not Developer ID/notarization qualification. Windows PE resources
show0.4.0.2/0.4.0-rc.2 on app and installer; the x86 NSIS stub contains the x64
app, not an x86 app distribution. Windows files are not Authenticode-signed by
the workflow. Linux TAR keeps executable mode and the DEB/RPM contain its exact
x64 binary at /usr/local/bin/archscope with version0.4.0~rc.2-1. All133docs/example
files, both release notes and the synthetic conformance corpus match the tag.
These checks do not establish clean-host installation or actual upgrade.

## Remaining scope

All251numbered task states remain243DONE/5REVIEW/1PENDING/2TODO:
T-601/T-611/T-627/T-663/T-664 REVIEW, T-613 PENDING, T-654/T-662 TODO.
Ordinary body-rich imports still fail4MiB/cycle on both Windows architectures;
no allocator cause or production correction has been established. Actual new
collector/full UI runner execution, full IME/keyboard/OS-DPI/fluent-language/
reference/vendor/driver, clean-host installation/actual upgrade, GPU/natural
pressure and independent acceptance remain scoped gates. Existing signed
bootstrap activation passes with RC2; its before-stable0.4.0 upper bound and
signed bytes are unchanged. Stable0.4.0 needs a newly qualified compatible pack.

[English notes](../../../en/releases/v0.4.0-rc.2.md) ·
[Korean notes](../../../ko/releases/v0.4.0-rc.2.md).
