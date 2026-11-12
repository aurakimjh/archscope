import { useEffect, useRef, useState } from "react";
import { Dialogs } from "@wailsio/runtime";
import * as bridge from "@/bridge/engine";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { ResultRequestScope, safeDisplayText, type Cancelable } from "@/state/resultProviders";

export function InputAssistance({ path, scope, chooseImporter }: { path: string; scope: string; chooseImporter: (id: string) => void }): React.JSX.Element {
  const { locale } = useI18n(); const words = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const requests = useRef(new ResultRequestScope());
  const [capture, setCapture] = useState(""); const [tool, setTool] = useState("");
  const [report, setReport] = useState<bridge.InputDiscovery | null>(null);
  const [busy, setBusy] = useState(false); const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  useEffect(() => { requests.current.cancel(); setReport(null); setBusy(false); setNotice(""); setError(""); return () => requests.current.cancel(); }, [path, scope, capture]);
  async function perform<T>(action: () => Cancelable<T>, success: (value: T) => void): Promise<void> {
    if (busy) return; const revision = requests.current.begin(); setBusy(true); setError(""); setNotice("");
    try { const value = await requests.current.wait(revision, action()); success(value); }
    catch (e) { if (requests.current.current(revision)) setError(String(e)); }
    finally { if (requests.current.current(revision)) setBusy(false); }
  }
  async function chooseFile(update: (value: string) => void, title: string): Promise<void> {
    const revision = requests.current.begin();
    try { const picked = await Dialogs.OpenFile({ Title: title }); if (picked && requests.current.current(revision)) update(picked); } catch (e) { if (requests.current.current(revision)) setError(String(e)); }
  }
  async function save(kind: "kit" | "draft" | "convert"): Promise<void> {
    const selection = requests.current.begin();
    try {
      const output = await Dialogs.SaveFile({ Title: words("Choose a new output file", "새 출력 파일 선택"), Filename: kind === "kit" ? "manual-collection.zip" : kind === "draft" ? "capture-context.json" : "converted-jfr.json" });
      if (!output || !requests.current.current(selection)) return;
      if (kind === "kit") await perform(() => bridge.exportCollectionKit(output), () => setNotice(words("Manual collection kit saved. Commands are never run by the app.", "수동 수집 키트를 저장했습니다. 앱은 수집 명령을 실행하지 않습니다.")));
      else if (kind === "draft") await perform(() => bridge.exportCaptureDraft(path, capture, scope, output), () => setNotice(words("New draft saved. Review unknown capture fields before use.", "새 초안을 저장했습니다. 사용 전 unknown 수집 항목을 검토하세요.")));
      else await perform(() => bridge.convertEvidenceJFR(path, tool, output), value => setNotice(words("JSON and .conversion.json saved. Open JSON in the JFR view. Tool version: ", "JSON과 .conversion.json을 저장했습니다. JFR 화면에서 JSON을 여세요. 도구 버전: ") + value.version + " · SHA-256 " + value.output_sha256));
    } catch (e) { setError(String(e)); }
  }
  const names: Record<string, string> = { producer: words("Producer", "생성 도구"), producer_version: words("Producer version", "생성 도구 버전"), collection_start: words("Collection start", "수집 시작"), collection_end: words("Collection end", "수집 종료"), clock_domain: words("Clock domain", "시계 기준"), timezone: words("Timezone", "시간대"), host_identity: words("Host identity", "호스트 식별"), process_lifetime: words("PID and process start", "PID와 프로세스 시작"), identity_basis: words("Identity evidence", "식별 근거") };
  return <details className="rounded border border-border p-3" aria-busy={busy}>
    <summary>{words("Input assistance and manual collection", "입력 안내와 수동 수집")}</summary>
    <p>{words("Inspect format candidates and missing context before importing. A bounded probe is a hint, not full validation.", "가져오기 전에 형식 후보와 부족한 맥락을 확인합니다. 제한된 감지는 전체 검증과 구분됩니다.")}</p>
    <div className="flex flex-wrap gap-2 my-2">
      <Button variant="outline" disabled={busy || !path} onClick={() => void perform(() => bridge.discoverEvidenceInput(path, capture, scope), value => { if (value.version !== "1.0.0") throw new Error("unsupported_discovery_contract"); setReport(value); })}>{words("Inspect selected input", "선택 입력 확인")}</Button>
      <Button variant="outline" disabled={busy} onClick={() => void chooseFile(setCapture, words("Optional capture sidecar", "선택적 수집 sidecar"))}>{words("Choose capture sidecar", "수집 sidecar 선택")}</Button>
      {capture && <Button variant="outline" disabled={busy} onClick={() => setCapture("")}>{words("Clear sidecar", "sidecar 해제")}</Button>}
      <Button variant="outline" disabled={busy} onClick={() => void save("kit")}>{words("Save manual collection kit", "수동 수집 키트 저장")}</Button>
      <Button variant="outline" disabled={busy || !path} onClick={() => void save("draft")}>{words("Save new capture draft", "새 수집 초안 저장")}</Button>
      {busy && <Button variant="outline" onClick={() => { requests.current.cancel(); setBusy(false); setNotice(words("Request canceled.", "요청을 취소했습니다.")); }}>{words("Cancel assistance", "안내 작업 취소")}</Button>}
    </div>
    {capture && <p className="break-all">{words("Explicit sidecar", "명시적 sidecar")}: {safeDisplayText(capture)}</p>}
    {report && <div className="space-y-2">
      <p className="break-all">{safeDisplayText(report.source.name)} · {report.source.size} bytes · SHA-256 {report.source.sha256}</p>
      <p>{words("Probe bytes", "감지 바이트")}: {report.probe_bytes} · {safeDisplayText(report.detection.state)}</p>
      {report.detection.candidates.map(candidate => { const descriptor = report.importers.find(item => item.id === candidate.importerId); return <div key={candidate.importerId} className="rounded border border-border p-2">
        <p>{safeDisplayText(candidate.importerId)} · {safeDisplayText(candidate.grade)} · {safeDisplayText(candidate.reason)}</p>
        <p>{words("Declared capabilities", "선언된 기능")}: {descriptor?.capabilities.map(safeDisplayText).join(", ")}</p>
        {descriptor?.limitations.map((limit, index) => <p key={index}>{safeDisplayText(limit)}</p>)}
        <Button variant="outline" disabled={busy} onClick={() => chooseImporter(candidate.importerId)}>{words("Use this importer; validate on Analyze", "이 형식 선택 · 분석 시 검증")}</Button>
      </div>; })}
      {report.detection.candidates.length === 0 && <p>{words("No generic importer detected. Check the dedicated analysis views and support guide.", "일반 importer가 감지되지 않았습니다. 개별 분석 화면과 지원 안내를 확인하세요.")}</p>}
      {report.mapping_previews.map(preview => <div key={preview.importer} className="rounded border border-border p-2">
        <p>{words("First-row mapping preview only", "첫 행 매핑 미리보기만 제공")}: {safeDisplayText(preview.importer)}</p>
        {preview.status === "sample_only" ? <p className="break-all">{safeDisplayText(preview.metric ?? "")} · {safeDisplayText(preview.source_value ?? "")} → {safeDisplayText(preview.normalized_value ?? "")} {safeDisplayText(preview.unit ?? "")} · {safeDisplayText(preview.kind ?? "")}</p> : <p>{safeDisplayText(preview.code ?? "preview_unavailable")}</p>}
        <p className="break-all">{words("Pack SHA-256", "팩 SHA-256")}: {preview.pack_sha256}</p>
        {preview.escaped && <p>{words("Display controls escaped or text bounded.", "표시 제어 문자를 이스케이프하거나 길이를 제한했습니다.")}</p>}
      </div>)}
      <p>{words("Capture context", "수집 맥락")}: {report.context.status === "matched_declared" ? words("Sidecar hash matched; declarations remain user evidence.", "sidecar 해시 일치 · 선언 내용은 사용자 근거입니다.") : words("No sidecar supplied.", "sidecar가 없습니다.")}</p>
      <p>{words("Missing context", "부족한 맥락")}: {report.context.missing.map(field => names[field] ?? safeDisplayText(field)).join(", ") || words("None in the basic checklist", "기본 확인 항목 없음")}</p>
      <p>{words("Unknown context limits correlation and comparison; it is never filled from file modification time.", "알 수 없는 맥락은 상관·비교를 제한하며 파일 수정 시각으로 채우지 않습니다.")}</p>
      {report.advice.some(item => item.route === "jfr") && <p>{words("JFR uses its dedicated analysis view. Binary recordings need compatible JDK conversion first.", "JFR은 개별 JFR 분석 화면을 사용합니다. 바이너리는 호환 JDK로 먼저 변환하세요.")}</p>}
    </div>}
    <details className="my-2"><summary>{words("Explicit local JFR conversion", "명시적 로컬 JFR 변환")}</summary>
      <p>{words("Prefer conversion in the collection environment. Select an installed trusted jfr executable. No tool is downloaded; offline sessions refuse subprocess conversion. Fixed print --json arguments, 2 minute / 64 MiB limits apply. Existing output files are refused.", "수집 환경에서의 변환을 우선합니다. 설치된 신뢰하는 jfr 실행 파일을 선택하세요. 도구를 다운로드하지 않으며 오프라인 세션은 변환 실행을 거부합니다. 고정 print --json 인자, 2분·64 MiB 한도를 적용하고 기존 출력 파일을 덮어쓰지 않습니다.")}</p>
      <input aria-label={words("Installed jfr absolute path", "설치된 jfr 절대 경로")} value={tool} onChange={e => setTool(e.target.value)} disabled={busy} className="w-full rounded border border-border bg-background p-2" />
      <Button variant="outline" disabled={busy} onClick={() => void chooseFile(setTool, words("Choose installed jfr executable", "설치된 jfr 실행 파일 선택"))}>{words("Choose installed tool", "설치 도구 선택")}</Button>{" "}
      <Button disabled={busy || !path || !tool} onClick={() => void save("convert")}>{words("Convert selected recording", "선택한 기록 변환")}</Button>
    </details>
    {notice && <p role="status" className="break-all">{safeDisplayText(notice)}</p>}
    {error && <p role="alert">{safeDisplayText(error)}</p>}
  </details>;
}
