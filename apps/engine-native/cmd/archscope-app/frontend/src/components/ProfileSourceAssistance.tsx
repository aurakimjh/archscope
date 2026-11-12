import { useEffect, useRef, useState } from "react";
import { Dialogs } from "@wailsio/runtime";
import * as bridge from "@/bridge/engine";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { ResultRequestScope, safeDisplayText } from "@/state/resultProviders";

export function ProfileSourceAssistance({ path, disabled, useNormalized }: { path: string; disabled: boolean; useNormalized: (path: string, selection: string) => void }): React.JSX.Element {
  const { locale } = useI18n(); const words = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const requests = useRef(new ResultRequestScope());
  const [format, setFormat] = useState("auto"); const [producer, setProducer] = useState(""); const [version, setVersion] = useState(""); const [capture, setCapture] = useState("{}");
  const [inventory, setInventory] = useState<bridge.ProfileSourceInventory | null>(null);
  const [metric, setMetric] = useState(""); const [profiles, setProfiles] = useState<string[]>([]);
  const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const options = `{ "format": ${JSON.stringify(format)}, "producer": ${JSON.stringify(producer)}, "producer_version": ${JSON.stringify(version)}, "capture_options": ${capture} }`;
  useEffect(() => { requests.current.cancel(); setInventory(null); setMetric(""); setProfiles([]); setBusy(false); setError(""); return () => requests.current.cancel(); }, [path, options, disabled]);
  async function inspect(): Promise<void> {
    const revision = requests.current.begin(); setBusy(true); setError("");
    try { const value = await requests.current.wait(revision, bridge.inspectProfileSource(path, options)); setInventory(value); }
    catch (e) { if (requests.current.current(revision)) setError(String(e)); }
    finally { if (requests.current.current(revision)) setBusy(false); }
  }
  async function save(): Promise<void> {
    if (!inventory || !metric || profiles.length === 0) return;
    const revision = requests.current.begin(); setBusy(true); setError("");
    try {
      const output = await Dialogs.SaveFile({ Title: words("Save a new normalized profile", "새 정규화 프로파일 저장"), Filename: "profile-source-set.json" });
      if (!output || !requests.current.current(revision)) return;
      await requests.current.wait(revision, bridge.normalizeProfileSource(path, options, inventory.source_sha256, output));
      useNormalized(output, JSON.stringify({ profile_ids: profiles, metric_id: metric }));
    } catch (e) { if (requests.current.current(revision)) setError(String(e)); }
    finally { if (requests.current.current(revision)) setBusy(false); }
  }
  return <details className="rounded border border-border p-3" aria-busy={busy}>
    <summary>{words("Original py-spy / Speedscope / pprof profile", "원본 py-spy / Speedscope / pprof 프로파일")}</summary>
    <p>{words("Preserves exact weights and all profiles. Select a metric and profiles, save a new model, then Analyze. New results use semantics 0.4.0; existing results retain their revision.", "정확한 가중치와 모든 프로파일을 보존합니다. metric과 프로파일을 선택해 새 모델을 저장한 뒤 분석하세요. 새 결과는 의미 버전 0.4.0을 사용하며 기존 결과는 원래 버전을 유지합니다.")}</p>
    <fieldset disabled={disabled || busy} className="space-y-2 my-2">
      <label>{words("Source format", "원본 형식")} <select value={format} onChange={e => setFormat(e.target.value)}>{["auto", "speedscope-json", "pyspy-raw", "pprof"].map(id => <option key={id}>{id}</option>)}</select></label>
      <label className="block">{words("Declared producer (optional)", "선언된 생성 도구 (선택)")} <input value={producer} onChange={e => setProducer(e.target.value)} placeholder="py-spy / go.runtime/pprof" /></label>
      <label className="block">{words("Declared version (optional)", "선언된 버전 (선택)")} <input value={version} onChange={e => setVersion(e.target.value)} placeholder="0.4.1 / go1.26.10" /></label>
      <label className="block">{words("Sanitized capture options (JSON)", "민감 정보를 제거한 수집 옵션 (JSON)")} <textarea className="w-full rounded border border-border bg-background p-2" value={capture} onChange={e => setCapture(e.target.value)} /></label>
      <Button variant="outline" disabled={!path} onClick={() => void inspect()}>{words("Inspect original profile", "원본 프로파일 확인")}</Button>
      {inventory && <div className="space-y-2">
        <p className="break-all">{safeDisplayText(inventory.producer)} {safeDisplayText(inventory.producer_version)} · SHA-256 {inventory.source_sha256}</p>
        <label>{words("Metric (explicit selection required)", "metric (명시적 선택 필수)")} <select value={metric} onChange={e => { setMetric(e.target.value); setProfiles([]); }}><option value="">{words("Choose metric", "metric 선택")}</option>{inventory.metrics.map(m => <option key={m.id} value={m.id}>{safeDisplayText(`${m.id} · ${m.source_type ?? ""}/${m.source_unit ?? ""} · ${m.kind} · ${m.unit} × ${m.scale}`)}</option>)}</select></label>
        {inventory.profiles.map(p => <label key={p.id} className="block"><input type="checkbox" disabled={!metric || !p.metrics.includes(metric)} checked={profiles.includes(p.id)} onChange={e => setProfiles(ids => e.target.checked ? [...ids, p.id] : ids.filter(id => id !== p.id))} /> {safeDisplayText(p.name || p.id)} · {safeDisplayText(p.id)}</label>)}
        {inventory.warnings.map((warning, i) => <p key={i}>{safeDisplayText(warning)}</p>)}
        <Button disabled={!metric || profiles.length === 0} onClick={() => void save()}>{words("Save new model and use selection", "새 모델 저장 후 선택 적용")}</Button>
      </div>}
    </fieldset>
    {busy && <Button variant="outline" onClick={() => { requests.current.cancel(); setBusy(false); }}>{words("Cancel", "취소")}</Button>}
    {error && <p role="alert">{safeDisplayText(error)}</p>}
  </details>;
}
