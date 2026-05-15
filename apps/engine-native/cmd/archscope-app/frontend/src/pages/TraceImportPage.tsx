import { Loader2, Plus, Play, Square } from "lucide-react";
import type React from "react";
import { useCallback, useMemo, useRef, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  BridgeError,
  JvmFinding,
  TraceImportAnalysisResult,
  TraceImportDependencyRow,
  TraceImportSpanRow,
  TraceImportTraceRow,
} from "@/bridge/types";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { AnalyzerOptionsDock } from "@/components/AnalyzerOptionsDock";
import { ChartPanel } from "@/components/ChartPanel";
import { MetricCard } from "@/components/MetricCard";
import {
  WailsFileDock,
  type FileDockSelection,
} from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import {
  traceDependencyOption,
  traceServiceOption,
  type TraceImportChartLabels,
} from "@/charts/traceImportCharts";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import { addEvidenceCard } from "@/state/evidenceBoard";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
} from "@/utils/formatters";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

const MAX_ROWS = 200;
const TRACE_FORMATS = [
  "auto",
  "otlp-json",
  "zipkin-v2-json",
  "elastic-apm-search-json",
  "elastic-apm-source-ndjson",
  "jaeger-query-json",
  "skywalking-graphql-json",
] as const;

export function TraceImportPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(
    null,
  );
  const [format, setFormat] = useState<(typeof TRACE_FORMATS)[number]>("auto");
  const [topN, setTopN] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<TraceImportAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const inflightRef = useRef<symbol | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;
  const findings = result?.metadata?.findings ?? ([] as JvmFinding[]);
  const sourceFile = result?.source_files?.[0] ?? filePath;

  const chartLabels = useMemo<TraceImportChartLabels>(
    () => ({
      dependencyTitle: t("traceDependencyChart"),
      serviceTitle: t("traceServiceChart"),
      durationAxis: t("millisecondsAxis"),
      countAxis: t("count"),
    }),
    [t],
  );
  const dependencyOption = useMemo(
    () => traceDependencyOption(result, chartLabels),
    [chartLabels, result],
  );
  const serviceOption = useMemo(
    () => traceServiceOption(result, chartLabels),
    [chartLabels, result],
  );

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setState("ready");
    setError(null);
    setEngineMessages([]);
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    if (state !== "running") setState("idle");
  }, [state]);

  const analyze = useCallback(async () => {
    if (!canAnalyze) return;
    const parsedTopN = parseOptionalPositiveInteger(topN);
    if (parsedTopN === null) {
      setError({ code: "INVALID_OPTION", message: t("invalidAnalyzerOptions") });
      setState("error");
      return;
    }
    setState("running");
    setError(null);
    setEngineMessages([]);
    const token = Symbol("trace-import-analysis");
    inflightRef.current = token;
    try {
      const response = await engine.analyzeTraceImport({
        path: filePath,
        format,
        topN: parsedTopN ?? undefined,
      });
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      setResult(response);
      addWorkspaceResult({
        result: response,
        title: `trace_import: ${selectedFile?.originalName ?? response.source_files?.[0] ?? ""}`,
        sourceLabel: selectedFile?.originalName,
      });
      setState("success");
    } catch (caught) {
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      setError({
        code: "ENGINE_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
      setState("error");
    }
  }, [canAnalyze, filePath, format, selectedFile?.originalName, t, topN]);

  const cancelAnalysis = useCallback(() => {
    inflightRef.current = null;
    setState("ready");
    setEngineMessages([t("analysisCanceled")]);
  }, [t]);

  const addFindingEvidence = useCallback(
    (finding: JvmFinding) => {
      addEvidenceCard({
        analyzer: "trace_import",
        source_kind: "finding",
        title: finding.code,
        summary: finding.message,
        severity: finding.severity,
        source_file: sourceFile,
        source_ref: finding.code,
        payload: finding as unknown as Record<string, unknown>,
      });
      setEngineMessages([t("evidenceAdded")]);
    },
    [sourceFile, t],
  );

  return (
    <main className="flex flex-col gap-5 p-5">
      <div className="flex items-stretch gap-3">
        <WailsFileDock
          className="min-w-0 flex-1"
          label={t("selectTraceImportFile")}
          description={t("dropOrBrowseTraceImport")}
          accept=".json,.jsonl,.ndjson,.txt"
          selected={selectedFile}
          onSelect={handleFileSelected}
          onClear={handleClearFile}
          browseLabel={t("browseFile")}
          dropHereLabel={t("dropHere")}
          errorLabel={t("error")}
          fileFilters={[
            { displayName: "Trace exports", pattern: "*.json;*.jsonl;*.ndjson;*.txt" },
            { displayName: "All files", pattern: "*.*" },
          ]}
          rightSlot={
            <div className="flex items-center gap-2">
              <Button
                type="button"
                size="sm"
                disabled={!canAnalyze}
                onClick={() => void analyze()}
              >
                {state === "running" ? (
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
              {state === "running" && (
                <Button type="button" variant="outline" size="sm" onClick={cancelAnalysis}>
                  <Square className="h-3.5 w-3.5" />
                  {t("cancel")}
                </Button>
              )}
            </div>
          }
        />
        <AnalyzerOptionsDock title={t("analyzerOptions")}>
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("format")}</span>
            <select
              className="h-9 w-56 rounded-md border border-input bg-background px-3 text-sm"
              value={format}
              onChange={(event) =>
                setFormat(event.currentTarget.value as (typeof TRACE_FORMATS)[number])
              }
            >
              {TRACE_FORMATS.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("topN")}</span>
            <Input
              value={topN}
              onChange={(event) => setTopN(event.currentTarget.value)}
              placeholder="50"
              inputMode="numeric"
            />
          </label>
        </AnalyzerOptionsDock>
      </div>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      <Tabs defaultValue="summary">
        <TabsList>
          <TabsTrigger value="summary">{t("tabSummaryGc")}</TabsTrigger>
          <TabsTrigger value="services">{t("traceTabServices")}</TabsTrigger>
          <TabsTrigger value="traces">{t("traceTabTraces")}</TabsTrigger>
          <TabsTrigger value="spans">{t("traceTabSpans")}</TabsTrigger>
          <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
        </TabsList>

        <TabsContent value="summary" className="mt-4">
          <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
            <MetricCard label={t("traceTotalSpans")} value={formatNumber(summary?.total_spans)} />
            <MetricCard label={t("traceUniqueTraces")} value={formatNumber(summary?.unique_traces)} />
            <MetricCard label={t("traceUniqueServices")} value={formatNumber(summary?.unique_services)} />
            <MetricCard label={t("traceDependencies")} value={formatNumber(summary?.unique_dependencies)} />
            <MetricCard label={t("traceErrorSpans")} value={formatNumber(summary?.error_spans)} />
            <MetricCard label={t("traceMissingParents")} value={formatNumber(summary?.missing_parent_spans)} />
            <MetricCard label={t("traceP95Duration")} value={formatMilliseconds(summary?.p95_trace_duration_ms)} />
            <MetricCard label={t("traceMaxDuration")} value={formatMilliseconds(summary?.max_trace_duration_ms)} />
            <MetricCard label={t("traceClockSkew")} value={formatNumber(summary?.clock_skew_suspected_spans)} />
            <MetricCard label={t("traceHighErrorEdges")} value={formatNumber(summary?.high_error_service_edges)} />
          </section>
          <FindingsList
            findings={findings}
            t={t}
            onAddEvidence={addFindingEvidence}
          />
        </TabsContent>

        <TabsContent value="services" className="mt-4">
          <div className="grid gap-4">
            <ChartPanel title={t("traceDependencyChart")} option={dependencyOption} busy={state === "running"} height={320} />
            <ChartPanel title={t("traceServiceChart")} option={serviceOption} busy={state === "running"} height={320} />
            <DependencyTable
              rows={result?.tables.service_dependencies?.slice(0, MAX_ROWS) ?? []}
              sourceFile={sourceFile}
              t={t}
              onEvidence={(row) => {
                addEvidenceCard({
                  analyzer: "trace_import",
                  source_kind: "table_row",
                  title: `${row.caller} -> ${row.callee}`,
                  summary: `${row.call_count} calls, ${row.error_count} errors`,
                  severity: row.error_count > 0 ? "warning" : "info",
                  source_file: sourceFile,
                  source_ref: "service_dependencies",
                  payload: row as unknown as Record<string, unknown>,
                });
                setEngineMessages([t("evidenceAdded")]);
              }}
            />
          </div>
        </TabsContent>

        <TabsContent value="traces" className="mt-4">
          <TraceTable
            rows={result?.tables.traces?.slice(0, MAX_ROWS) ?? []}
            sourceFile={sourceFile}
            t={t}
            onEvidence={(row) => {
              addEvidenceCard({
                analyzer: "trace_import",
                source_kind: "table_row",
                title: row.trace_id,
                summary: `${row.span_count} spans, ${row.duration_ms.toFixed(2)} ms`,
                severity: row.error_count > 0 ? "warning" : "info",
                source_file: sourceFile,
                source_ref: "traces",
                payload: row as unknown as Record<string, unknown>,
              });
              setEngineMessages([t("evidenceAdded")]);
            }}
          />
          <CriticalPathTable
            rows={result?.tables.critical_paths?.slice(0, MAX_ROWS) ?? []}
            t={t}
          />
        </TabsContent>

        <TabsContent value="spans" className="mt-4">
          <SpanTable rows={result?.tables.spans?.slice(0, MAX_ROWS) ?? []} t={t} />
        </TabsContent>

        <TabsContent value="diagnostics" className="mt-4">
          <DiagnosticsPanel
            diagnostics={result?.metadata?.diagnostics}
            labels={{
              title: t("parserDiagnostics"),
              parsedRecords: t("parsedRecords"),
              skippedLines: t("skippedLines"),
              samples: t("diagnosticSamples"),
            }}
          />
        </TabsContent>
      </Tabs>
    </main>
  );
}

function FindingsList({
  findings,
  t,
  onAddEvidence,
}: {
  findings: JvmFinding[];
  t: (key: MessageKey) => string;
  onAddEvidence: (finding: JvmFinding) => void;
}): JSX.Element {
  return (
    <Card className="mt-4">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{t("findings")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 pt-0 text-sm">
        {findings.length === 0 ? (
          <p className="text-muted-foreground">—</p>
        ) : (
          findings.map((finding, idx) => (
            <div key={`${finding.code}-${idx}`} className="flex items-start gap-2">
              <span className="mt-0.5 rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">
                {finding.severity}
              </span>
              <p className="min-w-0 flex-1 leading-relaxed">{finding.message}</p>
              <Button type="button" size="sm" variant="outline" onClick={() => onAddEvidence(finding)}>
                <Plus className="h-3.5 w-3.5" />
                {t("evidenceAdd")}
              </Button>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}

function DependencyTable({
  rows,
  sourceFile,
  t,
  onEvidence,
}: {
  rows: TraceImportDependencyRow[];
  sourceFile: string;
  t: (key: MessageKey) => string;
  onEvidence: (row: TraceImportDependencyRow) => void;
}): JSX.Element {
  void sourceFile;
  return (
    <TableCard title={t("traceDependencyTable")}>
      <thead>
        <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
          <th className="px-3 py-2 text-left">{t("traceCaller")}</th>
          <th className="px-3 py-2 text-left">{t("traceCallee")}</th>
          <th className="px-3 py-2 text-right">{t("count")}</th>
          <th className="px-3 py-2 text-right">{t("traceAvgDuration")}</th>
          <th className="px-3 py-2 text-right">{t("traceErrorRate")}</th>
          <th className="px-3 py-2 text-right">{t("evidence")}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row, idx) => (
          <tr key={`${row.caller}-${row.callee}-${idx}`} className="border-b border-border last:border-0">
            <td className="px-3 py-1.5 font-mono text-[11px]">{row.caller}</td>
            <td className="px-3 py-1.5 font-mono text-[11px]">{row.callee}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.call_count}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.avg_duration_ms.toFixed(2)}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{formatPercent((row.error_rate ?? 0) * 100)}</td>
            <td className="px-3 py-1.5 text-right">
              <Button type="button" size="sm" variant="outline" onClick={() => onEvidence(row)}>
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </td>
          </tr>
        ))}
      </tbody>
    </TableCard>
  );
}

function TraceTable({
  rows,
  sourceFile,
  t,
  onEvidence,
}: {
  rows: TraceImportTraceRow[];
  sourceFile: string;
  t: (key: MessageKey) => string;
  onEvidence: (row: TraceImportTraceRow) => void;
}): JSX.Element {
  void sourceFile;
  return (
    <TableCard title={t("traceTable")}>
      <thead>
        <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
          <th className="px-3 py-2 text-left">{t("traceId")}</th>
          <th className="px-3 py-2 text-right">{t("traceSpanCount")}</th>
          <th className="px-3 py-2 text-right">{t("traceDuration")}</th>
          <th className="px-3 py-2 text-right">{t("traceErrors")}</th>
          <th className="px-3 py-2 text-left">{t("traceSlowestSpan")}</th>
          <th className="px-3 py-2 text-right">{t("evidence")}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.trace_id} className="border-b border-border last:border-0">
            <td className="px-3 py-1.5 font-mono text-[11px]">{row.trace_id}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.span_count}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.duration_ms.toFixed(2)}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.error_count}</td>
            <td className="px-3 py-1.5">{row.slowest_span}</td>
            <td className="px-3 py-1.5 text-right">
              <Button type="button" size="sm" variant="outline" onClick={() => onEvidence(row)}>
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </td>
          </tr>
        ))}
      </tbody>
    </TableCard>
  );
}

