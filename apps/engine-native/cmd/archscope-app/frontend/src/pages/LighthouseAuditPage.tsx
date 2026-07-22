// ─────────────────────────────────────────────────────────────────────
// [한글] LighthouseAuditPage.tsx — 파일 우선 Lighthouse 브라우저 감사
// 전용 데스크톱 페이지 (T-585/T-586).
//
// 책임/목적:
//   - engine.analyzeBrowserAudit 만 호출해 로컬 Lighthouse 리포트 JSON 을
//     분석하고, 가져온 점수 출처 공시, 카테고리 점수, Core Web Vitals,
//     리소스 분포, 감사/네트워크 표, 발견 사항, 파서 진단을 표시.
//   - 결과를 Analysis Workspace 에 등록해 Export Center/Report Diff 로
//     흐르게 하고, 발견 사항/감사 행을 Evidence Board 로 캡처.
//   - CPU 프로파일 샘플 모델(profile_evidence)과 절대 섞지 않음 —
//     별도 browser_audit_evidence 계약만 사용.
// ─────────────────────────────────────────────────────────────────────
import { Dialogs } from "@wailsio/runtime";
import { FileDown, Loader2, Play, Plus } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import { engine } from "@/bridge/engine";
import type { BrowserAuditAnalysisResult } from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { DiagnosticsPanel } from "@/components/DiagnosticsPanel";
import { MetricCard } from "@/components/MetricCard";
import { WailsFileDock, type FileDockSelection } from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import {
  browserAuditDiagnosticIssueCount,
  buildBrowserAuditEvidence,
  extractBrowserAuditDiagnostics,
  scorePctBand,
  scoreToPct,
  selectAuditRows,
  selectBrowserAuditContract,
  selectBrowserAuditFindings,
  selectBrowserAuditSummary,
  selectCategoryScores,
  selectCoreMetrics,
  selectNetworkRequests,
  selectResourceDistribution,
  selectScoreProvenance,
  type ScoreBand,
} from "@/state/browserAudit";
import { addEvidenceCard } from "@/state/evidenceBoard";
import { formatNumber } from "@/utils/formatters";

const BAND_CLASS: Record<ScoreBand, string> = {
  good: "text-emerald-600 dark:text-emerald-400",
  average: "text-amber-600 dark:text-amber-400",
  poor: "text-red-600 dark:text-red-400",
  unknown: "text-muted-foreground",
};

function formatBytes(value: unknown): string {
  const bytes = Number(value ?? 0);
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let scaled = bytes;
  let unit = 0;
  while (scaled >= 1024 && unit < units.length - 1) {
    scaled /= 1024;
    unit += 1;
  }
  return `${scaled >= 100 || unit === 0 ? Math.round(scaled) : scaled.toFixed(1)} ${units[unit]}`;
}

