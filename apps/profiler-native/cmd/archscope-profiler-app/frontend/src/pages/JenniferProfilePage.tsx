// MVP1 Jennifer Profile Export analyzer page.
//
// Surface: file pick (multiple) → Analyze → render Summary metric
// cards + per-profile table + file errors. MSA call-graph / network
// gap / signature stats land in MVP2-MVP3.

import { Loader2, Play, X } from "lucide-react";
import { useCallback, useState } from "react";
import { Dialogs } from "@wailsio/runtime";

import { engine } from "../bridge/engine";

import {
  CustomCategoriesEditor,
  type CategoryRules,
  type Preset,
  type SegmentSpec,
} from "../components/CustomCategoriesEditor";
import { MsaStackedBar } from "../components/MsaStackedBar";
import { MetricCard } from "../components/MetricCard";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "../components/ui/tabs";
import { useI18n } from "../i18n/I18nProvider";

// MSA event-category segments — these match JenniferEventType values
// that the backend classifier accepts via EventCategoryPatterns.
const MSA_EVENT_SEGMENTS: SegmentSpec[] = [
  { id: "EXTERNAL_CALL", label: "EXTERNAL_CALL" },
  { id: "NETWORK_PREP_METHOD", label: "Network prep (sendToService 등)" },
  { id: "TWO_PC_UNKNOWN", label: "2PC / XA" },
  { id: "CHECK_QUERY", label: "CHECK_QUERY" },
  { id: "CONNECTION_ACQUIRE", label: "Connection acquire" },
  { id: "FETCH", label: "FETCH" },
];

const MSA_EVENT_PRESETS: Preset[] = [
  {
    id: "msa-network-prep",
    label: "MSA — Network prep (sendToService)",
    rules: {
      NETWORK_PREP_METHOD: [
        "IntegrationUtil.sendToService",
        "sendToService(",
      ],
    },
  },
  {
    id: "msa-2pc-extra",
    label: "MSA — 2PC / XA extras",
    rules: {
      TWO_PC_UNKNOWN: [
        "javax.transaction.xa",
        "atomikos",
        "JTATransaction.commit",
      ],
    },
  },
  {
    id: "msa-check-query-extra",
    label: "MSA — Check query (드라이버별)",
    rules: {
      CHECK_QUERY: [
        "select 1 from dual",
        "select getdate()",
        "select current_timestamp",
      ],
    },
  },
];

const FILE_FILTERS = [
  {
    DisplayName: "Jennifer profile export",
    Pattern: "*.txt;*.log;*.profile",
  },
  { DisplayName: "All files", Pattern: "*.*" },
];

type Selection = {
  filePath: string;
  originalName: string;
};

function basenameOf(p: string): string {
  const segments = p.split(/[\\/]/);
  return segments[segments.length - 1] ?? p;
}

