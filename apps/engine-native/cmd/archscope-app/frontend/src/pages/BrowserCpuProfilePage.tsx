import { Loader2, Play } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import { engine } from "@/bridge/engine";
import type { ProfileEvidenceAnalysisResult } from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { CanvasFlameGraph } from "@/components/CanvasFlameGraph";
import { DiagnosticsPanel } from "@/components/DiagnosticsPanel";
import { MetricCard } from "@/components/MetricCard";
import { WailsFileDock, type FileDockSelection } from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import {
  describeTimelineState,
  extractProfileDiagnostics,
  extractProfileFlamegraph,
  profileDiagnosticIssueCount,
  selectPartialResult,
  selectSampledRuns,
  selectTopChildFrames,
  type TimelineEmptyReason,
} from "@/state/browserCpuProfile";
import { formatNumber } from "@/utils/formatters";

const RUNS_EMPTY_MESSAGE_KEY: Record<
  Exclude<TimelineEmptyReason, null>,
  "browserCpuRunsEmptyHitcount" | "browserCpuRunsEmptyDownsampled" | "browserCpuRunsEmptyGeneric"
> = {
  hitcount_only: "browserCpuRunsEmptyHitcount",
  downsampled: "browserCpuRunsEmptyDownsampled",
  empty: "browserCpuRunsEmptyGeneric",
};

