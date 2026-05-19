import {
  AlertTriangle,
  ClipboardPlus,
  Filter,
  Gauge,
  ListChecks,
  Search,
  ShieldAlert,
} from "lucide-react";
import { useMemo, useState } from "react";

import { HelpTip, HelpedLabel, HelpedTitle } from "@/components/HelpTip";
import { MetricCard } from "@/components/MetricCard";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { useAnalysisWorkspace } from "@/state/analysisWorkspace";
import { addEvidenceCard } from "@/state/evidenceBoard";
import {
  analyzeSloViolations,
  buildGoldenSignalInventory,
  buildSliMetricModel,
  DEFAULT_SLO_TARGETS,
  type GoldenSignal,
  type GoldenSignalKind,
  type SliMetric,
  type SloSeverity,
  type SloViolation,
} from "@/state/sloGoldenSignals";

const ALL = "__all__";

export function SloGoldenSignalsPage(): JSX.Element {
  const { locale, t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const inventory = useMemo(
    () => buildGoldenSignalInventory(workspace.entries),
    [workspace.entries],
  );
  const sliModel = useMemo(() => buildSliMetricModel(inventory), [inventory]);
  const sloAnalysis = useMemo(() => analyzeSloViolations(sliModel), [sliModel]);
  const [kind, setKind] = useState<string>(ALL);
  const [severity, setSeverity] = useState<string>(ALL);
  const [query, setQuery] = useState("");
  const [message, setMessage] = useState<string | null>(null);

  const filteredViolations = useMemo(
    () =>
      sloAnalysis.violations.filter((violation) => {
        if (kind !== ALL && violation.kind !== kind) return false;
        if (severity !== ALL && violation.severity !== severity) return false;
        return matchesText(
          query,
          violation.target_name,
          violation.metric_name,
          violation.affected_scope,
          violation.source_analyzers.join(" "),
          violation.evidence_refs.join(" "),
        );
      }),
    [kind, query, severity, sloAnalysis.violations],
  );

  const filteredMetrics = useMemo(
    () =>
      sliModel.metrics.filter((metric) => {
        if (kind !== ALL && metric.kind !== kind) return false;
        return matchesText(
          query,
          metric.name,
          metric.scope,
          metric.source_analyzers.join(" "),
          metric.evidence_refs.join(" "),
        );
      }),
    [kind, query, sliModel.metrics],
  );

  const filteredSignals = useMemo(
    () =>
      inventory.signals.filter((signal) => {
        if (kind !== ALL && signal.kind !== kind) return false;
        return matchesText(
          query,
          signal.name,
          signal.scope,
          signal.source_analyzer,
          signal.evidence_ref,
          signal.source_title,
        );
      }),
    [inventory.signals, kind, query],
  );

  const addViolationEvidence = (violation: SloViolation): void => {
    addEvidenceCard({
      analyzer: "slo_golden_signals",
      source_kind: "slo_violation",
      title: violation.target_name,
      summary: `${violation.metric_name}: ${formatNumber(violation.actual)} ${violation.unit} ${formatComparator(
        violation.comparator,
      )} ${formatNumber(violation.threshold)} ${violation.unit}`,
      severity: violation.severity,
      source_ref: violation.evidence_refs.join(", "),
      payload: {
        ...violation,
        target: DEFAULT_SLO_TARGETS.find((target) => target.id === violation.target_id),
      },
    });
    setMessage(t("evidenceAdded"));
  };

  const criticalCount = sloAnalysis.violations_by_severity.critical;
  const warningCount = sloAnalysis.violations_by_severity.warning;

  return (
    <main className="content">
      <section className="card workspace-header">
        <div>
          <h2>
            <HelpedTitle help={getHelpText(locale, "pageSloGoldenSignals")}>
              {t("navSloGoldenSignals")}
            </HelpedTitle>
          </h2>
          <p className="muted">{t("sloGoldenSignalsDescription")}</p>
        </div>
      </section>

      <section className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <MetricCard label={t("sloSignals")} value={inventory.signal_count.toLocaleString()} />
        <MetricCard label={t("sloMetrics")} value={sliModel.metric_count.toLocaleString()} />
        <MetricCard label={t("sloViolations")} value={sloAnalysis.violation_count.toLocaleString()} />
        <MetricCard label={t("sloCritical")} value={criticalCount.toLocaleString()} />
        <MetricCard label={t("sloWarning")} value={warningCount.toLocaleString()} />
      </section>

      <Card className="mt-4">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Filter className="h-4 w-4" />
            {t("sloFilters")}
            <HelpTip text={getHelpText(locale, "sectionFilters")} />
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-[1fr_12rem_12rem]">
          <label className="space-y-1 text-xs">
            <HelpedLabel help={getHelpText(locale, "optionSearch")} className="text-muted-foreground">
              {t("sloSearch")}
            </HelpedLabel>
            <div className="relative">
              <Search className="pointer-events-none absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                value={query}
                onChange={(event) => setQuery(event.currentTarget.value)}
                placeholder={t("sloSearchPlaceholder")}
              />
            </div>
          </label>
          <label className="space-y-1 text-xs">
            <HelpedLabel help={getHelpText(locale, "optionSignalKind")} className="text-muted-foreground">
              {t("sloKind")}
            </HelpedLabel>
            <select
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={kind}
              onChange={(event) => setKind(event.currentTarget.value)}
            >
              <option value={ALL}>{t("sloAll")}</option>
              {(["latency", "traffic", "errors", "saturation"] satisfies GoldenSignalKind[]).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            <HelpedLabel help={getHelpText(locale, "optionSeverity")} className="text-muted-foreground">
              {t("sloSeverity")}
            </HelpedLabel>
            <select
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={severity}
              onChange={(event) => setSeverity(event.currentTarget.value)}
            >
              <option value={ALL}>{t("sloAll")}</option>
              <option value="critical">critical</option>
              <option value="warning">warning</option>
              <option value="info">info</option>
            </select>
          </label>
        </CardContent>
      </Card>

      {message && (
        <p className="mt-3 rounded-md border border-border bg-muted/50 px-3 py-2 text-sm">
          {message}
        </p>
      )}

      <Card className="mt-4">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <ShieldAlert className="h-4 w-4" />
            {t("sloViolationTable")}
            <HelpTip text={getHelpText(locale, "sectionSloViolations")} />
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {filteredViolations.length === 0 ? (
            <EmptyState text={sloAnalysis.violation_count === 0 ? t("sloNoViolations") : t("sloNoMatches")} />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left">{t("sloSeverity")}</th>
                    <th className="px-3 py-2 text-left">{t("sloTarget")}</th>
                    <th className="px-3 py-2 text-left">{t("sloMetric")}</th>
                    <th className="px-3 py-2 text-right">{t("sloActual")}</th>
                    <th className="px-3 py-2 text-right">{t("sloThreshold")}</th>
                    <th className="px-3 py-2 text-left">{t("sloScope")}</th>
                    <th className="px-3 py-2 text-right">{t("sloBudget")}</th>
                    <th className="px-3 py-2 text-right">{t("evidence")}</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredViolations.map((violation) => (
                    <ViolationRow
                      key={violation.id}
                      violation={violation}
                      onAddEvidence={() => addViolationEvidence(violation)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <section className="mt-4 grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-sm">
              <Gauge className="h-4 w-4" />
              {t("sloSliMetrics")}
              <HelpTip text={getHelpText(locale, "sectionSliMetrics")} />
            </CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {filteredMetrics.length === 0 ? (
              <EmptyState text={sliModel.metric_count === 0 ? t("sloEmpty") : t("sloNoMatches")} />
            ) : (
              <CompactMetricTable metrics={filteredMetrics.slice(0, 80)} />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-sm">
              <ListChecks className="h-4 w-4" />
              {t("sloAffectedBreakdown")}
              <HelpTip text={getHelpText(locale, "sectionSloAffected")} />
            </CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {sloAnalysis.affected_breakdown.length === 0 ? (
              <EmptyState text={t("sloNoViolations")} />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                      <th className="px-3 py-2 text-left">{t("sloScope")}</th>
                      <th className="px-3 py-2 text-right">{t("sloViolationCount")}</th>
                      <th className="px-3 py-2 text-left">{t("sloSeverity")}</th>
                      <th className="px-3 py-2 text-right">{t("sloBurnRate")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sloAnalysis.affected_breakdown.slice(0, 80).map((row) => (
                      <tr
                        key={`${row.affected_scope_type}:${row.affected_scope}`}
                        className="border-b border-border align-top last:border-0"
                      >
                        <td className="px-3 py-2">
                          <div className="font-medium">{row.affected_scope}</div>
                          <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
                            {row.affected_scope_type}
                          </div>
                        </td>
                        <td className="px-3 py-2 text-right font-mono">{row.violation_count}</td>
                        <td className="px-3 py-2">
                          <SeverityBadge severity={row.max_severity} />
                        </td>
                        <td className="px-3 py-2 text-right font-mono">{formatNumber(row.max_burn_rate)}x</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      <Card className="mt-4">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Gauge className="h-4 w-4" />
            {t("sloSignalInventory")}
            <HelpTip text={getHelpText(locale, "sectionSignalInventory")} />
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {filteredSignals.length === 0 ? (
            <EmptyState text={inventory.signal_count === 0 ? t("sloEmpty") : t("sloNoMatches")} />
          ) : (
            <SignalInventoryTable signals={filteredSignals.slice(0, 120)} />
          )}
        </CardContent>
      </Card>
    </main>
  );
}

function ViolationRow({
  violation,
  onAddEvidence,
}: {
  violation: SloViolation;
  onAddEvidence: () => void;
}): JSX.Element {
  return (
    <tr className="border-b border-border align-top last:border-0">
      <td className="px-3 py-2">
        <SeverityBadge severity={violation.severity} />
      </td>
      <td className="px-3 py-2">
        <div className="font-semibold">{violation.target_name}</div>
        <div className="font-mono text-[10px] text-muted-foreground">{violation.window_label}</div>
      </td>
      <td className="px-3 py-2">
        <div>{violation.metric_name}</div>
        <div className="text-[10px] uppercase tracking-wider text-muted-foreground">{violation.kind}</div>
      </td>
      <td className="px-3 py-2 text-right font-mono">
        {formatNumber(violation.actual)} {violation.unit}
      </td>
      <td className="px-3 py-2 text-right font-mono">
        {formatComparator(violation.comparator)} {formatNumber(violation.threshold)}
      </td>
      <td className="px-3 py-2">
        <div className="max-w-[18rem] truncate" title={violation.affected_scope}>
          {violation.affected_scope}
        </div>
        <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
          {violation.affected_scope_type}
        </div>
      </td>
      <td className="px-3 py-2 text-right font-mono">
        <div>{formatNumber(violation.burn_rate)}x</div>
        <div className="text-[10px] text-muted-foreground">
          {formatNumber(violation.error_budget_remaining_percent)}%
        </div>
      </td>
      <td className="px-3 py-2 text-right">
        <Button type="button" size="sm" variant="outline" onClick={onAddEvidence}>
          <ClipboardPlus className="h-3.5 w-3.5" />
        </Button>
      </td>
    </tr>
  );
}

function CompactMetricTable({ metrics }: { metrics: SliMetric[] }): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
            <th className="px-3 py-2 text-left">{t("sloMetric")}</th>
            <th className="px-3 py-2 text-left">{t("sloScope")}</th>
            <th className="px-3 py-2 text-right">{t("sloValue")}</th>
            <th className="px-3 py-2 text-left">{t("sloSource")}</th>
          </tr>
        </thead>
        <tbody>
          {metrics.map((metric) => (
            <tr key={metric.id} className="border-b border-border align-top last:border-0">
              <td className="px-3 py-2">
                <div className="font-medium">{metric.name}</div>
                <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  {metric.kind} / {metric.aggregation}
                </div>
              </td>
              <td className="px-3 py-2">
                <div className="max-w-[16rem] truncate" title={metric.scope}>
                  {metric.scope}
                </div>
                <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  {metric.scope_type}
                </div>
              </td>
              <td className="px-3 py-2 text-right font-mono">
                {formatNumber(metric.value)} {metric.unit}
              </td>
              <td className="px-3 py-2">
                <div className="font-mono text-[11px]">{metric.source_analyzers.join(", ")}</div>
                <div className="text-[10px] text-muted-foreground">
                  {metric.contributor_count.toLocaleString()} signals
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SignalInventoryTable({ signals }: { signals: GoldenSignal[] }): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
            <th className="px-3 py-2 text-left">{t("sloKindColumn")}</th>
            <th className="px-3 py-2 text-left">{t("sloMetric")}</th>
            <th className="px-3 py-2 text-left">{t("sloScope")}</th>
            <th className="px-3 py-2 text-right">{t("sloValue")}</th>
            <th className="px-3 py-2 text-left">{t("sloEvidenceRef")}</th>
          </tr>
        </thead>
        <tbody>
          {signals.map((signal) => (
            <tr key={signal.id} className="border-b border-border align-top last:border-0">
              <td className="px-3 py-2">
                <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">
                  {signal.kind}
                </span>
              </td>
              <td className="px-3 py-2">
                <div className="font-medium">{signal.name}</div>
                <div className="font-mono text-[10px] text-muted-foreground">{signal.source_analyzer}</div>
              </td>
              <td className="px-3 py-2">
                <div className="max-w-[18rem] truncate" title={signal.scope}>
                  {signal.scope}
                </div>
                <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  {signal.scope_type}
                </div>
              </td>
              <td className="px-3 py-2 text-right font-mono">
                {formatNumber(signal.value)} {signal.unit}
              </td>
              <td className="px-3 py-2 font-mono text-[11px]">{signal.evidence_ref}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EmptyState({ text }: { text: string }): JSX.Element {
  return (
    <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
      <AlertTriangle className="h-4 w-4" />
      {text}
    </div>
  );
}

function SeverityBadge({ severity }: { severity: SloSeverity }): JSX.Element {
  return (
    <span
      className={`rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${severityClass(
        severity,
      )}`}
    >
      {severity}
    </span>
  );
}

function severityClass(severity: SloSeverity): string {
  switch (severity) {
    case "critical":
      return "bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-200";
    case "warning":
      return "bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-200";
    default:
      return "bg-slate-100 text-slate-700 dark:bg-slate-900 dark:text-slate-200";
  }
}

function matchesText(query: string, ...values: string[]): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return values.join(" ").toLowerCase().includes(needle);
}

function formatComparator(comparator: "lte" | "gte"): string {
  return comparator === "lte" ? "<=" : ">=";
}

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) return String(value);
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
}
