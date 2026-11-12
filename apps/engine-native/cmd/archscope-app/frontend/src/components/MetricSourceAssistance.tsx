import { useEffect, useRef, useState } from "react";
import { Dialogs } from "@wailsio/runtime";
import * as bridge from "@/bridge/engine";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { ResultRequestScope, safeDisplayText } from "@/state/resultProviders";

export function MetricSourceAssistance({ path, disabled, useNormalized }: { path: string; disabled: boolean; useNormalized: (path: string, selection: string) => void }): React.JSX.Element {
  const { locale } = useI18n(); const words = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const requests = useRef(new ResultRequestScope());
  const [format, setFormat] = useState("auto"); const [producer, setProducer] = useState(""); const [version, setVersion] = useState(""); const [queryProvenance, setQueryProvenance] = useState("{}");
  const [inventory, setInventory] = useState<bridge.MetricSourceInventory | null>(null);
  const [seriesIDs, setSeriesIDs] = useState<string[]>([]);
  const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const options = `{ "format": ${JSON.stringify(format)}, "producer": ${JSON.stringify(producer)}, "producer_version": ${JSON.stringify(version)}, "query": ${queryProvenance} }`;
  useEffect(() => { requests.current.cancel(); setInventory(null); setSeriesIDs([]); setBusy(false); setError(""); return () => requests.current.cancel(); }, [path, options, disabled]);
  async function inspect(): Promise<void> {
    const revision = requests.current.begin(); setBusy(true); setError("");
    try { const value = await requests.current.wait(revision, bridge.inspectMetricSource(path, options)); setInventory(value); }
    catch (e) { if (requests.current.current(revision)) setError(String(e)); }
    finally { if (requests.current.current(revision)) setBusy(false); }
  }
  async function save(): Promise<void> {
    if (!inventory || seriesIDs.length === 0) return;
    const revision = requests.current.begin(); setBusy(true); setError("");
    try {
      const output = await Dialogs.SaveFile({ Title: words("Save a new metric model", "새 메트릭 모델 저장"), Filename: "metric-history-set.json" });
      if (!output || !requests.current.current(revision)) return;
      await requests.current.wait(revision, bridge.normalizeMetricSource(path, options, inventory.source_sha256, output));
      useNormalized(output, JSON.stringify({ series_ids: seriesIDs }));
    } catch (e) { if (requests.current.current(revision)) setError(String(e)); }
    finally { if (requests.current.current(revision)) setBusy(false); }
  }
  return <details className="rounded border border-border p-3" aria-busy={busy}>
    <summary>{words("Metric history: OTLP / snapshots / Prometheus / Apple Silicon", "메트릭 이력: OTLP / snapshots / Prometheus / Apple Silicon")}</summary>
    <p>{format === "apple-ollama-capture-json" ? words("Open a manually saved Apple Silicon / Ollama capture, select exact series, save a new model, then Analyze. System shared memory and model residency estimates are separate. Missing GPU activity, power or thermal observations remain unavailable. Collection versions and local host/boot identity come from the capture.", "수동 저장한 Apple Silicon / Ollama 캡처를 열고 정확한 시계열을 선택해 새 모델 저장 후 분석하세요. 시스템 공유 메모리와 모델 상주 추정치를 구분합니다. 미관측 GPU 사용률·전력·thermal은 관측 불가로 남습니다. 수집 버전과 로컬 host/boot 식별자는 캡처에서 읽습니다.") : words("Select exact series, save a new model, then Analyze. Range exports require query provenance. Identities are declared; client/server and OTLP/Prometheus series stay separate.", "정확한 시계열을 선택해 새 모델을 저장한 뒤 분석하세요. Range export에는 query 출처가 필요합니다. 식별자는 선언된 값이며 client/server 및 OTLP/Prometheus 시계열을 분리합니다.")}</p>
    <fieldset disabled={disabled || busy} className="space-y-2 my-2">
      <label>{words("Source format", "원본 형식")} <select value={format} onChange={e => { setFormat(e.target.value); if (e.target.value === "apple-ollama-capture-json") { setProducer(""); setVersion(""); setQueryProvenance("{}"); } }}>{["auto", "otlp-json", "snapshot-sequence-json", "prometheus-range-json", "apple-ollama-capture-json"].map(id => <option key={id}>{id}</option>)}</select></label>
      {format !== "apple-ollama-capture-json" && <>
      <label className="block">{words("Declared producer (optional)", "선언된 생성 도구 (선택)")} <input value={producer} onChange={e => setProducer(e.target.value)} placeholder="opentelemetry-python" /></label>
      <label className="block">{words("Declared version (optional)", "선언된 버전 (선택)")} <input value={version} onChange={e => setVersion(e.target.value)} placeholder="1.38.0" /></label>
      <label className="block">{words("Query provenance (JSON)", "Query 출처 (JSON)")} <textarea className="w-full rounded border border-border bg-background p-2" value={queryProvenance} onChange={e => setQueryProvenance(e.target.value)} /></label>
      </>}
      <Button variant="outline" disabled={!path} onClick={() => void inspect()}>{words("Inspect metric source", "메트릭 원본 확인")}</Button>
      {inventory && <div className="space-y-2">
        <p className="break-all">SHA-256 {inventory.source_sha256} · {inventory.point_count} {words("points", "관측점")}</p>
        {inventory.series.map(p => <div key={p.id} className="block"><label><input type="checkbox" checked={seriesIDs.includes(p.id)} onChange={e => setSeriesIDs(ids => e.target.checked ? [...ids, p.id] : ids.filter(id => id !== p.id))} /> {safeDisplayText(`${p.name} · ${p.unit} · ${p.kind}/${p.temporality}`)}</label><details><summary>{words("Identity", "식별자")}</summary><pre className="overflow-auto text-xs">{safeDisplayText(p.identity_json)}</pre></details></div>)}
        {inventory.warnings.map((warning, i) => <p key={i}>{safeDisplayText(warning)}</p>)}
        <Button disabled={seriesIDs.length === 0} onClick={() => void save()}>{words("Save new model and use selection", "새 모델 저장 후 선택 적용")}</Button>
      </div>}
    </fieldset>
    {busy && <Button variant="outline" onClick={() => { requests.current.cancel(); setBusy(false); }}>{words("Cancel", "취소")}</Button>}
    {error && <p role="alert">{safeDisplayText(error)}</p>}
  </details>;
}