export function BrowserCpuProfilePage(): React.JSX.Element {
  const { t } = useI18n();
  const [file, setFile] = useState<FileDockSelection | null>(null);
  const [result, setResult] = useState<ProfileEvidenceAnalysisResult | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<{ code: string; message: string } | null>(null);

  const analyze = useCallback(async () => {
    if (!file || running) return;
    setRunning(true);
    setError(null);
    try {
      const next = await engine.analyzeProfileEvidence({
        path: file.filePath,
        format: "auto",
        profileKind: "cpu",
      });
      setResult(next);
      addWorkspaceResult({
        result: next,
        title: `browser_cpu: ${file.originalName}`,
        sourceLabel: file.originalName,
      });
    } catch (caught) {
      setError({
        code: "BROWSER_CPU_PROFILE_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    } finally {
      setRunning(false);
    }
  }, [file, running]);

  const summary = result?.summary as Record<string, unknown> | undefined;
  const rows = useMemo(() => selectSampledRuns(result), [result]);
  const flamegraph = useMemo(() => extractProfileFlamegraph(result), [result]);
  const diagnostics = useMemo(() => extractProfileDiagnostics(result), [result]);
  const topFrames = useMemo(() => selectTopChildFrames(result), [result]);
  const timeline = useMemo(() => describeTimelineState(result), [result]);
  const partial = useMemo(() => selectPartialResult(result), [result]);
  const diagnosticIssues = profileDiagnosticIssueCount(diagnostics);

  const exportName = useMemo(() => {
    const name = file?.originalName ?? "browser-cpu";
    return `${name.replace(/\.[^.]+$/, "")}-flamegraph`;
  }, [file]);

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("browserCpuLabel")}
        description={t("browserCpuDescription")}
        accept=".json,.cpuprofile,.gz,.json.gz"
        selected={file}
        onSelect={setFile}
        onClear={() => setFile(null)}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          {
            displayName: t("browserCpuFilterProfiles"),
            pattern: "*.json;*.cpuprofile;*.json.gz;*.gz",
          },
          { displayName: "All files", pattern: "*.*" },
        ]}
        rightSlot={
          <Button
            type="button"
            size="sm"
            disabled={!file || running}
            onClick={() => void analyze()}
          >
            {running ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("analyzing")}
              </>
            ) : (
              <>
                <Play className="h-3.5 w-3.5" />
                {t("analyze")}
              </>
            )}
          </Button>
        }
      />
      <ErrorPanel error={error} labels={{ title: t("analysisError"), code: t("errorCode") }} />

      {!result && !error && (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            {t("browserCpuEmpty")}
          </CardContent>
        </Card>
      )}

      {summary && (
        <section className="grid gap-3 sm:grid-cols-4">
          <MetricCard
            label={t("browserCpuMetricSamples")}
            value={formatNumber(Number(summary.total_samples ?? 0))}
          />
          <MetricCard
            label={t("browserCpuMetricFormat")}
            value={String(result?.metadata?.format ?? "")}
          />
          <MetricCard
            label={t("browserCpuMetricCpuMs")}
            value={formatNumber(Number(summary.total_duration_ms ?? 0))}
          />
          <MetricCard
            label={t("browserCpuMetricStacks")}
            value={formatNumber(Number(summary.unique_stacks ?? 0))}
          />
        </section>
      )}

      {partial.partial && (
        <Card className="border-amber-500/60">
          <CardHeader>
            <CardTitle>{t("browserCpuPartialTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            <p>{t("browserCpuPartialBody")}</p>
            {partial.fromSamples > 0 && (
              <p className="mt-1 font-mono text-xs tabular-nums text-foreground">
                {t("browserCpuReduced")}: {formatNumber(partial.fromSamples)} →{" "}
                {formatNumber(partial.toSamples)} {t("samples")}
              </p>
            )}
          </CardContent>
        </Card>
      )}

      {result && flamegraph && (flamegraph.value > 0 || (flamegraph.children?.length ?? 0) > 0) && (
        <Card>
          <CardHeader>
            <CardTitle>{t("flamegraphTitle")}</CardTitle>
          </CardHeader>
          <CardContent>
            <CanvasFlameGraph data={flamegraph} exportName={exportName} />
            <p className="mt-2 text-xs text-muted-foreground">{t("browserCpuFlamegraphHint")}</p>
          </CardContent>
        </Card>
      )}

      {result && topFrames.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t("browserCpuTopFramesTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <caption className="sr-only">{t("browserCpuTopFramesCaption")}</caption>
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th scope="col" className="p-2 font-medium">
                    {t("framesColumn")}
                  </th>
                  <th scope="col" className="p-2 text-right font-medium">
                    {t("samplesColumn")}
                  </th>
                  <th scope="col" className="p-2 text-right font-medium">
                    {t("ratioColumn")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {topFrames.slice(0, 50).map((row, index) => (
                  <tr key={index} className="border-t">
                    <td className="max-w-xl truncate p-2 font-mono" title={String(row.frame ?? "")}>
                      {String(row.frame ?? "")}
                    </td>
                    <td className="p-2 text-right tabular-nums">
                      {formatNumber(Number(row.samples ?? 0))}
                    </td>
                    <td className="p-2 text-right tabular-nums">
                      {(Number(row.ratio ?? 0) * 100).toFixed(2)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}

      {result && (
        <Card>
          <CardHeader>
            <CardTitle>{t("browserCpuRunsTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <p className="mb-2 text-xs text-muted-foreground">{t("browserCpuRunsDisclaimer")}</p>
            {timeline.hasRuns ? (
              <table className="w-full text-left text-xs">
                <caption className="sr-only">{t("browserCpuRunsCaption")}</caption>
                <thead>
                  <tr className="border-b border-border text-muted-foreground">
                    <th scope="col" className="p-2 font-medium">
                      {t("browserCpuColStack")}
                    </th>
                    <th scope="col" className="p-2 text-right font-medium">
                      {t("browserCpuColDurationMs")}
                    </th>
                    <th scope="col" className="p-2 text-right font-medium">
                      {t("browserCpuColStartUs")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(0, 50).map((row, index) => (
                    <tr key={index} className="border-t">
                      <td className="max-w-xl truncate p-2" title={String(row.stack ?? "")}>
                        {String(row.stack ?? "")}
                      </td>
                      <td className="p-2 text-right tabular-nums">{String(row.duration_ms ?? "")}</td>
                      <td className="p-2 text-right tabular-nums">{String(row.start_us ?? "")}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p
                className="rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
                role="status"
              >
                {t(RUNS_EMPTY_MESSAGE_KEY[timeline.reason ?? "empty"])}
              </p>
            )}
          </CardContent>
        </Card>
      )}

      {result && (
        <Card>
          <CardHeader>
            <CardTitle className="inline-flex items-center gap-2">
              {t("browserCpuDiagnosticsTitle")}
              {diagnosticIssues > 0 && (
                <span className="rounded-full bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-bold text-amber-700 dark:text-amber-400">
                  {diagnosticIssues}
                </span>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <DiagnosticsPanel diagnostics={diagnostics as never} />
          </CardContent>
        </Card>
      )}
    </main>
  );
}
