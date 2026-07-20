import { Loader2, Play } from "lucide-react";
import { useCallback, useState } from "react";

import { engine } from "@/bridge/engine";
import type { HttpCaptureAnalysisResult } from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { MetricCard } from "@/components/MetricCard";
import { WailsFileDock, type FileDockSelection } from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import { formatNumber } from "@/utils/formatters";

// The Phase-1 page deliberately supports import only. It makes HAR fidelity
// and source format visible instead of suggesting a live proxy capability.
export function HttpCapturePage(): React.JSX.Element {
  const { t } = useI18n();
  const [file, setFile] = useState<FileDockSelection | null>(null);
  const [result, setResult] = useState<HttpCaptureAnalysisResult | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<{ code: string; message: string } | null>(null);
  const analyze = useCallback(async () => {
    if (!file || running) return;
    setRunning(true); setError(null);
    try {
      const next = await engine.analyzeHttpCapture({ path: file.filePath, format: "auto" });
      setResult(next);
      addWorkspaceResult({ result: next, title: `http_capture: ${file.originalName}`, sourceLabel: file.originalName });
    } catch (caught) {
      setError({ code: "HTTP_CAPTURE_IMPORT_FAILED", message: caught instanceof Error ? caught.message : String(caught) });
    } finally { setRunning(false); }
  }, [file, running]);
  const summary = result?.summary as Record<string, unknown> | undefined;
  const transactions = (result?.tables?.transactions ?? []) as Array<Record<string, unknown>>;
  const endpoints = (result?.tables?.endpoints ?? []) as Array<Record<string, unknown>>;
  return <main className="flex flex-col gap-5 p-5">
    <WailsFileDock label="HAR file" description="Import a HAR export. Tokens in URLs are redacted before analysis." accept=".har,.json" selected={file} onSelect={setFile} onClear={() => setFile(null)} browseLabel={t("browseFile")} dropHereLabel={t("dropHere")} errorLabel={t("error")} fileFilters={[{ displayName: "HAR exports", pattern: "*.har;*.json" }, { displayName: "All files", pattern: "*.*" }]} rightSlot={<Button type="button" size="sm" disabled={!file || running} onClick={() => void analyze()}>{running ? <><Loader2 className="h-3.5 w-3.5 animate-spin" />{t("analyzing")}</> : <><Play className="h-3.5 w-3.5" />{t("analyze")}</>}</Button>} />
    <ErrorPanel error={error} labels={{ title: t("analysisError"), code: t("errorCode") }} />
    {summary && <section className="grid gap-3 sm:grid-cols-4"><MetricCard label="Transactions" value={formatNumber(Number(summary.total_transactions ?? 0))} /><MetricCard label="Errors" value={formatNumber(Number(summary.error_transactions ?? 0))} /><MetricCard label="Hosts" value={formatNumber(Number(summary.unique_hosts ?? 0))} /><MetricCard label="Dialect" value={String(summary.dialect ?? "generic")} /></section>}
    {result && <section className="grid gap-5 lg:grid-cols-2"><TableCard title="Endpoints" rows={endpoints} /><TableCard title="Transactions" rows={transactions} /></section>}
  </main>;
}

function TableCard({ title, rows }: { title: string; rows: Array<Record<string, unknown>> }): React.JSX.Element {
  const columns = rows.length ? Object.keys(rows[0]).slice(0, 6) : [];
  return <Card><CardHeader><CardTitle>{title}</CardTitle></CardHeader><CardContent className="overflow-auto"><table className="w-full text-left text-xs"><thead><tr>{columns.map((column) => <th key={column} className="p-2">{column}</th>)}</tr></thead><tbody>{rows.slice(0, 50).map((row, index) => <tr key={index} className="border-t">{columns.map((column) => <td key={column} className="max-w-56 truncate p-2">{String(row[column] ?? "")}</td>)}</tr>)}</tbody></table>{rows.length === 0 && <p className="text-sm text-muted-foreground">No rows yet.</p>}</CardContent></Card>;
}
