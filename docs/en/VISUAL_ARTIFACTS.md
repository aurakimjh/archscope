# Pinned report images and tables

T-636 implements a desktop visual renderer over existing immutable analysis
queries. It creates PNG images, self-contained SVG, editable HTML tables and TSV,
and retains a portable artifact ZIP. This is separate from slide authoring and
from the existing `SaveChartArtifact` API, which only saves already-rendered bytes.
Implementation/local verification is complete; independent review and native
Office acceptance remain open. See the paired [Korean guide](../ko/VISUAL_ARTIFACTS.md).

## Use the selected analysis view

Open **Report image and table** below a generic result page, ProfileSet drilldown
or an eligible strict profile comparison. The current backend query/page is
frozen, rather than the visible screen DOM. Generic tables initially select up
to eight columns; choose the required columns explicitly. Choose title,
annotation, width (1600/2400/3200), height, font, rows per image and optional
transparent background, then **Render pinned view**. Invalid combinations return
an actionable limit; reduce rows or increase height. Defaults are 2400×1600,
26 px, 20 rows per image and a white background.

| Template | Qualified input and limitations |
|---|---|
| `provider_table` | Existing available generic table/summary/detail provider, selected page and at most eight columns. Units/availability remain per cell; no common denominator is invented. |
| `profile_top` | Existing exact Top/self, callers/callees or evidence query with its metric, filter, selection, sort and cursor. |
| `profile_flamegraph` | Nonnegative existing tree layout. Inclusive/self strings remain authoritative; geometry is approximate. Only the selected query page/depth fits; omitted nodes keep their horizontal space, and the vertical origin follows the selected page. Focus a subtree when needed. Signed/unavailable layouts are refused. |
| `profile_diff` | Existing strict comparison eligibility, separate baseline/target result digests, exact delta strings and a shared zero-centered delta axis. Mapped/reanalyzed comparisons use their own eligibility flow; this template does not loosen it. |

New GPU/serving/latency producers are future consumers, not already qualified
chart templates. The renderer never parses original logs or computes new
observations. Viewport, scroll, hover, app theme and screen device-pixel ratio do
not change the fixed canvas. A hidden, separate native WebView verifies the
bundled font SHA-256 and waits for font readiness before rasterizing the SVG.
Each job has a 60-second backend deadline, a 50-second surface abort, two-job
admission limit, native window/token binding and cleanup on completion/cancel.

Images show time scope (or explicitly undeclared), unit, denominator, row range,
page count, source/view/query identity, quality summary and limitations. Long
labels/axes can be shortened; exact selected strings and complete context remain
in `view.json` and table files. Selection is bounded to 100 query rows, 200 view
nodes/rows, 8 columns, 5 images, 8 million pixels and a 4 MiB view. It does not
silently claim to export the entire dataset.

## Copy, save and retain

**Copy image** writes native PNG plus Windows DIBV5 or macOS TIFF compatibility
representations. **Copy complete selected table** writes HTML plus Unicode TSV,
including every row in this pinned selection across all split images. Success
appears only after the native clipboard write completes; a busy/unavailable
clipboard returns an error. Save PNG/SVG/HTML/TSV instead when paste is unavailable.
Linux/headless builds explicitly lack native clipboard support.

HTML cells request spreadsheet text formatting. HTML/TSV prefix dangerous
formula-like text, long numeric strings and fractions/date-like strings with a
text-protection apostrophe where needed. This transport protection can be visible
in applications that do not interpret it. Exact original selected strings remain
in `view.json`; no numeric coercion or formula execution is requested. PNG is the
primary Office paste format. SVG insertion depends on the Office/WebView build;
it includes the pinned font, style and OFL license without external references.
Review output before sharing: known-pattern index masking is not a privacy
certification, and explicit query identities/provenance are part of the artifact.

