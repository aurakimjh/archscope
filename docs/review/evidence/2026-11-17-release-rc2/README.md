# v0.4.0-rc.2 release receipt

Preparation baseline: d74b7953. Version/tag: v0.4.0-rc.2; Windows0.4.0.2;
macOS short0.4.0/build4002; LinuxRC2. RC1 notes/tag/evidence remain unchanged.
The user authorized this prerelease while native acceptance remains open.

Status: PREPARING. Local/hosted/package/publication evidence will be added as
checks complete; this receipt does not itself assert publication.

The preceding native386 job133690527741 at d74b7953 failed the new tagged
publication test on Win32 sharing violation32 while the final file was renamed.
RC2 retries Windows EBUSY in the reader within the existing bounded deadline;
the Go test tolerates only ENOENT and transient Windows sharing violations while
requiring eventual exact complete bytes. Permanent busy files time out, exited
children fail and EACCES/EPERM remain immediate errors. libuv's
[official Windows mapping](https://raw.githubusercontent.com/libuv/libuv/v1.x/src/win/error.c)
identifies sharing/lock violations as EBUSY. Original failure receipts and the
prior23-case local remediation evidence remain historical, unmodified records.

The original signed bootstrap is tested with RC2; its before-stable0.4.0 upper
bound and signed bytes are unchanged. All251numbered task states remain:
243DONE/5REVIEW/1PENDING/2TODO. T-601/T-611/T-627/T-663/T-664 are REVIEW,
T-613 PENDING, T-654/T-662 TODO. Ordinary body-rich memory still fails its
original4MiB/cycle slope limit on both Windows architectures; no allocator cause
or production correction has been established. Collector execution, full native
UI/IME/DPI/language/reference/vendor/driver, actual installation/upgrade,
GPU/natural-pressure and independent acceptance remain scoped gates.

See the [English notes](../../../en/releases/v0.4.0-rc.2.md) and
[Korean notes](../../../ko/releases/v0.4.0-rc.2.md) for exact support and limitations.
