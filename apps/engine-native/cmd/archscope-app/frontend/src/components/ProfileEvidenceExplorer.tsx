import { VisualArtifactActions } from "@/components/VisualArtifactActions";
import type { VisualQuery } from "@/bridge/visuals";
import { useEffect, useRef, useState } from "react";
import * as bridge from "@/bridge/engine";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { ResultRequestScope, safeDisplayText } from "@/state/resultProviders";
import { addWorkspaceResult, getWorkspaceGeneration, type SavedCaseReference } from "@/state/analysisWorkspace";

export function ProfileEvidenceExplorer({ handle, caseRef }: { handle: string; caseRef?: SavedCaseReference }): React.JSX.Element {
  const { locale } = useI18n(); const w = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const scope = useRef(new ResultRequestScope());
  const [inventory, setInventory] = useState<bridge.ProfileQueryInventory | null>(null);
  const [selection, setSelection] = useState<bridge.ProfileSelection>({ profile_ids: [], metric_id: "" });
  const [sort, setSort] = useState("self");
  const [view, setView] = useState("top"); const [frame, setFrame] = useState(""); const [text, setText] = useState("");
  const [filters, setFilters] = useState("[]"); const [from, setFrom] = useState(""); const [to, setTo] = useState("");
  const [original, setOriginal] = useState<Awaited<ReturnType<typeof bridge.openProfileEvidence>> | null>(null);
  const [visualQuery, setVisualQuery] = useState<VisualQuery | null>(null);
  const [page, setPage] = useState<bridge.ProfileQueryPage | null>(null); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  useEffect(() => { const r = scope.current.begin(); setInventory(null); setPage(null); setOriginal(null); setBusy(true); setError(""); void scope.current.wait(r, bridge.describeProfileQuery(handle)).then(value => { setInventory(value); setSelection(value.selection); setFilters(JSON.stringify(value.selection.filters ?? [])); }).catch(e => { if (scope.current.current(r)) setError(String(e)); }).finally(() => { if (scope.current.current(r)) setBusy(false); }); return () => scope.current.cancel(); }, [handle]);
  const key = JSON.stringify([selection, view, sort, frame, text, filters, from, to]);
  useEffect(() => { if (!inventory) return; scope.current.cancel(); setPage(null); setOriginal(null); setBusy(false); }, [key]);
  async function query(cursor = ""): Promise<void> {
    if (!inventory) return; const r = scope.current.begin(); setBusy(true); setError("");
    try {
      // Filter paths are IDs only. No weights or unknown metadata pass through JS.
      const q = { selection: { ...selection, filters: JSON.parse(filters) as unknown }, view, ...(view === "top" ? { sort } : {}), frame_id: frame, text, cursor, limit: 25, ...(from ? { time_from: from } : {}), ...(to ? { time_to: to } : {}) };
      const value = await scope.current.wait(r, bridge.queryProfileEvidence(handle, inventory.result_sha256, JSON.stringify(q)));
      if (value.version !== "1.0.0" || value.result_sha256 !== inventory.result_sha256) throw new Error("stale_profile_query"); setPage(value); setVisualQuery({ kind: view === "tree" ? "profile_flamegraph" : "profile_top", offset: 0, limit: 25, profile: q });
    } catch (e) { if (scope.current.current(r)) setError(String(e)); }
    finally { if (scope.current.current(r)) setBusy(false); }
  }
  async function materialize(): Promise<void> {
    if (!inventory) return; const r = scope.current.begin(); const generation = getWorkspaceGeneration(); setBusy(true); setError("");
    try {
      // Allow creation to settle so a late owned result can always be released.
      const value = JSON.parse(await bridge.materializeProfileView(handle, inventory.result_sha256, JSON.stringify({ ...selection, filters: JSON.parse(filters) as unknown }))) as { view: { handle: string; type: string }; lease: { id: string } };
      if (!scope.current.current(r) || generation !== getWorkspaceGeneration()) { await bridge.releaseEvidenceLease(value.lease.id); return; }
      addWorkspaceResult({ backendResultHandle: value.view.handle, backendLeaseID: value.lease.id, result: { type: value.view.type, summary: {} }, title: w("Selected profile", "선택한 프로파일") });
    } catch (e) { if (scope.current.current(r)) setError(String(e)); }
    finally { if (scope.current.current(r)) setBusy(false); }
  }
  async function openSource(row: bridge.ProfileQueryPage["rows"][number]): Promise<void> {
    if (!caseRef || !inventory || !row.source_ref) return; const r = scope.current.begin(); setBusy(true); setError("");
    try { const m = JSON.parse(await bridge.getCase(caseRef.case_id)) as bridge.CaseEvidenceManifest; if (!scope.current.current(r)) return; const result = m.results.find(item => item.id === caseRef.result_id && item.sha256 === inventory.result_sha256); if (!result) throw new Error("case_result_stale"); const sources = m.sources.filter(item => result.source_ids.includes(item.id) && row.source_sha256?.includes(item.sha256)); if (sources.length !== 1) throw new Error("original_reference_missing_or_ambiguous"); setOriginal(await scope.current.wait(r, bridge.openProfileEvidence(m.id, m.revision, result.id, inventory.result_sha256, sources[0].id, sources[0].sha256, row.source_ref))); }
    catch (e) { if (scope.current.current(r)) setError(String(e)); } finally { if (scope.current.current(r)) setBusy(false); }
  }
  return <details className="rounded border border-border p-3"><summary>{w("Canonical profile drilldown and evidence", "공통 프로파일 drilldown과 근거")}</summary>
    {inventory && <>
      <p className="text-xs">{w("Search, caller/callee and time-window rows share the selected denominator. Time windows filter evidence rows in the profile's declared clock; they do not align clocks or infer sampled timestamps.", "검색·caller/callee·시간창 행은 선택한 분모를 공유합니다. 시간창은 선언된 profile clock의 근거 행만 필터링하며 시계 정렬이나 sampled 시각을 추정하지 않습니다.")}</p>
      <fieldset disabled={busy} className="space-y-2">
        <select aria-label={w("Profile metric", "프로파일 metric")} value={selection.metric_id} onChange={e => setSelection(s => ({ ...s, metric_id: e.target.value }))}>{inventory.metrics.map(m => <option key={m.id} value={m.id}>{safeDisplayText(`${m.id} · ${m.source_type ?? ""}/${m.source_unit ?? ""} · ${m.kind} · ${m.unit}`)}</option>)}</select>
        {inventory.profiles.map(p => <label className="block" key={p.id}><input type="checkbox" checked={selection.profile_ids.includes(p.id)} onChange={e => setSelection(s => ({ ...s, profile_ids: e.target.checked ? [...s.profile_ids, p.id] : s.profile_ids.filter(id => id !== p.id) }))} /> {safeDisplayText(p.worker || p.id)} · {safeDisplayText(p.process || "")}</label>)}
        <label className="block">{w("Ordered conjunctive frame-ID filters (JSON)", "순서 있는 frame-ID 결합 필터 (JSON)")}<textarea className="w-full rounded border border-border bg-background p-2" value={filters} onChange={e => setFilters(e.target.value)} /></label>
        <select aria-label={w("Profile view", "프로파일 보기")} value={view} onChange={e => setView(e.target.value)}>{["top", "tree", "callers", "callees", "evidence"].map(id => <option key={id}>{id}</option>)}</select>
        {view === "top" && <select aria-label={w("Top sort", "Top 정렬")} value={sort} onChange={e => setSort(e.target.value)}><option value="self">Top self</option><option value="inclusive">Top inclusive</option></select>}
        <input aria-label={w("Frame ID", "Frame ID")} value={frame} onChange={e => setFrame(e.target.value)} placeholder={w("Frame ID for callers/callees/evidence", "callers/callees/evidence의 frame ID")} className="w-full" />
        <input aria-label={w("Profile search", "프로파일 검색")} value={text} onChange={e => setText(e.target.value)} placeholder={w("Literal frame / ID / reference search", "frame·ID·근거 참조 문자열 검색")} />
        <input aria-label={w("Evidence window start", "근거 시간창 시작")} value={from} onChange={e => setFrom(e.target.value)} placeholder={w("Start (evidence only)", "시작 (evidence만)")} /> <input aria-label={w("Evidence window end", "근거 시간창 끝")} value={to} onChange={e => setTo(e.target.value)} placeholder={w("Exclusive end", "끝 (미포함)")} />
        <Button disabled={!selection.profile_ids.length} onClick={() => void query()}>{w("Query selection", "선택 조회")}</Button>{" "}<Button variant="outline" disabled={!selection.profile_ids.length} onClick={() => void materialize()}>{w("Add selection as new result", "선택을 새 결과로 추가")}</Button>
      </fieldset>
    </>}
    {busy && <Button variant="outline" onClick={() => { scope.current.cancel(); setBusy(false); }}>{w("Cancel", "취소")}</Button>}
    {page && <>
      {visualQuery && <VisualArtifactActions handle={handle} title={w("Selected profile", "선택한 프로파일")} query={visualQuery} />}<p>{w("Selected total", "선택 전체 합계")}: {page.denominator} {page.metric.unit} · {safeDisplayText(page.availability)} · {page.matched}/{page.total} · {w("Untimed", "시각 없음")}: {page.untimed_observations}</p>
      {view === "tree" && <div className="max-h-96 overflow-auto"><p className="text-xs">{w("Paged flamegraph: hidden nodes keep their original space. Areas are approximate layout; labels retain exact selected values. Click a frame to focus its path.", "페이지 단위 flamegraph: 숨긴 노드의 공간을 유지합니다. 면적은 표시용 근삿값이며 값은 정확한 선택 결과입니다. Frame을 누르면 경로를 focus합니다.")}</p><svg role="img" aria-label={w("Selected profile flamegraph", "선택한 프로파일 flamegraph")} viewBox={`0 0 1000 ${Math.max(1, ...page.rows.map(row => row.depth + 1)) * 28}`} className="w-full" style={{ minHeight: 56 }}>{page.rows.filter(row => row.layout_x != null && row.layout_width != null && row.layout_width > 0).map(row => <g key={row.id} onClick={() => { if (row.path) setFilters(JSON.stringify([...(JSON.parse(filters) as object[]), { path: row.path, match: "prefix" }])); }}><title>{safeDisplayText(row.name || row.id)} · {row.inclusive} {page.metric.unit} · self {row.self}</title><rect x={row.layout_x! * 1000} width={row.layout_width! * 1000} y={row.depth * 28} height={26} fill={`hsl(${(row.depth * 43) % 360} 65% 50%)`} stroke="currentColor" /><text x={row.layout_x! * 1000 + 3} y={row.depth * 28 + 18} fontSize={12} style={{ pointerEvents: "none" }}>{row.layout_width! > 0.08 ? safeDisplayText(row.name || "").slice(0, Math.floor(row.layout_width! * 120)) : ""}</text></g>)}</svg></div>}
      <code className="break-all text-xs">{page.population} · {page.query_sha256}</code>
      <div className="max-h-96 overflow-auto"><table className="w-full text-sm"><thead><tr><th>{w("Frame / evidence", "Frame / 근거")}</th><th>{w("Inclusive", "포함 값")}</th><th>{w("Self", "자체 값")}</th><th>{w("Original coordinate", "원본 좌표")}</th></tr></thead><tbody>{page.rows.map(row => <tr key={row.id}><td className="max-w-96 break-words"><button onClick={() => { if (row.frame_id) setFrame(row.frame_id); }}><bdi>{safeDisplayText(row.name || row.id)}</bdi></button>{row.frame_id && <code className="block text-xs">{row.frame_id}</code>}{row.path && <Button size="sm" variant="outline" onClick={() => { setFilters(JSON.stringify([...(JSON.parse(filters) as object[]), { path: row.path, match: "prefix" }])); setFrom(""); setTo(""); }}>{w("Focus path", "경로 focus")}</Button>}</td><td>{row.inclusive}</td><td>{row.self}</td><td className="max-w-72 break-all">{safeDisplayText(row.source_ref || "")} {row.timestamp ?? ""}<small className="block">{row.source_sha256?.join(", ")}</small>{caseRef && row.source_ref?.startsWith("line:") && <Button size="sm" variant="outline" disabled={busy} onClick={() => void openSource(row)}>{w("Open case original", "케이스 원본 열기")}</Button>}</td></tr>)}</tbody></table></div>
      <Button variant="outline" disabled={busy || !page.next} onClick={() => void query(page.next)}>{w("Next", "다음")}</Button>
    </>}
    {original && <div><p>SHA-256 {original.source_sha256}</p>{original.lines.map(line => <pre key={line.number} className="overflow-auto text-xs">{line.number} [{line.raw_start}..{line.raw_end}]: {safeDisplayText(line.text)}</pre>)}</div>}
    {error && <p role="alert">{safeDisplayText(error)}</p>}
  </details>;
}
