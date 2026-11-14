# Windows remaining-test follow-up

This is an incremental scoped receipt. T-613 remains PENDING and T-627 REVIEW;
the original independent CONDITIONAL review is unchanged. Production import
memory and passive layout-audit native UI are separately identified below.

## 2. Unicode notes and editing guards

Native Windows amd64 layout-audit build at adfc9544, with the ordinary production
frontend and passive observer, opens a synthetic32-row session. A selected star
and exact33UTF-8-byte Hangul/emoji/combining-accent note survive native Save,
CtrlS to a new file and controller-driven native reopen. Full-field comparator
policy1.0.0 verifies all32rows and selection; absent false/empty annotations are
the declared Go omitempty defaults. The original source is unchanged.

CtrlS while the textarea is focused does not open a dialog; after clicking the
grid it opens exactly one Save dialog. Native file-name keyboard entry/Enter
completes the save. The modal accessibility indexes were not actionable through
the helper, so observed Alt+N/keyboard entry was used instead.

Two column-editor native drag attempts did not establish reordering; they are
INCONCLUSIVE, not PASS or an established product defect. The helper rejects the
Hangul composition key, so actual IME composition is still unverified. Literal
type_text duplicated a Unicode delivery; it is excluded as an input-fidelity
claim. Exact set_value plus native Save and independent file comparison establish
the final value, not ordinary IME/paste fidelity. Existing4096-byte boundary,
redaction-growth and persistence-failure receipts remain unchanged.

Files: notes-unicode-*.jpg/txt, columns-drag-attempt.*, unicode-readback.json,
native-cases.json. Private controllers, tokens and session files stay ignored.

## 1. Import memory and reentry (measurement continuing)

The initial10-cycle32/512/4096-row import screen exceeded tree growth budgets on
both production architectures. It is retained as GROWTH_REQUIRES_DIAGNOSIS, not
retrospectively relabeled PASS. Native Workspace shows30result cards after30
imports. The workspace intentionally retains up to80results; a separately
declared36-cycle run warms up28cycles before8measured endpoints to cross that
limit. Its result will be recorded after completion. No GC/cache trim is forced.

Native amd64 Workspace→Capture reentry restores32rows, selected first request and
Raw200. The route transition is separate from the automatic matched samples.
