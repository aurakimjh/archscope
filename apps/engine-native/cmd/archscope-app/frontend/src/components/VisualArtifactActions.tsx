import { useEffect, useRef, useState } from "react";
import { Dialogs } from "@/bridge/dialogs";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { ResultRequestScope, safeDisplayText } from "@/state/resultProviders";
import { addEvidenceCard } from "@/state/evidenceBoard";
import * as visual from "@/bridge/visuals";

export function VisualArtifactActions({ handle, target = "", query, title, availableColumns }: { handle: string; target?: string; query: visual.VisualQuery; title: string; availableColumns?: string[] }): React.JSX.Element {
  const { locale } = useI18n(); const w = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const scope = useRef(new ResultRequestScope()); const [caps, setCaps] = useState<visual.VisualCapabilities | null>(null);
  const [presentation, setPresentation] = useState<visual.VisualPresentation>({ width: 2400, height: 1600, font_size: 26, rows_per_page: 20, transparent: false, title, annotation: "" });
  const [artifact, setArtifact] = useState<visual.VisualDescriptor | null>(null); const [preview, setPreview] = useState<visual.VisualPreview | null>(null); const [page, setPage] = useState(0);
  const [busy, setBusy] = useState(false); const [message, setMessage] = useState(""); const [error, setError] = useState("");
  useEffect(() => { let alive = true; void visual.capabilities().then(raw => { if (alive) setCaps(JSON.parse(raw) as visual.VisualCapabilities); }).catch(() => {}); return () => { alive = false; scope.current.cancel(); }; }, []);
  const [columns, setColumns] = useState<string[]>(query.columns ?? []);
  const columnsKey = JSON.stringify(query.columns ?? []);
  useEffect(() => setColumns(query.columns ?? []), [columnsKey]);
  const selectedQuery = availableColumns ? { ...query, columns } : query;
  const selectionKey = JSON.stringify([handle, target, selectedQuery, presentation]);
  useEffect(() => { scope.current.cancel(); setArtifact(null); setPreview(null); setPage(0); setBusy(false); setMessage(""); setError(""); }, [selectionKey]);
  async function run(action: (revision: number) => Promise<void>): Promise<void> { const r = scope.current.begin(); setBusy(true); setError(""); setMessage(""); try { await action(r); } catch (e) { if (scope.current.current(r)) setError(String(e)); } finally { if (scope.current.current(r)) setBusy(false); } }
  async function create(): Promise<void> { await run(async r => { const value = JSON.parse(await scope.current.wait(r, visual.render(JSON.stringify({ handle, target, query: selectedQuery, presentation })))) as visual.VisualDescriptor; setArtifact(value); setPage(0); setPreview(JSON.parse(await scope.current.wait(r, visual.preview(value.id, 0))) as visual.VisualPreview); window.dispatchEvent(new CustomEvent("archscope:visual-artifacts-changed")); }); }
  async function changePage(next: number): Promise<void> { if (!artifact) return; await run(async r => { setPreview(JSON.parse(await scope.current.wait(r, visual.preview(artifact.id, next))) as visual.VisualPreview); setPage(next); }); }
  async function save(format: string): Promise<void> { if (!artifact) return; await run(async r => { const path = await Dialogs.SaveFile({ Title: w("Save new visual file", "새 시각 자료 파일 저장"), Filename: `${artifact.id.slice(0, 20)}-${page + 1}.${format}` }); if (!path || !scope.current.current(r)) return; await scope.current.wait(r, visual.exportArtifact(artifact.id, page, format, path)); setMessage(w("Saved.", "저장했습니다.")); }); }
  async function copy(kind: string): Promise<void> { if (!artifact) return; await run(async r => { await scope.current.wait(r, visual.copy(artifact.id, page, kind)); setMessage(w("Copied to the native clipboard. Paste in your report application.", "OS 클립보드에 복사했습니다. 보고서 앱에서 붙여넣으세요.")); }); }
  return <details className="rounded border border-border p-3"><summary>{w("Report image and table", "보고서용 이미지·표")}</summary>
    <p className="text-xs">{w("This fixed view pins its result, query, unit, denominator and quality. Images include the selected row range; long labels are shortened in images and retained in the table. PNG is the primary paste format; SVG insertion depends on the report application.", "결과·조회·단위·분모·품질을 고정합니다. 이미지에 선택 행 범위를 표시하고 긴 이름은 줄이되 표에는 보존합니다. PNG를 기본 복사 형식으로 사용하며 SVG 삽입은 보고서 앱에 따라 다릅니다.")}</p>
    {availableColumns && <fieldset disabled={busy} className="flex flex-wrap gap-2"><legend>{w("Report columns (1–8)", "보고서 열 선택 (1–8)")}</legend>{availableColumns.map(column => <label key={column}><input type="checkbox" disabled={!columns.includes(column) && columns.length >= 8} checked={columns.includes(column)} onChange={e => setColumns(value => e.target.checked ? [...value, column] : value.filter(name => name !== column))} />{safeDisplayText(column)}</label>)}</fieldset>}
    <fieldset disabled={busy} className="flex flex-wrap gap-2 my-2">
      <input aria-label={w("Visual title", "그림 제목")} value={presentation.title} onChange={e => setPresentation(p => ({ ...p, title: e.target.value }))} />
      <select aria-label={w("Image width", "이미지 폭")} value={presentation.width} onChange={e => setPresentation(p => ({ ...p, width: Number(e.target.value) }))}>{[1600, 2400, 3200].map(width => <option key={width} value={width}>{width}px</option>)}</select>
      <label>{w("Height", "높이")} <input aria-label={w("Image height", "이미지 높이")} type="number" min={800} max={2400} value={presentation.height} onChange={e => setPresentation(p => ({ ...p, height: Number(e.target.value) }))} className="w-24" /></label>
      <label>{w("Font", "글자 크기")} <input type="number" min={20} max={48} value={presentation.font_size} onChange={e => setPresentation(p => ({ ...p, font_size: Number(e.target.value) }))} className="w-20" /></label>
      <label>{w("Rows per image", "이미지당 행")} <input type="number" min={5} max={25} value={presentation.rows_per_page} onChange={e => setPresentation(p => ({ ...p, rows_per_page: Number(e.target.value) }))} className="w-20" /></label>
      <label><input type="checkbox" checked={presentation.transparent} onChange={e => setPresentation(p => ({ ...p, transparent: e.target.checked }))} /> {w("Transparent background", "투명 배경")}</label>
      <input aria-label={w("Visual annotation", "그림 주석")} value={presentation.annotation} onChange={e => setPresentation(p => ({ ...p, annotation: e.target.value }))} placeholder={w("Optional annotation (does not change values)", "선택 주석 (통계 값은 바뀌지 않음)")} />
      <Button disabled={!caps?.renderer} onClick={() => void create()}>{w("Render pinned view", "선택 고정 후 이미지 생성")}</Button>
    </fieldset>
    {caps && !caps.renderer && <p>{w("A native desktop render session is required.", "네이티브 데스크톱 렌더 세션이 필요합니다.")}</p>}
    {artifact && <><p className="text-xs">{w("Stored / expanded bytes", "저장 / 해제 byte")}: {artifact.bundle_bytes ?? "?"} / {artifact.expanded_bytes ?? "?"} · {w("Fonts are embedded per SVG. Library: 32 items / 512 MiB; Report Pack: 192 MiB expanded including analysis.", "글꼴은 SVG마다 포함합니다. 보관함: 32건 / 512 MiB; Report Pack: 분석 부록 포함 해제 192 MiB.")}</p>{(preview?.table_protection||artifact.table_protection)&&<p className="text-xs">{w("Table text protection changed cells (TSV / HTML / formula headers)","표 텍스트 보호로 달라진 셀 수 (TSV / HTML / 수식형 머리글)")}: {(preview?.table_protection||artifact.table_protection)!.tsv_changed_cells} / {(preview?.table_protection||artifact.table_protection)!.html_changed_cells} / {(preview?.table_protection||artifact.table_protection)!.formula_headers}</p>}<p className="break-all text-xs">{artifact.id} · {page + 1}/{artifact.pages} · {artifact.spec.view_sha256}</p>
      {preview && <img src={preview.png} alt={safeDisplayText(artifact.title)} className="max-w-full max-h-[32rem] object-contain border border-border" />}
      <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={busy || page === 0} onClick={() => void changePage(page - 1)}>{w("Previous image", "이전 이미지")}</Button><Button variant="outline" disabled={busy || page + 1 >= artifact.pages} onClick={() => void changePage(page + 1)}>{w("Next image", "다음 이미지")}</Button>
        <Button disabled={busy || !caps?.native_clipboard} onClick={() => void copy("image")}>{w("Copy image", "이미지 복사")}</Button><Button disabled={busy || !caps?.native_clipboard} onClick={() => void copy("table")}>{w("Copy complete selected table", "선택한 표 전체 복사")}</Button>
        {["png", "svg", "tsv", "html", "zip"].map(format => <Button key={format} variant="outline" disabled={busy} onClick={() => void save(format)}>{format === "zip" ? w("Save all images and provenance", "모든 이미지·출처 묶음 저장") : `${format.toUpperCase()} ${w("save", "저장")}`}</Button>)}
        <Button variant="outline" disabled={busy} onClick={() => { addEvidenceCard({ analyzer: "visual_artifact", source_kind: "visual_artifact", title: artifact.title, source_ref: artifact.spec.view_sha256, summary: w("Pinned rendered snapshot", "고정된 렌더 스냅샷"), payload: { artifact_id: artifact.id, view_sha256: artifact.spec.view_sha256, result_sha256: artifact.spec.sources.map(s => s.sha256) } }); setMessage(w("Added to Evidence Board.", "Evidence Board에 보관했습니다.")); }}>{w("Keep on Evidence Board", "Evidence Board에 보관")}</Button>
      </div><p className="text-xs">{w("Copy table includes every row in this pinned selection, across images. HTML requests text formatting and protects formula-like cells. TSV selectively protects formulas, numeric fractions/dates, scientific notation and long numbers. Paths/URLs and ordinary headers retain their strings. Review before sharing.", "표 복사는 여러 이미지에 나뉜 선택 행 전체를 포함합니다. HTML은 텍스트 서식과 수식형 셀 보호를 요청합니다. TSV는 수식·숫자 분수/날짜·지수 표기·긴 숫자만 선택 보호합니다. 경로/URL과 일반 머리글은 원래 문자열을 유지합니다. 공유 전에 내용을 확인하세요.")}</p>
    </>}
    {busy && <Button variant="outline" onClick={() => { scope.current.cancel(); setBusy(false); setMessage(w("Cancellation requested. Refresh the visual library if a save already finished.", "취소를 요청했습니다. 이미 저장이 끝났다면 시각 자료 보관함을 새로 고침하세요.")); }}>{w("Cancel", "취소")}</Button>}
    {message && <p role="status">{message}</p>}{error && <p role="alert">{safeDisplayText(error)}</p>}
  </details>;
}