export function JenniferProfilePage(): JSX.Element {
  const { t } = useI18n();
  const [selected, setSelected] = useState<Selection[]>([]);
  const [analyzing, setAnalyzing] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string>("");
  const [fallbackToTxid, setFallbackToTxid] = useState(false);
  // networkPrepPatterns is reserved for a future "stand-alone wrapper
  // patterns" UI; for now the editor's NETWORK_PREP_METHOD bucket is
  // the single source of truth and feeds both the parser hook and the
  // network_prep_cum_ms metric. We keep the state slot so callers can
  // pre-seed it programmatically without prop drilling.
  const [networkPrepPatterns] = useState<string[]>([]);
  const [eventCategoryPatterns, setEventCategoryPatterns] =
    useState<CategoryRules>({});
  const [activeTab, setActiveTab] = useState<string>("summary");

  const handlePick = useCallback(async () => {
    setError("");
    try {
      const picked = await Dialogs.OpenFile({
        Title: "Select Jennifer profile export",
        Filters: FILE_FILTERS,
        AllowsMultipleSelection: true,
      });
      const list: string[] = Array.isArray(picked)
        ? (picked as string[])
        : typeof picked === "string" && picked
        ? [picked]
        : [];
      if (list.length === 0) return;
      const additions: Selection[] = list.map((p) => ({
        filePath: p,
        originalName: basenameOf(p),
      }));
      setSelected((prev) => {
        const seen = new Set(prev.map((s) => s.filePath));
        return [...prev, ...additions.filter((a) => !seen.has(a.filePath))];
      });
    } catch (err: any) {
      setError(String(err?.message ?? err));
    }
  }, []);

  const handleClear = (path: string) => {
    setSelected((prev) => prev.filter((s) => s.filePath !== path));
  };

  const handleClearAll = () => {
    setSelected([]);
    setResult(null);
    setError("");
  };

  const handleAnalyze = async () => {
    if (selected.length === 0) return;
    setError("");
    setAnalyzing(true);
    try {
      // Patterns flow into the parser's EventCategoryPatterns and
      // NetworkPrepPatterns hooks. Built-in matches still bracket
      // EXTERNAL_CALL/FETCH so user typos can't corrupt edge counts.
      // The eventCategoryPatterns map already keys off
      // JenniferEventType values, so we forward it as-is. NETWORK_PREP_METHOD
      // patterns are split out into the dedicated NetworkPrepPatterns
      // bucket so the analyzer can also fold them into the
      // network_prep_cum_ms metric (subtract from external_call elapsed).
      const prepFromEditor =
        eventCategoryPatterns["NETWORK_PREP_METHOD"] ?? [];
      const prepCombined = Array.from(
        new Set([...networkPrepPatterns, ...prepFromEditor]),
      );
      const otherCategories: CategoryRules = { ...eventCategoryPatterns };
      delete otherCategories["NETWORK_PREP_METHOD"];

      const res = await engine.analyzeJenniferProfile({
        path: selected.length === 1 ? selected[0].filePath : undefined,
        paths: selected.length > 1 ? selected.map((s) => s.filePath) : undefined,
        fallbackCorrelationToTxid: fallbackToTxid,
        headerBodyToleranceMs: 0,
        networkPrepPatterns: prepCombined.length > 0 ? prepCombined : undefined,
        eventCategoryPatterns:
          Object.keys(otherCategories).length > 0 ? otherCategories : undefined,
      } as any);
      setResult(res);
      setActiveTab("summary");
    } catch (err: any) {
      setError(String(err?.message ?? err));
      setResult(null);
    } finally {
      setAnalyzing(false);
    }
  };

  const summary = result?.summary ?? {};
  const profileRows: any[] = result?.tables?.profiles ?? [];
  const fileErrors: any[] = result?.tables?.file_errors ?? [];
  const fileSummary: any[] = result?.series?.file_summary ?? [];
  const guidGroups: any[] = result?.series?.guid_groups ?? [];
  const msaEdges: any[] = result?.tables?.msa_edges ?? [];
  const signatureStats: any[] = result?.series?.signature_statistics ?? [];

  return (
    <main className="flex flex-col gap-5 p-5 overflow-y-auto">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">
            {t("dropOrBrowseJennifer")}
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" size="sm" onClick={handlePick} disabled={analyzing}>
              {t("browseFile")}
            </Button>
            {selected.length > 0 && (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={handleClearAll}
                disabled={analyzing}
              >
                <X className="h-3.5 w-3.5" />
                Clear all
              </Button>
            )}
            <Button
              type="button"
              size="sm"
              variant="default"
              onClick={() => void handleAnalyze()}
              disabled={analyzing || selected.length === 0}
            >
              {analyzing ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  {t("analyzing")}
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5" />
                  {t("analyze")} ({selected.length})
                </>
              )}
            </Button>
            <label className="ml-3 flex items-center gap-1.5 text-xs">
              <input
                type="checkbox"
                checked={fallbackToTxid}
                onChange={(e) => setFallbackToTxid(e.target.checked)}
              />
              fallback correlation = TXID
            </label>
          </div>
          <div>
            <p className="mb-2 text-xs font-semibold text-foreground/80">
              MSA 이벤트 분류 패턴 (METHOD/UNKNOWN 라인 추가 분류)
            </p>
            <CustomCategoriesEditor
              segments={MSA_EVENT_SEGMENTS}
              presets={MSA_EVENT_PRESETS}
              value={eventCategoryPatterns}
              onChange={setEventCategoryPatterns}
              disabled={analyzing}
            />
          </div>
          {selected.length > 0 && (
            <ul className="flex flex-col gap-1 rounded-md border border-border bg-muted/30 p-2">
              {selected.map((s) => (
                <li
                  key={s.filePath}
                  className="flex items-center justify-between gap-2 rounded px-2 py-1 text-xs"
                >
                  <span className="truncate font-mono" title={s.filePath}>
                    {s.originalName}
                  </span>
                  <button
                    type="button"
                    className="text-muted-foreground hover:text-destructive"
                    onClick={() => handleClear(s.filePath)}
                    aria-label="Remove"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {error && (
        <div
          role="alert"
          className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
        >
          <strong className="block">{t("error") || "Error"}</strong>
          <p className="mt-1 text-foreground">{error}</p>
        </div>
      )}

      {result && (
        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <TabsList>
            <TabsTrigger value="summary">요약</TabsTrigger>
            <TabsTrigger value="msa">MSA</TabsTrigger>
            <TabsTrigger value="profiles">프로파일</TabsTrigger>
            <TabsTrigger value="parser">파서 보고서</TabsTrigger>
          </TabsList>

          <TabsContent value="summary" className="mt-4 flex flex-col gap-4">
          <section className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-8">
            <MetricCard
              label="Total files"
              value={(summary.total_files ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Total profiles"
              value={(summary.total_profiles ?? 0).toLocaleString()}
            />
            <MetricCard
              label="GUID groups"
              value={(summary.guid_group_count ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Signatures"
              value={(summary.unique_signature_count ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Matched edges"
              value={(summary.matched_external_call_count ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Unmatched edges"
              value={(summary.unmatched_external_call_count ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Net Gap (cum ms)"
              value={(summary.total_network_gap_cum_ms ?? 0).toLocaleString()}
            />
            <MetricCard
              label="SQL Execute (cum ms)"
              value={(summary.sql_execute_cum_ms ?? 0).toLocaleString()}
            />
            <MetricCard
              label="네트워크 준비시간 (cum ms)"
              value={(summary.network_prep_cum_ms ?? 0).toLocaleString()}
            />
            <MetricCard
              label="네트워크 wrapper 합계 (cum ms)"
              value={(summary.network_prep_method_cum_ms ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Fetch rows"
              value={(summary.fetch_total_rows ?? 0).toLocaleString()}
            />
          </section>

          {fileErrors.length > 0 && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">File errors</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-4 py-2 text-left font-medium">Source file</th>
                      <th className="px-4 py-2 text-left font-medium">Code</th>
                      <th className="px-4 py-2 text-left font-medium">Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    {fileErrors.map((row, idx) => (
                      <tr key={idx} className="border-b border-border last:border-0">
                        <td className="px-4 py-2 font-mono text-xs" title={row.source_file}>
                          {basenameOf(row.source_file ?? "")}
                        </td>
                        <td className="px-4 py-2 font-mono text-xs">{row.code}</td>
                        <td className="px-4 py-2 text-xs">{row.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {fileSummary.length > 0 && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">File summary</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-4 py-2 text-left font-medium">Source file</th>
                      <th className="px-4 py-2 text-right font-medium">Declared</th>
                      <th className="px-4 py-2 text-right font-medium">Detected</th>
                      <th className="px-4 py-2 text-right font-medium">Profiles</th>
                    </tr>
                  </thead>
                  <tbody>
                    {fileSummary.map((row, idx) => (
                      <tr key={idx} className="border-b border-border last:border-0">
                        <td className="px-4 py-2 font-mono text-xs" title={row.source_file}>
                          {basenameOf(row.source_file ?? "")}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {(row.declared_transaction_count ?? 0).toLocaleString()}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {(row.detected_transaction_count ?? 0).toLocaleString()}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {(row.profile_count ?? 0).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {signatureStats.length > 0 && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  MSA timeline signatures ({signatureStats.length})
                </CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">Hash</th>
                      <th className="px-3 py-2 text-left font-medium">Root</th>
                      <th className="px-3 py-2 text-right font-medium">Edges</th>
                      <th className="px-3 py-2 text-right font-medium">Samples</th>
                      <th className="px-3 py-2 text-right font-medium">Resp avg</th>
                      <th className="px-3 py-2 text-right font-medium">Resp p95</th>
                      <th className="px-3 py-2 text-right font-medium">Ext avg</th>
                      <th className="px-3 py-2 text-right font-medium">Ext p95</th>
                      <th className="px-3 py-2 text-right font-medium">Gap avg</th>
                      <th className="px-3 py-2 text-right font-medium">Gap p95</th>
                      <th className="px-3 py-2 text-right font-medium">SQL avg</th>
                    </tr>
                  </thead>
                  <tbody>
                    {signatureStats.map((s, idx) => {
                      const m = s.metrics ?? {};
                      const fmtStat = (st: any, key: string) =>
                        st && st[key] != null
                          ? Math.round(Number(st[key])).toLocaleString()
                          : "—";
                      return (
                        <tr key={idx} className="border-b border-border last:border-0">
                          <td className="px-3 py-2 font-mono text-xs" title={s.signature_hash}>
                            …{String(s.signature_hash ?? "").slice(-8)}
                          </td>
                          <td className="px-3 py-2 text-xs" title={s.root_application}>
                            {s.root_application || "—"}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(s.edge_count ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(s.sample_count ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.root_response_time_ms, "avg")}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.root_response_time_ms, "p95")}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.total_external_call_cumulative_ms, "avg")}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.total_external_call_cumulative_ms, "p95")}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.total_network_gap_cumulative_ms, "avg")}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.total_network_gap_cumulative_ms, "p95")}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {fmtStat(m.total_sql_execute_ms, "avg")}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {signatureStats.some((s) => (s.edges ?? []).length > 0) && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">Edge statistics</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">Caller → Callee</th>
                      <th className="px-3 py-2 text-right font-medium">#</th>
                      <th className="px-3 py-2 text-right font-medium">Count</th>
                      <th className="px-3 py-2 text-right font-medium">Ext avg</th>
                      <th className="px-3 py-2 text-right font-medium">Ext p95</th>
                      <th className="px-3 py-2 text-right font-medium">Ext max</th>
                      <th className="px-3 py-2 text-right font-medium">Callee RT avg</th>
                      <th className="px-3 py-2 text-right font-medium">Adj Gap avg</th>
                      <th className="px-3 py-2 text-right font-medium">Adj Gap p95</th>
                      <th className="px-3 py-2 text-right font-medium">Adj Gap max</th>
                    </tr>
                  </thead>
                  <tbody>
                    {signatureStats.flatMap((s) =>
                      (s.edges ?? []).map((e: any, idx: number) => {
                        const fmt = (st: any, key: string) =>
                          st && st[key] != null
                            ? Math.round(Number(st[key])).toLocaleString()
                            : "—";
                        return (
                          <tr
                            key={`${s.signature_hash}-${idx}`}
                            className="border-b border-border last:border-0"
                          >
                            <td
                              className="px-3 py-2 text-xs"
                              title={`${e.caller_application} → ${e.callee_application}`}
                            >
                              {e.caller_application} → {e.callee_application}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              #{e.occurrence_index}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.external_call_elapsed_ms, "count")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.external_call_elapsed_ms, "avg")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.external_call_elapsed_ms, "p95")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.external_call_elapsed_ms, "max")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.callee_response_time_ms, "avg")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.adjusted_network_gap_ms, "avg")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.adjusted_network_gap_ms, "p95")}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {fmt(e.adjusted_network_gap_ms, "max")}
                            </td>
                          </tr>
                        );
                      }),
                    )}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {guidGroups.length > 0 && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  MSA GUID groups ({guidGroups.length})
                </CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">GUID</th>
                      <th className="px-3 py-2 text-left font-medium">Root</th>
                      <th className="px-3 py-2 text-right font-medium">Profiles</th>
                      <th className="px-3 py-2 text-right font-medium">Matched</th>
                      <th className="px-3 py-2 text-right font-medium">Cum ms</th>
                      <th className="px-3 py-2 text-right font-medium">Wall ms</th>
                      <th className="px-3 py-2 text-right font-medium">Ratio</th>
                      <th className="px-3 py-2 text-right font-medium">Conc</th>
                      <th className="px-3 py-2 text-right font-medium">Net Gap</th>
                      <th className="px-3 py-2 text-right font-medium">Mode</th>
                      <th className="px-3 py-2 text-right font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {guidGroups.map((g, idx) => {
                      const m = g.metrics ?? {};
                      const mode = m.group_execution_mode ?? "—";
                      const ratio = m.external_call_parallelism_ratio ?? 1;
                      const modePill =
                        mode === "PARALLEL" || mode === "MIXED"
                          ? "rounded bg-amber-500/10 px-1.5 py-0.5 text-xs text-amber-600 dark:text-amber-400"
                          : "rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary";
                      return (
                        <tr key={idx} className="border-b border-border last:border-0">
                          <td className="px-3 py-2 font-mono text-xs" title={g.guid}>
                            …{String(g.guid ?? "").slice(-12)}
                          </td>
                          <td className="px-3 py-2 text-xs" title={g.root_application}>
                            {g.root_application || "—"}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(g.profile_count ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(g.matched_edge_count ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(m.total_external_call_cumulative_ms ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(m.total_external_call_wall_time_ms ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {Number(ratio).toFixed(2)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(m.max_external_call_concurrency ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {(m.total_network_gap_cumulative_ms ?? 0).toLocaleString()}
                          </td>
                          <td className="px-3 py-2 text-right">
                            <span className={modePill}>{mode}</span>
                          </td>
                          <td className="px-3 py-2 text-right">
                            {g.validation_status === "OK" ? (
                              <span className="rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary">
                                OK
                              </span>
                            ) : (
                              <span className="rounded bg-destructive/10 px-1.5 py-0.5 text-xs text-destructive">
                                {g.validation_status}
                              </span>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {msaEdges.length > 0 && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  External call edges ({msaEdges.length})
                </CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">Caller</th>
                      <th className="px-3 py-2 text-left font-medium">URL</th>
                      <th className="px-3 py-2 text-left font-medium">Callee</th>
                      <th className="px-3 py-2 text-right font-medium">Ext ms</th>
                      <th className="px-3 py-2 text-right font-medium">Callee RT</th>
                      <th className="px-3 py-2 text-right font-medium">Raw Gap</th>
                      <th className="px-3 py-2 text-right font-medium">Adj Gap</th>
                      <th className="px-3 py-2 text-left font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {msaEdges.map((e, idx) => (
                      <tr key={idx} className="border-b border-border last:border-0">
                        <td className="px-3 py-2 text-xs" title={e.caller_application}>
                          {e.caller_application}
                        </td>
                        <td className="px-3 py-2 font-mono text-xs" title={e.external_call_url}>
                          {e.external_call_url}
                        </td>
                        <td className="px-3 py-2 text-xs" title={e.callee_application}>
                          {e.callee_application || "—"}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(e.external_call_elapsed_ms ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {e.callee_response_time_ms ?? "—"}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {e.raw_network_gap_ms ?? "—"}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {e.adjusted_network_gap_ms ?? "—"}
                        </td>
                        <td className="px-3 py-2">
                          {e.match_status === "MATCHED" ? (
                            <span className="rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary">
                              MATCHED
                            </span>
                          ) : (
                            <span className="rounded bg-destructive/10 px-1.5 py-0.5 text-xs text-destructive">
                              {e.match_status}
                            </span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">Transaction profiles</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">TXID</th>
                    <th className="px-3 py-2 text-left font-medium">GUID</th>
                    <th className="px-3 py-2 text-left font-medium">Application</th>
                    <th className="px-3 py-2 text-right font-medium">Resp ms</th>
                    <th className="px-3 py-2 text-right font-medium">SQL Exec ms</th>
                    <th className="px-3 py-2 text-right font-medium">Check Q ms</th>
                    <th className="px-3 py-2 text-right font-medium">2PC ms</th>
                    <th className="px-3 py-2 text-right font-medium">Fetch ms</th>
                    <th className="px-3 py-2 text-right font-medium">Ext call</th>
                    <th className="px-3 py-2 text-right font-medium">Ext call ms</th>
                    <th className="px-3 py-2 text-right font-medium">Issues</th>
                  </tr>
                </thead>
                <tbody>
                  {profileRows.map((row, idx) => {
                    const bm = row.body_metrics ?? {};
                    const hdr = row.header ?? {};
                    const issues = (row.errors?.length ?? 0) + (row.warnings?.length ?? 0);
                    return (
                      <tr key={idx} className="border-b border-border last:border-0">
                        <td className="px-3 py-2 font-mono text-xs">{row.txid}</td>
                        <td className="px-3 py-2 font-mono text-xs" title={row.guid}>
                          {row.guid ? `${String(row.guid).slice(-8)}…` : "—"}
                        </td>
                        <td className="px-3 py-2 text-xs" title={row.application}>
                          {row.application}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {hdr.response_time_ms ?? "—"}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(bm.sql_execute_cum_ms ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(bm.check_query_cum_ms ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(bm.two_pc_cum_ms ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(bm.fetch_cum_ms ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(bm.external_call_count ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {(bm.external_call_cum_ms ?? 0).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {issues > 0 ? (
                            <span className="rounded bg-destructive/10 px-1.5 py-0.5 text-xs text-destructive">
                              {issues}
                            </span>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </CardContent>
          </Card>
          </TabsContent>

          <TabsContent value="msa" className="mt-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  MSA 캡처 상태 (체크쿼리 / 2PC / 외부호출 / 네트워크 준비)
                </CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <MetricCard
                  label="2PC (cum ms)"
                  value={(summary.two_pc_cum_ms ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Check Query (cum ms)"
                  value={(summary.check_query_cum_ms ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="External call (cum ms)"
                  value={(summary.external_call_cum_ms ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="External call count"
                  value={(summary.external_call_count ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Network prep wrapper (ms)"
                  value={(summary.network_prep_method_cum_ms ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Network prep wrapper count"
                  value={(summary.network_prep_method_count ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Network prep (cum ms)"
                  value={(summary.network_prep_cum_ms ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Connection acquire (ms)"
                  value={(summary.connection_acquire_cum_ms ?? 0).toLocaleString()}
                />
              </CardContent>
            </Card>
            {(summary.external_call_cum_ms ?? 0) === 0 &&
              (summary.two_pc_cum_ms ?? 0) === 0 &&
              (summary.check_query_cum_ms ?? 0) === 0 && (
                <p className="mt-3 rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-700 dark:text-amber-300">
                  EXTERNAL_CALL / 2PC / CHECK_QUERY 합계가 모두 0입니다. 본문
                  이벤트가 잡히지 않았을 수 있습니다 — "파서 보고서" 탭의
                  파일 오류와 프로파일별 이슈를 확인하고, 필요하면 위 폼에서
                  사용자 분류 패턴을 추가해 다시 분석하세요.
                </p>
              )}
          </TabsContent>

          <TabsContent value="profiles" className="mt-4 flex flex-col gap-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  GUID 그룹별 카테고리 누적 시간 (Stacked Bar)
                </CardTitle>
              </CardHeader>
              <CardContent>
                <MsaStackedBar groups={guidGroups as any} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  Swimlane 그래프 (다음 단계)
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-xs text-muted-foreground">
                  서비스별 가로 막대 형태의 시간축 그래프는 본문 이벤트의 절대
                  오프셋(start_offset_ms / start_time_ms)을 분석 결과에 노출한
                  뒤 추가합니다. 현재는 위 Stacked Bar로 카테고리별 비중을 비교하고,
                  필요하면 "MSA" 탭의 Edge / GUID 그룹 표를 함께 보세요.
                </p>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="parser" className="mt-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">파서 보고서</CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <MetricCard
                  label="Files"
                  value={(summary.total_files ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Profiles parsed"
                  value={(summary.total_profiles ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Full profiles"
                  value={(summary.full_profile_count ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Total errors"
                  value={(summary.total_errors ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Total warnings"
                  value={(summary.total_warnings ?? 0).toLocaleString()}
                />
                <MetricCard
                  label="Unique signatures"
                  value={(summary.unique_signature_count ?? 0).toLocaleString()}
                />
              </CardContent>
            </Card>
            <Card className="mt-3">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">파일 오류</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                {fileErrors.length === 0 ? (
                  <p className="px-4 py-3 text-xs text-muted-foreground">
                    파일 단위 오류가 없습니다.
                  </p>
                ) : (
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                        <th className="px-4 py-2 text-left font-medium">Source file</th>
                        <th className="px-4 py-2 text-left font-medium">Code</th>
                        <th className="px-4 py-2 text-left font-medium">Message</th>
                      </tr>
                    </thead>
                    <tbody>
                      {fileErrors.map((row, idx) => (
                        <tr key={idx} className="border-b border-border last:border-0">
                          <td className="px-4 py-2 font-mono text-xs" title={row.source_file}>
                            {basenameOf(row.source_file ?? "")}
                          </td>
                          <td className="px-4 py-2 font-mono text-xs">{row.code}</td>
                          <td className="px-4 py-2 text-xs">{row.message}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </CardContent>
            </Card>
            <Card className="mt-3">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">프로파일별 이슈 (errors / warnings)</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                {(() => {
                  const issueRows: any[] = [];
                  for (const row of profileRows) {
                    for (const e of row.errors ?? []) {
                      issueRows.push({
                        txid: row.txid,
                        application: row.application,
                        severity: "error",
                        code: e.code,
                        message: e.message,
                      });
                    }
                    for (const w of row.warnings ?? []) {
                      issueRows.push({
                        txid: row.txid,
                        application: row.application,
                        severity: "warning",
                        code: w.code,
                        message: w.message,
                      });
                    }
                  }
                  if (issueRows.length === 0) {
                    return (
                      <p className="px-4 py-3 text-xs text-muted-foreground">
                        프로파일 단위 이슈가 없습니다.
                      </p>
                    );
                  }
                  return (
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                          <th className="px-3 py-2 text-left font-medium">TXID</th>
                          <th className="px-3 py-2 text-left font-medium">Application</th>
                          <th className="px-3 py-2 text-left font-medium">Severity</th>
                          <th className="px-3 py-2 text-left font-medium">Code</th>
                          <th className="px-3 py-2 text-left font-medium">Message</th>
                        </tr>
                      </thead>
                      <tbody>
                        {issueRows.slice(0, 200).map((iss, idx) => (
                          <tr key={idx} className="border-b border-border last:border-0">
                            <td className="px-3 py-2 font-mono text-xs">{iss.txid}</td>
                            <td className="px-3 py-2 text-xs" title={iss.application}>{iss.application}</td>
                            <td className="px-3 py-2 text-xs">
                              <span
                                className={
                                  iss.severity === "error"
                                    ? "rounded bg-destructive/10 px-1.5 py-0.5 text-destructive"
                                    : "rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-700 dark:text-amber-300"
                                }
                              >
                                {iss.severity}
                              </span>
                            </td>
                            <td className="px-3 py-2 font-mono text-xs">{iss.code}</td>
                            <td className="px-3 py-2 text-xs">{iss.message}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  );
                })()}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      )}
    </main>
  );
}
