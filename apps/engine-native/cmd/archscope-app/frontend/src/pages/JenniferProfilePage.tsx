// ─────────────────────────────────────────────────────────────────────
// [한글] pages/JenniferProfilePage.tsx — Jennifer 프로파일 분석기 페이지(MVP1).
//
// 책임/목적:
//   - Jennifer profile export(.csv 다중 선택) 을 받아
//     engine.analyzeJenniferProfile 호출 → AnalysisResult 수신.
//   - Summary metric cards + per-profile 표 + 파일별 에러 리스트 렌더.
//   - 다중 파일 호출 시 fallbackCorrelationToTxid / headerBodyToleranceMs
//     같은 옵션 plumbing 도 이 페이지에서 보유.
//
// 다루는 결과 형식: AnalysisResult (Jennifer profile 전용 — analyzer
// type = "jennifer_profile" 등). MSA call-graph / network gap /
// signature stats 등 고급 표시는 MVP2-MVP3 에서 추가 예정.
//
// 데이터 흐름: WailsFileDock (native multi-select / drop) → paths[] →
// engine.analyzeJenniferProfile({ paths, fallbackCorrelationToTxid,
// headerBodyToleranceMs }) → 결과 렌더.
// ─────────────────────────────────────────────────────────────────────
// MVP1 Jennifer Profile Export analyzer page.
//
// Surface: file pick (multiple) → Analyze → render Summary metric
// cards + per-profile table + file errors. MSA call-graph / network
// gap / signature stats land in MVP2-MVP3.

import {
  Loader2,
  Network,
  Play,
  Plus,
  Rows3,
  Save,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import {
  useCallback,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";

import { engine } from "../bridge/engine";

import {
  CustomCategoriesEditor,
  type CategoryRules,
  type Preset,
  type SegmentSpec,
} from "../components/CustomCategoriesEditor";
import { MsaResponseTimeBreakdown } from "../components/MsaResponseTimeBreakdown";
import { MsaTimeline, MsaTimelineTreemap } from "../components/MsaTimeline";
import { MsaTopology } from "../components/MsaTopology";
import { AnalyzerOptionsDock } from "../components/AnalyzerOptionsDock";
import { MetricCard } from "../components/MetricCard";
import { RecentFilesPanel } from "../components/RecentFilesPanel";
import {
  WailsFileDock,
  type FileDockSelection,
} from "../components/WailsFileDock";
import { useRecentFiles } from "../hooks/useRecentFiles";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "../components/ui/tabs";
import { useI18n } from "../i18n/I18nProvider";
import { addWorkspaceResult } from "../state/analysisWorkspace";

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
    displayName: "Jennifer profile export",
    pattern: "*.txt;*.log;*.profile",
  },
  { displayName: "All files", pattern: "*.*" },
];

const MSA_TABLE_PREVIEW_LIMIT = 200;
const DEFAULT_SLOW_SQL_THRESHOLD_MS = 1000;
const CUSTOM_RULE_PRESET_VERSION = 1;

type Selection = {
  filePath: string;
  originalName: string;
};

type MsaTimelineMode = "single" | "average";
type MsaTimelineLayout = "grouped" | "individual";
type CustomAnalysisRuleSource =
  | "profile_application"
  | "method"
  | "external_call_url";
type CustomAnalysisRuleGroup = "database" | "external" | "internal";

type CustomAnalysisRule = {
  id: string;
  label: string;
  group: CustomAnalysisRuleGroup;
  source: CustomAnalysisRuleSource;
  patterns: string[];
};

const CUSTOM_RULE_GROUPS: Array<{
  id: CustomAnalysisRuleGroup;
  label: string;
}> = [
  { id: "database", label: "Database" },
  { id: "external", label: "External" },
  { id: "internal", label: "Internal" },
];

const CUSTOM_RULE_SOURCES: Array<{
  id: CustomAnalysisRuleSource;
  label: string;
}> = [
  { id: "profile_application", label: "프로파일 URL 전체시간" },
  { id: "method", label: "메소드/이벤트 처리시간" },
  { id: "external_call_url", label: "EXTERNAL_CALL URL" },
];

function basenameOf(p: string): string {
  const segments = p.split(/[\\/]/);
  return segments[segments.length - 1] ?? p;
}

function toFiniteNumber(value: unknown): number | undefined {
  const n = Number(value);
  return Number.isFinite(n) ? n : undefined;
}

function statValue(stats: any, key: string): number | undefined {
  if (!stats || typeof stats !== "object") return undefined;
  return toFiniteNumber(stats[key]);
}

function buildAverageTimelineEdges(signature: any): any[] {
  const edges: any[] = signature?.edges ?? [];
  let cursor = 0;
  return [...edges]
    .sort(
      (a, b) =>
        Number(a?.occurrence_index ?? 0) - Number(b?.occurrence_index ?? 0),
    )
    .map((edge, idx) => {
      const elapsed = Math.max(
        0,
        Math.round(statValue(edge?.external_call_elapsed_ms, "avg") ?? 0),
      );
      const start = cursor;
      cursor += elapsed;
      const networkGap =
        statValue(edge?.adjusted_network_gap_ms, "avg") ??
        statValue(edge?.raw_network_gap_ms, "avg") ??
        0;
      return {
        caller_application: edge?.caller_application ?? "?",
        callee_application: edge?.callee_application ?? "?",
        caller_txid: `avg-${idx}-caller`,
        callee_txid: `avg-${idx}-callee`,
        external_call_elapsed_ms: elapsed,
        callee_response_time_ms: Math.max(
          0,
          Math.round(statValue(edge?.callee_response_time_ms, "avg") ?? 0),
        ),
        network_gap_ms: Math.max(0, Math.round(networkGap)),
        caller_event_start_ms: start,
        match_status: "MATCHED",
      };
    })
    .filter((edge) => edge.external_call_elapsed_ms > 0);
}

function uniqueTimelineEdges(edges: any[]): any[] {
  const seen = new Set<string>();
  const out: any[] = [];
  for (const edge of edges) {
    const key = timelineEdgeKey(edge);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(edge);
  }
  return out;
}

function timelineEdgeKey(edge: any): string {
  return [
    edge?.timeline_kind ?? "external_call",
    edge?.match_status ?? "MATCHED",
    edge?.guid ?? "",
    edge?.caller_txid ?? "",
    edge?.callee_txid ?? "",
    edge?.external_call_sequence ?? "",
    edge?.caller_application ?? "",
    edge?.callee_application ?? edge?.external_call_target ?? edge?.external_call_url ?? "",
    Math.round(toFiniteNumber(edge?.caller_event_start_ms) ?? -1),
    Math.round(toFiniteNumber(edge?.external_call_elapsed_ms) ?? -1),
  ].join("\u0000");
}

function buildNetworkTimeByTxid(groups: any[]): Map<string, number> {
  const out = new Map<string, number>();
  for (const group of groups) {
    for (const edge of group?.call_graph ?? []) {
      const txid = String(edge?.caller_txid ?? "");
      if (!txid) continue;
      out.set(txid, (out.get(txid) ?? 0) + Number(edge?.network_gap_ms ?? 0));
    }
  }
  return out;
}

function txidSetFromProfiles(rows: any[]): Set<string> {
  return new Set(rows.map((row) => String(row?.txid ?? "")).filter(Boolean));
}

