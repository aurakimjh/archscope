# T-663 / T-664 local validation

Date: 2026-11-16. This is the initial, pre-commit implementation snapshot, not
an independent review. Later same-day UI tests and scoped commit/push were
requested by the user; see [the follow-up UI receipt](../2026-11-16-ribbon-ui/README.md).
This folder's source hashes retain the earlier snapshot, before the popup fix.
Existing unrelated HTTP documentation, verifier and tracker history are preserved.

## Checked scope

- Frontend `npm run test:state` and `npm run build`: PASS. Language negotiation,
  five-catalog key/placeholder parity, Taiwan terms, ribbon preference recovery
  and native/keyboard/ribbon command dispatch regressions are covered.
- `python3 scripts/check-desktop-i18n.py` and `python3 -O` execution: PASS;
  2,197 required phrases in each of ja, zh-CN and zh-TW, with no missing entries,
  duplicate source keys, empty values or modified placeholders.
- Desktop-only guard and native smoke script syntax: PASS. The application
  remains a Wails desktop app; no browser preview exception was introduced.
- `go test -race ./cmd/archscope-app`: PASS (89.946 seconds). Final catalog menu
  tests and `go vet ./cmd/archscope-app`: PASS. The Go source changes are limited
  to desktop menu state, labels, availability and shortcut ownership.
- Production Windows amd64 and 386 cross-builds: PASS. Actual Windows UI,
  keyboard, font, IME and OS DPI acceptance is still required.
- Production Mac build with the existing explicit `layoutaudit` tag: PASS.
  The local SDK emitted macOS 27.2 versus minimum 11.0 linker warnings.
- Task registry: 251 unique entries, 243 DONE, 5 REVIEW, 1 PENDING and 2 TODO.
  T-663/T-664 remain REVIEW; existing acceptance tasks keep their statuses.

The locale data lives in one explicit translations chunk (approximately 522 kB
before gzip), leaving the startup index around 371 kB and the unchanged largest
ECharts chunk around 698 kB, within the existing 700 kB warning budget. No runtime
translation service or new application dependency was added.

## Native smoke provenance

The maintained `scripts/verify-ribbon-native.js` runs inside the real native
WebView using the existing layout audit mechanism. The app uses an owned
temporary state root and an explicit local automation descriptor, whose token
and private paths are excluded from repository evidence. No capture is started
and no private input is imported. Preference values are restored before exit.

The receipt checks five language choices, persistence, one tab row, command tab
switching without remounting the current Settings page, File/Back preservation,
disabled HTTP actions outside the HTTP page, collapse, temporary expansion,
Escape, hide/restore and restore-button focus. This is scoped renderer evidence,
not the full menu/OS shortcut, analyzer, DPI or screenshot acceptance matrix.

Earlier harness attempts are retained in `before-harness-readiness.json` and
`before-harness-text-read.json`: the first queried the previous React tab before
the state update, and the second relied on rendered `innerText` in the inactive
test window. Explicit state waits and `textContent` corrected those harness
assumptions. An initial animation-frame wait also stalled in this environment;
the smoke uses bounded timer waits so it can finish in an inactive native window.
`before-sizing.json` records the subsequent passing smoke with a 138 px ribbon;
its test build overlaid only debug logger setup in main.go. The duplicated
language label was then removed to reduce the height. Final receipt provenance
and results are in `local.json` and `native-final.json`.

Computer-use access to the native window returned `cgWindowNotFound`; no visual
screenshot or manual readability PASS is claimed. Native renderer execution is
reported separately from this observation limit.

## Remaining review

Japanese and Simplified Chinese are initial UI translations. Traditional Chinese
started with an offline OpenCC s2twp conversion and explicit Taiwan UI wording
adjustments; the development-only converter is not an application dependency.
Independent fluent-speaker review remains required. Manuals stay in separate
English/Korean files and the added UI languages link to the English manual.
Original technical identifiers and imported/engine-produced evidence remain
unchanged. The common AnalysisResult contract is untouched.

Reproduce from the repository root with the commands above (Go commands from
`apps/engine-native`, npm commands from its Wails frontend directory). Build the
native app using `go build -tags 'production,layoutaudit' ./cmd/archscope-app`,
then run it with a disposable state root, `ARCHSCOPE_LAYOUT_SCRIPT` pointing to
the maintained smoke and `ARCHSCOPE_LAYOUT_OUT` pointing to a private receipt.
The ordinary product binary excludes this audit mechanism.
