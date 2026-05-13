import { Dialogs } from "@wailsio/runtime";
import { GitCompareArrows, Loader2, Play } from "lucide-react";
import { useState } from "react";

import { engine } from "@/bridge/engine";
import type { AnalysisResult } from "@/bridge/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useI18n } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import { formatNumber } from "@/utils/formatters";

type MetricDeltaRow = {
  metric?: string;
  before?: number;
  after?: number;
  delta?: number;
  change_percent?: number | null;
};

export function ReportDiffPage(): JSX.Element {
  const { t } = useI18n();
  const [beforePath, setBeforePath] = useState("");
  const [afterPath, setAfterPath] = useState("");
  const [label, setLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<AnalysisResult | null>(null);

  const pickJson = async (setter: (path: string) => void): Promise<void> => {
    const picked = await Dialogs.OpenFile({
      Title: t("selectJsonResultFile"),
      Filters: [
        { DisplayName: "JSON files", Pattern: "*.json" },
        { DisplayName: "All files", Pattern: "*.*" },
      ],
    });
    if (Array.isArray(picked)) setter(picked[0] ?? "");
    else if (picked) setter(picked);
  };

  const runDiff = async (): Promise<void> => {
    if (!beforePath || !afterPath) return;
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      const response = (await engine.diffReports({
        beforePath,
        afterPath,
        label: label.trim() || undefined,
      })) as unknown as AnalysisResult;
      setResult(response);
      addWorkspaceResult({
        result: response,
        title: label.trim() || t("navReportDiff"),
        sourceLabel: "comparison",
      });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setBusy(false);
    }
  };

  const summary = result?.summary as Record<string, unknown> | undefined;
  const rows = ((result?.tables as Record<string, unknown> | undefined)?.summary_metric_deltas ??
    []) as MetricDeltaRow[];

  return (
    <main className="content">
      <section className="card">
        <h2>{t("navReportDiff")}</h2>
        <div className="diff-form">
          <PathPicker label={t("beforeJson")} value={beforePath} onPick={() => void pickJson(setBeforePath)} onChange={setBeforePath} />
          <PathPicker label={t("afterJson")} value={afterPath} onPick={() => void pickJson(setAfterPath)} onChange={setAfterPath} />
          <Input value={label} onChange={(event) => setLabel(event.target.value)} placeholder={t("diffLabelPlaceholder")} />
          <Button type="button" disabled={busy || !beforePath || !afterPath} onClick={() => void runDiff()}>
            {busy ? (
              <>
                <Loader2 className="nav-lucide-sm animate-spin" />
                {t("analyzing")}
              </>
            ) : (
              <>
                <Play className="nav-lucide-sm" />
                {t("diffRun")}
              </>
            )}
          </Button>
        </div>
      </section>

      {error ? (
        <div className="alert error">
          <strong>{t("error")}:</strong> {error}
        </div>
      ) : null}

      {summary ? (
        <section className="card">
          <h2>{t("diffSummary")}</h2>
          <div className="metric-grid">
            <Metric label={t("diffBeforeType")} value={String(summary.before_type ?? "-")} />
            <Metric label={t("diffAfterType")} value={String(summary.after_type ?? "-")} />
            <Metric label={t("diffChangedMetrics")} value={formatNumber(Number(summary.changed_metrics ?? 0))} />
            <Metric label={t("diffFindingDelta")} value={formatNumber(Number(summary.finding_delta ?? 0))} />
          </div>
        </section>
      ) : null}

      {rows.length > 0 ? (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm">
              <GitCompareArrows className="nav-lucide-sm" />
              {t("diffMetricDeltas")}
            </CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t("metric")}</th>
                  <th className="num">{t("before")}</th>
                  <th className="num">{t("after")}</th>
                  <th className="num">Delta</th>
                  <th className="num">%</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row, index) => (
                  <tr key={`${row.metric ?? "metric"}-${index}`}>
                    <td className="frame-cell">{row.metric}</td>
                    <td className="num">{formatNumber(row.before)}</td>
                    <td className="num">{formatNumber(row.after)}</td>
                    <td className="num">{formatNumber(row.delta)}</td>
                    <td className="num">
                      {row.change_percent == null ? "-" : `${row.change_percent.toFixed(2)}%`}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      ) : result ? (
        <p className="muted">{t("diffNoMetricDeltas")}</p>
      ) : (
        <p className="muted">{t("reportDiffEmpty")}</p>
      )}
    </main>
  );
}

function PathPicker({
  label,
  value,
  onPick,
  onChange,
}: {
  label: string;
  value: string;
  onPick: () => void;
  onChange: (value: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="file-row">
      <Input value={value} onChange={(event) => onChange(event.target.value)} placeholder={label} />
      <Button type="button" variant="outline" onClick={onPick}>
        {t("browseFile")}
      </Button>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="metric">
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
    </div>
  );
}
