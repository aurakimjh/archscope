import { Download } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { type ECharts, type EChartsOption } from "@/charts/echartsCore";
import { ChartPanel } from "@/components/ChartPanel";
import { HelpedLabel, HelpedTitle } from "@/components/HelpTip";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  selectWorkspaceResult,
  useAnalysisWorkspace,
  type AnalysisWorkspaceEntry,
} from "@/state/analysisWorkspace";

type Renderer = "canvas" | "svg";

type ChartTemplate = {
  id: string;
  labelKey: MessageKey;
  resultTypes: string[];
  build: (entry: AnalysisWorkspaceEntry, labels: ChartStudioLabels) => ChartBuild;
};

type ChartBuild = {
  option: EChartsOption;
  rows: Array<Record<string, string | number | null>>;
};

type ChartStudioLabels = {
  count: string;
  milliseconds: string;
  duration: string;
  delta: string;
};

const TEMPLATES: ChartTemplate[] = [
  {
    id: "access.requests",
    labelKey: "chartTemplateAccessRequests",
    resultTypes: ["access_log"],
    build: (entry, labels) => timeSeriesOption(entry, "requests_per_minute", labels.count),
  },
  {
    id: "access.p95",
    labelKey: "chartTemplateAccessP95",
    resultTypes: ["access_log"],
    build: (entry, labels) => timeSeriesOption(entry, "p95_response_time_per_minute", labels.milliseconds),
  },
  {
    id: "gc.pause",
    labelKey: "chartTemplateGcPause",
    resultTypes: ["gc_log"],
    build: (entry, labels) => timeSeriesOption(entry, "pause_timeline", labels.milliseconds),
  },
  {
    id: "gc.heap",
    labelKey: "chartTemplateGcHeap",
    resultTypes: ["gc_log"],
    build: (entry, labels) => timeSeriesOption(entry, "heap_after_mb", "MB"),
  },
  {
    id: "trace.services",
    labelKey: "chartTemplateTraceServices",
    resultTypes: ["trace_import"],
    build: (entry, labels) =>
      barRowsOption(entry, "service_summary", "service", "total_duration_ms", labels.duration),
  },
  {
    id: "trace.dependencies",
    labelKey: "chartTemplateTraceDependencies",
    resultTypes: ["trace_import"],
    build: (entry, labels) =>
      barRowsOption(entry, "service_dependencies", "callee", "total_duration_ms", labels.duration),
  },
  {
    id: "profiler.components",
    labelKey: "chartTemplateProfilerComponents",
    resultTypes: ["profiler_collapsed"],
    build: (entry, labels) =>
      barSeriesOption(entry, "component_breakdown", "component", "samples", labels.count),
  },
  {
    id: "comparison.deltas",
    labelKey: "chartTemplateComparisonDeltas",
    resultTypes: ["comparison_report"],
    build: (entry, labels) =>
      barRowsOption(entry, "summary_metric_deltas", "metric", "delta", labels.delta),
  },
];

