import {
  AlertTriangle,
  ClipboardPlus,
  Clock3,
  Filter,
  Search,
} from "lucide-react";
import { useMemo, useState } from "react";

import { MetricCard } from "@/components/MetricCard";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useI18n } from "@/i18n/I18nProvider";
import { useAnalysisWorkspace } from "@/state/analysisWorkspace";
import { addEvidenceCard } from "@/state/evidenceBoard";
import {
  buildIncidentTimelineEvents,
  type IncidentTimelineEvent,
} from "@/state/incidentTimeline";

const ALL = "__all__";

export function IncidentTimelinePage(): JSX.Element {
  const { t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const events = useMemo(
    () => buildIncidentTimelineEvents(workspace.entries),
    [workspace.entries],
  );
  const [analyzer, setAnalyzer] = useState(ALL);
  const [severity, setSeverity] = useState(ALL);
  const [query, setQuery] = useState("");
  const [message, setMessage] = useState<string | null>(null);

  const analyzers = useMemo(
    () => Array.from(new Set(events.map((event) => event.source_analyzer))).sort(),
    [events],
  );
  const filtered = useMemo(
    () =>
      events.filter((event) => {
        if (analyzer !== ALL && event.source_analyzer !== analyzer) return false;
        if (severity !== ALL && event.severity !== severity) return false;
        const needle = query.trim().toLowerCase();
        if (!needle) return true;
        return [
          event.label,
          event.description,
          event.category,
          event.source_analyzer,
          event.source_title,
          event.source_file ?? "",
        ]
          .join(" ")
          .toLowerCase()
          .includes(needle);
      }),
    [analyzer, events, query, severity],
  );

  const criticalCount = events.filter((event) => event.severity === "critical").length;
  const warningCount = events.filter((event) => event.severity === "warning").length;
  const sourceCount = new Set(events.map((event) => event.source_result_id)).size;
  const timeWindow = eventWindow(events);

  const addEventEvidence = (event: IncidentTimelineEvent): void => {
    addEvidenceCard({
      analyzer: "incident_timeline",
      source_kind: "timeline_event",
      title: event.label,
      summary: event.description,
      severity: event.severity,
      source_file: event.source_file,
      source_ref: event.evidence_ref,
      payload: {
        timestamp: event.timestamp,
        time_label: event.time_label,
        source_analyzer: event.source_analyzer,
        source_result_id: event.source_result_id,
        source_title: event.source_title,
        category: event.category,
        evidence_ref: event.evidence_ref,
        payload: event.payload,
      },
    });
    setMessage(t("evidenceAdded"));
  };

  return (
    <main className="content">
      <section className="card workspace-header">
        <div>
          <h2>{t("navIncidentTimeline")}</h2>
          <p className="muted">{t("incidentTimelineDescription")}</p>
        </div>
      </section>

      <section className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricCard label={t("incidentTimelineEvents")} value={events.length.toLocaleString()} />
        <MetricCard
          label={t("incidentTimelineCritical")}
          value={criticalCount.toLocaleString()}
        />
        <MetricCard
          label={t("incidentTimelineWarnings")}
          value={warningCount.toLocaleString()}
        />
        <MetricCard label={t("incidentTimelineSources")} value={sourceCount.toLocaleString()} />
      </section>

      <Card className="mt-4">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Filter className="h-4 w-4" />
            {t("incidentTimelineFilters")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-[1fr_12rem_12rem]">
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("incidentTimelineSearch")}</span>
            <div className="relative">
              <Search className="pointer-events-none absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                value={query}
                onChange={(event) => setQuery(event.currentTarget.value)}
                placeholder={t("incidentTimelineSearchPlaceholder")}
              />
            </div>
          </label>
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("incidentTimelineAnalyzer")}</span>
            <select
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={analyzer}
              onChange={(event) => setAnalyzer(event.currentTarget.value)}
            >
              <option value={ALL}>{t("incidentTimelineAll")}</option>
              {analyzers.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("incidentTimelineSeverity")}</span>
            <select
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={severity}
              onChange={(event) => setSeverity(event.currentTarget.value)}
            >
              <option value={ALL}>{t("incidentTimelineAll")}</option>
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
          <CardTitle className="flex items-center justify-between gap-3 text-sm">
            <span className="flex items-center gap-2">
              <Clock3 className="h-4 w-4" />
              {t("incidentTimeline")}
            </span>
            <span className="text-xs font-normal text-muted-foreground">{timeWindow}</span>
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {filtered.length === 0 ? (
            <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
              <AlertTriangle className="h-4 w-4" />
              {events.length === 0
                ? t("incidentTimelineEmpty")
                : t("incidentTimelineNoMatches")}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left">{t("incidentTimelineTime")}</th>
                    <th className="px-3 py-2 text-left">{t("incidentTimelineSeverity")}</th>
                    <th className="px-3 py-2 text-left">{t("incidentTimelineSource")}</th>
                    <th className="px-3 py-2 text-left">{t("incidentTimelineEvent")}</th>
                    <th className="px-3 py-2 text-left">{t("incidentTimelineEvidenceRef")}</th>
                    <th className="px-3 py-2 text-right">{t("evidence")}</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((event) => (
                    <TimelineRow
                      key={event.id}
                      event={event}
                      onAddEvidence={() => addEventEvidence(event)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </main>
  );
}

function TimelineRow({
  event,
  onAddEvidence,
}: {
  event: IncidentTimelineEvent;
  onAddEvidence: () => void;
}): JSX.Element {
  return (
    <tr className="border-b border-border align-top last:border-0">
      <td className="whitespace-nowrap px-3 py-2 font-mono text-[11px]">
        {event.time_label}
      </td>
      <td className="px-3 py-2">
        <span
          className={`rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${severityClass(
            event.severity,
          )}`}
        >
          {event.severity}
        </span>
      </td>
      <td className="px-3 py-2">
        <div className="font-mono text-[11px]">{event.source_analyzer}</div>
        <div className="max-w-[16rem] truncate text-muted-foreground" title={event.source_title}>
          {event.source_title}
        </div>
      </td>
      <td className="px-3 py-2">
        <div className="font-semibold">{event.label}</div>
        <div className="max-w-[32rem] text-muted-foreground">{event.description}</div>
        <div className="mt-1 text-[10px] uppercase tracking-wider text-muted-foreground">
          {event.category}
        </div>
      </td>
      <td className="px-3 py-2 font-mono text-[11px]">{event.evidence_ref}</td>
      <td className="px-3 py-2 text-right">
        <Button type="button" size="sm" variant="outline" onClick={onAddEvidence}>
          <ClipboardPlus className="h-3.5 w-3.5" />
        </Button>
      </td>
    </tr>
  );
}

function severityClass(severity: IncidentTimelineEvent["severity"]): string {
  switch (severity) {
    case "critical":
      return "bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-200";
    case "warning":
      return "bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-200";
    default:
      return "bg-slate-100 text-slate-700 dark:bg-slate-900 dark:text-slate-200";
  }
}

function eventWindow(events: IncidentTimelineEvent[]): string {
  if (events.length === 0) return "-";
  const ordered = [...events].sort((left, right) => left.sort_time - right.sort_time);
  const first = ordered[0];
  const last = ordered[ordered.length - 1];
  if (first.timestamp === last.timestamp) return first.time_label;
  return `${first.time_label} - ${last.time_label}`;
}
