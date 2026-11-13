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
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
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
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";

import { engine } from "../bridge/engine";
import { Dialogs } from "@/bridge/dialogs";
import type { AnalysisResult } from "../bridge/types";
import { readJenniferDrilldown, isCurrentJenniferProjection, type MsaDrilldownScope, type JenniferDrilldownSelection } from "../state/jenniferDrilldown";

import {
  CustomCategoriesEditor,
  type CategoryRules,
  type Preset,
  type SegmentSpec,
} from "../components/CustomCategoriesEditor";
import { MsaMethodHotspots } from "../components/MsaMethodHotspots";
import { MsaResponseTimeBreakdown } from "../components/MsaResponseTimeBreakdown";
import { MsaTimeline, MsaTimelineTreemap } from "../components/MsaTimeline";
import { MsaTopology } from "../components/MsaTopology";
import { AnalyzerOptionsDock } from "../components/AnalyzerOptionsDock";
import { HelpTip, HelpedLabel } from "../components/HelpTip";
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
import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "../i18n/I18nProvider";
import { addWorkspaceResult } from "../state/analysisWorkspace";

// MSA event-category segments — these match JenniferEventType values
// that the backend classifier accepts via EventCategoryPatterns.
const MSA_EVENT_SEGMENTS: SegmentSpec[] = [
  { id: "EXTERNAL_CALL", label: "EXTERNAL_CALL" },
  { id: "NETWORK_PREP_METHOD", label: "Network prep (sendToService 등)" },
  { id: "SERVLET_DISPATCH_METHOD", label: "Servlet dispatch (service 등)" },
  { id: "TWO_PC_UNKNOWN", label: "2PC / XA" },
  { id: "CHECK_QUERY", label: "CHECK_QUERY" },
  { id: "CONNECTION_ACQUIRE", label: "Connection acquire" },
  { id: "FETCH", label: "FETCH" },
];

const MSA_EVENT_PRESETS: Preset[] = [
  {
    id: "msa-servlet-dispatch",
    label: "MSA — Servlet dispatch (HttpServlet.service)",
    rules: {
      SERVLET_DISPATCH_METHOD: [
        "jakarta.servlet.http.HttpServlet.service",
        "javax.servlet.http.HttpServlet.service",
      ],
    },
  },
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

const MSA_TABLE_PREVIEW_LIMIT = 10;
const DEFAULT_SLOW_SQL_THRESHOLD_MS = 1000;
const CUSTOM_RULE_PRESET_VERSION = 1;

type Selection = {
  filePath: string;
  originalName: string;
};

type MsaTimelineMode = "single" | "average";
type MsaTimelineLayout = "grouped" | "individual";
type ApiCallValueMode = "sum" | "avg";
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

type ApiCallSortKey =
  | "apiUrl"
  | "target"
  | "count"
  | "apiTotalMs"
  | "apiAvgMs"
  | "apiP95Ms"
  | "apiMaxMs"
  | "calleeAvgMs"
  | "calleeTotalMs"
  | "networkTotalMs"
  | "networkP95Ms"
  | "networkMaxMs";

type SortDirection = "asc" | "desc";

type ApiCallAggregate = {
  apiUrl: string;
  target: string;
  callers: string;
  callees: string;
  count: number;
  matchedCount: number;
  unmatchedCount: number;
  apiTotalMs: number;
  apiAvgMs: number;
  apiP95Ms: number;
  apiMaxMs: number;
  calleeAvgMs: number;
  calleeTotalMs: number;
  networkTotalMs: number;
  networkAvgMs: number;
  networkP95Ms: number;
  networkMaxMs: number;
};

type MsaDrilldownOption = {
  value: string;
  label: string;
  txid?: string;
  application?: string;
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
        external_call_url:
          edge?.external_call_url ??
          edge?.external_call_target ??
          edge?.callee_application ??
          "?",
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
        call_count: Math.max(
          1,
          Math.round(statValue(edge?.external_call_elapsed_ms, "count") ?? 1),
        ),
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
      const guid = String(group?.guid ?? "");
      const key = `${guid}\u0000${txid}`;
      out.set(key, (out.get(key) ?? 0) + Number(edge?.network_gap_ms ?? 0));
      if (guid.startsWith("standalone:") || guid === txid) out.set(`\u0000${txid}`, out.get(key)!);
    }
  }
  return out;
}

function txidSetFromProfiles(rows: any[]): Set<string> {
  return new Set(rows.map((row) => String(row?.txid ?? "")).filter(Boolean));
}

function txidOf(value: unknown): string {
  return String(value ?? "").trim();
}

function profileResponseTime(row: any): number {
  return Math.max(0, Math.round(toFiniteNumber(row?.header?.response_time_ms) ?? 0));
}

function usePreviewRows<T>(rows: T[], limit = MSA_TABLE_PREVIEW_LIMIT) {
  const [expanded, setExpanded] = useState(false);
  const visibleRows = expanded ? rows : rows.slice(0, limit);
  return {
    expanded,
    setExpanded,
    visibleRows,
  };
}

function PreviewRowsToggle({
  total,
  visibleCount,
  expanded,
  onToggle,
}: {
  total: number;
  visibleCount: number;
  expanded: boolean;
  onToggle: () => void;
}): React.JSX.Element | null {
  if (total <= MSA_TABLE_PREVIEW_LIMIT) return null;
  const hiddenCount = Math.max(0, total - visibleCount);
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border px-3 py-2 text-xs text-muted-foreground">
      <span>
        {expanded
          ? `전체 ${total.toLocaleString()}건을 표시 중입니다.`
          : `상위 ${visibleCount.toLocaleString()}건만 표시합니다. 숨김 ${hiddenCount.toLocaleString()}건.`}
      </span>
      <Button type="button" size="sm" variant="outline" onClick={onToggle}>
        {expanded ? "접기" : `더 보기 (${hiddenCount.toLocaleString()}건)`}
      </Button>
    </div>
  );
}

function msaDrilldownOptionLabel(row: any): string {
  const app = String(row?.application || "unknown");
  const txid = txidOf(row?.txid);
  const response = profileResponseTime(row);
  const suffix = txid ? `TXID …${txid.slice(-10)}` : "TXID 없음";
  return response > 0
    ? `${app} · ${suffix} · ${response.toLocaleString()} ms`
    : `${app} · ${suffix}`;
}

function buildMsaDrilldownOptions(group: any, rows: any[]): MsaDrilldownOption[] {
  if (!group) return [];
  const options: MsaDrilldownOption[] = [
    {
      value: "__whole__",
      label: "전체 트랜잭션 기준",
    },
  ];
  const txidOrder = new Set(
    ((group?.profile_txids ?? []) as string[]).map((txid) => txidOf(txid)),
  );
  const candidates = rows
    .filter((row) => txidOrder.has(txidOf(row?.txid)))
    .sort((a, b) => {
      const ar = txidOf(a?.txid) === txidOf(group?.root_txid) ? -1 : 0;
      const br = txidOf(b?.txid) === txidOf(group?.root_txid) ? -1 : 0;
      if (ar !== br) return ar - br;
      return profileResponseTime(b) - profileResponseTime(a);
    });
  const seen = new Set<string>();
  for (const row of candidates) {
    const txid = txidOf(row?.txid);
    if (!txid || seen.has(txid)) continue;
    seen.add(txid);
    options.push({
      value: txid,
      txid,
      label: msaDrilldownOptionLabel(row),
    });
  }
  return options;
}

