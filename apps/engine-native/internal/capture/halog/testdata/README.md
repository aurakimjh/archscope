# V7.6.4 native fixture

`reference-v764.halog` was saved as NativeLog=6 by installed HTTP Analyzer
7.6.4.508 on Windows on 2026-11-11. It contains one synthetic POST to
`http://127.0.0.1:8080/fixture?a=1&a=2`, duplicate request/response headers,
UTF-8 Korean request/response text, a star/comment and 125 ms timing.

The synthetic request was constructed from the observed format schema with
all original field values cleared, then loaded and saved by the reference
application. Cache paths, original URLs, socket/process handles, GUIDs and
payloads from the private source are absent. No real traffic is in this fixture.

Size: 2,201 bytes. SHA-256:
`3f5bba364b478468ae41da4a16cd98f929d1f59a164f0aec6dadf59b807c7b11`.

`reference-v764-states.halog` is a second original-program native re-save of
16 variations of that synthetic record: stage 1/4, SR_Result false/true,
TimeEnd absent/present and RequestComplete tick absent/present. Original COM
`IsComplete` is true for stage 4 or a nonzero RequestComplete tick, independently
of TimeEnd/SR_Result. TimeDuration is 125 ms with a completion tick, otherwise 0.
SHA-256: `40bb5f2d57829fcd28f42d38624eafe3331246ef1e8968f8a10a782f4161b15c`.

The separate 675-entry user sample is intentionally local. Opt-in verification:

```powershell
$env:ARCHSCOPE_HALOG_SOURCE = 'ABS_PRIVATE_SAMPLE.halog'
$env:ARCHSCOPE_HALOG_ROUNDTRIP = 'ABS_PRIVATE_OUTPUT.halog'
$env:ARCHSCOPE_HALOG_PROJECTION = 'ABS_PRIVATE_PROJECTION.json'
go test -v ./internal/capture/halog -run TestPrivateReferenceSample -count=1
```

Run from `apps/engine-native`. The first output contains unredacted original
wire data; the projection contains field/body hashes. Keep both private.
`scripts/verify-halog-codec.ps1` compares that projection against the installed
reference COM API and exercises both native interoperability directions.