function makeCustomRule(): CustomAnalysisRule {
  return {
    id: `custom-${Date.now().toString(36)}-${Math.random()
      .toString(36)
      .slice(2, 7)}`,
    label: "",
    group: "external",
    source: "external_call_url",
    patterns: [],
  };
}

function parsePatternText(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/\r?\n|,/)
        .map((part) => part.trim())
        .filter(Boolean),
    ),
  );
}

function normalizeStringList(input: unknown): string[] {
  if (!Array.isArray(input)) return [];
  return Array.from(
    new Set(
      input
        .map((value) => String(value ?? "").trim())
        .filter(Boolean),
    ),
  );
}

function normalizeCustomAnalysisRules(input: unknown): CustomAnalysisRule[] {
  if (!Array.isArray(input)) return [];
  const groups = new Set(CUSTOM_RULE_GROUPS.map((group) => group.id));
  const sources = new Set(CUSTOM_RULE_SOURCES.map((source) => source.id));
  return input
    .map((raw: any) => {
      const group = groups.has(raw?.group) ? raw.group : "external";
      const source = sources.has(raw?.source) ? raw.source : "external_call_url";
      const patterns = Array.isArray(raw?.patterns)
        ? parsePatternText(raw.patterns.join("\n"))
        : parsePatternText(String(raw?.patterns ?? ""));
      return {
        id: String(raw?.id || makeCustomRule().id),
        label: String(raw?.label ?? "").trim(),
        group,
        source,
        patterns,
      } as CustomAnalysisRule;
    })
    .filter((rule) => rule.label && rule.patterns.length > 0);
}

function compactJenniferResult(raw: any): any {
  const tables = raw?.tables;
  if (!tables || !Array.isArray(tables.msa_edges)) return raw;
  const unmatched = Array.isArray(tables.unmatched_external_calls)
    ? tables.unmatched_external_calls
    : tables.msa_edges.filter(
        (edge: any) => edge?.match_status && edge.match_status !== "MATCHED",
      );
  return {
    ...raw,
    tables: {
      ...tables,
      // Matched edges are already present in series.guid_groups[*].call_graph.
      // Keep only non-matched rows here so the renderer does not hold the
      // same large edge list twice.
      msa_edges: unmatched,
    },
  };
}

function guidOptionLabel(group: any): string {
  const guid = String(group?.guid ?? "");
  const root = group?.root_application || "unknown";
  const responseTime = toFiniteNumber(group?.root_response_time_ms);
  const txids = (group?.profile_txids ?? []) as string[];
  const suffix =
    guid.startsWith("standalone:") && txids.length > 0
      ? `TXID …${String(txids[0]).slice(-10)}`
      : guid
        ? `…${guid.slice(-10)}`
        : "단일 프로파일";
  return responseTime != null
    ? `${root} · ${suffix} · ${Math.round(responseTime).toLocaleString()} ms`
    : `${root} · ${suffix}`;
}

function signatureOptionLabel(signature: any): string {
  const hash = String(signature?.signature_hash ?? "");
  const root = signature?.root_application || "unknown";
  const samples = Number(signature?.sample_count ?? 0).toLocaleString();
  const suffix = hash ? `…${hash.slice(-8)}` : "signature 없음";
  return `${root} · ${suffix} · ${samples}건`;
}

function GuidTransactionSummary({ group }: { group: any }): JSX.Element {
  const metrics = group?.metrics ?? {};
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">단일 트랜잭션 기준값</CardTitle>
        <p className="text-xs text-muted-foreground">
          선택한 GUID 하나의 실제 호출 순서와 elapsed 값을 그대로 표시합니다.
        </p>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-6">
        <MetricCard
          label="Root RT"
          value={(group?.root_response_time_ms ?? 0).toLocaleString()}
        />
        <MetricCard
          label="Profiles"
          value={(group?.profile_count ?? 0).toLocaleString()}
        />
        <MetricCard
          label="Matched"
          value={(group?.matched_edge_count ?? 0).toLocaleString()}
        />
        <MetricCard
          label="External cum"
          value={(metrics.total_external_call_cumulative_ms ?? 0).toLocaleString()}
        />
        <MetricCard
          label="Network gap"
          value={(metrics.total_network_gap_cumulative_ms ?? 0).toLocaleString()}
        />
        <MetricCard
          label="Mode"
          value={String(metrics.group_execution_mode ?? "—")}
        />
      </CardContent>
    </Card>
  );
}

function MsaCaptureStatusCard({ summary }: { summary: any }): JSX.Element {
  return (
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
  );
}

function IncompleteProfileWarning({ summary }: { summary: any }): JSX.Element | null {
  const incomplete = Number(summary?.incomplete_profile_count ?? 0);
  const excludedGroups = Number(summary?.signature_excluded_group_count ?? 0);
  if (incomplete <= 0 && excludedGroups <= 0) return null;
  return (
    <Card className="border-amber-500/50 bg-amber-500/5">
      <CardHeader className="pb-2">
        <CardTitle className="text-sm text-amber-800 dark:text-amber-200">
          불완전 프로파일 감지
        </CardTitle>
      </CardHeader>
      <CardContent className="text-xs text-amber-900 dark:text-amber-100">
        Jennifer의 5MB Profile capacity exceeded 문구가 포함된 프로파일{" "}
        {incomplete.toLocaleString()}건이 있습니다. 해당 GUID 그룹{" "}
        {excludedGroups.toLocaleString()}건은 시그니처 평균 통계에서 제외했습니다.
      </CardContent>
    </Card>
  );
}