function buildMsaAverageDrilldownOptions(rows: any[]): MsaDrilldownOption[] {
  const options: MsaDrilldownOption[] = [
    {
      value: "__whole__",
      label: "전체 평균 트랜잭션 기준",
    },
  ];
  const byApplication = new Map<
    string,
    {
      application: string;
      count: number;
      responseTotal: number;
      maxResponse: number;
    }
  >();
  for (const row of rows) {
    const application = String(row?.application ?? "").trim();
    if (!application) continue;
    const response = profileResponseTime(row);
    const current =
      byApplication.get(application) ?? {
        application,
        count: 0,
        responseTotal: 0,
        maxResponse: 0,
      };
    current.count += 1;
    current.responseTotal += response;
    current.maxResponse = Math.max(current.maxResponse, response);
    byApplication.set(application, current);
  }
  return [
    ...options,
    ...Array.from(byApplication.values())
      .sort((a, b) => {
        if (a.maxResponse !== b.maxResponse) return b.maxResponse - a.maxResponse;
        return a.application.localeCompare(b.application);
      })
      .map((entry) => {
        const avg = entry.count > 0 ? entry.responseTotal / entry.count : 0;
        return {
          value: entry.application,
          application: entry.application,
          label:
            avg > 0
              ? `${entry.application} · ${entry.count.toLocaleString()}건 · avg ${Math.round(avg).toLocaleString()} ms`
              : `${entry.application} · ${entry.count.toLocaleString()}건`,
        };
      }),
  ];
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

function GuidTransactionSummary({ group }: { group: any }): React.JSX.Element {
  const { locale } = useI18n();
  const metrics = group?.metrics ?? {};
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          단일 트랜잭션 기준값
          <HelpTip text={getHelpText(locale, "sectionMsaSingleBaseline")} />
        </CardTitle>
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

function MsaCaptureStatusCard({ summary }: { summary: any }): React.JSX.Element {
  const { locale } = useI18n();
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          MSA 캡처 상태 (체크쿼리 / 2PC / 외부호출 / 네트워크 준비)
          <HelpTip text={getHelpText(locale, "sectionMsaCaptureStatus")} />
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

function IncompleteProfileWarning({ summary }: { summary: any }): React.JSX.Element | null {
  const { locale } = useI18n();
  const incomplete = Number(summary?.incomplete_profile_count ?? 0);
  const excludedGroups = Number(summary?.signature_excluded_group_count ?? 0);
  if (incomplete <= 0 && excludedGroups <= 0) return null;
  return (
    <Card className="border-amber-500/50 bg-amber-500/5">
      <CardHeader className="pb-2">
        <CardTitle className="inline-flex items-center gap-2 text-sm text-amber-800 dark:text-amber-200">
          불완전 프로파일 감지
          <HelpTip text={getHelpText(locale, "sectionMsaIncomplete")} variant="warning" />
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
}): React.JSX.Element {
  const { locale } = useI18n();
  const { visibleRows, expanded, setExpanded } = usePreviewRows(rows);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          Transaction profiles ({rows.length})
          <HelpTip text={getHelpText(locale, "sectionMsaTransactions")} />
        </CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
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
              const networkMs = networkTimeByTxid.get(`${String(row.guid ?? "")}\u0000${String(row.txid ?? "")}`) ?? 0;
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
        <PreviewRowsToggle
          total={rows.length}
          visibleCount={visibleRows.length}
          expanded={expanded}
          onToggle={() => setExpanded((value) => !value)}
        />
      </CardContent>
    </Card>
  );
}

function NetworkPrepMethodsTable({ rows }: { rows: any[] }): React.JSX.Element {
  const { locale } = useI18n();
  const { visibleRows, expanded, setExpanded } = usePreviewRows(rows);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          네트워크 준비시간 포함 메소드 ({rows.length})
          <HelpTip text={getHelpText(locale, "sectionMsaNetworkPrep")} />
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
            <PreviewRowsToggle
              total={rows.length}
              visibleCount={visibleRows.length}
              expanded={expanded}
              onToggle={() => setExpanded((value) => !value)}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

function UnprofiledExternalCallGroupsTable({ rows }: { rows: any[] }): React.JSX.Element {
  const { locale } = useI18n();
  const { visibleRows, expanded, setExpanded } = usePreviewRows(rows);
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          프로파일 미수집 외부호출 ({rows.length})
          <HelpTip text={getHelpText(locale, "sectionMsaUnprofiledExternal")} />
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
            <PreviewRowsToggle
              total={rows.length}
              visibleCount={visibleRows.length}
              expanded={expanded}
              onToggle={() => setExpanded((value) => !value)}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

function ApiCallAnalysisPanel({
  edges,
  scopeLabel,
  valueMode = "sum",
  sampleCount = 1,
}: {
  edges: any[];
  scopeLabel?: string;
  valueMode?: ApiCallValueMode;
  sampleCount?: number;
}): React.JSX.Element {
  const { locale } = useI18n();
  const [sortKey, setSortKey] = useState<ApiCallSortKey>("apiTotalMs");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const normalizedSampleCount = Math.max(1, Math.round(sampleCount || 1));
  const effectiveValueMode: ApiCallValueMode =
    valueMode === "avg" && normalizedSampleCount > 1 ? "avg" : "sum";
  const rows = useMemo(
    () =>
      aggregateApiCalls(edges, {
        valueMode: effectiveValueMode,
        sampleCount: normalizedSampleCount,
      }),
    [edges, effectiveValueMode, normalizedSampleCount],
  );
  const sortedRows = useMemo(
    () => sortApiCallRows(rows, sortKey, sortDirection),
    [rows, sortDirection, sortKey],
  );
  const { visibleRows, expanded, setExpanded } = usePreviewRows(sortedRows);
  const slowest = rows.reduce<ApiCallAggregate | undefined>(
    (best, row) => (!best || row.apiMaxMs > best.apiMaxMs ? row : best),
    undefined,
  );
  const mostCalled = rows.reduce<ApiCallAggregate | undefined>(
    (best, row) => (!best || row.count > best.count ? row : best),
    undefined,
  );
  const highestTotal = rows.reduce<ApiCallAggregate | undefined>(
    (best, row) => (!best || row.apiTotalMs > best.apiTotalMs ? row : best),
    undefined,
  );
  const highestP95 = rows.reduce<ApiCallAggregate | undefined>(
    (best, row) => (!best || row.apiP95Ms > best.apiP95Ms ? row : best),
    undefined,
  );
  const isAverageView = effectiveValueMode === "avg";
  const countUnit = isAverageView ? "calls/tx" : "calls";

  const updateSort = (nextKey: ApiCallSortKey) => {
    if (nextKey === sortKey) {
      setSortDirection((current) => (current === "desc" ? "asc" : "desc"));
      return;
    }
    setSortKey(nextKey);
    setSortDirection(nextKey === "apiUrl" || nextKey === "target" ? "asc" : "desc");
  };

  const sortHeader = (
    label: string,
    key: ApiCallSortKey,
    align: "left" | "right" = "left",
  ) => {
    const Icon =
      sortKey === key
        ? sortDirection === "asc"
          ? ArrowUp
          : ArrowDown
        : ArrowUpDown;
    return (
      <button
        type="button"
        className={`inline-flex w-full items-center gap-1 hover:text-foreground ${
          align === "right" ? "justify-end" : "justify-start"
        }`}
        onClick={() => updateSort(key)}
      >
        <span>{label}</span>
        <Icon className="h-3 w-3" aria-hidden="true" />
      </button>
    );
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          External Call 응답시간 분석 ({rows.length})
          <HelpTip text={getHelpText(locale, "sectionMsaExternalTop")} />
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          EXTERNAL_CALL 원본 URL별로 묶어 External_Call(callee 응답시간)·API(호출자
          측정 시간)·네트워크 시간을 함께 집계합니다.
        </p>
        {scopeLabel ? (
          <div className="mt-2 inline-flex w-fit rounded-md border border-border bg-muted/20 px-2 py-1 text-[11px] text-muted-foreground">
            {scopeLabel}
            {isAverageView
              ? ` · 평균 트랜잭션 기준 · n=${normalizedSampleCount.toLocaleString()}`
              : " · 단일/선택 범위 기준"}
          </div>
        ) : null}
      </CardHeader>
      <CardContent className="p-0">
        {rows.length === 0 ? (
          <p className="px-4 py-3 text-xs text-muted-foreground">
            표시할 API 호출이 없습니다.
          </p>
        ) : (
          <>
            <div className="grid gap-3 border-b border-border p-3 sm:grid-cols-2 xl:grid-cols-4">
              <ApiCallHighlight
                label="가장 느린 API"
                row={slowest}
                value={formatMsValue(slowest?.apiMaxMs)}
              />
              <ApiCallHighlight
                label="최다 호출 API"
                row={mostCalled}
                value={
                  mostCalled
                    ? `${formatCountValue(mostCalled.count)} ${countUnit}`
                    : "—"
                }
              />
              <ApiCallHighlight
                label="누적 수행시간 상위"
                row={highestTotal}
                value={formatMsValue(highestTotal?.apiTotalMs)}
              />
              <ApiCallHighlight
                label="p95 응답시간 상위"
                row={highestP95}
                value={formatMsValue(highestP95?.apiP95Ms)}
              />
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {sortHeader("API URL", "apiUrl")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {sortHeader("Target", "target")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader(isAverageView ? "Calls / tx" : "Calls", "count", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader("External_Call avg", "calleeAvgMs", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader(
                        isAverageView ? "External_Call Sum / tx" : "External_Call Sum",
                        "calleeTotalMs",
                        "right",
                      )}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader("API avg", "apiAvgMs", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader(isAverageView ? "API cum / tx" : "API cum", "apiTotalMs", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader("API p95", "apiP95Ms", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader("API max", "apiMaxMs", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader(
                        isAverageView ? "Network cum / tx" : "Network cum",
                        "networkTotalMs",
                        "right",
                      )}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader("Network p95", "networkP95Ms", "right")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {sortHeader("Network max", "networkMaxMs", "right")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {visibleRows.map((row, idx) => (
                    <tr key={`${row.apiUrl}-${idx}`} className="border-b border-border last:border-0">
                      <td
                        className="max-w-[420px] px-3 py-2 font-mono text-xs"
                        title={row.apiUrl}
                      >
                        <span className="block truncate">{row.apiUrl}</span>
                      </td>
                      <td
                        className="max-w-[260px] px-3 py-2 font-mono text-xs"
                        title={`${row.target}\nCaller: ${row.callers}\nCallee: ${row.callees}`}
                      >
                        <span className="block truncate">{row.target || "—"}</span>
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {formatCountValue(row.count)}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {row.calleeAvgMs > 0
                          ? Math.round(row.calleeAvgMs).toLocaleString()
                          : "—"}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {row.calleeTotalMs > 0
                          ? Math.round(row.calleeTotalMs).toLocaleString()
                          : "—"}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.apiAvgMs).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.apiTotalMs).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.apiP95Ms).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.apiMaxMs).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.networkTotalMs).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.networkP95Ms).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        {Math.round(row.networkMaxMs).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <PreviewRowsToggle
              total={sortedRows.length}
              visibleCount={visibleRows.length}
              expanded={expanded}
              onToggle={() => setExpanded((value) => !value)}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

function ApiCallHighlight({
  label,
  row,
  value,
}: {
  label: string;
  row?: ApiCallAggregate;
  value: string;
}): React.JSX.Element {
  return (
    <div className="min-w-0 rounded-lg border border-border bg-card p-3">
      <div className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="mt-1 truncate font-mono text-xs" title={row?.apiUrl}>
        {row?.apiUrl ?? "—"}
      </div>
      <div className="mt-1 text-sm font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function SlowSqlTable({ rows }: { rows: any[] }): React.JSX.Element {
  const { locale } = useI18n();
  const [minElapsedInput, setMinElapsedInput] = useState(
    String(DEFAULT_SLOW_SQL_THRESHOLD_MS),
  );
  const minElapsedMs = Math.max(0, Number(minElapsedInput) || 0);
  const filteredRows = rows.filter(
    (row) => Number(row?.elapsed_ms ?? 0) >= minElapsedMs,
  );
  const sortedRows = [...filteredRows].sort(
    (a, b) => Number(b?.elapsed_ms ?? 0) - Number(a?.elapsed_ms ?? 0),
  );
  const { visibleRows, expanded, setExpanded } = usePreviewRows(sortedRows);
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <CardTitle className="inline-flex items-center gap-2 text-sm">
              성능 저하 SQL 후보 ({filteredRows.length}/{rows.length})
              <HelpTip text={getHelpText(locale, "sectionMsaSlowSql")} />
            </CardTitle>
            <p className="mt-1 text-xs text-muted-foreground">
              기본값은 1초 이상 수행된 SQL만 표시합니다.
            </p>
          </div>
          <div className="flex flex-wrap items-end gap-2 text-xs">
            <label className="flex flex-col gap-1">
              <HelpedLabel help={getHelpText(locale, "sectionMsaSlowSql")} className="font-medium text-foreground/80">
                최소 응답시간(ms)
              </HelpedLabel>
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
          <>
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
            <PreviewRowsToggle
              total={sortedRows.length}
              visibleCount={visibleRows.length}
              expanded={expanded}
              onToggle={() => setExpanded((value) => !value)}
            />
          </>
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
  presetInputRef: RefObject<HTMLInputElement | null>;
  onLoadPresetFile: (file?: File) => void;
}): React.JSX.Element {
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

function CustomRuleStatsPanel({ rows }: { rows: any[] }): React.JSX.Element | null {
  const { locale } = useI18n();
  const { visibleRows, expanded, setExpanded } = usePreviewRows(rows);
  if (rows.length === 0) return null;
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          사용자 정의 카드 통계 ({rows.length})
          <HelpTip text={getHelpText(locale, "sectionMsaCustomCards")} />
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          분석 옵션에서 추가한 카드별 집계입니다. 프로파일 URL은 전체
          응답시간, 메소드는 내부 DB/External 등 포함 비용을 제외한 처리시간,
          EXTERNAL_CALL은 호출 URL elapsed를 합산합니다.
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
        <PreviewRowsToggle
          total={rows.length}
          visibleCount={visibleRows.length}
          expanded={expanded}
          onToggle={() => setExpanded((value) => !value)}
        />
      </CardContent>
    </Card>
  );
}

function aggregateApiCalls(
  edges: any[],
  options: {
    valueMode?: ApiCallValueMode;
    sampleCount?: number;
  } = {},
): ApiCallAggregate[] {
  type State = {
    apiUrl: string;
    target: string;
    callers: Set<string>;
    callees: Set<string>;
    count: number;
    matchedCount: number;
    unmatchedCount: number;
    apiTotal: number;
    apiMetricCount: number;
    calleeTotal: number;
    calleeMetricCount: number;
    networkTotal: number;
    networkMetricCount: number;
    apiValues: number[];
    calleeValues: number[];
    networkValues: number[];
  };
  const byApi = new Map<string, State>();
  const divisor =
    options.valueMode === "avg"
      ? Math.max(1, Math.round(options.sampleCount || 1))
      : 1;
  for (const edge of edges) {
    const apiUrl = String(
      edge?.external_call_url ??
        edge?.external_call_target ??
        edge?.callee_application ??
        "",
    ).trim();
    if (!apiUrl) continue;
    const target = String(
      edge?.external_call_target ?? edge?.callee_application ?? "",
    ).trim();
    const key = apiUrl;
    const state =
      byApi.get(key) ??
      {
        apiUrl,
        target,
        callers: new Set<string>(),
        callees: new Set<string>(),
        count: 0,
        matchedCount: 0,
        unmatchedCount: 0,
        apiTotal: 0,
        apiMetricCount: 0,
        calleeTotal: 0,
        calleeMetricCount: 0,
        networkTotal: 0,
        networkMetricCount: 0,
        apiValues: [],
        calleeValues: [],
        networkValues: [],
      };
    if (!state.target && target) state.target = target;
    const caller = String(edge?.caller_application ?? "").trim();
    const callee = String(edge?.callee_application ?? "").trim();
    if (caller) state.callers.add(caller);
    if (callee) state.callees.add(callee);
    const metricCount = Math.max(
      1,
      Math.round(
        toFiniteNumber(edge?.call_count) ??
          statValue(edge?.external_call_elapsed_ms, "count") ??
          1,
      ),
    );
    state.count += metricCount;
    if (edge?.match_status === "MATCHED") {
      state.matchedCount += metricCount;
    } else {
      state.unmatchedCount += metricCount;
    }
    const apiElapsed =
      toFiniteNumber(edge?.external_call_elapsed_ms) ??
      statValue(edge?.external_call_elapsed_ms, "avg");
    if (apiElapsed != null) {
      const value = Math.max(0, apiElapsed);
      state.apiTotal +=
        toFiniteNumber(edge?.external_call_elapsed_ms?.total) ??
        value * metricCount;
      state.apiMetricCount += metricCount;
      state.apiValues.push(value);
    }
    const calleeResponse =
      toFiniteNumber(edge?.callee_response_time_ms) ??
      statValue(edge?.callee_response_time_ms, "avg");
    if (calleeResponse != null) {
      const value = Math.max(0, calleeResponse);
      state.calleeTotal +=
        toFiniteNumber(edge?.callee_response_time_ms?.total) ??
        toFiniteNumber(edge?.total_callee_response_time_ms) ??
        value * metricCount;
      state.calleeMetricCount += metricCount;
      state.calleeValues.push(value);
    }
    const network =
      toFiniteNumber(edge?.adjusted_network_gap_ms) ??
      toFiniteNumber(edge?.network_gap_ms) ??
      toFiniteNumber(edge?.raw_network_gap_ms) ??
      statValue(edge?.adjusted_network_gap_ms, "avg") ??
      statValue(edge?.network_gap_ms, "avg") ??
      statValue(edge?.raw_network_gap_ms, "avg");
    if (network != null) {
      const value = Math.max(0, network);
      state.networkTotal +=
        toFiniteNumber(edge?.adjusted_network_gap_ms?.total) ??
        toFiniteNumber(edge?.network_gap_ms?.total) ??
        toFiniteNumber(edge?.raw_network_gap_ms?.total) ??
        toFiniteNumber(edge?.total_network_gap_ms) ??
        value * metricCount;
      state.networkMetricCount += metricCount;
      state.networkValues.push(value);
    }
    byApi.set(key, state);
  }
  return Array.from(byApi.values()).map((state) => {
    const apiTotalMs = state.apiTotal / divisor;
    const networkTotalMs = state.networkTotal / divisor;
    return {
      apiUrl: state.apiUrl,
      target: state.target,
      callers: compactLabels(state.callers),
      callees: compactLabels(state.callees),
      count: state.count / divisor,
      matchedCount: state.matchedCount / divisor,
      unmatchedCount: state.unmatchedCount / divisor,
      apiTotalMs,
      apiAvgMs: state.apiTotal / Math.max(1, state.apiMetricCount),
      apiP95Ms: percentile(state.apiValues, 95),
      apiMaxMs: Math.max(0, ...state.apiValues),
      calleeAvgMs: state.calleeTotal / Math.max(1, state.calleeMetricCount),
      calleeTotalMs: state.calleeTotal / divisor,
      networkTotalMs,
      networkAvgMs: state.networkTotal / Math.max(1, state.networkMetricCount),
      networkP95Ms: percentile(state.networkValues, 95),
      networkMaxMs: Math.max(0, ...state.networkValues),
    };
  });
}

function sortApiCallRows(
  rows: ApiCallAggregate[],
  key: ApiCallSortKey,
  direction: SortDirection,
): ApiCallAggregate[] {
  const factor = direction === "asc" ? 1 : -1;
  return [...rows].sort((a, b) => {
    const av = apiCallSortValue(a, key);
    const bv = apiCallSortValue(b, key);
    let delta = 0;
    if (typeof av === "string" || typeof bv === "string") {
      delta = String(av).localeCompare(String(bv));
    } else {
      delta = av - bv;
    }
    if (delta !== 0) return delta * factor;
    return a.apiUrl.localeCompare(b.apiUrl);
  });
}

function apiCallSortValue(
  row: ApiCallAggregate,
  key: ApiCallSortKey,
): string | number {
  switch (key) {
    case "apiUrl":
      return row.apiUrl;
    case "target":
      return row.target;
    case "count":
      return row.count;
    case "apiTotalMs":
      return row.apiTotalMs;
    case "apiAvgMs":
      return row.apiAvgMs;
    case "apiP95Ms":
      return row.apiP95Ms;
    case "apiMaxMs":
      return row.apiMaxMs;
    case "calleeAvgMs":
      return row.calleeAvgMs;
    case "calleeTotalMs":
      return row.calleeTotalMs;
    case "networkTotalMs":
      return row.networkTotalMs;
    case "networkP95Ms":
      return row.networkP95Ms;
    case "networkMaxMs":
      return row.networkMaxMs;
  }
}

function percentile(values: number[], pct: number): number {
  const finite = values.filter((value) => Number.isFinite(value)).sort((a, b) => a - b);
  if (finite.length === 0) return 0;
  if (finite.length === 1) return finite[0];
  const rank = (Math.max(0, Math.min(100, pct)) / 100) * (finite.length - 1);
  const lower = Math.floor(rank);
  const upper = Math.ceil(rank);
  if (lower === upper) return finite[lower];
  const weight = rank - lower;
  return finite[lower] * (1 - weight) + finite[upper] * weight;
}

function formatCountValue(value: number): string {
  const finite = Number.isFinite(value) ? value : 0;
  const maximumFractionDigits =
    finite > 0 && finite < 10 && !Number.isInteger(finite) ? 1 : 0;
  return finite.toLocaleString(undefined, { maximumFractionDigits });
}

function compactLabels(values: Set<string>): string {
  const labels = Array.from(values).filter(Boolean).sort();
  if (labels.length <= 2) return labels.join(", ");
  return `${labels.slice(0, 2).join(", ")} +${labels.length - 2}`;
}

function formatMsValue(value: unknown): string {
  const n = toFiniteNumber(value);
  return n == null ? "—" : `${Math.round(n).toLocaleString()} ms`;
}

export function JenniferProfilePage(): React.JSX.Element {
  const { locale, t } = useI18n();
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
  const [servletDispatchPatterns] = useState<string[]>([]);
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
  const [selectedMsaDrilldown, setSelectedMsaDrilldown] =
    useState<string>("__whole__");
  const [selectedAverageMsaDrilldown, setSelectedAverageMsaDrilldown] =
    useState<string>("__whole__");
  // "all" 은 부가/검증 탭이 모든 파일/그룹을 보여주는 기존 동작이고,
  // "drilldown" 은 MSA 탭에서 고른 단일/평균 + 드릴다운 범위에
  // 맞춰 부가/검증의 리스트들도 함께 좁혀준다.
  const [profilesViewScope, setProfilesViewScope] = useState<
    "all" | "drilldown"
  >("all");
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
      servletDispatchPatterns,
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
  }, [customAnalysisRules, eventCategoryPatterns, fallbackToTxid, networkPrepPatterns, servletDispatchPatterns]);

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
          const loadedDispatch = normalizeStringList(parsed?.servletDispatchPatterns);
          if (loadedDispatch.length > 0) {
            loadedCategories.SERVLET_DISPATCH_METHOD = Array.from(
              new Set([
                ...(loadedCategories.SERVLET_DISPATCH_METHOD ?? []),
                ...loadedDispatch,
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
      // NetworkPrepPatterns hooks. Built-in network-prep defaults are
      // retained by the backend and these values only extend them.
      // Built-in event matches still bracket EXTERNAL_CALL/FETCH so
      // user typos can't corrupt edge counts.
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
      const dispatchFromEditor =
        eventCategoryPatterns["SERVLET_DISPATCH_METHOD"] ?? [];
      const dispatchCombined = Array.from(
        new Set([...servletDispatchPatterns, ...dispatchFromEditor]),
      );
      const otherCategories: CategoryRules = { ...eventCategoryPatterns };
      delete otherCategories["NETWORK_PREP_METHOD"];
      delete otherCategories["SERVLET_DISPATCH_METHOD"];
      const activeCustomRules = normalizeCustomAnalysisRules(customAnalysisRules);

      const res = await engine.analyzeJenniferProfile({
        path: selected.length === 1 ? selected[0].filePath : undefined,
        paths: selected.length > 1 ? selected.map((s) => s.filePath) : undefined,
        fallbackCorrelationToTxid: fallbackToTxid,
        headerBodyToleranceMs: 0,
        networkPrepPatterns: prepCombined.length > 0 ? prepCombined : undefined,
        servletDispatchPatterns:
          dispatchCombined.length > 0 ? dispatchCombined : undefined,
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
      setSelectedMsaDrilldown("__whole__");
      setSelectedAverageMsaDrilldown("__whole__");
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
            servletDispatchPatterns: dispatchCombined,
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
  const networkPrepRows: any[] = result?.tables?.network_prep_methods ?? [];
  const unprofiledExternalCallRows: any[] =
    result?.tables?.unprofiled_external_call_groups ?? [];
  const slowSqlRows: any[] = result?.tables?.slow_sql_events ?? [];
  const customRuleRows: any[] = result?.tables?.custom_rule_stats ?? [];
  const signatureStats: any[] = result?.series?.signature_statistics ?? [];
  const hasResult = Boolean(result);
  const showMsaTablesInSummary = false;

  const allProfileNetworkTimeByTxid = useMemo(
    () => buildNetworkTimeByTxid(guidGroups),
    [guidGroups],
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

  const singleProfileRows: any[] = useMemo(() => {
    if (!selectedGuidGroup?.guid) return [];
    const txids = new Set((selectedGuidGroup?.profile_txids ?? []) as string[]);
    return profileRows.filter(
      (row) => row?.guid === selectedGuidGroup.guid || (!row?.guid && txids.has(row?.txid)),
    );
  }, [profileRows, selectedGuidGroup]);

  const msaDrilldownOptions = useMemo(
    () => buildMsaDrilldownOptions(selectedGuidGroup, singleProfileRows),
    [selectedGuidGroup, singleProfileRows],
  );

  const selectedMsaDrilldownValue = useMemo(() => {
    if (msaDrilldownOptions.length === 0) return "__whole__";
    return msaDrilldownOptions.some((option) => option.value === selectedMsaDrilldown)
      ? selectedMsaDrilldown
      : "__whole__";
  }, [msaDrilldownOptions, selectedMsaDrilldown]);

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

  const signatureProfileRows: any[] = useMemo(() => {
    // standalone:<txid> 합성 GUID 그룹의 프로파일은 row.guid 가 비어
    // 있어 selectedSignatureGuidSet 매칭에서 빠진다. profile_txids
    // fallback 으로 그 케이스도 살린다 (singleProfileRows 와 같은 규칙).
    const txids = new Set<string>();
    for (const g of signatureGroups) {
      for (const t of (g?.profile_txids ?? []) as string[]) {
        if (t) txids.add(t);
      }
    }
    return profileRows.filter(
      (row) => selectedSignatureGuidSet.has(row?.guid) || (!row?.guid && txids.has(row?.txid)),
    );
  }, [profileRows, selectedSignatureGuidSet, signatureGroups]);

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

  const averageDrilldownOptions = useMemo(
    () => buildMsaAverageDrilldownOptions(signatureProfileRows),
    [signatureProfileRows],
  );

  const selectedAverageMsaDrilldownValue = useMemo(() => {
    if (averageDrilldownOptions.length === 0) return "__whole__";
    return averageDrilldownOptions.some(
      (option) => option.value === selectedAverageMsaDrilldown,
    )
      ? selectedAverageMsaDrilldown
      : "__whole__";
  }, [averageDrilldownOptions, selectedAverageMsaDrilldown]);

  const drilldownSelection = useMemo<JenniferDrilldownSelection | null>(() => {
    if (!result) return null;
    if (msaTimelineMode === "single") {
      if (!selectedGuidGroup?.guid) return null;
      return { mode: "single", guid: selectedGuidGroup.guid,
        ...(selectedMsaDrilldownValue === "__whole__" ? {} : { txid: selectedMsaDrilldownValue }) };
    }
    if (!selectedSignature?.signature_hash) return null;
    return { mode: "average", signature_hash: selectedSignature.signature_hash,
      ...(selectedAverageMsaDrilldownValue === "__whole__" ? {} : { application: selectedAverageMsaDrilldownValue }) };
  }, [result, msaTimelineMode, selectedGuidGroup, selectedSignature, selectedMsaDrilldownValue, selectedAverageMsaDrilldownValue]);
  const drilldownKey = JSON.stringify(drilldownSelection);
  const [drilldownSnapshot, setDrilldownSnapshot] = useState<{
    source: unknown; key: string; projection?: AnalysisResult; scopes: MsaDrilldownScope[]; error?: string;
  } | null>(null);
  const [drilldownExporting, setDrilldownExporting] = useState(false);
  const [drilldownNotice, setDrilldownNotice] = useState("");
  useEffect(() => {
    if (!result || !drilldownSelection) return;
    let disposed = false;
    setDrilldownNotice("");
    // A changed selection/source invalidates the snapshot immediately.
    // A late response cannot replace the current selection.
    void engine.projectJenniferDrilldown({ result, selection: drilldownSelection }).then((projection) => {
      if (disposed) return;
      const scopes = readJenniferDrilldown(projection);
      setDrilldownSnapshot({ source: result, key: drilldownKey, projection, scopes });
    }).catch((caught: unknown) => {
      if (!disposed) setDrilldownSnapshot({ source: result, key: drilldownKey, scopes: [], error: String(caught) });
    });
    return () => { disposed = true; };
  }, [result, drilldownKey]);
  const currentDrilldown = isCurrentJenniferProjection(drilldownSnapshot, result, drilldownKey) ? drilldownSnapshot : null;
  const singleDrilldownScope = msaTimelineMode === "single" ? currentDrilldown?.scopes[0] : undefined;
  const averageDrilldownScopes = msaTimelineMode === "average" ? currentDrilldown?.scopes ?? [] : [];
  const exportDrilldown = async (format: "json" | "html") => {
    const projection = currentDrilldown?.projection;
    if (!projection || drilldownExporting) return;
    setDrilldownExporting(true);
    try {
      const path = await Dialogs.SaveFile({ Title: t("exportChooseOutput"), Filename: `jennifer-drilldown.${format}`,
        Filters: [{ DisplayName: format.toUpperCase(), Pattern: `*.${format}` }] });
      if (!path) return;
      if (format === "json") await engine.exportJSON({ path, result: projection });
      else await engine.exportHTML({ path, result: projection });
      setDrilldownNotice(`${t("exportCompleted")}: ${path}`);
    } catch (caught) { setDrilldownNotice(String(caught)); }
    finally { setDrilldownExporting(false); }
  };

  const averageDrilldownGroups = useMemo(
    () =>
      selectedAverageMsaDrilldownValue === "__whole__"
        ? signatureGroups
        : averageDrilldownScopes.map((scope) => scope.group),
    [averageDrilldownScopes, selectedAverageMsaDrilldownValue, signatureGroups],
  );

  const averageDrilldownSlowSqlRows = useMemo(
    () =>
      selectedAverageMsaDrilldownValue === "__whole__"
        ? signatureSlowSqlRows
        : averageDrilldownScopes.flatMap((scope) => scope.slowSqlRows),
    [averageDrilldownScopes, selectedAverageMsaDrilldownValue, signatureSlowSqlRows],
  );

  const averageDrilldownActualEdges = useMemo(
    () =>
      selectedAverageMsaDrilldownValue === "__whole__"
        ? signatureTimelineEdges
        : uniqueTimelineEdges(
            averageDrilldownScopes.flatMap((scope) => scope.timelineEdges),
          ),
    [averageDrilldownScopes, selectedAverageMsaDrilldownValue, signatureTimelineEdges],
  );

  const averageDrilldownTimelineEdges = useMemo(
    () =>
      selectedAverageMsaDrilldownValue === "__whole__"
        ? averageTimelineEdges
        : (currentDrilldown?.projection?.series.drilldown_average_timeline_edges as any[] ?? []),
    [
      averageDrilldownActualEdges,
      currentDrilldown,
      averageTimelineEdges,
      selectedAverageMsaDrilldownValue,
    ],
  );

  const averageDrilldownRootApplication =
    selectedAverageMsaDrilldownValue === "__whole__"
      ? selectedSignature?.root_application
      : selectedAverageMsaDrilldownValue;

  const averageDrilldownRootDurationMs =
    selectedAverageMsaDrilldownValue === "__whole__"
      ? statValue(selectedSignature?.metrics?.root_response_time_ms, "avg")
      : toFiniteNumber((currentDrilldown?.projection?.series.drilldown_average_breakdown as any)?.root_response_time_ms);

  // 부가/검증 탭에서 "MSA 드릴다운 기준" 토글이 켜져 있을 때 적용할
  // 활성 범위. 단일/평균 + 전체/드릴다운 4분면을 한 곳으로 모아
  // 다운스트림 리스트들이 같은 TXID/GUID 셋으로 좁혀지도록 한다.
  const activeMsaScope = useMemo(() => {
    if (msaTimelineMode === "single") {
      const profiles = singleDrilldownScope?.profileRows ?? [];
      const txids = new Set(
        profiles.map((row: any) => String(row?.txid ?? "")).filter(Boolean),
      );
      const guids = new Set<string>();
      if (singleDrilldownScope?.group?.guid)
        guids.add(String(singleDrilldownScope.group.guid));
      const label =
        singleDrilldownScope?.isWholeTransaction === false
          ? `단일 · ${singleDrilldownScope?.selectedApplication ?? ""}`
          : `단일 · ${selectedGuidGroup?.root_application ?? "트랜잭션"}`;
      return { txids, guids, label, available: profiles.length > 0 };
    }
    const groupRows =
      selectedAverageMsaDrilldownValue === "__whole__"
        ? [signatureProfileRows]
        : averageDrilldownScopes.map((scope) => scope.profileRows);
    const profiles = groupRows.flat();
    const txids = new Set(
      profiles.map((row: any) => String(row?.txid ?? "")).filter(Boolean),
    );
    const guids = new Set<string>();
    for (const g of averageDrilldownGroups) {
      if (g?.guid) guids.add(String(g.guid));
    }
    const label =
      selectedAverageMsaDrilldownValue === "__whole__"
        ? `평균 · ${selectedSignature?.root_application ?? "시그니처"}`
        : `평균 · ${selectedAverageMsaDrilldownValue}`;
    return { txids, guids, label, available: profiles.length > 0 };
  }, [
    averageDrilldownGroups,
    averageDrilldownScopes,
    msaTimelineMode,
    selectedAverageMsaDrilldownValue,
    selectedGuidGroup,
    selectedSignature,
    signatureProfileRows,
    singleDrilldownScope,
  ]);

  const profilesScopeActive = profilesViewScope === "drilldown";
  const scopedProfileRows = profilesScopeActive
    ? (currentDrilldown?.projection?.tables.profiles as any[] ?? []) : profileRows;
  const scopedNetworkPrepRows = profilesScopeActive
    ? (currentDrilldown?.projection?.tables.network_prep_methods as any[] ?? []) : networkPrepRows;
  // Parent rollups cannot be relabeled as selected-subtree costs. Keep them
  // in the whole-result view until the analyzer can recompute those rollups.
  const scopedUnprofiledExternalCallRows = profilesScopeActive ? [] : unprofiledExternalCallRows;
  const scopedCustomRuleRows = profilesScopeActive ? [] : customRuleRows;
  const scopedProfileNetworkTime = useMemo(() => {
    const map = new Map<string, number>();
    for (const scope of currentDrilldown?.scopes ?? []) {
      for (const row of scope.profileRows) {
        map.set(`${String(row.guid ?? "")}\u0000${String(row.txid ?? "")}`, scope.networkTimeByTxid.get(String(row.txid ?? "")) ?? 0);
      }
    }
    return map;
  }, [currentDrilldown]);
  const scopedMethodHotspotRows = (currentDrilldown?.projection?.tables.method_hotspots as any[] ?? []);

  const msaTabApiEdges = useMemo(() => {
    if (msaTimelineMode === "single") {
      return singleDrilldownScope?.timelineEdges ?? [];
    }
    if (!selectedSignature) return [];
    return averageDrilldownActualEdges;
  }, [
    averageDrilldownActualEdges,
    msaTimelineMode,
    selectedSignature,
    singleDrilldownScope,
  ]);

  const msaTabApiSampleCount =
    msaTimelineMode === "average" ? Math.max(1, averageDrilldownGroups.length) : 1;

  const emptyResultCard = (
    <Card>
      <CardContent className="p-4 text-xs text-muted-foreground">
          {t("nothingYet")}
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
    if (meta.eventCategoryPatterns && typeof meta.eventCategoryPatterns === "object") {
      const restoredCategories: CategoryRules = {
        ...(meta.eventCategoryPatterns as CategoryRules),
      };
      const restoredPrep = normalizeStringList(meta.networkPrepPatterns);
      if (restoredPrep.length > 0) {
        restoredCategories.NETWORK_PREP_METHOD = Array.from(
          new Set([
            ...(restoredCategories.NETWORK_PREP_METHOD ?? []),
            ...restoredPrep,
          ]),
        );
      }
      const restoredDispatch = normalizeStringList(meta.servletDispatchPatterns);
      if (restoredDispatch.length > 0) {
        restoredCategories.SERVLET_DISPATCH_METHOD = Array.from(
          new Set([
            ...(restoredCategories.SERVLET_DISPATCH_METHOD ?? []),
            ...restoredDispatch,
          ]),
        );
      }
      setEventCategoryPatterns(restoredCategories);
    } else {
      const restoredPrep = normalizeStringList(meta.networkPrepPatterns);
      const restoredDispatch = normalizeStringList(meta.servletDispatchPatterns);
      if (restoredPrep.length > 0 || restoredDispatch.length > 0) {
        setEventCategoryPatterns({
          ...(restoredPrep.length > 0
            ? { NETWORK_PREP_METHOD: restoredPrep }
            : {}),
          ...(restoredDispatch.length > 0
            ? { SERVLET_DISPATCH_METHOD: restoredDispatch }
            : {}),
        });
      }
    }
    if (Array.isArray(meta.customAnalysisRules)) {
      setCustomAnalysisRules(
        normalizeCustomAnalysisRules(meta.customAnalysisRules),
      );
    }
    setError("");
  };

  return (
    <main className="analysis-tool flex min-w-0 flex-col gap-2 p-2 overflow-y-auto">
      <div className="flex min-w-0 items-center gap-2">
        <WailsFileDock
          className="min-w-0 flex-1"
          label={t("dropOrBrowseJennifer")}
          helpText={getHelpText(locale, "pageMsaProfile")}
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
            <HelpedLabel help={getHelpText(locale, "analyzerOptions")}>
              fallback correlation = TXID
            </HelpedLabel>
            <span className="text-muted-foreground">
              (기본: 꺼짐 — GUID 누락 시에만 켜세요)
            </span>
          </label>
          <div>
            <p className="mb-2 text-xs font-semibold text-foreground/80">
              MSA 이벤트 분류 패턴 (METHOD/UNKNOWN 라인 추가 분류)
            </p>
            <p className="mb-2 text-[11px] text-muted-foreground">
              기본값: NETWORK_PREP_METHOD = "IntegrationUtil.sendToService"는 항상 적용.
              여기에 추가한 값은 기본값에 더해집니다.
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
            <TabsTrigger value="summary">{t("summary")}</TabsTrigger>
            <TabsTrigger value="msa">MSA</TabsTrigger>
            <TabsTrigger value="profiles">{t("msaValidationTab")}</TabsTrigger>
            <TabsTrigger value="parser">{t("diagnosticsTitle")}</TabsTrigger>
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  File errors
                  <HelpTip text={getHelpText(locale, "sectionMsaFileErrors")} />
                </CardTitle>
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  File summary
                  <HelpTip text={getHelpText(locale, "sectionMsaFileSummary")} />
                </CardTitle>
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  MSA timeline signatures ({signatureStats.length})
                  <HelpTip text={getHelpText(locale, "sectionMsaSignatures")} />
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  Edge statistics
                  <HelpTip text={getHelpText(locale, "sectionMsaEdgeStats")} />
                </CardTitle>
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  MSA GUID groups ({guidGroups.length})
                  <HelpTip text={getHelpText(locale, "sectionMsaGuidGroups")} />
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  External call edges ({msaEdges.length})
                  <HelpTip text={getHelpText(locale, "sectionMsaExternalEdges")} />
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  MSA 타임라인 분석 모드
                  <HelpTip text={getHelpText(locale, "sectionMsaTimelineMode")} />
                </CardTitle>
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
                  <>
                    <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                      <span className="font-medium text-foreground/80">
                        <HelpedLabel
                          help={getHelpText(locale, "optionMsaTimelineGuid")}
                        >
                          분석할 트랜잭션
                        </HelpedLabel>
                      </span>
                      <select
                        className="h-9 rounded-md border border-input bg-background px-3 text-xs"
                        value={selectedGuidGroup?.guid ?? ""}
                        onChange={(event) => {
                          setSelectedGuid(event.target.value);
                          setSelectedMsaDrilldown("__whole__");
                        }}
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
                    <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                      <span className="font-medium text-foreground/80">
                        분석 기준 앱
                      </span>
                      <select
                        className="h-9 rounded-md border border-input bg-background px-3 text-xs"
                        value={selectedMsaDrilldownValue}
                        onChange={(event) =>
                          setSelectedMsaDrilldown(event.target.value)
                        }
                      >
                        {msaDrilldownOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </label>
                  </>
                ) : (
                  <>
                    <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                      <span className="font-medium text-foreground/80">
                        <HelpedLabel
                          help={getHelpText(locale, "optionMsaTimelineSignature")}
                        >
                          평균을 낼 signature
                        </HelpedLabel>
                      </span>
                      <select
                        className="h-9 rounded-md border border-input bg-background px-3 text-xs"
                        value={selectedSignature?.signature_hash ?? ""}
                        onChange={(event) => {
                          setSelectedSignatureHash(event.target.value);
                          setSelectedAverageMsaDrilldown("__whole__");
                        }}
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
                    <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                      <span className="font-medium text-foreground/80">
                        평균 기준 앱
                      </span>
                      <select
                        className="h-9 rounded-md border border-input bg-background px-3 text-xs"
                        value={selectedAverageMsaDrilldownValue}
                        onChange={(event) =>
                          setSelectedAverageMsaDrilldown(event.target.value)
                        }
                      >
                        {averageDrilldownOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </label>
                  </>
                )}
              </CardContent>
            </Card>

            <div className="flex flex-wrap items-center gap-2 text-xs" aria-live="polite">
              {!currentDrilldown && drilldownSelection && <span>{locale === "ko" ? "선택 범위 계산 중…" : "Calculating selected scope…"}</span>}
              {currentDrilldown?.error && <span role="alert" className="text-destructive">{currentDrilldown.error}</span>}
              {currentDrilldown?.projection && <span>{locale === "ko" ? "Go 드릴다운 표본" : "Go drilldown samples"}: {String(currentDrilldown.projection.summary.sample_count)} / {String(currentDrilldown.projection.summary.candidate_group_count)}</span>}
              <Button size="sm" variant="outline" disabled={!currentDrilldown?.projection || drilldownExporting} onClick={() => void exportDrilldown("json")}>{locale === "ko" ? "선택 범위 JSON 저장" : "Save scope JSON"}</Button>
              <Button size="sm" variant="outline" disabled={!currentDrilldown?.projection || drilldownExporting} onClick={() => void exportDrilldown("html")}>{locale === "ko" ? "선택 범위 HTML 저장" : "Save scope HTML"}</Button>
              <Button size="sm" variant="outline" disabled={!currentDrilldown?.projection} onClick={() => {
                if (!currentDrilldown?.projection) return;
                addWorkspaceResult({ result: currentDrilldown.projection, title: "Jennifer MSA drilldown" });
                setDrilldownNotice(locale === "ko" ? "선택 범위를 Workspace에 추가했습니다." : "Selected scope added to Workspace.");
              }}>{locale === "ko" ? "선택 범위 Workspace 추가" : "Add scope to Workspace"}</Button>
              {drilldownNotice && <span>{drilldownNotice}</span>}
            </div>
            {!currentDrilldown?.projection ? null : msaTimelineMode === "single" ? (
              singleDrilldownScope ? (
                <>
                  <MsaResponseTimeBreakdown
                    groups={[singleDrilldownScope.group] as any}
                  />
                  <MsaTopology
                    title={
                      singleDrilldownScope.isWholeTransaction
                        ? "단일 트랜잭션 호출 토폴로지"
                        : `드릴다운 호출 토폴로지 (${singleDrilldownScope.selectedApplication})`
                    }
                    edges={singleDrilldownScope.topologyEdges as any}
                    rootApplication={singleDrilldownScope.selectedApplication}
                  />
                  <ApiCallAnalysisPanel
                    edges={msaTabApiEdges}
                    scopeLabel={
                      activeMsaScope.available ? activeMsaScope.label : undefined
                    }
                    valueMode="sum"
                    sampleCount={msaTabApiSampleCount}
                  />
                  <MsaMethodHotspots
                    rows={scopedMethodHotspotRows}
                    txids={activeMsaScope.available ? activeMsaScope.txids : null}
                    defaultAggregationMode="sum"
                    sampleCount={1}
                    scopeLabel={activeMsaScope.label}
                  />
                  <MsaTimeline
                    title={
                      singleTimelineLayout === "grouped"
                        ? singleDrilldownScope.isWholeTransaction
                          ? "단일 트랜잭션 타임라인 (서비스 그룹)"
                          : "드릴다운 타임라인 (서비스 그룹)"
                        : singleDrilldownScope.isWholeTransaction
                          ? "단일 트랜잭션 타임라인 (개별 호출)"
                          : "드릴다운 타임라인 (개별 호출)"
                    }
                    edges={singleDrilldownScope.timelineEdges as any}
                    mode={
                      singleTimelineLayout === "grouped"
                        ? "by-service"
                        : "overall"
                    }
                    layoutToggle={{
                      value: singleTimelineLayout,
                      onChange: setSingleTimelineLayout,
                    }}
                    rootApplication={singleDrilldownScope.selectedApplication}
                    rootDurationMs={toFiniteNumber(
                      singleDrilldownScope.group?.root_response_time_ms,
                    )}
                    rootStartMs={toFiniteNumber(
                      singleDrilldownScope.group?.root_body_start_ms,
                    )}
                  />
                  <MsaTimelineTreemap
                    title={
                      singleDrilldownScope.isWholeTransaction
                        ? "단일 트랜잭션 트리맵 (elapsed 합계)"
                        : "드릴다운 트리맵 (elapsed 합계)"
                    }
                    edges={singleDrilldownScope.timelineEdges as any}
                    rootApplication={singleDrilldownScope.selectedApplication}
                  />
                  <SlowSqlTable rows={singleDrilldownScope.slowSqlRows} />
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
                  groups={averageDrilldownGroups as any}
                  defaultAggregationMode="avg"
                />
                <MsaTopology
                  title={
                    selectedAverageMsaDrilldownValue === "__whole__"
                      ? "평균 트랜잭션 호출 토폴로지"
                      : `평균 드릴다운 호출 토폴로지 (${selectedAverageMsaDrilldownValue})`
                  }
                  edges={averageDrilldownTimelineEdges as any}
                  rootApplication={averageDrilldownRootApplication}
                />
                <ApiCallAnalysisPanel
                  edges={msaTabApiEdges}
                  scopeLabel={
                    activeMsaScope.available ? activeMsaScope.label : undefined
                  }
                  valueMode="avg"
                  sampleCount={msaTabApiSampleCount}
                />
                <MsaMethodHotspots
                  rows={scopedMethodHotspotRows}
                  txids={activeMsaScope.available ? activeMsaScope.txids : null}
                  defaultAggregationMode="avg"
                  sampleCount={Math.max(1, averageDrilldownGroups.length)}
                  scopeLabel={activeMsaScope.label}
                />
                <MsaTimeline
                  title={
                    selectedAverageMsaDrilldownValue === "__whole__"
                      ? "평균 트랜잭션 타임라인 (서비스 그룹)"
                      : "평균 드릴다운 타임라인 (서비스 그룹)"
                  }
                  edges={averageDrilldownTimelineEdges as any}
                  mode="by-service"
                  rootApplication={averageDrilldownRootApplication}
                  rootDurationMs={averageDrilldownRootDurationMs}
                />
                <MsaTimelineTreemap
                  title={
                    selectedAverageMsaDrilldownValue === "__whole__"
                      ? "평균 트랜잭션 트리맵 (avg elapsed 합계)"
                      : "평균 드릴다운 트리맵 (avg elapsed 합계)"
                  }
                  edges={averageDrilldownTimelineEdges as any}
                  rootApplication={averageDrilldownRootApplication}
                />
                <SlowSqlTable rows={averageDrilldownSlowSqlRows} />
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
            <Card>
              <CardHeader className="pb-3">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <CardTitle className="inline-flex items-center gap-2 text-sm">
                    부가/검증 범위
                  </CardTitle>
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="flex items-center gap-1 rounded-md border border-border bg-muted/20 p-1">
                      <Button
                        type="button"
                        size="sm"
                        variant={profilesViewScope === "all" ? "default" : "outline"}
                        onClick={() => setProfilesViewScope("all")}
                        title="모든 파일의 프로파일/규칙/네트워크준비/외부호출을 그대로 표시"
                      >
                        전체
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant={
                          profilesViewScope === "drilldown" ? "default" : "outline"
                        }
                        onClick={() => setProfilesViewScope("drilldown")}
                        disabled={!activeMsaScope.available}
                        title={
                          activeMsaScope.available
                            ? "MSA 탭에서 선택한 단일/평균 드릴다운에 맞춰 좁혀 표시"
                            : "MSA 탭에서 드릴다운 범위를 먼저 선택하세요"
                        }
                      >
                        MSA 드릴다운 기준
                      </Button>
                    </div>
                    {profilesScopeActive && (
                      <span className="rounded bg-primary/10 px-2 py-1 text-[11px] text-primary">
                        {activeMsaScope.label} · TXID {activeMsaScope.txids.size.toLocaleString()}건
                      </span>
                    )}
                  </div>
                </div>
              </CardHeader>
            </Card>
            <TransactionProfilesTable
              rows={scopedProfileRows}
              networkTimeByTxid={profilesScopeActive ? scopedProfileNetworkTime : allProfileNetworkTimeByTxid}
            />
            {profilesScopeActive && <p className="text-xs text-muted-foreground">{locale === "ko" ? "사용자 규칙·미수집 외부 호출의 집계는 선택 범위에서 재계산하지 않습니다. 해당 집계는 전체 보기에서 확인하세요." : "Custom-rule and unprofiled-call rollups are available in the whole-result view; they are not recomputed for this scope."}</p>}
            <CustomRuleStatsPanel rows={scopedCustomRuleRows} />
            <NetworkPrepMethodsTable rows={scopedNetworkPrepRows} />
            <UnprofiledExternalCallGroupsTable rows={scopedUnprofiledExternalCallRows} />
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  파서 보고서
                  <HelpTip text={getHelpText(locale, "sectionMsaParserReport")} />
                </CardTitle>
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  파일 오류
                  <HelpTip text={getHelpText(locale, "sectionMsaFileErrors")} />
                </CardTitle>
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
                <CardTitle className="inline-flex items-center gap-2 text-sm">
                  프로파일별 이슈 (errors / warnings)
                  <HelpTip text={getHelpText(locale, "sectionMsaProfileIssues")} />
                </CardTitle>
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