function CriticalPathTable({
  rows,
  t,
}: {
  rows: NonNullable<TraceImportAnalysisResult["tables"]["critical_paths"]>;
  t: (key: MessageKey) => string;
}): JSX.Element {
  return (
    <TableCard title={t("traceCriticalPaths")}>
      <thead>
        <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
          <th className="px-3 py-2 text-left">{t("traceId")}</th>
          <th className="px-3 py-2 text-right">{t("traceCriticalPath")}</th>
          <th className="px-3 py-2 text-right">{t("traceSpanCount")}</th>
          <th className="px-3 py-2 text-left">{t("traceServices")}</th>
          <th className="px-3 py-2 text-left">{t("traceSpanNames")}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.trace_id} className="border-b border-border last:border-0">
            <td className="px-3 py-1.5 font-mono text-[11px]">{row.trace_id}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.critical_path_ms.toFixed(2)}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.span_count}</td>
            <td className="px-3 py-1.5">{row.services.join(", ")}</td>
            <td className="max-w-[28rem] truncate px-3 py-1.5">{row.span_names.join(" > ")}</td>
          </tr>
        ))}
      </tbody>
    </TableCard>
  );
}

function SpanTable({
  rows,
  t,
}: {
  rows: TraceImportSpanRow[];
  t: (key: MessageKey) => string;
}): JSX.Element {
  return (
    <TableCard title={t("traceSpanTable")}>
      <thead>
        <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
          <th className="px-3 py-2 text-left">{t("traceId")}</th>
          <th className="px-3 py-2 text-left">{t("traceSpanId")}</th>
          <th className="px-3 py-2 text-left">{t("traceService")}</th>
          <th className="px-3 py-2 text-left">{t("traceSpanName")}</th>
          <th className="px-3 py-2 text-left">{t("traceKind")}</th>
          <th className="px-3 py-2 text-right">{t("traceDuration")}</th>
          <th className="px-3 py-2 text-right">{t("traceErrors")}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={`${row.trace_id}-${row.span_id}`} className="border-b border-border last:border-0">
            <td className="px-3 py-1.5 font-mono text-[11px]">{row.trace_id}</td>
            <td className="px-3 py-1.5 font-mono text-[11px]">{row.span_id}</td>
            <td className="px-3 py-1.5">{row.service_name ?? "-"}</td>
            <td className="px-3 py-1.5">{row.name}</td>
            <td className="px-3 py-1.5">{row.kind}</td>
            <td className="px-3 py-1.5 text-right tabular-nums">{row.duration_ms.toFixed(2)}</td>
            <td className="px-3 py-1.5 text-right">{row.error ? "Y" : "-"}</td>
          </tr>
        ))}
      </tbody>
    </TableCard>
  );
}

function TableCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <Card className="mt-4">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-xs">{children}</table>
      </CardContent>
    </Card>
  );
}

function parseOptionalPositiveInteger(value: string): number | undefined | null {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return null;
  return parsed;
}
