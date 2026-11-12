import { SystemSourceAssistance, SystemEvidenceActions } from "@/components/SystemEvidence";
import { VisualArtifactActions } from "@/components/VisualArtifactActions";
import { MetricSourceAssistance } from "@/components/MetricSourceAssistance";
import { MetricHistoryExplorer } from "@/components/MetricHistoryExplorer";
import { ProfileEvidenceExplorer } from "@/components/ProfileEvidenceExplorer";
import { ProfileSourceAssistance } from "@/components/ProfileSourceAssistance";
import { InputAssistance } from "@/components/InputAssistance";
import { useEffect, useRef, useState } from "react";
import { Dialogs } from "@wailsio/runtime";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import * as bridge from "@/bridge/engine";
import { addWorkspaceResult, getWorkspaceGeneration, reopenWorkspaceCase, type AnalysisWorkspaceEntry } from "@/state/analysisWorkspace";
import { ResultRequestScope, safeDisplayText, comparisonEligibility, type ComparisonEligibility, startScopedImport, providerUsable, resultProblem, validateProviderPage, type ImportJob, type ProviderComparison, type ProviderPage, type ResultCapabilities } from "@/state/resultProviders";

// The consumer understands provider schemas, never importer/result type names.
export function EvidenceResultPanel({ entries, active }: { entries: AnalysisWorkspaceEntry[]; active?: AnalysisWorkspaceEntry }): React.JSX.Element {
  const { locale } = useI18n(); const ko = locale === "ko";
  const words = (en: string, kr: string): string => ko ? kr : en;
  const jobLabel = (state: string): string => ({ queued: words("Queued", "대기 중"), probing: words("Detecting", "형식 감지 중"), validating: words("Validating", "검증 중"), parsing: words("Reading", "읽는 중"), normalizing: words("Normalizing", "정규화 중"), analyzing: words("Analyzing", "분석 중"), validating_result: words("Checking result", "결과 검증 중"), committing: words("Saving", "저장 중"), needs_selection: words("Choose a format", "형식 선택 필요"), completed: words("Completed", "완료"), partial: words("Partial result", "부분 결과"), failed: words("Failed", "실패"), cancelled: words("Canceled", "취소됨") })[state] ?? words("Unknown state", "알 수 없는 상태");
  const viewLabel = (kind: string, path: string): string => {
    const labels: Record<string, string> = { summary: words("Summary", "요약"), diagnostics: words("Diagnostics", "진단"), table: words("Table", "표"), detail: words("Detail", "상세"), timeline: words("Timeline events", "타임라인 이벤트"), sli: words("Service indicators", "서비스 지표"), compare: words("Compare", "비교"), chart: words("Charts", "차트") };
    const key = path.split("/").pop()?.replace(/~1/g, "/").replace(/~0/g, "~") ?? "";
    return [labels[kind] ?? kind, ["summary", "diagnostics", "chart"].includes(kind) ? "" : key].filter(Boolean).map(safeDisplayText).join(" · ");
  };
  const [presetScope, setPresetScope] = useState("");
  const [path, setPath] = useState(""); const [importer, setImporter] = useState("auto");
  const [formats, setFormats] = useState<Array<{ id: string; limitations: string[] }>>([]);
  const [options, setOptions] = useState("{}"); const [partial, setPartial] = useState(false);
  const [handle, setHandle] = useState(active?.backend_result_handle ?? "");
  const [caps, setCaps] = useState<ResultCapabilities | null>(null);
  const [provider, setProvider] = useState("generic:summary"); const [page, setPage] = useState<ProviderPage | null>(null);
  const [job, setJob] = useState<ImportJob | null>(null); const jobRef = useRef<ImportJob | null>(null);
  const [comparison, setComparison] = useState<ProviderComparison | null>(null); const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false); const [problem, setProblem] = useState<ReturnType<typeof resultProblem> | null>(null);
  const load = useRef(new ResultRequestScope()); const query = useRef(new ResultRequestScope());
  const targetScope = useRef(new ResultRequestScope());
  const [targetStates, setTargetStates] = useState<{ key: string; values: Record<string, ComparisonEligibility> }>({ key: "", values: {} });
  const targetHandles = [...new Set(entries.flatMap(entry => entry.backend_result_handle ? [entry.backend_result_handle] : []))];
  const targetKey = JSON.stringify(targetHandles);
  const eligibilityKey = JSON.stringify([caps?.handle, caps?.result_sha256, provider, targetKey]);
  const eligibleTargets = targetStates.key === eligibilityKey ? targetStates.values : {};
  const [reload, setReload] = useState(0);
  useEffect(() => { setHandle(active?.backend_result_handle ?? ""); setProblem(null); }, [active?.backend_result_handle]);
  useEffect(() => {
    let alive = true; const request = bridge.listScopedEvidenceImporters(presetScope);
    void request.then(value => { if (alive) setFormats(value); }).catch(error => { if (alive) setProblem(resultProblem(error)); });
    return () => { alive = false;  };
  }, [presetScope]);
  function cancel(): void {
    load.current.cancel(); query.current.cancel();
    const pending = jobRef.current;
    if (pending && !["completed", "partial", "failed", "cancelled"].includes(pending.state)) {
      void bridge.cancelEvidenceImport(pending.id, pending.generation).catch(error => setProblem(resultProblem(error)));
    }
    jobRef.current = null; setJob(null); setBusy(false);
  }
  useEffect(() => () => {
    load.current.cancel(); query.current.cancel();
    const pending = jobRef.current;
    if (pending && !["completed", "partial", "failed", "cancelled"].includes(pending.state)) void bridge.cancelEvidenceImport(pending.id, pending.generation).catch(() => {});
  }, []);
  useEffect(() => {
    const scope = load.current; const revision = scope.begin(); query.current.cancel();
    setCaps(null); setPage(null); setComparison(null); setProvider("generic:summary"); setProblem(null);
    if (!handle) { setBusy(false); return; }
    setBusy(true); let lease = "";
    void (async () => {
      // Acquisition is allowed to settle so a late successful lease is released.
      const owned = await bridge.acquireEvidenceResult(handle); lease = owned.id;
      if (!scope.current(revision)) { await bridge.releaseEvidenceLease(lease); lease = ""; return; }
      const description = await scope.wait(revision, bridge.describeEvidenceResult(handle));
      if (description.version !== "1.0.0" || description.identity_basis !== "declared") throw new Error("unsupported_provider_contract");
      setCaps(description);
    })().catch(error => { if (scope.current(revision)) setProblem(resultProblem(error)); }).finally(() => { if (scope.current(revision)) setBusy(false); });
    return () => { scope.cancel(); query.current.cancel(); if (lease) { void bridge.releaseEvidenceLease(lease).catch(() => {}); lease = ""; } };
  }, [handle, reload]);
  async function readPage(offset: number): Promise<void> {
    if (!caps) return; const scope = query.current; const revision = scope.begin(); setBusy(true); setProblem(null); setComparison(null); setPage(null);
    try { const value = await scope.wait(revision, bridge.queryEvidenceProvider(caps.handle, provider, offset, 25)); setPage(validateProviderPage(value, caps, provider)); }
    catch (error) { if (scope.current(revision)) setProblem(resultProblem(error)); }
    finally { if (scope.current(revision)) setBusy(false); }
  }
  useEffect(() => {
    setPage(null); setComparison(null);
    const cap = caps?.providers.find(c => c.id === provider);
    if (cap && providerUsable(cap) && cap.kind !== "compare") void readPage(0);
    else { query.current.cancel(); setBusy(false); }
    // Each immutable handle/description and provider starts a new request scope.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caps, provider]);
  async function openSaved(): Promise<void> {
    cancel(); const scope = load.current; const revision = scope.begin(); setBusy(true); setProblem(null);
    try { const value = await scope.wait(revision, bridge.openEvidenceResult(path)); setHandle(value.handle); setReload(n => n + 1); }
    catch (error) { if (scope.current(revision)) setProblem(resultProblem(error)); }
    finally { if (scope.current(revision)) setBusy(false); }
  }
  async function watchJob(initial: ImportJob, revision: number, selected = false): Promise<void> {
    const scope = load.current; let value = initial;
    while (scope.current(revision)) {
      jobRef.current = value; setJob(value);
      if (["completed", "partial", "failed", "cancelled"].includes(value.state)) {
        if (value.resultHandle) { setHandle(value.resultHandle); setReload(n => n + 1); }
        else if (value.problem) setProblem({ code: value.problem.code, message: value.problem.message, stale: false });
        return;
      }
      if (value.state === "needs_selection" && !selected) return;
      await new Promise(resolve => setTimeout(resolve, 200));
      if (!scope.current(revision)) return;
      value = await scope.wait(revision, bridge.getImportJob(value.id, value.generation));
    }
  }
  async function startImport(): Promise<void> {
    cancel(); const scope = load.current; const revision = scope.begin(); setBusy(true); setProblem(null);
    try {
      // Start is not canceled as an IPC call: a late job ID must still receive
      // CancelImport after cancellation/unmount, otherwise it runs orphaned.
      const initial = await startScopedImport(scope, revision, bridge.analyzeEvidence(path, importer, options, partial, presetScope), bridge.cancelEvidenceImport);
      if (!initial) return;
      await watchJob(initial, revision);
    } catch (error) { if (scope.current(revision)) setProblem(resultProblem(error)); }
    finally { if (scope.current(revision)) setBusy(false); }
  }
  async function chooseImporter(id: string): Promise<void> {
    if (!job) return; const scope = load.current; const revision = scope.begin(); setBusy(true); setProblem(null);
    try {
      const value = await bridge.selectEvidenceImporter(job.id, job.generation, id);
      if (!scope.current(revision)) { await bridge.cancelEvidenceImport(value.id, value.generation); return; }
      // SelectImporter may still report needs_selection until the worker wakes.
      const next = { ...value, state: "validating" }; await watchJob(next, revision, true);
    } catch (error) { if (scope.current(revision)) setProblem(resultProblem(error)); }
    finally { if (scope.current(revision)) setBusy(false); }
  }
  useEffect(() => {
    const scope = targetScope.current; const revision = scope.begin();
    setTarget(""); setTargetStates({ key: eligibilityKey, values: {} });
    if (!caps || caps.providers.find(candidate => candidate.id === provider)?.kind !== "compare") return;
    const handles: string[] = JSON.parse(targetKey); let cursor = 0;
    async function worker(): Promise<void> {
      while (scope.current(revision) && cursor < handles.length) {
        const handle = handles[cursor++];
        try {
          const candidate = handle === caps!.handle ? caps! : await scope.wait(revision, bridge.describeEvidenceResult(handle));
          if (scope.current(revision)) setTargetStates(previous => previous.key === eligibilityKey ? { key: eligibilityKey, values: { ...previous.values, [handle]: comparisonEligibility(caps!, candidate, provider) } } : previous);
        } catch { if (scope.current(revision)) setTargetStates(previous => previous.key === eligibilityKey ? { key: eligibilityKey, values: { ...previous.values, [handle]: { allowed: false, reason: "target_unavailable" } } } : previous); }
      }
    }
    void Promise.all([worker(), worker()]);
    return () => scope.cancel();
  }, [caps, provider, targetKey, eligibilityKey]);
  async function compare(): Promise<void> {
    if (!caps || !target || !eligibleTargets[target]?.allowed) return; const scope = query.current; const revision = scope.begin(); setBusy(true); setProblem(null); setComparison(null);
    try { setComparison(await scope.wait(revision, bridge.compareEvidenceProvider(caps.handle, target, provider))); }
    catch (error) { if (scope.current(revision)) setProblem(resultProblem(error)); }
    finally { if (scope.current(revision)) setBusy(false); }
  }
  async function addToWorkspace(): Promise<void> {
    if (!caps) return; const scope = query.current; const revision = scope.begin(); const generation = getWorkspaceGeneration(); setBusy(true); setProblem(null);
    try {
      const summary = validateProviderPage(await scope.wait(revision, bridge.queryEvidenceProvider(caps.handle, "generic:summary", 0, 8)), caps, "generic:summary");
      if (generation !== getWorkspaceGeneration()) throw new Error("workspace_changed");
      addWorkspaceResult({ backendResultHandle: caps.handle, result: { type: caps.type, source_files: caps.sources.map(source => source.text), summary: {} },
        displaySummary: summary.rows.map(row => ({ key: row[0].text, value: row[1].text })) });
    } catch (error) { if (scope.current(revision)) setProblem(resultProblem(error)); }
    finally { if (scope.current(revision)) setBusy(false); }
  }
  const selected = caps?.providers.find(c => c.id === provider);
  const selectedFormat = formats.find(f => f.id === importer);
  return <section className="card min-w-0 space-y-3" aria-label={words("Evidence results", "근거 결과")}>
    <h3>{words("Import and explore evidence", "근거 가져오기 및 조회")}</h3>
    <div className="flex flex-wrap items-center gap-2">
      <input className="min-w-0 flex-1 rounded border border-border bg-background p-2" aria-label={words("Local evidence path", "로컬 근거 경로")} value={path} onChange={e => setPath(e.target.value)} placeholder={words("Local file", "로컬 파일")} disabled={busy} />
      <Button variant="outline" disabled={busy} onClick={() => void (async () => { try { const picked = await Dialogs.OpenFile({ Title: words("Choose evidence", "근거 선택") }); if (typeof picked === "string") setPath(picked); } catch (error) { setProblem(resultProblem(error)); } })()}>{words("Browse", "파일 선택")}</Button>
      <select aria-label={words("Importer", "가져오기 형식")} value={importer} disabled={busy} onChange={e => setImporter(e.target.value)} className="max-w-full rounded border border-border bg-background p-2"><option value="auto">{words("Detect format", "형식 감지")}</option>{formats.map(f => <option key={f.id} value={f.id}>{f.id}</option>)}</select>
      <Button disabled={busy || !path} onClick={() => void startImport()}>{words("Analyze", "분석")}</Button>
      <Button variant="outline" disabled={busy || !path} onClick={() => void openSaved()}>{words("Open saved result", "저장 결과 열기")}</Button>
      <Button variant="outline" disabled={!busy && !job} onClick={cancel}>{words("Cancel", "취소")}</Button>
    </div>
    <SystemSourceAssistance path={path} disabled={busy} useNormalized={file=>{setPath(file);setImporter("system.evidence-json");setOptions("{}");}} />
    <MetricSourceAssistance path={path} disabled={busy} useNormalized={(file, selection) => { setPath(file); setImporter("metrics.history-json"); setOptions(selection); }} />
    <ProfileSourceAssistance path={path} disabled={busy} useNormalized={(file, selection) => { setPath(file); setImporter("profile.source-set-json"); setOptions(selection); }} />
    <InputAssistance path={path} scope={presetScope} chooseImporter={setImporter} />
    <details><summary>{words("Import options", "가져오기 옵션")}</summary>
      <label>{words("Preset scope (empty: application only)", "프리셋 범위 (빈 값: 앱 기본만)")} <input value={presetScope} onChange={e => { setPresetScope(e.target.value); setImporter("auto"); }} disabled={busy} placeholder="user / case:case-…" /></label>
      <textarea aria-label={words("JSON options", "JSON 옵션")} value={options} onChange={e => setOptions(e.target.value)} disabled={busy} className="w-full rounded border border-border bg-background p-2" />
      <label><input type="checkbox" checked={partial} disabled={busy} onChange={e => setPartial(e.target.checked)} /> {words("Allow explicitly bounded partial results", "명시적으로 제한된 부분 결과 허용")}</label>
      {selectedFormat?.limitations?.map(text => <p key={text} className="text-xs">{text}</p>)}
    </details>
    {job && <div role="status"><span>{words("Import state", "가져오기 상태")}: {jobLabel(job.state)}</span>{job.packs?.map(pack => <p key={`${pack.scope}|${pack.sha256}`} className="break-all">{pack.id} {pack.version} · {pack.origin} · {pack.scope} · SHA-256 {pack.sha256}</p>)}{job.cleanupPending && <p>{words("Temporary cleanup remains pending.", "임시 자료 정리가 남아 있습니다.")}</p>}
      {job.state === "needs_selection" && job.detection.candidates.map(c => <Button key={c.importerId} variant="outline" disabled={busy} onClick={() => void chooseImporter(c.importerId)}>{c.importerId} · {c.reason}</Button>)}
    </div>}
    {busy && <p role="status">{words("Working…", "처리 중…")}</p>}
    {problem && <div role="alert" className="break-words"><p>{safeDisplayText(problem.code)}: {safeDisplayText(problem.message)}</p>{problem.stale && <p>{words("Reopen the saved result file or a saved case snapshot.", "저장 결과 파일 또는 저장된 케이스 스냅샷을 다시 열어 주세요.")}</p>}
      {problem.stale && active?.saved_case && <Button variant="outline" onClick={() => void reopenWorkspaceCase(active.saved_case!.case_id, active.saved_case!.result_id).catch(error => setProblem(resultProblem(error)))}>{words("Reopen case snapshot", "케이스 스냅샷 다시 열기")}</Button>}
      <Button variant="outline" disabled={busy} onClick={() => setReload(n => n + 1)}>{words("Retry result", "결과 다시 조회")}</Button>
    </div>}
    {caps && <>
      <div className="flex flex-wrap items-center gap-2"><strong>{safeDisplayText(caps.type)}</strong><Button size="sm" variant="outline" disabled={busy} onClick={() => void addToWorkspace()}>{words("Add to Workspace", "Workspace에 추가")}</Button>
        <Button size="sm" variant="outline" disabled={busy} onClick={() => void (async () => { try { const destination = await Dialogs.SaveFile({ Title: words("Save exact result", "원본 정밀도 결과 저장"), Filename: "evidence.json" }); if (destination) await bridge.exportEvidenceResult(caps.handle, destination, "raw-json"); } catch (error) { setProblem(resultProblem(error)); } })()}>{words("Save result JSON", "결과 JSON 저장")}</Button>
      </div>
      {caps.type==="system_evidence"&&caps.payload_version==="0.1.0"&&<SystemEvidenceActions handle={caps.handle} caseRef={active?.saved_case} histogramView={provider==="system:histograms"} eventIDs={[...new Set(page?.rows.flatMap(row=>page.columns.flatMap((name,i)=>["event_id","start_ref","end_ref"].includes(name)&&row[i].text?[row[i].text]:[]))??[])]}/>}
      {caps.type === "metrics_timeseries" && ["0.1.0", "0.2.0"].includes(caps.payload_version ?? "") && <MetricHistoryExplorer handle={caps.handle} />}
      {caps.type === "profile_evidence" && ["0.2.0", "0.3.0", "0.4.0"].includes(caps.payload_version ?? "") && <ProfileEvidenceExplorer handle={caps.handle} caseRef={active?.saved_case} />}
      <p className="text-xs">{words("Declared identity; original sources have not been verified by reimport.", "선언된 식별 정보이며 원본 재가져오기로 검증한 상태가 아닙니다.")}</p>
      {!caps.compatibility.allowed && <p role="status">{words("Read-only fallback", "읽기 전용 기본 표시")}: {safeDisplayText(caps.compatibility.reason || "")}</p>}
      <details><summary>{words("Sources and revision", "출처 및 리비전")}</summary><code className="break-all text-xs">{caps.result_sha256}</code>{caps.sources.map((s, i) => <p className="break-words text-xs" key={i}><bdi>{s.text}</bdi>{s.escaped && <small> {words("Controls shown", "제어문자 표시")}</small>}</p>)}{caps.sources_truncated && <p>{words("Source labels limited.", "출처 표시가 제한되었습니다.")}</p>}</details>
      <select className="max-w-full rounded border border-border bg-background p-2" aria-label={words("Result view", "결과 보기")} value={provider} onChange={e => { query.current.cancel(); setProvider(e.target.value); }}>
        {caps.providers.map(p => <option key={p.id} value={p.id} disabled={!providerUsable(p)}>{viewLabel(p.kind, p.payload_path)}{p.reason ? ` (${safeDisplayText(p.reason)})` : ""}</option>)}
      </select>
      {caps.truncated && <p>{words("Provider list limited; use backend export for remaining data.", "기능 목록이 제한되었습니다. 나머지 자료는 내보내기로 확인하세요.")}</p>}
      {selected?.kind === "compare" && <div className="flex flex-wrap gap-2">
        <span>{words("Baseline: current result", "기준: 현재 결과")}</span>
        <select aria-label={words("Comparison target", "비교 대상")} className="max-w-full rounded border border-border bg-background p-2" value={target} onChange={e => { query.current.cancel(); setBusy(false); setComparison(null); setTarget(e.target.value); }}><option value="">{words("Choose target", "대상 선택")}</option>{entries.filter(e => e.backend_result_handle).map(e => { const status = eligibleTargets[e.backend_result_handle!]; return <option value={e.backend_result_handle} key={e.id} disabled={!status?.allowed}>{safeDisplayText(e.title)}{!status?.allowed ? ` (${status?.reason || words("Checking", "확인 중")})` : ""}</option>; })}</select>
        <Button disabled={busy || !target || !eligibleTargets[target]?.allowed || !providerUsable(selected)} onClick={() => void compare()}>{words("Compare", "비교")}</Button>
      </div>}
      {comparison?.allowed && caps.type === "profile_evidence" && target && <VisualArtifactActions handle={caps.handle} target={target} title={words("Profile difference of saved selections", "저장 선택의 프로파일 차이")} query={{ kind: "profile_diff", offset: 0, limit: 25 }} />}
      {comparison && <div role="status">{comparison.allowed ? <><p>Δ {comparison.delta?.value ?? "—"} · {comparison.delta?.percent ?? "—"}%</p><p className="text-xs break-all">{comparison.delta?.comparison?.baseline.result_sha256} → {comparison.delta?.comparison?.target.result_sha256}</p></> : <p>{words("Comparison refused", "비교 불가")}: {comparison.code || comparison.reason}</p>}{comparison.code && <p>{comparison.code}</p>}</div>}
      {page && <>
        <VisualArtifactActions handle={caps.handle} availableColumns={page.columns} title={words("Selected evidence table", "선택한 근거 표")} query={{ kind: "provider_table", provider, offset: page.offset, limit: 25, columns: page.columns.slice(0, 8) }} />
        {page.columns.length > 8 && <p>{words("The initial report selection uses the first eight columns. Choose the required columns above; the source result retains the full table.", "처음에는 첫 8개 열을 선택합니다. 위에서 필요한 열을 고르세요. 원본 결과에는 전체 표가 보존됩니다.")}</p>}
        <div className="max-h-96 overflow-auto rounded border border-border"><table className="w-full text-left text-sm"><thead><tr>{page.columns.map(c => <th key={c} className="p-2">{c}</th>)}</tr></thead><tbody>{page.rows.map((row, i) => <tr key={page.offset + i}>{row.map((cell, j) => <td key={j} className="max-w-96 break-words border-t border-border p-2"><bdi>{cell.text}</bdi>{cell.escaped && <small> {words("Controls shown", "제어문자 표시")}</small>}{cell.unit && <small> {cell.unit}</small>}{(cell.availability || cell.reason || cell.population) && <p className="text-xs text-muted-foreground">{[cell.availability, cell.reason, cell.population].filter(Boolean).join(" · ")}</p>}{cell.truncated && <span>…</span>}</td>)}</tr>)}</tbody></table></div>
        {page.total === 0 && <p>{words("No rows", "표시할 행이 없습니다")}</p>}
        <div className="flex flex-wrap items-center gap-2"><Button variant="outline" disabled={busy || page.offset === 0} onClick={() => void readPage(Math.max(0, page.offset - 25))}>{words("Previous", "이전")}</Button><span>{page.offset + (page.rows.length ? 1 : 0)}–{page.offset + page.rows.length} / {page.total}</span><Button variant="outline" disabled={busy || page.next < 0} onClick={() => void readPage(page.next)}>{words("Next", "다음")}</Button></div>
        {page.escaped && <p>{words("Control and bidirectional formatting characters are shown as code points. Original exports are unchanged.", "제어 및 양방향 서식 문자는 코드 포인트로 표시합니다. 내보내기 원본은 유지됩니다.")}</p>}
        {page.truncated && <p>{words("Display limited. Exact data remains in backend exports.", "표시가 제한되었습니다. 정밀 데이터는 백엔드 내보내기에 보존됩니다.")}</p>}
      </>}
    </>}
  </section>;
}
