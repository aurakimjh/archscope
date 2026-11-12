import { useEffect, useRef, useState } from "react";
import * as bridge from "@/bridge/engine";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { ResultRequestScope, safeDisplayText } from "@/state/resultProviders";
import { addWorkspaceResult, getWorkspaceGeneration } from "@/state/analysisWorkspace";

export function MetricHistoryExplorer({ handle }: { handle: string }): React.JSX.Element {
  const { locale } = useI18n(); const w = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const scope = useRef(new ResultRequestScope());
  const [inventory, setInventory] = useState<bridge.MetricQueryInventory | null>(null);
  const [ids, setIDs] = useState<string[]>([]); const [from, setFrom] = useState(""); const [to, setTo] = useState("");
  const [page, setPage] = useState<bridge.MetricQueryPage | null>(null); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  useEffect(() => { const r = scope.current.begin(); setInventory(null); setPage(null); setBusy(true); setError(""); void scope.current.wait(r, bridge.describeMetricQuery(handle)).then(value => { setInventory(value); setIDs(value.selection.series_ids); setFrom(value.selection.from_ns ?? ""); setTo(value.selection.to_ns ?? ""); }).catch(e => { if (scope.current.current(r)) setError(String(e)); }).finally(() => { if (scope.current.current(r)) setBusy(false); }); return () => scope.current.cancel(); }, [handle]);
  const key = JSON.stringify([ids, from, to]);
  useEffect(() => { if (!inventory) return; scope.current.cancel(); setPage(null); setBusy(false); }, [key]);
  const selection = (): bridge.MetricSelection => ({ series_ids: ids, ...(from ? { from_ns: from } : {}), ...(to ? { to_ns: to } : {}) });
  async function query(cursor = ""): Promise<void> {
    if (!inventory) return; const r = scope.current.begin(); setBusy(true); setError("");
    try { const value = await scope.current.wait(r, bridge.queryMetricHistory(handle, inventory.result_sha256, JSON.stringify({ selection: selection(), cursor, limit: 25 }))); if (value.version !== "1.0.0" || value.result_sha256 !== inventory.result_sha256) throw new Error("stale_metric_query"); setPage(value); }
    catch (e) { if (scope.current.current(r)) setError(String(e)); } finally { if (scope.current.current(r)) setBusy(false); }
  }
  async function materialize(): Promise<void> {
    if (!inventory) return; const r = scope.current.begin(); const generation = getWorkspaceGeneration(); setBusy(true); setError("");
    try { const value = JSON.parse(await bridge.materializeMetricView(handle, inventory.result_sha256, JSON.stringify(selection()))) as { view: { handle: string; type: string }; lease: { id: string } }; if (!scope.current.current(r) || generation !== getWorkspaceGeneration()) { await bridge.releaseEvidenceLease(value.lease.id); return; } addWorkspaceResult({ backendResultHandle: value.view.handle, backendLeaseID: value.lease.id, result: { type: value.view.type, summary: {} }, title: w("Selected metric history", "선택한 메트릭 이력") }); }
    catch (e) { if (scope.current.current(r)) setError(String(e)); } finally { if (scope.current.current(r)) setBusy(false); }
  }
  return <details className="rounded border border-border p-3"><summary>{w("Exact metric intervals and provenance", "정확한 메트릭 구간과 출처")}</summary>
    <p className="text-xs">{w("Time windows select point end times [start, end). Intervals are retained whole. Rates are exact fractions per second; gaps are not interpolated. Missing timestamps, resets and ambiguous points remain explicit. Series are never summed automatically.", "시간창은 관측 종료 시각 [시작, 끝)을 선택하며 구간 전체를 유지합니다. Rate는 초당 정확한 분수이고 gap을 보간하지 않습니다. 시각 누락·reset·모호한 관측을 표시하며 시계열을 자동 합산하지 않습니다.")}</p>
    {inventory && <fieldset disabled={busy} className="space-y-2">{inventory.series.map(series => <div key={series.id}><label><input type="checkbox" checked={ids.includes(series.id)} onChange={e => setIDs(value => e.target.checked ? [...value, series.id] : value.filter(id => id !== series.id))} /> {safeDisplayText(`${series.name} · ${series.unit} · ${series.kind}/${series.temporality}`)}</label><details><summary>{w("Resource / scope / attributes", "Resource / scope / 속성")}</summary><pre className="overflow-auto text-xs">{safeDisplayText(series.identity_json)}</pre></details></div>)}
      <input aria-label={w("Metric window start nanoseconds", "메트릭 시간창 시작 나노초")} value={from} onChange={e => setFrom(e.target.value)} placeholder={w("Inclusive Unix nanoseconds", "포함 시작 Unix 나노초")} /> <input aria-label={w("Metric window end nanoseconds", "메트릭 시간창 끝 나노초")} value={to} onChange={e => setTo(e.target.value)} placeholder={w("Exclusive Unix nanoseconds", "미포함 끝 Unix 나노초")} />
      <Button disabled={!ids.length} onClick={() => void query()}>{w("Query intervals", "구간 조회")}</Button>{" "}<Button variant="outline" disabled={!ids.length} onClick={() => void materialize()}>{w("Add selection as new result", "선택을 새 결과로 추가")}</Button>
      {inventory.warnings.map((warning, i) => <p key={i} className="text-xs">{safeDisplayText(warning)}</p>)}
    </fieldset>}
    {busy && <Button variant="outline" onClick={() => { scope.current.cancel(); setBusy(false); }}>{w("Cancel", "취소")}</Button>}
    {page && <><p>{page.total} · {page.query_sha256}</p><div className="max-h-96 overflow-auto"><table className="w-full text-sm"><thead><tr><th>{w("Metric / unit", "메트릭 / 단위")}</th><th>{w("Observed value", "관측 값")}</th><th>{w("Interval / rate", "구간 / rate")}</th><th>{w("Status / evidence", "상태 / 근거")}</th></tr></thead><tbody>{page.rows.map(row => <tr key={row.id}><td>{safeDisplayText(row.name)} · {safeDisplayText(row.unit)}</td><td>{row.value ?? row.special ?? "—"}{row.observed_histogram != null && <details><summary>{w("Observed buckets", "관측 bucket")}</summary><pre>{safeDisplayText(JSON.stringify(row.observed_histogram, null, 2))}</pre></details>}</td><td>{row.start_ns ?? "?"} → {row.end_ns ?? "?"}<p>Δ {row.delta ?? "—"} · {row.rate ?? "—"} /s</p><small>{row.rate_meaning}</small>{row.histogram != null && <details><summary>{w("Interval buckets", "구간 bucket")}</summary><pre>{safeDisplayText(JSON.stringify(row.histogram, null, 2))}</pre></details>}</td><td>{safeDisplayText(row.status)}{row.gap ? " · gap" : ""}{row.reordered ? " · reordered" : ""}<code className="block text-xs">{row.point_id}</code>{row.refs.map(ref => <p key={`${ref.source_id}/${ref.position}`} className="text-xs">{safeDisplayText(`${ref.source_id}: ${ref.position}`)}</p>)}</td></tr>)}</tbody></table></div><Button variant="outline" disabled={busy || !page.next} onClick={() => void query(page.next)}>{w("Next", "다음")}</Button></>}
    {error && <p role="alert">{safeDisplayText(error)}</p>}
  </details>;
}
