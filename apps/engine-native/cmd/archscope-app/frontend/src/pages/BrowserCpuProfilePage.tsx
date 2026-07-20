import { Loader2, Play } from "lucide-react";
import { useCallback, useState } from "react";

import { engine } from "@/bridge/engine";
import type { ProfileEvidenceAnalysisResult } from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { MetricCard } from "@/components/MetricCard";
import { WailsFileDock, type FileDockSelection } from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import { formatNumber } from "@/utils/formatters";

export function BrowserCpuProfilePage(): JSX.Element {
  const { t } = useI18n();
  const [file, setFile] = useState<FileDockSelection | null>(null);
  const [result, setResult] = useState<ProfileEvidenceAnalysisResult | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<{ code: string; message: string } | null>(null);
  const analyze = useCallback(async () => {
    if (!file || running) return;
    setRunning(true); setError(null);
    try { const next = await engine.analyzeProfileEvidence({ path: file.filePath, format: "auto", profileKind: "cpu" }); setResult(next); addWorkspaceResult({ result: next, title: `browser_cpu: ${file.originalName}`, sourceLabel: file.originalName }); }
    catch (caught) { setError({ code: "BROWSER_CPU_PROFILE_FAILED", message: caught instanceof Error ? caught.message : String(caught) }); }
    finally { setRunning(false); }
  }, [file, running]);
  const summary = result?.summary as Record<string, unknown> | undefined;
  const rows = (result?.tables?.cpu_sample_runs ?? []) as Array<Record<string, unknown>>;
  const parserMetadata = result?.metadata?.parser_metadata as Record<string, unknown> | undefined;
  const partialResult = parserMetadata?.partial_result === true;
  const sampledFrom = Number(parserMetadata?.downsampled_from_samples ?? 0);
  const sampledTo = Number(parserMetadata?.downsampled_to_samples ?? 0);
  return <main className="flex flex-col gap-5 p-5">
    <WailsFileDock label="Chrome CPU profile" description="Save a Chrome Performance trace or import a V8 .cpuprofile. Sampled CPU runs are not browser Long Tasks." accept=".json,.cpuprofile" selected={file} onSelect={setFile} onClear={() => setFile(null)} browseLabel={t("browseFile")} dropHereLabel={t("dropHere")} errorLabel={t("error")} fileFilters={[{ displayName: "Chrome profiles", pattern: "*.json;*.cpuprofile" }, { displayName: "All files", pattern: "*.*" }]} rightSlot={<Button type="button" size="sm" disabled={!file || running} onClick={() => void analyze()}>{running ? <><Loader2 className="h-3.5 w-3.5 animate-spin" />{t("analyzing")}</> : <><Play className="h-3.5 w-3.5" />{t("analyze")}</>}</Button>} />
    <ErrorPanel error={error} labels={{ title: t("analysisError"), code: t("errorCode") }} />
    {summary && <section className="grid gap-3 sm:grid-cols-4"><MetricCard label="Samples" value={formatNumber(Number(summary.total_samples ?? 0))} /><MetricCard label="Format" value={String(result?.metadata?.format ?? "")}/><MetricCard label="CPU duration (ms)" value={formatNumber(Number(summary.total_duration_ms ?? 0))}/><MetricCard label="Stacks" value={formatNumber(Number(summary.unique_stacks ?? 0))}/></section>}
    {partialResult && <Card className="border-amber-500/60"><CardHeader><CardTitle>Large-profile partial result</CardTitle></CardHeader><CardContent className="text-sm text-muted-foreground">The profile was reduced from {formatNumber(sampledFrom)} to {formatNumber(sampledTo)} time-weighted samples. Total sampled CPU duration is preserved, but individual sample-run detail is aggregated.</CardContent></Card>}
    {result && <Card><CardHeader><CardTitle>Sampled CPU runs</CardTitle></CardHeader><CardContent className="overflow-auto"><p className="mb-2 text-xs text-muted-foreground">These are sampled CPU observations, not browser Long Tasks.</p><table className="w-full text-left text-xs"><thead><tr><th className="p-2">Stack</th><th className="p-2">Duration (ms)</th><th className="p-2">Start (µs)</th></tr></thead><tbody>{rows.slice(0,50).map((row,index)=><tr key={index} className="border-t"><td className="max-w-xl truncate p-2">{String(row.stack ?? "")}</td><td className="p-2">{String(row.duration_ms ?? "")}</td><td className="p-2">{String(row.start_us ?? "")}</td></tr>)}</tbody></table></CardContent></Card>}
  </main>;
}