**Keep on Evidence Board** stores an artifact reference. The Board's **Rendered
visual library** stores actual immutable image/spec/view/manifest bytes separately
from case lifecycle, with explicit deletion (32 artifacts, 512 MiB total). Export
its ZIP before moving computers. Identical ZIP bytes deduplicate; saving a new
revision never overwrites an existing snapshot. The Library can import a verified
ZIP, open historical PNG, save the bundle or export 1–4 selected artifacts in a
visual Report Pack. An optional appendix uses existing backend result JSON/
HTML/export contracts for up to eight active results. No original source data or
additional analysis is included implicitly. Managed paths/existing destinations,
unsafe ZIP names, symlinks, missing hashes, modified view/spec and SVG scripts/
external references are refused. Bundles cap at 96 MiB compressed/192 MiB expanded.

## Reproduction and portability

`spec.json` pins source result SHA-256/type/payload version, actual query, view
hash, presentation, template/renderer revision, font hash and masking profile.
`view.json` freezes values, units, denominator, population, quality and warnings.
`manifest.json` hashes every payload, records page count/environment and keeps
PNG/SVG/table/license files together. No transient result handle is persisted.

Historical images always open from saved bytes. **Regenerate pinned revision**
requires the same pinned results reopened in Workspace and the exact current
supported template/renderer/font/masking revisions; missing dependencies or
changed view/presentation are refused. **Upgrade into new revision** is explicit
and links the previous artifact ID while preserving the old bytes. This promises
pinned selection/value/layout inputs, not cross-platform rasterizer byte identity.
Byte equality was observed only under the same Chromium renderer environment.

## CLI

```sh
archscope-engine visual create-spec --in result.json --query '{"kind":"profile_top","offset":0,"limit":25}' --out visual.json
archscope-engine visual validate --in visual.json
archscope-engine visual create-spec --in before.json --target-in after.json --query '{"kind":"profile_diff","offset":0,"limit":25}' --out difference.json
archscope-engine visual inspect --in saved-visual.zip
archscope-engine visual export --in saved-visual.zip --format png --page 0 --out image.png
archscope-engine visual export --in saved-visual.zip --format html --out table.html
```

Spec creation/validation and saved ZIP/PNG/SVG/HTML/TSV export work without a
browser. CLI `visual render` and `visual clipboard` explicitly refuse with
`VISUAL_DESKTOP_RENDERER_REQUIRED` / `VISUAL_CLIPBOARD_UNSUPPORTED`; exporting an
existing SVG/PNG does not generate a new image. Every output uses a new file.

## Native Windows and Office acceptance still required

On the Windows PC, use the packaged desktop and record commit/build ID,
Windows/WebView2/Office versions, canvas/font options and artifact/view hashes.
Use synthetic input containing Korean, long labels, negative deltas,
`9007199254740993`, `1/3`, formula-like text and at least two split images.

1. Generate a profile Top, focused flamegraph, eligible positive/negative diff
   and generic table; verify selected totals/denominators/quality against the
   current backend view. Resize/scroll/change theme and repeat: values, geometry
   and row ranges must remain fixed. Verify readable Korean and transparent PNG
   on a white Office slide; PNG and SVG file dimensions must match the spec.
2. Copy an image from ArchScope and paste into PowerPoint. Confirm a real picture,
   legible labels/units/quality, correct dimensions and transparency. Save/reopen
   the presentation. Insert the saved SVG separately; record unsupported insertion
   honestly and retain PNG as fallback.
3. Copy the complete selected table into Word, PowerPoint and Excel where
   installed. Confirm editable cells, all selected rows/columns, Unicode and
   exact long integers/fractions. Formula-like text must stay text, with the
   disclosed protective apostrophe where the application displays it. Record
   each Office application's actual behavior; one successful paste does not
   qualify every Office version.
4. Cancel rendering/close the initiating view, retry with a busy clipboard,
   save/import ZIP, reopen a historical image and verify hashes. Reopen pinned
   results and create an exact child revision; modify presentation only through
   an explicit new revision. An unsupported old template still opens saved PNG
   and refuses exact regeneration.
5. Disconnect networking and repeat render/copy/save/library/report-pack flows.
   Observe the desktop process tree's outbound behavior independently. CLI
   syscall evidence and cross-builds do not prove native WebView offline behavior.

Local evidence is under `docs/review/evidence/2026-11-12-t636/`. Chromium raster
checks, contract tests, clipboard wire-format tests and cross-builds are not
native Windows/Office acceptance. The Mac desktop was locked during this run.