function TransactionProfilesTable({
  rows,
  networkTimeByTxid,
}: {
  rows: any[];
  networkTimeByTxid: Map<string, number>;
}): JSX.Element {
  const visibleRows = rows.slice(0, MSA_TABLE_PREVIEW_LIMIT);
  const hiddenCount = Math.max(0, rows.length - visibleRows.length);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">Transaction profiles ({rows.length})</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {hiddenCount > 0 && (
          <p className="border-b border-border px-3 py-2 text-xs text-muted-foreground">
            상위 {visibleRows.length.toLocaleString()}건만 표시합니다. 숨김{" "}
            {hiddenCount.toLocaleString()}건.
          </p>
        )}
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              <th className="px-3 py-2 text-left font-medium">TXID</th>
              <th className="px-3 py-2 text-left font-medium">GUID</th>
              <th className="px-3 py-2 text-left font-medium">Application</th>
              <th className="px-3 py-2 text-right font-medium">Resp ms</th>
              <th className="px-3 py-2 text-right font-medium">Network ms</th>
              <th className="px-3 py-2 text-right font-medium">Prep ms</th>
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
            {visibleRows.map((row, idx) => {
              const bm = row.body_metrics ?? {};
              const hdr = row.header ?? {};
              const issues = (row.errors?.length ?? 0) + (row.warnings?.length ?? 0);
              const networkMs = networkTimeByTxid.get(String(row.txid ?? "")) ?? 0;
              return (
                <tr key={idx} className="border-b border-border last:border-0">
                  <td className="px-3 py-2 font-mono text-xs">{row.txid}</td>
                  <td className="px-3 py-2 font-mono text-xs" title={row.guid}>
                    {row.guid ? `${String(row.guid).slice(-8)}...` : "-"}
                  </td>
                  <td className="px-3 py-2 text-xs" title={row.application}>
                    {row.application}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {hdr.response_time_ms ?? "-"}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {Math.round(networkMs).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {(bm.network_prep_cum_ms ?? 0).toLocaleString()}
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
                      <span className="text-muted-foreground">-</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

function NetworkPrepMethodsTable({ rows }: { rows: any[] }): JSX.Element {
  const visibleRows = rows.slice(0, MSA_TABLE_PREVIEW_LIMIT);
  const hiddenCount = Math.max(0, rows.length - visibleRows.length);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">
          네트워크 준비시간 포함 메소드 ({rows.length})
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          wrapper 메소드 안에 포함된 EXTERNAL_CALL 합계를 빼고 남은 시간을
          네트워크 준비시간으로 계산합니다.
        </p>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {rows.length === 0 ? (
          <p className="px-4 py-3 text-xs text-muted-foreground">
            네트워크 준비시간으로 포함된 메소드가 없습니다.
          </p>
        ) : (
          <>
            {hiddenCount > 0 && (
              <p className="border-b border-border px-3 py-2 text-xs text-muted-foreground">
                상위 {visibleRows.length.toLocaleString()}건만 표시합니다. 숨김{" "}
                {hiddenCount.toLocaleString()}건.
              </p>
            )}
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                  <th className="px-3 py-2 text-left font-medium">TXID</th>
                  <th className="px-3 py-2 text-left font-medium">Application</th>
                  <th className="px-3 py-2 text-left font-medium">Method</th>
                  <th className="px-3 py-2 text-right font-medium">Calls</th>
                  <th className="px-3 py-2 text-right font-medium">Method ms</th>
                  <th className="px-3 py-2 text-right font-medium">Call sum</th>
                  <th className="px-3 py-2 text-right font-medium">Prep ms</th>
                  <th className="px-3 py-2 text-left font-medium">URLs</th>
                  <th className="px-3 py-2 text-left font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {visibleRows.map((row, idx) => {
                  const urls = Array.isArray(row.external_call_urls)
                    ? row.external_call_urls.filter(Boolean).join(", ")
                    : "";
                  return (
                    <tr key={idx} className="border-b border-border last:border-0">
                      <td className="px-3 py-2 font-mono text-xs">
                        {row.txid || "—"}
                      </td>
                      <td className="px-3 py-2 text-xs" title={row.application}>
                        {row.application || "—"}
                      </td>
                      <td className="max-w-[360px] px-3 py-2 font-mono text-xs">
                        <span className="block truncate" title={row.raw_message}>
                          {row.raw_message || "—"}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.external_call_count ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.method_elapsed_ms ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.external_call_cum_ms ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.network_prep_ms ?? 0).toLocaleString()}
                      </td>
                      <td className="max-w-[320px] px-3 py-2 font-mono text-xs">
                        <span className="block truncate" title={urls}>
                          {urls || "—"}
                        </span>
                      </td>
                      <td className="px-3 py-2">
                        {row.suspicious ? (
                          <span
                            className="rounded bg-amber-500/10 px-1.5 py-0.5 text-xs text-amber-700 dark:text-amber-300"
                            title={(row.warnings ?? []).join(", ")}
                          >
                            확인 필요
                          </span>
                        ) : (
                          <span className="rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary">
                            포함
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </>
        )}
      </CardContent>
    </Card>
  );
}

function UnprofiledExternalCallGroupsTable({ rows }: { rows: any[] }): JSX.Element {
  const visibleRows = rows.slice(0, MSA_TABLE_PREVIEW_LIMIT);
  const hiddenCount = Math.max(0, rows.length - visibleRows.length);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">
          프로파일 미수집 외부호출 ({rows.length})
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          callee profile이 없는 EXTERNAL_CALL은 Method 잔여시간에서 분리해
          별도 외부호출 시간으로 계산합니다.
        </p>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {rows.length === 0 ? (
          <p className="px-4 py-3 text-xs text-muted-foreground">
            프로파일 미수집 외부호출이 없습니다.
          </p>
        ) : (
          <>
            {hiddenCount > 0 && (
              <p className="border-b border-border px-3 py-2 text-xs text-muted-foreground">
                상위 {visibleRows.length.toLocaleString()}건만 표시합니다. 숨김{" "}
                {hiddenCount.toLocaleString()}건.
              </p>
            )}
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                  <th className="px-3 py-2 text-left font-medium">Caller</th>
                  <th className="px-3 py-2 text-left font-medium">Target</th>
                  <th className="px-3 py-2 text-left font-medium">Client</th>
                  <th className="px-3 py-2 text-right font-medium">Count</th>
                  <th className="px-3 py-2 text-right font-medium">Total ms</th>
                  <th className="px-3 py-2 text-right font-medium">Avg ms</th>
                  <th className="px-3 py-2 text-right font-medium">Max ms</th>
                  <th className="px-3 py-2 text-left font-medium">Sample URLs</th>
                </tr>
              </thead>
              <tbody>
                {visibleRows.map((row, idx) => {
                  const urls = Array.isArray(row.external_call_urls)
                    ? row.external_call_urls.filter(Boolean).join(", ")
                    : "";
                  const client = [row.protocol, row.client].filter(Boolean).join(" / ");
                  return (
                    <tr key={idx} className="border-b border-border last:border-0">
                      <td
                        className="max-w-[220px] px-3 py-2 font-mono text-xs"
                        title={row.caller_application}
                      >
                        <span className="block truncate">
                          {row.caller_application || "—"}
                        </span>
                      </td>
                      <td
                        className="max-w-[280px] px-3 py-2 font-mono text-xs"
                        title={row.target}
                      >
                        <span className="block truncate">{row.target || "—"}</span>
                      </td>
                      <td className="px-3 py-2 text-xs" title={client}>
                        {client || "—"}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.count ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.total_elapsed_ms ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.avg_elapsed_ms ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {(row.max_elapsed_ms ?? 0).toLocaleString()}
                      </td>
                      <td className="max-w-[320px] px-3 py-2 font-mono text-xs">
                        <span className="block truncate" title={urls}>
                          {urls || "—"}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </>
        )}
      </CardContent>
    </Card>
  );
}

function ExternalCallTopTable({ edges }: { edges: any[] }): JSX.Element {
  const rows = aggregateExternalCallPairs(edges).slice(0, MSA_TABLE_PREVIEW_LIMIT);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">
          EXTERNAL_CALL 누적 시간 상위 ({rows.length})
        </CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {rows.length === 0 ? (
          <p className="px-4 py-3 text-xs text-muted-foreground">
            표시할 외부 호출이 없습니다.
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">Caller → Callee</th>
                <th className="px-3 py-2 text-right font-medium">Calls</th>
                <th className="px-3 py-2 text-right font-medium">Total ms</th>
                <th className="px-3 py-2 text-right font-medium">Avg ms</th>
                <th className="px-3 py-2 text-right font-medium">Network ms</th>
                <th className="px-3 py-2 text-right font-medium">Max ms</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, idx) => (
                <tr key={idx} className="border-b border-border last:border-0">
                  <td
                    className="max-w-[420px] px-3 py-2 font-mono text-xs"
                    title={`${row.caller} → ${row.callee}`}
                  >
                    <span className="block truncate">
                      {row.caller} → {row.callee}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {row.count.toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {Math.round(row.totalMs).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {Math.round(row.totalMs / row.count).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {Math.round(row.networkMs).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {Math.round(row.maxMs).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  );
}

function SlowSqlTable({ rows }: { rows: any[] }): JSX.Element {
  const [minElapsedInput, setMinElapsedInput] = useState(
    String(DEFAULT_SLOW_SQL_THRESHOLD_MS),
  );
  const minElapsedMs = Math.max(0, Number(minElapsedInput) || 0);
  const filteredRows = rows.filter(
    (row) => Number(row?.elapsed_ms ?? 0) >= minElapsedMs,
  );
  const visibleRows = [...filteredRows]
    .sort((a, b) => Number(b?.elapsed_ms ?? 0) - Number(a?.elapsed_ms ?? 0))
    .slice(0, MSA_TABLE_PREVIEW_LIMIT);
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <CardTitle className="text-sm">
              성능 저하 SQL 후보 ({filteredRows.length}/{rows.length})
            </CardTitle>
            <p className="mt-1 text-xs text-muted-foreground">
              기본값은 1초 이상 수행된 SQL만 표시합니다.
            </p>
          </div>
          <div className="flex flex-wrap items-end gap-2 text-xs">
            <label className="flex flex-col gap-1">
              <span className="font-medium text-foreground/80">
                최소 응답시간(ms)
              </span>
              <input
                type="number"
                min={0}
                step={100}
                value={minElapsedInput}
                onChange={(event) => setMinElapsedInput(event.target.value)}
                className="h-8 w-32 rounded-md border border-input bg-background px-2 text-right text-xs"
              />
            </label>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setMinElapsedInput("0")}
            >
              전체
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setMinElapsedInput("1000")}
            >
              1초+
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setMinElapsedInput("3000")}
            >
              3초+
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {visibleRows.length === 0 ? (
          <p className="px-4 py-3 text-xs text-muted-foreground">
            조건에 맞는 SQL 실행 이벤트가 없습니다.
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">TXID</th>
                <th className="px-3 py-2 text-left font-medium">Application</th>
                <th className="px-3 py-2 text-left font-medium">Type</th>
                <th className="px-3 py-2 text-right font-medium">Elapsed ms</th>
                <th className="px-3 py-2 text-left font-medium">SQL / Message</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row, idx) => {
                const sql = String(row?.sql_text || row?.raw_message || "-");
                return (
                  <tr key={idx} className="border-b border-border last:border-0">
                    <td className="px-3 py-2 font-mono text-xs">{row?.txid || "-"}</td>
                    <td className="px-3 py-2 text-xs" title={row?.application}>
                      {row?.application || "-"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs">
                      {row?.event_type || "-"}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {(row?.elapsed_ms ?? 0).toLocaleString()}
                    </td>
                    <td className="max-w-[560px] px-3 py-2 font-mono text-xs">
                      <span className="block truncate" title={sql}>
                        {sql}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  );
}

function CustomAnalysisRulesEditor({
  rules,
  onChange,
  disabled,
  onSavePreset,
  onLoadPresetClick,
  presetInputRef,
  onLoadPresetFile,
}: {
  rules: CustomAnalysisRule[];
  onChange: (rules: CustomAnalysisRule[]) => void;
  disabled?: boolean;
  onSavePreset: () => void;
  onLoadPresetClick: () => void;
  presetInputRef: RefObject<HTMLInputElement>;
  onLoadPresetFile: (file?: File) => void;
}): JSX.Element {
  const updateRule = (id: string, patch: Partial<CustomAnalysisRule>) => {
    onChange(
      rules.map((rule) => (rule.id === id ? { ...rule, ...patch } : rule)),
    );
  };
  return (
    <div className="flex flex-col gap-3 rounded-md border border-border bg-muted/20 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-xs font-semibold text-foreground/80">
            분석 옵션 프리셋 / 사용자 정의 카드
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">
            MSA 이벤트 분류 패턴과 사용자 정의 카드를 함께 저장하고 읽습니다.
            카드는 응답시간 구성에도 반영됩니다.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={disabled}
            onClick={() => onChange([...rules, makeCustomRule()])}
          >
            <Plus className="h-3.5 w-3.5" />
            카드
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={disabled}
            onClick={onSavePreset}
          >
            <Save className="h-3.5 w-3.5" />
            전체 설정 저장
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={disabled}
            onClick={onLoadPresetClick}
          >
            <Upload className="h-3.5 w-3.5" />
            전체 설정 읽기
          </Button>
          <input
            ref={presetInputRef}
            type="file"
            accept="application/json,.json"
            className="hidden"
            onChange={(event) => onLoadPresetFile(event.target.files?.[0])}
          />
        </div>
      </div>

      {rules.length === 0 ? (
        <p className="rounded border border-dashed border-border bg-background/50 px-3 py-2 text-xs text-muted-foreground">
          추가된 카드가 없습니다.
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-3">
          {rules.map((rule) => (
            <div
              key={rule.id}
              className="grid gap-2 rounded border border-border bg-background/60 p-2"
            >
              <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_150px_190px_auto]">
                <input
                  type="text"
                  value={rule.label}
                  disabled={disabled}
                  placeholder="카드 이름"
                  onChange={(event) =>
                    updateRule(rule.id, { label: event.target.value })
                  }
                  className="h-8 rounded-md border border-input bg-background px-2 text-xs"
                />
                <select
                  value={rule.group}
                  disabled={disabled}
                  onChange={(event) =>
                    updateRule(rule.id, {
                      group: event.target.value as CustomAnalysisRuleGroup,
                    })
                  }
                  className="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground"
                >
                  {CUSTOM_RULE_GROUPS.map((group) => (
                    <option key={group.id} value={group.id}>
                      {group.label}
                    </option>
                  ))}
                </select>
                <select
                  value={rule.source}
                  disabled={disabled}
                  onChange={(event) =>
                    updateRule(rule.id, {
                      source: event.target.value as CustomAnalysisRuleSource,
                    })
                  }
                  className="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground"
                >
                  {CUSTOM_RULE_SOURCES.map((source) => (
                    <option key={source.id} value={source.id}>
                      {source.label}
                    </option>
                  ))}
                </select>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={disabled}
                  onClick={() => onChange(rules.filter((r) => r.id !== rule.id))}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
              <textarea
                value={rule.patterns.join("\n")}
                disabled={disabled}
                placeholder="패턴을 줄 단위로 입력"
                onChange={(event) =>
                  updateRule(rule.id, {
                    patterns: parsePatternText(event.target.value),
                  })
                }
                className="min-h-16 rounded-md border border-input bg-background px-2 py-1.5 font-mono text-xs"
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CustomRuleStatsPanel({ rows }: { rows: any[] }): JSX.Element | null {
  if (rows.length === 0) return null;
  const visibleRows = rows.slice(0, MSA_TABLE_PREVIEW_LIMIT);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">
          사용자 정의 카드 통계 ({rows.length})
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          분석 옵션에서 추가한 카드별 집계입니다. 프로파일 URL은 전체
          응답시간, 메소드는 이벤트 elapsed, EXTERNAL_CALL은 호출 URL elapsed를
          합산합니다.
        </p>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              <th className="px-3 py-2 text-left font-medium">Group</th>
              <th className="px-3 py-2 text-left font-medium">Card</th>
              <th className="px-3 py-2 text-left font-medium">Source</th>
              <th className="px-3 py-2 text-right font-medium">Count</th>
              <th className="px-3 py-2 text-right font-medium">Total ms</th>
              <th className="px-3 py-2 text-right font-medium">Avg ms</th>
              <th className="px-3 py-2 text-right font-medium">Max ms</th>
              <th className="px-3 py-2 text-left font-medium">Patterns</th>
            </tr>
          </thead>
          <tbody>
            {visibleRows.map((row, idx) => (
              <tr key={idx} className="border-b border-border last:border-0">
                <td className="px-3 py-2 text-xs">{row?.group || "-"}</td>
                <td className="px-3 py-2 text-xs font-medium">
                  {row?.label || "-"}
                </td>
                <td className="px-3 py-2 font-mono text-xs">
                  {row?.source || "-"}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {(row?.count ?? 0).toLocaleString()}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {Math.round(Number(row?.total_ms ?? 0)).toLocaleString()}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {Math.round(Number(row?.avg_ms ?? 0)).toLocaleString()}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {Math.round(Number(row?.max_ms ?? 0)).toLocaleString()}
                </td>
                <td
                  className="max-w-[360px] px-3 py-2 font-mono text-xs"
                  title={(row?.patterns ?? []).join(", ")}
                >
                  <span className="block truncate">
                    {(row?.patterns ?? []).join(", ") || "-"}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

function aggregateExternalCallPairs(edges: any[]): Array<{
  caller: string;
  callee: string;
  count: number;
  totalMs: number;
  networkMs: number;
  maxMs: number;
}> {
  const byPair = new Map<string, {
    caller: string;
    callee: string;
    count: number;
    totalMs: number;
    networkMs: number;
    maxMs: number;
  }>();
  for (const edge of edges) {
    const caller = String(edge?.caller_application ?? "?");
    const callee = String(
      edge?.callee_application ??
        edge?.external_call_target ??
        edge?.external_call_url ??
        "?",
    );
    const key = `${caller}\u0000${callee}`;
    const elapsed = Number(edge?.external_call_elapsed_ms ?? 0);
    const network = Number(
      edge?.network_gap_ms ?? edge?.adjusted_network_gap_ms ?? edge?.raw_network_gap_ms ?? 0,
    );
    const cur =
      byPair.get(key) ??
      { caller, callee, count: 0, totalMs: 0, networkMs: 0, maxMs: 0 };
    cur.count += 1;
    cur.totalMs += elapsed;
    cur.networkMs += Number.isFinite(network) ? Math.max(0, network) : 0;
    cur.maxMs = Math.max(cur.maxMs, elapsed);
    byPair.set(key, cur);
  }
  return Array.from(byPair.values()).sort((a, b) => b.totalMs - a.totalMs);
}

function ServiceNetworkTimeSummary({ rows }: { rows: any[] }): JSX.Element {
  const visibleRows = [...rows]
    .sort((a, b) => {
      const groupDelta =
        Number(b.network_time_group_index ?? 0) -
        Number(a.network_time_group_index ?? 0);
      if (groupDelta !== 0) return groupDelta;
      const totalDelta =
        Number(b.total_network_gap_ms ?? 0) - Number(a.total_network_gap_ms ?? 0);
      if (totalDelta !== 0) return totalDelta;
      return String(a.caller_application ?? "").localeCompare(
        String(b.caller_application ?? ""),
      );
    })
    .slice(0, 20);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">
          서비스 호출 네트워크 타임 ({rows.length})
        </CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              <th className="px-3 py-2 text-left font-medium">Caller → Callee</th>
              <th className="px-3 py-2 text-right font-medium">Calls</th>
              <th className="px-3 py-2 text-right font-medium">Ext avg</th>
              <th className="px-3 py-2 text-right font-medium">Callee avg</th>
              <th className="px-3 py-2 text-right font-medium">Network avg</th>
              <th className="px-3 py-2 text-right font-medium">Network p95</th>
              <th className="px-3 py-2 text-right font-medium">Network max</th>
              <th className="px-3 py-2 text-left font-medium">Group</th>
            </tr>
          </thead>
          <tbody>
            {visibleRows.map((row, idx) => {
              const caller = row.caller_application || "?";
              const callee = row.callee_application || "?";
              return (
                <tr key={idx} className="border-b border-border last:border-0">
                  <td
                    className="max-w-[360px] px-3 py-2 font-mono text-xs"
                    title={`${caller} → ${callee}`}
                  >
                    <span className="block truncate">
                      {caller} → {callee}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {(row.call_count ?? 0).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatMsValue(statValue(row.external_call_elapsed_ms, "avg"))}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatMsValue(statValue(row.callee_response_time_ms, "avg"))}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatMsValue(row.avg_network_gap_ms)}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatMsValue(row.p95_network_gap_ms)}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatMsValue(row.max_network_gap_ms)}
                  </td>
                  <td className="px-3 py-2">
                    <span className={networkGroupClass(row.network_time_group_index)}>
                      {row.network_time_group_label || row.network_time_group || "—"}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

function formatMsValue(value: unknown): string {
  const n = toFiniteNumber(value);
  return n == null ? "—" : `${Math.round(n).toLocaleString()} ms`;
}

function networkGroupClass(index: unknown): string {
  const n = Number(index ?? 0);
  if (n >= 4) {
    return "rounded bg-destructive/10 px-1.5 py-0.5 text-xs text-destructive";
  }
  if (n >= 3) {
    return "rounded bg-amber-500/10 px-1.5 py-0.5 text-xs text-amber-700 dark:text-amber-300";
  }
  return "rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary";
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
  const [msaTimelineMode, setMsaTimelineMode] =
    useState<MsaTimelineMode>("single");
  const [singleTimelineLayout, setSingleTimelineLayout] =
    useState<MsaTimelineLayout>("grouped");
  const [customAnalysisRules, setCustomAnalysisRules] = useState<
    CustomAnalysisRule[]
  >([]);
  const [selectedGuid, setSelectedGuid] = useState<string>("");
  const [selectedSignatureHash, setSelectedSignatureHash] =
    useState<string>("");
  const customPresetInputRef = useRef<HTMLInputElement>(null);
  // Jennifer-scoped recent-files history. The UI lets users add
  // multiple files at once, so a "recent" entry restores the full
  // selection vector (paths array + analyzer options).
  const recent = useRecentFiles({ category: "jennifer" });

  const addSelections = useCallback((files: FileDockSelection[]) => {
    setError("");
    const additions: Selection[] = files.map((file) => ({
      filePath: file.filePath,
      originalName: file.originalName || basenameOf(file.filePath),
    }));
    setSelected((prev) => {
      const seen = new Set(prev.map((s) => s.filePath));
      return [...prev, ...additions.filter((a) => !seen.has(a.filePath))];
    });
  }, []);

  const handleFileSelected = useCallback(
    (file: FileDockSelection) => addSelections([file]),
    [addSelections],
  );

  const handleFilesSelected = useCallback(
    (files: FileDockSelection[]) => addSelections(files),
    [addSelections],
  );

  const handleClear = (path: string) => {
    setSelected((prev) => prev.filter((s) => s.filePath !== path));
  };

  const handleClearAll = () => {
    setSelected([]);
    setResult(null);
    setError("");
  };

  const handleSaveAnalysisPreset = useCallback(() => {
    const preset = {
      version: CUSTOM_RULE_PRESET_VERSION,
      kind: "jennifer_analysis_options",
      savedAt: new Date().toISOString(),
      fallbackToTxid,
      networkPrepPatterns,
      eventCategoryPatterns,
      customAnalysisRules: normalizeCustomAnalysisRules(customAnalysisRules),
    };
    const blob = new Blob([JSON.stringify(preset, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "archscope-jennifer-analysis-preset.json";
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }, [customAnalysisRules, eventCategoryPatterns, fallbackToTxid, networkPrepPatterns]);

  const handleLoadAnalysisPresetFile = useCallback(
    (file?: File) => {
      if (!file) return;
      void file
        .text()
        .then((text) => {
          const parsed = JSON.parse(text);
          const loadedCategories: CategoryRules =
            parsed?.eventCategoryPatterns &&
            typeof parsed.eventCategoryPatterns === "object"
              ? { ...(parsed.eventCategoryPatterns as CategoryRules) }
              : {};
          const loadedPrep = normalizeStringList(parsed?.networkPrepPatterns);
          if (loadedPrep.length > 0) {
            loadedCategories.NETWORK_PREP_METHOD = Array.from(
              new Set([
                ...(loadedCategories.NETWORK_PREP_METHOD ?? []),
                ...loadedPrep,
              ]),
            );
          }
          setEventCategoryPatterns(loadedCategories);
          if (
            typeof parsed?.fallbackToTxid === "boolean"
          ) {
            setFallbackToTxid(parsed.fallbackToTxid);
          } else {
            setFallbackToTxid(false);
          }
          setCustomAnalysisRules(
            normalizeCustomAnalysisRules(parsed?.customAnalysisRules),
          );
          setError("");
        })
        .catch((err) => {
          setError(`프리셋 파일을 읽지 못했습니다: ${String(err?.message ?? err)}`);
        })
        .finally(() => {
          if (customPresetInputRef.current) {
            customPresetInputRef.current.value = "";
          }
        });
    },
    [],
  );

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
      const activeCustomRules = normalizeCustomAnalysisRules(customAnalysisRules);

      const res = await engine.analyzeJenniferProfile({
        path: selected.length === 1 ? selected[0].filePath : undefined,
        paths: selected.length > 1 ? selected.map((s) => s.filePath) : undefined,
        fallbackCorrelationToTxid: fallbackToTxid,
        headerBodyToleranceMs: 0,
        networkPrepPatterns: prepCombined.length > 0 ? prepCombined : undefined,
        eventCategoryPatterns:
          Object.keys(otherCategories).length > 0 ? otherCategories : undefined,
        customAnalysisRules:
          activeCustomRules.length > 0 ? activeCustomRules : undefined,
      });
      const compactedResult = compactJenniferResult(res);
      setResult(compactedResult);
      addWorkspaceResult({
        result: compactedResult,
        title: `jennifer_profile: ${selected[0]?.originalName ?? compactedResult.source_files?.[0] ?? ""}`,
        sourceLabel: selected[0]?.originalName,
      });
      setSelectedGuid("");
      setSelectedSignatureHash("");
      setActiveTab("msa");
      // Jennifer is multi-file by design — the recent entry stores
      // the first file as the keyed path and stashes the full list +
      // analyzer toggles in meta so a one-click reload re-creates the
      // exact selection.
      const firstPath = selected[0]?.filePath;
      if (firstPath) {
        recent.push({
          path: firstPath,
          analyzer: "jennifer",
          name: selected.length > 1
            ? `${selected[0].originalName} +${selected.length - 1}`
            : selected[0].originalName,
          meta: {
            paths: selected.map((s) => s.filePath),
            originalNames: selected.map((s) => s.originalName),
            fallbackToTxid,
            networkPrepPatterns: prepCombined,
            eventCategoryPatterns: otherCategories,
            customAnalysisRules: activeCustomRules,
          },
        });
      }
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
  const serviceNetworkRows: any[] =
    result?.series?.service_call_network_summary ?? [];
  const networkPrepRows: any[] = result?.tables?.network_prep_methods ?? [];
  const unprofiledExternalCallRows: any[] =
    result?.tables?.unprofiled_external_call_groups ?? [];
  const slowSqlRows: any[] = result?.tables?.slow_sql_events ?? [];
  const customRuleRows: any[] = result?.tables?.custom_rule_stats ?? [];
  const signatureStats: any[] = result?.series?.signature_statistics ?? [];
  const hasResult = Boolean(result);
  const showMsaTablesInSummary = false;

  // topologyEdges flattens every group's call_graph into one
  // caller→callee list — these are the matched edges with timing
  // metadata, used by the topology graph and the overall timeline.
  const topologyEdges: any[] = useMemo(
    () => guidGroups.flatMap((g) => g?.call_graph ?? []),
    [guidGroups],
  );

  const networkTimeByTxid = useMemo(
    () => buildNetworkTimeByTxid(guidGroups),
    [guidGroups],
  );

  // overallTimelineEdges keeps the unmatched edges too (with caller-
  // side timing) so the MSA timeline can show "this call had no
  // matching callee profile" alongside healthy ones.
  const overallTimelineEdges: any[] = useMemo(
    () =>
      uniqueTimelineEdges([
        ...topologyEdges,
        ...msaEdges.filter((e) => e?.match_status && e.match_status !== "MATCHED"),
      ]),
    [topologyEdges, msaEdges],
  );

  const selectedFileDockItems: FileDockSelection[] = useMemo(
    () =>
      selected.map((file) => ({
        ...file,
        size: 0,
      })),
    [selected],
  );

  const groupsWithCallGraph: any[] = useMemo(
    () => guidGroups.filter((g) => (g?.call_graph ?? []).length > 0),
    [guidGroups],
  );

  const selectedGuidGroup = useMemo(() => {
    const candidates =
      groupsWithCallGraph.length > 0 ? groupsWithCallGraph : guidGroups;
    if (candidates.length === 0) return undefined;
    return (
      candidates.find((g) => g?.guid === selectedGuid) ?? candidates[0]
    );
  }, [groupsWithCallGraph, guidGroups, selectedGuid]);

  const singleGroupEdges: any[] = useMemo(() => {
    if (!selectedGuidGroup?.guid) return [];
    return msaEdges.filter((edge) => edge?.guid === selectedGuidGroup.guid);
  }, [msaEdges, selectedGuidGroup]);

  const singleTopologyEdges: any[] = useMemo(
    () => selectedGuidGroup?.call_graph ?? [],
    [selectedGuidGroup],
  );

  const singleTimelineEdges: any[] = useMemo(
    () =>
      uniqueTimelineEdges([
        ...singleTopologyEdges,
        ...singleGroupEdges.filter(
          (edge) => edge?.match_status && edge.match_status !== "MATCHED",
        ),
      ]),
    [singleGroupEdges, singleTopologyEdges],
  );

  const singleProfileRows: any[] = useMemo(() => {
    if (!selectedGuidGroup?.guid) return [];
    const txids = new Set((selectedGuidGroup?.profile_txids ?? []) as string[]);
    return profileRows.filter(
      (row) => row?.guid === selectedGuidGroup.guid || txids.has(row?.txid),
    );
  }, [profileRows, selectedGuidGroup]);

  const singleSlowSqlRows: any[] = useMemo(() => {
    const txids = txidSetFromProfiles(singleProfileRows);
    return slowSqlRows.filter((row) => txids.has(String(row?.txid ?? "")));
  }, [singleProfileRows, slowSqlRows]);

  const selectedSignature = useMemo(() => {
    if (signatureStats.length === 0) return undefined;
    return (
      signatureStats.find(
        (signature) => signature?.signature_hash === selectedSignatureHash,
      ) ?? signatureStats[0]
    );
  }, [selectedSignatureHash, signatureStats]);

  const selectedSignatureGuidSet = useMemo(
    () => new Set((selectedSignature?.guids ?? []) as string[]),
    [selectedSignature],
  );

  const signatureGroups: any[] = useMemo(
    () => guidGroups.filter((g) => selectedSignatureGuidSet.has(g?.guid)),
    [guidGroups, selectedSignatureGuidSet],
  );

  const signatureTopologyEdges: any[] = useMemo(
    () => signatureGroups.flatMap((g) => g?.call_graph ?? []),
    [signatureGroups],
  );

  const signatureMsaEdges: any[] = useMemo(
    () => msaEdges.filter((edge) => selectedSignatureGuidSet.has(edge?.guid)),
    [msaEdges, selectedSignatureGuidSet],
  );

  const signatureProfileRows: any[] = useMemo(
    () => profileRows.filter((row) => selectedSignatureGuidSet.has(row?.guid)),
    [profileRows, selectedSignatureGuidSet],
  );

  const signatureSlowSqlRows: any[] = useMemo(() => {
    const txids = txidSetFromProfiles(signatureProfileRows);
    return slowSqlRows.filter((row) => txids.has(String(row?.txid ?? "")));
  }, [signatureProfileRows, slowSqlRows]);

  const averageTimelineEdges: any[] = useMemo(
    () => uniqueTimelineEdges(buildAverageTimelineEdges(selectedSignature)),
    [selectedSignature],
  );

  const signatureTimelineEdges: any[] = useMemo(
    () =>
      uniqueTimelineEdges([
        ...signatureTopologyEdges,
        ...signatureMsaEdges.filter(
          (edge) => edge?.match_status && edge.match_status !== "MATCHED",
        ),
      ]),
    [signatureMsaEdges, signatureTopologyEdges],
  );

  const emptyResultCard = (
    <Card>
      <CardContent className="p-4 text-xs text-muted-foreground">
        분석 결과가 없습니다.
      </CardContent>
    </Card>
  );

  const handleRecentSelect = (entry: {
    path: string;
    name?: string;
    meta?: Record<string, unknown>;
  }) => {
    const meta = entry.meta ?? {};
    // Jennifer entries can carry an entire batch — restore the full
    // path list when present, fall back to the keyed single path.
    const paths = Array.isArray(meta.paths)
      ? (meta.paths as string[])
      : [entry.path];
    const names = Array.isArray(meta.originalNames)
      ? (meta.originalNames as string[])
      : paths.map((p) => p.split(/[\\/]/).pop() ?? p);
    setSelected(
      paths.map((p, idx) => ({
        filePath: p,
        originalName: names[idx] ?? p,
      })),
    );
    if (typeof meta.fallbackToTxid === "boolean") {
      setFallbackToTxid(meta.fallbackToTxid);
    }
    if (
      meta.eventCategoryPatterns &&
      typeof meta.eventCategoryPatterns === "object"
    ) {
      setEventCategoryPatterns(
        meta.eventCategoryPatterns as CategoryRules,
      );
    }
    if (Array.isArray(meta.customAnalysisRules)) {
      setCustomAnalysisRules(
        normalizeCustomAnalysisRules(meta.customAnalysisRules),
      );
    }
    setError("");
  };

  return (
    <main className="flex flex-col gap-5 p-5 overflow-y-auto">
      <div className="flex items-stretch gap-3">
        <WailsFileDock
          className="min-w-0 flex-1"
          label={t("dropOrBrowseJennifer")}
          description="Jennifer profile export 파일 또는 같은 트랜잭션 묶음 파일을 선택하세요."
          accept=".txt,.log,.profile"
          selectedFiles={selectedFileDockItems}
          onSelect={handleFileSelected}
          onSelectMany={handleFilesSelected}
          onClear={handleClearAll}
          allowsMultipleSelection
          browseLabel={t("browseFile")}
          dropHereLabel={t("dropHere")}
          errorLabel={t("error")}
          fileFilters={FILE_FILTERS}
          rightSlot={
            <div className="flex flex-wrap items-center gap-2">
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
            </div>
          }
        />
        <AnalyzerOptionsDock
          title="Jennifer 분석 옵션"
          label={t("analyzerOptions")}
          width={560}
          footer={
            <div className="flex justify-end">
              <Button
                type="button"
                size="sm"
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
            </div>
          }
        >
          <div className="flex flex-col gap-4 text-sm">
          <label className="flex items-center gap-1.5 text-xs">
            <input
              type="checkbox"
              checked={fallbackToTxid}
              onChange={(e) => setFallbackToTxid(e.target.checked)}
            />
            fallback correlation = TXID
            <span className="text-muted-foreground">
              (기본: 꺼짐 — GUID 누락 시에만 켜세요)
            </span>
          </label>
          <div>
            <p className="mb-2 text-xs font-semibold text-foreground/80">
              MSA 이벤트 분류 패턴 (METHOD/UNKNOWN 라인 추가 분류)
            </p>
            <p className="mb-2 text-[11px] text-muted-foreground">
              기본값: NETWORK_PREP_METHOD = "IntegrationUtil.sendToService".
              나머지 카테고리는 비어있음 — 빌트인 분류만 사용.
            </p>
            <CustomCategoriesEditor
              segments={MSA_EVENT_SEGMENTS}
              presets={MSA_EVENT_PRESETS}
              value={eventCategoryPatterns}
              onChange={setEventCategoryPatterns}
              disabled={analyzing}
            />
          </div>
          <CustomAnalysisRulesEditor
            rules={customAnalysisRules}
            onChange={setCustomAnalysisRules}
            disabled={analyzing}
            onSavePreset={handleSaveAnalysisPreset}
            onLoadPresetClick={() => customPresetInputRef.current?.click()}
            presetInputRef={customPresetInputRef}
            onLoadPresetFile={handleLoadAnalysisPresetFile}
          />
          </div>
        </AnalyzerOptionsDock>
      </div>
      <RecentFilesPanel
        entries={recent.entries}
        onSelect={handleRecentSelect}
        onRemove={recent.remove}
        onClear={recent.clear}
      />
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

      {error && (
        <div
          role="alert"
          className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
        >
          <strong className="block">{t("error") || "Error"}</strong>
          <p className="mt-1 text-foreground">{error}</p>
        </div>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <TabsList>
            <TabsTrigger value="summary">요약</TabsTrigger>
            <TabsTrigger value="msa">MSA</TabsTrigger>
            <TabsTrigger value="profiles">부가/검증</TabsTrigger>
            <TabsTrigger value="parser">파서 보고서</TabsTrigger>
          </TabsList>

          <TabsContent value="summary" className="mt-4 flex flex-col gap-4">
          {!hasResult ? (
            emptyResultCard
          ) : (
          <>
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
              label="Incomplete profiles"
              value={(summary.incomplete_profile_count ?? 0).toLocaleString()}
            />
            <MetricCard
              label="Avg excluded groups"
              value={(summary.signature_excluded_group_count ?? 0).toLocaleString()}
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
              label="미수집 외부호출 (cum ms)"
              value={(summary.total_unprofiled_external_call_ms ?? 0).toLocaleString()}
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

          {selectedGuidGroup && (
            <GuidTransactionSummary group={selectedGuidGroup} />
          )}

          <MsaCaptureStatusCard summary={summary} />

          <IncompleteProfileWarning summary={summary} />

          <CustomRuleStatsPanel rows={customRuleRows} />

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
                      <th className="px-4 py-2 text-right font-medium">Incomplete</th>
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
                        <td className="px-4 py-2 text-right tabular-nums">
                          {(row.incomplete_profile_count ?? 0).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}

          {showMsaTablesInSummary && signatureStats.length > 0 && (
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

          {showMsaTablesInSummary &&
            signatureStats.some((s) => (s.edges ?? []).length > 0) && (
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

          {showMsaTablesInSummary && guidGroups.length > 0 && (
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

          {showMsaTablesInSummary && msaEdges.length > 0 && (
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

          </>
          )}
          </TabsContent>

          <TabsContent value="msa" className="mt-4 flex flex-col gap-4">
            {!hasResult ? (
              emptyResultCard
            ) : (
            <>
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">MSA 타임라인 분석 모드</CardTitle>
                <p className="text-xs text-muted-foreground">
                  단일 트랜잭션은 GUID 하나의 실제 호출 흐름을, 평균
                  트랜잭션은 같은 signature로 묶인 여러 GUID의 평균 호출
                  흐름을 표시합니다.
                </p>
              </CardHeader>
              <CardContent className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant={msaTimelineMode === "single" ? "default" : "outline"}
                    onClick={() => setMsaTimelineMode("single")}
                  >
                    <Network className="h-3.5 w-3.5" />
                    단일 트랜잭션
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant={msaTimelineMode === "average" ? "default" : "outline"}
                    onClick={() => setMsaTimelineMode("average")}
                  >
                    <Rows3 className="h-3.5 w-3.5" />
                    평균 트랜잭션
                  </Button>
                </div>
                {msaTimelineMode === "single" ? (
                  <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                    <span className="font-medium text-foreground/80">
                      분석할 트랜잭션
                    </span>
                    <select
                      className="h-9 rounded-md border border-input bg-background px-3 text-xs"
                      value={selectedGuidGroup?.guid ?? ""}
                      onChange={(event) => setSelectedGuid(event.target.value)}
                    >
                      {(groupsWithCallGraph.length > 0
                        ? groupsWithCallGraph
                        : guidGroups
                      ).map((group) => (
                        <option key={group?.guid} value={group?.guid ?? ""}>
                          {guidOptionLabel(group)}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : (
                  <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                    <span className="font-medium text-foreground/80">
                      평균을 낼 signature
                    </span>
                    <select
                      className="h-9 rounded-md border border-input bg-background px-3 text-xs"
                      value={selectedSignature?.signature_hash ?? ""}
                      onChange={(event) =>
                        setSelectedSignatureHash(event.target.value)
                      }
                    >
                      {signatureStats.map((signature) => (
                        <option
                          key={signature?.signature_hash}
                          value={signature?.signature_hash ?? ""}
                        >
                          {signatureOptionLabel(signature)}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
              </CardContent>
            </Card>

            {msaTimelineMode === "single" ? (
              selectedGuidGroup ? (
                <>
                  <MsaResponseTimeBreakdown
                    groups={[selectedGuidGroup] as any}
                    edges={singleTimelineEdges as any}
                  />
                  <MsaTopology
                    title="단일 트랜잭션 호출 토폴로지"
                    edges={singleTopologyEdges as any}
                    rootApplication={selectedGuidGroup?.root_application}
                  />
                  <TransactionProfilesTable
                    rows={singleProfileRows}
                    networkTimeByTxid={networkTimeByTxid}
                  />
                  <MsaTimeline
                    title={
                      singleTimelineLayout === "grouped"
                        ? "단일 트랜잭션 타임라인 (서비스 그룹)"
                        : "단일 트랜잭션 타임라인 (개별 호출)"
                    }
                    edges={singleTimelineEdges as any}
                    mode={
                      singleTimelineLayout === "grouped"
                        ? "by-service"
                        : "overall"
                    }
                    layoutToggle={{
                      value: singleTimelineLayout,
                      onChange: setSingleTimelineLayout,
                    }}
                    rootApplication={selectedGuidGroup?.root_application}
                    rootDurationMs={toFiniteNumber(
                      selectedGuidGroup?.root_response_time_ms,
                    )}
                    rootStartMs={toFiniteNumber(
                      selectedGuidGroup?.root_body_start_ms,
                    )}
                  />
                  <MsaTimelineTreemap
                    title="단일 트랜잭션 트리맵 (elapsed 합계)"
                    edges={singleTimelineEdges as any}
                    rootApplication={selectedGuidGroup?.root_application}
                  />
                  <SlowSqlTable rows={singleSlowSqlRows} />
                </>
              ) : (
                <Card>
                  <CardContent className="p-4 text-xs text-muted-foreground">
                    분석할 트랜잭션 프로파일이 없습니다. 원본 프로파일의 TXID,
                    RESPONSE_TIME 또는 본문 이벤트를 확인하세요.
                  </CardContent>
                </Card>
              )
            ) : selectedSignature ? (
              <>
                <MsaResponseTimeBreakdown
                  groups={signatureGroups as any}
                  edges={signatureTimelineEdges as any}
                />
                <MsaTopology
                  title="평균 트랜잭션 호출 토폴로지"
                  edges={averageTimelineEdges as any}
                  rootApplication={selectedSignature?.root_application}
                />
                <TransactionProfilesTable
                  rows={signatureProfileRows}
                  networkTimeByTxid={networkTimeByTxid}
                />
                <MsaTimeline
                  title="평균 트랜잭션 타임라인 (서비스 그룹)"
                  edges={averageTimelineEdges as any}
                  mode="by-service"
                  rootApplication={selectedSignature?.root_application}
                  rootDurationMs={statValue(
                    selectedSignature?.metrics?.root_response_time_ms,
                    "avg",
                  )}
                />
                <MsaTimelineTreemap
                  title="평균 트랜잭션 트리맵 (avg elapsed 합계)"
                  edges={averageTimelineEdges as any}
                  rootApplication={selectedSignature?.root_application}
                />
                <SlowSqlTable rows={signatureSlowSqlRows} />
              </>
            ) : (
              <Card>
                <CardContent className="p-4 text-xs text-muted-foreground">
                  같은 호출 구조로 묶을 signature 통계가 없습니다. 같은 MSA
                  트랜잭션 파일 여러 개를 함께 분석하면 평균 모드를 사용할 수
                  있습니다.
                </CardContent>
              </Card>
            )}
            </>
            )}
          </TabsContent>

          <TabsContent value="profiles" className="mt-4 flex flex-col gap-4">
            {!hasResult ? (
              emptyResultCard
            ) : (
            <>
            <ServiceNetworkTimeSummary rows={serviceNetworkRows} />
            <CustomRuleStatsPanel rows={customRuleRows} />
            <ExternalCallTopTable edges={overallTimelineEdges as any} />
            <NetworkPrepMethodsTable rows={networkPrepRows} />
            <UnprofiledExternalCallGroupsTable rows={unprofiledExternalCallRows} />
            </>
            )}
          </TabsContent>

          <TabsContent value="parser" className="mt-4">
            {!hasResult ? (
              emptyResultCard
            ) : (
            <>
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
            </>
            )}
          </TabsContent>
      </Tabs>
    </main>
  );
}
