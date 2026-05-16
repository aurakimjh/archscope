import {
  AlertTriangle,
  ClipboardPlus,
  Download,
  FileJson,
  Filter,
  Network,
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
  buildServiceFlowAnalysis,
  buildServiceFlowExportPayload,
  buildServiceFlowMermaidSequence,
  type ServiceEdge,
  type ServiceFlowFinding,
  type ServiceFlowFindingSeverity,
} from "@/state/serviceFlow";

const ALL = "__all__";

export function ServiceFlowPage(): JSX.Element {
  const { t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const analysis = useMemo(
    () => buildServiceFlowAnalysis(workspace.entries),
    [workspace.entries],
  );
  const [query, setQuery] = useState("");
  const [severity, setSeverity] = useState(ALL);
  const [message, setMessage] = useState<string | null>(null);

  const filteredFindings = useMemo(
    () =>
      analysis.findings.filter((finding) => {
        if (severity !== ALL && finding.severity !== severity) return false;
        return matchesText(
          query,
          finding.code,
          finding.message,
          finding.caller ?? "",
          finding.callee ?? "",
          finding.evidence_refs.join(" "),
        );
      }),
    [analysis.findings, query, severity],
  );

  const filteredEdges = useMemo(
    () =>
      analysis.edge_model.edges.filter((edge) =>
        matchesText(
          query,
          edge.caller,
          edge.callee,
          edge.source_analyzers.join(" "),
          edge.evidence_refs.join(" "),
        ),
      ),
    [analysis.edge_model.edges, query],
  );

  const exportMermaid = (): void => {
    downloadBlob(
      buildServiceFlowMermaidSequence(analysis),
      `archscope-service-flow-${timestampSlug()}.mmd`,
      "text/plain",
    );
  };

  const exportJson = (): void => {
    downloadBlob(
      JSON.stringify(buildServiceFlowExportPayload(analysis), null, 2),
      `archscope-service-flow-${timestampSlug()}.json`,
      "application/json",
    );
  };

  const addFindingEvidence = (finding: ServiceFlowFinding): void => {
    addEvidenceCard({
      analyzer: "service_flow",
      source_kind: "service_flow_finding",
      title: finding.code,
      summary: finding.message,
      severity: finding.severity,
      source_ref: finding.evidence_refs.join(", "),
      payload: finding,
    });
    setMessage(t("evidenceAdded"));
  };

  const addEdgeEvidence = (edge: ServiceEdge): void => {
    addEvidenceCard({
      analyzer: "service_flow",
      source_kind: "service_edge",
      title: `${edge.caller} -> ${edge.callee}`,
      summary: `${edge.call_count.toLocaleString()} calls, ${edge.error_count.toLocaleString()} errors`,
      source_ref: edge.evidence_refs.join(", "),
      payload: edge,
    });
    setMessage(t("evidenceAdded"));
  };

  return (
    <main className="content">
      <section className="card workspace-header">
        <div>
          <h2>{t("navServiceFlow")}</h2>
          <p className="muted">{t("serviceFlowDescription")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" onClick={exportMermaid}>
            <Download className="h-4 w-4" />
            {t("serviceFlowExportMermaid")}
          </Button>
          <Button type="button" variant="outline" onClick={exportJson}>
            <FileJson className="h-4 w-4" />
            {t("serviceFlowExportJson")}
          </Button>
        </div>
      </section>

      <section className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <MetricCard label={t("serviceFlowEdges")} value={analysis.edge_model.edge_count.toLocaleString()} />
        <MetricCard label={t("serviceFlowCalls")} value={analysis.edge_model.total_call_count.toLocaleString()} />
        <MetricCard label={t("serviceFlowErrors")} value={analysis.edge_model.total_error_count.toLocaleString()} />
        <MetricCard
          label={t("serviceFlowUnmatched")}
          value={analysis.edge_model.total_unmatched_call_count.toLocaleString()}
        />
        <MetricCard label={t("serviceFlowFindings")} value={analysis.findings.length.toLocaleString()} />
      </section>

      <Card className="mt-4">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Filter className="h-4 w-4" />
            {t("serviceFlowFilters")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-[1fr_12rem]">
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("serviceFlowSearch")}</span>
            <div className="relative">
              <Search className="pointer-events-none absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                value={query}
                onChange={(event) => setQuery(event.currentTarget.value)}
                placeholder={t("serviceFlowSearchPlaceholder")}
              />
            </div>
          </label>
          <label className="space-y-1 text-xs">
            <span className="text-muted-foreground">{t("serviceFlowSeverity")}</span>
            <select
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={severity}
              onChange={(event) => setSeverity(event.currentTarget.value)}
            >
              <option value={ALL}>{t("serviceFlowAll")}</option>
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
            <AlertTriangle className="h-4 w-4" />
            {t("serviceFlowFindingsTable")}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {filteredFindings.length === 0 ? (
            <EmptyState text={analysis.findings.length === 0 ? t("serviceFlowNoFindings") : t("serviceFlowNoMatches")} />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left">{t("serviceFlowSeverity")}</th>
                    <th className="px-3 py-2 text-left">{t("serviceFlowCode")}</th>
                    <th className="px-3 py-2 text-left">{t("serviceFlowEdge")}</th>
                    <th className="px-3 py-2 text-left">{t("serviceFlowMessage")}</th>
                    <th className="px-3 py-2 text-right">{t("evidence")}</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredFindings.map((finding) => (
                    <FindingRow
                      key={finding.id}
                      finding={finding}
                      onAddEvidence={() => addFindingEvidence(finding)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="mt-4">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Network className="h-4 w-4" />
            {t("serviceFlowEdgesTable")}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {filteredEdges.length === 0 ? (
            <EmptyState text={analysis.edge_model.edge_count === 0 ? t("serviceFlowNoEdges") : t("serviceFlowNoMatches")} />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left">{t("serviceFlowCaller")}</th>
                    <th className="px-3 py-2 text-left">{t("serviceFlowCallee")}</th>
                    <th className="px-3 py-2 text-right">{t("serviceFlowCalls")}</th>
                    <th className="px-3 py-2 text-right">{t("serviceFlowAvgLatency")}</th>
                    <th className="px-3 py-2 text-right">{t("serviceFlowMaxLatency")}</th>
                    <th className="px-3 py-2 text-right">{t("serviceFlowErrors")}</th>
                    <th className="px-3 py-2 text-right">{t("serviceFlowNetworkGap")}</th>
                    <th className="px-3 py-2 text-left">{t("serviceFlowSources")}</th>
                    <th className="px-3 py-2 text-right">{t("evidence")}</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredEdges.map((edge) => (
                    <EdgeRow key={edge.id} edge={edge} onAddEvidence={() => addEdgeEvidence(edge)} />
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

function FindingRow({
  finding,
  onAddEvidence,
}: {
  finding: ServiceFlowFinding;
  onAddEvidence: () => void;
}): JSX.Element {
  return (
    <tr className="border-b border-border align-top last:border-0">
      <td className="px-3 py-2">
        <SeverityBadge severity={finding.severity} />
      </td>
      <td className="px-3 py-2 font-mono text-[11px]">{finding.code}</td>
      <td className="px-3 py-2">
        <div>{finding.caller ?? "-"}</div>
        <div className="text-muted-foreground">-&gt; {finding.callee ?? "-"}</div>
      </td>
      <td className="px-3 py-2">
        <div className="max-w-[36rem]">{finding.message}</div>
        <div className="mt-1 font-mono text-[10px] text-muted-foreground">
          {finding.evidence_refs.join(", ")}
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

function EdgeRow({ edge, onAddEvidence }: { edge: ServiceEdge; onAddEvidence: () => void }): JSX.Element {
  return (
    <tr className="border-b border-border align-top last:border-0">
      <td className="px-3 py-2 font-medium">{edge.caller}</td>
      <td className="px-3 py-2 font-medium">{edge.callee}</td>
      <td className="px-3 py-2 text-right font-mono">{edge.call_count.toLocaleString()}</td>
      <td className="px-3 py-2 text-right font-mono">{formatMs(edge.avg_latency_ms)}</td>
      <td className="px-3 py-2 text-right font-mono">{formatMs(edge.max_latency_ms)}</td>
      <td className="px-3 py-2 text-right font-mono">
        {edge.error_count.toLocaleString()}
        <div className="text-[10px] text-muted-foreground">{formatPercent(edge.error_rate)}</div>
      </td>
      <td className="px-3 py-2 text-right font-mono">{formatMs(edge.avg_network_gap_ms)}</td>
      <td className="px-3 py-2">
        <div className="font-mono text-[11px]">{edge.source_analyzers.join(", ")}</div>
        <div className="text-[10px] text-muted-foreground">{edge.evidence_refs.join(", ")}</div>
      </td>
      <td className="px-3 py-2 text-right">
        <Button type="button" size="sm" variant="outline" onClick={onAddEvidence}>
          <ClipboardPlus className="h-3.5 w-3.5" />
        </Button>
      </td>
    </tr>
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

function SeverityBadge({ severity }: { severity: ServiceFlowFindingSeverity }): JSX.Element {
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

function severityClass(severity: ServiceFlowFindingSeverity): string {
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

function formatMs(value: number | undefined): string {
  if (value === undefined) return "-";
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 2 })} ms`;
}

function formatPercent(value: number): string {
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 2 })}%`;
}

function downloadBlob(content: string, filename: string, type: string): void {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function timestampSlug(): string {
  return new Date().toISOString().replace(/[:.]/g, "-");
}