export function LighthouseAuditPage(): React.JSX.Element {
  const { t } = useI18n();
  const [file, setFile] = useState<FileDockSelection | null>(null);
  const [result, setResult] = useState<BrowserAuditAnalysisResult | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<{ code: string; message: string } | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const sourceFile = result?.source_files?.[0] ?? file?.filePath;

  const analyze = useCallback(async () => {
    if (!file || running) return;
    setRunning(true);
    setError(null);
    setNotice(null);
    try {
      const next = await engine.analyzeBrowserAudit({ path: file.filePath });
      setResult(next);
      addWorkspaceResult({
        result: next,
        title: `browser_audit: ${file.originalName}`,
        sourceLabel: file.originalName,
      });
    } catch (caught) {
      // A→B provenance: a failed run must drop any prior result so the page
      // never shows stale scores next to a new file.
      setResult(null);
      setError({
        code: "BROWSER_AUDIT_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    } finally {
      setRunning(false);
    }
  }, [file, running]);

  const summary = useMemo(() => selectBrowserAuditSummary(result), [result]);
  const provenance = useMemo(() => selectScoreProvenance(result), [result]);
  const contract = useMemo(() => selectBrowserAuditContract(result), [result]);
  const categories = useMemo(() => selectCategoryScores(result), [result]);
  const coreMetrics = useMemo(() => selectCoreMetrics(result), [result]);
  const resourceDist = useMemo(() => selectResourceDistribution(result), [result]);
  const audits = useMemo(() => selectAuditRows(result), [result]);
  const networkRequests = useMemo(() => selectNetworkRequests(result), [result]);
  const findings = useMemo(() => selectBrowserAuditFindings(result), [result]);
  const diagnostics = useMemo(() => extractBrowserAuditDiagnostics(result), [result]);
  const diagnosticIssues = browserAuditDiagnosticIssueCount(diagnostics);

  const captureFinding = useCallback(
    (finding: Record<string, unknown>) => {
      addEvidenceCard(buildBrowserAuditEvidence(finding, sourceFile));
      setNotice(t("evidenceAdded"));
    },
    [sourceFile, t],
  );

  const captureAudit = useCallback(
    (row: Record<string, unknown>) => {
      addEvidenceCard({
        analyzer: "browser_audit",
        source_kind: "table_row",
        title: String(row.title ?? row.id ?? "audit"),
        summary: row.display_value ? String(row.display_value) : undefined,
        source_file: sourceFile,
        source_ref: row.source_ref ? String(row.source_ref) : undefined,
        payload: row,
      });
      setNotice(t("evidenceAdded"));
    },
    [sourceFile, t],
  );

  const exportReport = useCallback(
    async (mode: "json" | "html") => {
      if (!result) return;
      try {
        const base = (file?.originalName ?? "lighthouse").replace(/\.[^.]+$/, "");
        const destination = await Dialogs.SaveFile({
          Title: t("exportChooseOutput"),
          Filename: `${base}-browser-audit.${mode}`,
          Filters: [
            { DisplayName: `${mode.toUpperCase()} files`, Pattern: `*.${mode}` },
            { DisplayName: "All files", Pattern: "*.*" },
          ],
        });
        if (!destination) return;
        const req = { path: destination, result };
        if (mode === "json") await engine.exportJSON(req);
        else await engine.exportHTML(req);
        setNotice(`${t("exportCompleted")}: ${destination}`);
      } catch (caught) {
        setError({
          code: "BROWSER_AUDIT_EXPORT_FAILED",
          message: caught instanceof Error ? caught.message : String(caught),
        });
      }
    },
    [file, result, t],
  );

  const performancePct =
    typeof summary.performance_score_pct === "number"
      ? (summary.performance_score_pct as number)
      : null;

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("lighthouseLabel")}
        description={t("lighthouseDescription")}
        accept=".json,.gz,.json.gz"
        selected={file}
        onSelect={setFile}
        onClear={() => setFile(null)}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          { displayName: t("lighthouseFilterReports"), pattern: "*.json;*.json.gz;*.gz" },
          { displayName: "All files", pattern: "*.*" },
        ]}
        rightSlot={
          <Button type="button" size="sm" disabled={!file || running} onClick={() => void analyze()}>
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
      {notice && (
        <p className="rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground" role="status">
          {notice}
        </p>
      )}

      {!result && !error && (
        <Card>
          <CardHeader>
            <CardTitle>{t("lighthouseGuidanceTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>{t("lighthouseEmpty")}</p>
            <ul className="list-disc space-y-1 pl-5">
              <li>{t("lighthouseGuidanceStep1")}</li>
              <li>{t("lighthouseGuidanceStep2")}</li>
              <li>{t("lighthouseGuidanceStep3")}</li>
            </ul>
          </CardContent>
        </Card>
      )}

      {result && (
        <>
          {/* Imported-score provenance — the page must be honest that these
              numbers come from the report, not from ArchScope. */}
          <Card className="border-sky-500/50">
            <CardHeader>
              <CardTitle>{t("lighthouseProvenanceTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-1 text-sm text-muted-foreground">
              <p>{provenance.scoreDisclosure || t("lighthouseProvenanceFallback")}</p>
              <p className="font-mono text-xs">
                {t("lighthouseProvenanceSource")}: {provenance.scoreSource || "—"} ·{" "}
                {t("lighthouseProvenanceRecomputed")}:{" "}
                {provenance.scoresRecomputed ? t("lighthouseRecomputedYes") : t("lighthouseRecomputedNo")}
                {contract?.version ? ` · v${contract.version}` : ""}
              </p>
            </CardContent>
          </Card>

          <section className="grid gap-3 sm:grid-cols-4">
            <MetricCard
              label={t("lighthouseMetricPerformance")}
              value={performancePct != null ? `${performancePct}` : "—"}
            />
            <MetricCard
              label={t("lighthouseMetricAudits")}
              value={formatNumber(Number(summary.audit_count ?? 0))}
            />
            <MetricCard
              label={t("lighthouseMetricRequests")}
              value={formatNumber(Number(summary.network_request_count ?? 0))}
            />
            <MetricCard
              label={t("lighthouseMetricTransfer")}
              value={formatBytes(summary.transfer_size_bytes)}
            />
          </section>

          {(summary.requested_url || summary.final_url || summary.form_factor) && (
            <Card>
              <CardContent className="grid gap-2 py-4 text-xs text-muted-foreground sm:grid-cols-2">
                {summary.final_url ? (
                  <div>
                    <span className="font-medium text-foreground">{t("lighthouseFinalUrl")}: </span>
                    <span className="break-all font-mono">{String(summary.final_url)}</span>
                  </div>
                ) : null}
                {summary.form_factor ? (
                  <div>
                    <span className="font-medium text-foreground">{t("lighthouseFormFactor")}: </span>
                    {String(summary.form_factor)}
                  </div>
                ) : null}
                {summary.lighthouse_version ? (
                  <div>
                    <span className="font-medium text-foreground">{t("lighthouseVersion")}: </span>
                    {String(summary.lighthouse_version)}
                  </div>
                ) : null}
                {summary.fetch_time ? (
                  <div>
                    <span className="font-medium text-foreground">{t("lighthouseFetchTime")}: </span>
                    <span className="font-mono">{String(summary.fetch_time)}</span>
                  </div>
                ) : null}
              </CardContent>
            </Card>
          )}

          {categories.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("lighthouseCategoriesTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {categories.map((row, index) => {
                  const pct = typeof row.score_pct === "number" ? (row.score_pct as number) : null;
                  const band = scorePctBand(pct);
                  return (
                    <div key={index} className="rounded-md border border-border p-3">
                      <div className="flex items-baseline justify-between gap-2">
                        <span className="truncate text-sm" title={String(row.title ?? row.id ?? "")}>
                          {String(row.title ?? row.id ?? "")}
                        </span>
                        <span className={`text-lg font-semibold tabular-nums ${BAND_CLASS[band]}`}>
                          {pct != null ? pct : "—"}
                        </span>
                      </div>
                      {pct != null && (
                        <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
                          <div
                            className={
                              band === "good"
                                ? "h-full bg-emerald-500"
                                : band === "average"
                                  ? "h-full bg-amber-500"
                                  : "h-full bg-red-500"
                            }
                            style={{ width: `${Math.max(0, Math.min(100, pct))}%` }}
                          />
                        </div>
                      )}
                    </div>
                  );
                })}
              </CardContent>
            </Card>
          )}

          {coreMetrics.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("lighthouseCoreVitalsTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <caption className="sr-only">{t("lighthouseCoreVitalsTitle")}</caption>
                  <thead>
                    <tr className="border-b border-border text-muted-foreground">
                      <th scope="col" className="p-2 font-medium">
                        {t("lighthouseColMetric")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColValue")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColScore")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {coreMetrics.map((row, index) => {
                      const pct = scoreToPct(row.score);
                      return (
                        <tr key={index} className="border-t">
                          <td className="p-2" title={String(row.id ?? "")}>
                            {String(row.title ?? row.id ?? "")}
                          </td>
                          <td className="p-2 text-right font-mono tabular-nums">
                            {row.display_value != null
                              ? String(row.display_value)
                              : `${formatNumber(Number(row.value ?? 0))} ${String(row.unit ?? "")}`.trim()}
                          </td>
                          <td className={`p-2 text-right tabular-nums ${BAND_CLASS[scorePctBand(pct)]}`}>
                            {pct != null ? pct : "—"}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {findings.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("lighthouseFindingsTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {findings.map((finding, index) => (
                  <div
                    key={index}
                    className="flex items-start justify-between gap-3 rounded-md border border-border p-3"
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase text-muted-foreground">
                          {String(finding.severity ?? "info")}
                        </span>
                        <span className="font-mono text-xs">{String(finding.code ?? "")}</span>
                      </div>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {String(finding.message ?? "")}
                      </p>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => captureFinding(finding)}
                    >
                      <Plus className="h-3.5 w-3.5" />
                      {t("evidenceAdd")}
                    </Button>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}

          {audits.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("lighthouseAuditsTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto">
                <p className="mb-2 text-xs text-muted-foreground">{t("lighthouseAuditsBounded")}</p>
                <table className="w-full text-left text-xs">
                  <caption className="sr-only">{t("lighthouseAuditsTitle")}</caption>
                  <thead>
                    <tr className="border-b border-border text-muted-foreground">
                      <th scope="col" className="p-2 font-medium">
                        {t("lighthouseColAudit")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColScore")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColValue")}
                      </th>
                      <th scope="col" className="p-2 font-medium" />
                    </tr>
                  </thead>
                  <tbody>
                    {audits.map((row, index) => {
                      const pct = typeof row.score_pct === "number" ? (row.score_pct as number) : null;
                      return (
                        <tr key={index} className="border-t align-top">
                          <td className="max-w-md p-2">
                            <div className="truncate font-medium" title={String(row.title ?? row.id ?? "")}>
                              {String(row.title ?? row.id ?? "")}
                            </div>
                            <div className="truncate font-mono text-[10px] text-muted-foreground" title={String(row.id ?? "")}>
                              {String(row.id ?? "")}
                            </div>
                          </td>
                          <td className={`p-2 text-right tabular-nums ${BAND_CLASS[scorePctBand(pct)]}`}>
                            {pct != null ? pct : "—"}
                          </td>
                          <td className="max-w-[12rem] truncate p-2 text-right font-mono" title={String(row.display_value ?? "")}>
                            {row.display_value != null ? String(row.display_value) : "—"}
                          </td>
                          <td className="p-2 text-right">
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              onClick={() => captureAudit(row)}
                              aria-label={`${t("evidenceAdd")}: ${String(row.title ?? row.id ?? "")}`}
                            >
                              <Plus className="h-3.5 w-3.5" />
                            </Button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {networkRequests.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("lighthouseNetworkTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto">
                <p className="mb-2 text-xs text-muted-foreground">{t("lighthouseNetworkBounded")}</p>
                <table className="w-full text-left text-xs">
                  <caption className="sr-only">{t("lighthouseNetworkTitle")}</caption>
                  <thead>
                    <tr className="border-b border-border text-muted-foreground">
                      <th scope="col" className="p-2 font-medium">
                        {t("lighthouseColUrl")}
                      </th>
                      <th scope="col" className="p-2 font-medium">
                        {t("lighthouseColType")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColTransfer")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColDuration")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {networkRequests.map((row, index) => (
                      <tr key={index} className="border-t">
                        <td className="max-w-md truncate p-2 font-mono" title={String(row.url ?? "")}>
                          {String(row.url ?? "")}
                        </td>
                        <td className="p-2">{String(row.resource_type ?? row.mime_type ?? "")}</td>
                        <td className="p-2 text-right tabular-nums">
                          {formatBytes(row.transfer_size_bytes)}
                        </td>
                        <td className="p-2 text-right tabular-nums">
                          {row.duration_ms != null ? `${formatNumber(Number(row.duration_ms))} ms` : "—"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {resourceDist.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("lighthouseResourcesTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <caption className="sr-only">{t("lighthouseResourcesTitle")}</caption>
                  <thead>
                    <tr className="border-b border-border text-muted-foreground">
                      <th scope="col" className="p-2 font-medium">
                        {t("lighthouseColType")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColRequests")}
                      </th>
                      <th scope="col" className="p-2 text-right font-medium">
                        {t("lighthouseColTransfer")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {resourceDist.map((row, index) => (
                      <tr key={index} className="border-t">
                        <td className="p-2">{String(row.resource_type ?? "")}</td>
                        <td className="p-2 text-right tabular-nums">
                          {formatNumber(Number(row.request_count ?? 0))}
                        </td>
                        <td className="p-2 text-right tabular-nums">
                          {formatBytes(row.transfer_size_bytes)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="inline-flex items-center gap-2">
                {t("lighthouseDiagnosticsTitle")}
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

          <Card>
            <CardHeader>
              <CardTitle>{t("lighthouseExportTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center gap-2">
              <p className="mr-2 text-xs text-muted-foreground">{t("lighthouseExportHint")}</p>
              <Button type="button" variant="outline" size="sm" onClick={() => void exportReport("html")}>
                <FileDown className="h-3.5 w-3.5" />
                {t("lighthouseExportHtml")}
              </Button>
              <Button type="button" variant="outline" size="sm" onClick={() => void exportReport("json")}>
                <FileDown className="h-3.5 w-3.5" />
                {t("lighthouseExportJson")}
              </Button>
            </CardContent>
          </Card>
        </>
      )}
    </main>
  );
}