export function ChartStudioPage(): React.JSX.Element {
  const { locale, t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const [selectedID, setSelectedID] = useState(workspace.active_id ?? "");
  const [templateID, setTemplateID] = useState(TEMPLATES[0].id);
  const [renderer, setRenderer] = useState<Renderer>("canvas");
  const [chart, setChart] = useState<ECharts | null>(null);
  const [error, setError] = useState<string | null>(null);

  const compatibleEntries = useMemo(
    () =>
      workspace.entries.filter((entry) =>
        TEMPLATES.some((template) => template.resultTypes.includes(entry.result_type)),
      ),
    [workspace.entries],
  );

  const selectedEntry = useMemo(
    () =>
      compatibleEntries.find((entry) => entry.id === selectedID) ??
      compatibleEntries.find((entry) => entry.id === workspace.active_id) ??
      compatibleEntries[0] ??
      null,
    [compatibleEntries, selectedID, workspace.active_id],
  );

  const templates = useMemo(
    () =>
      selectedEntry
        ? TEMPLATES.filter((template) => template.resultTypes.includes(selectedEntry.result_type))
        : [],
    [selectedEntry],
  );

  useEffect(() => {
    if (selectedEntry && selectedID !== selectedEntry.id) {
      setSelectedID(selectedEntry.id);
      selectWorkspaceResult(selectedEntry.id);
    }
  }, [selectedEntry, selectedID]);

  useEffect(() => {
    if (templates.length > 0 && !templates.some((template) => template.id === templateID)) {
      setTemplateID(templates[0].id);
    }
  }, [templateID, templates]);

  const labels = useMemo<ChartStudioLabels>(
    () => ({
      count: t("count"),
      milliseconds: t("millisecondsAxis"),
      duration: t("millisecondsAxis"),
      delta: "Delta",
    }),
    [t],
  );

  const selectedTemplate = templates.find((template) => template.id === templateID) ?? templates[0] ?? null;
  const build = selectedEntry && selectedTemplate ? selectedTemplate.build(selectedEntry, labels) : null;
  const title = selectedTemplate ? t(selectedTemplate.labelKey) : t("navChartStudio");

  const exportImage = (type: "png" | "svg"): void => {
    if (!chart) return;
    try {
      const url = chart.getDataURL({
        type,
        pixelRatio: type === "png" ? 2 : 1,
        backgroundColor: "#ffffff",
      });
      downloadURL(url, `archscope-${selectedTemplate?.id ?? "chart"}.${type}`);
      setError(null);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    }
  };

  const exportCSV = (): void => {
    if (!build) return;
    downloadBlob(rowsToCSV(build.rows), `archscope-${selectedTemplate?.id ?? "chart"}.csv`, "text/csv");
  };

  const onReady = useCallback((next: ECharts | null) => setChart(next), []);

  return (
    <main className="content">
      <section className="card">
        <h2>
          <HelpedTitle help={getHelpText(locale, "pageChartStudio")}>
            {t("navChartStudio")}
          </HelpedTitle>
        </h2>
        <div className="workspace-form-grid">
          <label className="workspace-field">
            <HelpedLabel help={getHelpText(locale, "optionWorkspaceResult")}>
              {t("workspaceResult")}
            </HelpedLabel>
            <select
              value={selectedEntry?.id ?? ""}
              onChange={(event) => {
                setSelectedID(event.target.value);
                selectWorkspaceResult(event.target.value || null);
              }}
            >
              {compatibleEntries.length === 0 ? (
                <option value="">{t("chartStudioEmpty")}</option>
              ) : (
                compatibleEntries.map((entry) => (
                  <option key={entry.id} value={entry.id}>
                    {entry.title}
                  </option>
                ))
              )}
            </select>
          </label>
          <label className="workspace-field">
            <HelpedLabel help={getHelpText(locale, "optionChartTemplate")}>
              {t("chartTemplate")}
            </HelpedLabel>
            <select value={selectedTemplate?.id ?? ""} onChange={(event) => setTemplateID(event.target.value)}>
              {templates.map((template) => (
                <option key={template.id} value={template.id}>
                  {t(template.labelKey)}
                </option>
              ))}
            </select>
          </label>
          <label className="workspace-field">
            <HelpedLabel help={getHelpText(locale, "optionRenderer")}>
              {t("renderer")}
            </HelpedLabel>
            <select value={renderer} onChange={(event) => setRenderer(event.target.value as Renderer)}>
              <option value="canvas">Canvas</option>
              <option value="svg">SVG</option>
            </select>
          </label>
        </div>
      </section>

      {build ? (
        <ChartPanel
          key={`${renderer}-${selectedTemplate?.id}-${selectedEntry?.id}`}
          title={title}
          option={build.option}
          renderer={renderer}
          height={380}
          onReady={onReady}
          actions={
            <div className="workspace-card-actions">
              <Button type="button" size="sm" variant="outline" onClick={() => exportImage("png")}>
                <Download className="nav-lucide-sm" />
                PNG
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={() => exportImage("svg")}>
                SVG
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={exportCSV}>
                CSV
              </Button>
            </div>
          }
        />
      ) : (
        <Card>
          <CardContent className="p-4">
            <p className="muted">{t("chartStudioEmpty")}</p>
          </CardContent>
        </Card>
      )}

      {error ? (
        <div className="alert error">
          <strong>{t("error")}:</strong> {error}
        </div>
      ) : null}
    </main>
  );
}

function timeSeriesOption(entry: AnalysisWorkspaceEntry, key: string, unit: string): ChartBuild {
  const rows = rowsFrom(entry.result.series, key)
    .map((row) => ({
      time: stringValue(row.time),
      value: numberValue(row.value),
    }))
    .filter((row) => row.time && row.value != null);
  return {
    rows,
    option: {
      grid: { left: 64, right: 28, top: 30, bottom: 48 },
      tooltip: { trigger: "axis" },
      xAxis: { type: "category", data: rows.map((row) => row.time), axisLabel: { fontSize: 10 } },
      yAxis: { type: "value", name: unit, axisLabel: { fontSize: 10 } },
      series: [
        {
          type: "line",
          smooth: true,
          showSymbol: rows.length <= 120,
          data: rows.map((row) => row.value),
          areaStyle: { opacity: 0.14 },
        },
      ],
    },
  };
}

function barRowsOption(
  entry: AnalysisWorkspaceEntry,
  key: string,
  labelField: string,
  valueField: string,
  unit: string,
): ChartBuild {
  const rows = rowsFrom(entry.result.tables, key)
    .map((row) => ({
      label: stringValue(row[labelField]),
      value: numberValue(row[valueField]),
    }))
    .filter((row) => row.label && row.value != null)
    .sort((a, b) => Math.abs(b.value ?? 0) - Math.abs(a.value ?? 0))
    .slice(0, 20);
  return {
    rows,
    option: {
      grid: { left: 160, right: 28, top: 24, bottom: 42 },
      tooltip: { trigger: "axis" },
      xAxis: { type: "value", name: unit, axisLabel: { fontSize: 10 } },
      yAxis: {
        type: "category",
        inverse: true,
        data: rows.map((row) => row.label),
        axisLabel: { fontSize: 10 },
      },
      series: [
        {
          type: "bar",
          data: rows.map((row) => row.value),
          itemStyle: { color: "#2563eb" },
        },
      ],
    },
  };
}

function barSeriesOption(
  entry: AnalysisWorkspaceEntry,
  key: string,
  labelField: string,
  valueField: string,
  unit: string,
): ChartBuild {
  const rows = rowsFrom(entry.result.series, key)
    .map((row) => ({
      label: stringValue(row[labelField]),
      value: numberValue(row[valueField]),
    }))
    .filter((row) => row.label && row.value != null)
    .sort((a, b) => Math.abs(b.value ?? 0) - Math.abs(a.value ?? 0))
    .slice(0, 20);
  return {
    rows,
    option: {
      grid: { left: 160, right: 28, top: 24, bottom: 42 },
      tooltip: { trigger: "axis" },
      xAxis: { type: "value", name: unit, axisLabel: { fontSize: 10 } },
      yAxis: {
        type: "category",
        inverse: true,
        data: rows.map((row) => row.label),
        axisLabel: { fontSize: 10 },
      },
      series: [{ type: "bar", data: rows.map((row) => row.value), itemStyle: { color: "#0f766e" } }],
    },
  };
}

function rowsFrom(container: object | undefined, key: string): Array<Record<string, unknown>> {
  const value = (container as Record<string, unknown> | undefined)?.[key];
  return Array.isArray(value) ? (value as Array<Record<string, unknown>>) : [];
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : value == null ? "" : String(value);
}

function numberValue(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function rowsToCSV(rows: Array<Record<string, string | number | null>>): string {
  if (rows.length === 0) return "";
  const headers = Object.keys(rows[0]);
  const lines = [
    headers.join(","),
    ...rows.map((row) => headers.map((header) => csvCell(row[header])).join(",")),
  ];
  return `${lines.join("\n")}\n`;
}

function csvCell(value: string | number | null | undefined): string {
  const text = value == null ? "" : String(value);
  return /[",\n]/.test(text) ? `"${text.replace(/"/g, "\"\"")}"` : text;
}

function downloadBlob(content: string, filename: string, type: string): void {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  downloadURL(url, filename);
  URL.revokeObjectURL(url);
}

function downloadURL(url: string, filename: string): void {
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
}
