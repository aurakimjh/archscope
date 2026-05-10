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

import { Loader2, Play, X } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import { engine } from "../bridge/engine";

import {
  CustomCategoriesEditor,
  type CategoryRules,
  type Preset,
  type SegmentSpec,
} from "../components/CustomCategoriesEditor";
import { MsaResponseTimeBreakdown } from "../components/MsaResponseTimeBreakdown";
import { MsaTimeline } from "../components/MsaTimeline";
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

type Selection = {
  filePath: string;
  originalName: string;
};

type MsaTimelineMode = "single" | "average";

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

function formatStat(stats: any, key: string): string {
  const value = statValue(stats, key);
  return value == null ? "—" : Math.round(value).toLocaleString();
}

function metricStat(metrics: any, key: string, stat: string): string {
  return formatStat(metrics?.[key], stat);
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

function guidOptionLabel(group: any): string {
  const guid = String(group?.guid ?? "");
  const root = group?.root_application || "unknown";
  const responseTime = toFiniteNumber(group?.root_response_time_ms);
  const suffix = guid ? `…${guid.slice(-10)}` : "GUID 없음";
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

function SignatureAverageSummary({ signature }: { signature: any }): JSX.Element {
  const metrics = signature?.metrics ?? {};
  const edges: any[] = signature?.edges ?? [];
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">평균 트랜잭션 기준값</CardTitle>
        <p className="text-xs text-muted-foreground">
          같은 호출 구조(signature)로 묶인 GUID들의 평균값으로 타임라인을
          구성합니다.
        </p>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-6">
          <MetricCard
            label="Samples"
            value={(signature?.sample_count ?? 0).toLocaleString()}
          />
          <MetricCard
            label="Edges"
            value={(signature?.edge_count ?? 0).toLocaleString()}
          />
          <MetricCard
            label="Root avg"
            value={metricStat(metrics, "root_response_time_ms", "avg")}
          />
          <MetricCard
            label="Root p95"
            value={metricStat(metrics, "root_response_time_ms", "p95")}
          />
          <MetricCard
            label="External avg"
            value={metricStat(metrics, "total_external_call_cumulative_ms", "avg")}
          />
          <MetricCard
            label="Gap avg"
            value={metricStat(metrics, "total_network_gap_cumulative_ms", "avg")}
          />
        </section>
        {edges.length > 0 && (
          <div className="overflow-x-auto rounded-md border border-border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                  <th className="px-3 py-2 text-left font-medium">Caller → Callee</th>
                  <th className="px-3 py-2 text-right font-medium">#</th>
                  <th className="px-3 py-2 text-right font-medium">Count</th>
                  <th className="px-3 py-2 text-right font-medium">Ext avg</th>
                  <th className="px-3 py-2 text-right font-medium">Ext p95</th>
                  <th className="px-3 py-2 text-right font-medium">Callee avg</th>
                  <th className="px-3 py-2 text-right font-medium">Gap avg</th>
                </tr>
              </thead>
              <tbody>
                {edges.map((edge, idx) => (
                  <tr key={idx} className="border-b border-border last:border-0">
                    <td
                      className="px-3 py-2 font-mono text-xs"
                      title={`${edge?.caller_application} → ${edge?.callee_application}`}
                    >
                      {edge?.caller_application || "?"} →{" "}
                      {edge?.callee_application || "?"}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      #{edge?.occurrence_index ?? idx + 1}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {formatStat(edge?.external_call_elapsed_ms, "count")}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {formatStat(edge?.external_call_elapsed_ms, "avg")}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {formatStat(edge?.external_call_elapsed_ms, "p95")}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {formatStat(edge?.callee_response_time_ms, "avg")}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {formatStat(edge?.adjusted_network_gap_ms, "avg")}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
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
  const [selectedGuid, setSelectedGuid] = useState<string>("");
  const [selectedSignatureHash, setSelectedSignatureHash] =
    useState<string>("");
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
  const signatureStats: any[] = result?.series?.signature_statistics ?? [];
  const [profileTimelineActivated, setProfileTimelineActivated] = useState(false);
  const hasResult = Boolean(result);
  const showMsaTablesInSummary = false;

  // primaryRootApplication: pick the first GUID group's root for the
  // topology / per-service ordering. When the result spans multiple
  // unrelated traces, the topology still merges them but the root
  // hint anchors the most-frequent caller at the top.
  const primaryRootApplication: string | undefined = useMemo(
    () => guidGroups.find((g) => g?.root_application)?.root_application,
    [guidGroups],
  );

  // topologyEdges flattens every group's call_graph into one
  // caller→callee list — these are the matched edges with timing
  // metadata, used by the topology graph and the overall timeline.
  const topologyEdges: any[] = useMemo(
    () => guidGroups.flatMap((g) => g?.call_graph ?? []),
    [guidGroups],
  );

  // overallTimelineEdges keeps the unmatched edges too (with caller-
  // side timing) so the MSA timeline can show "this call had no
  // matching callee profile" alongside healthy ones. We backfill
  // missing match_status as MATCHED for call_graph entries because
  // call_graph only carries matched edges.
  const overallTimelineEdges: any[] = useMemo(
    () => [
      ...topologyEdges.map((e) => ({
        ...e,
        match_status: e.match_status ?? "MATCHED",
      })),
      ...msaEdges.filter((e) => e?.match_status && e.match_status !== "MATCHED"),
    ],
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
    () =>
      (selectedGuidGroup?.call_graph ?? []).map((edge: any) => ({
        ...edge,
        match_status: edge?.match_status ?? "MATCHED",
      })),
    [selectedGuidGroup],
  );

  const singleTimelineEdges: any[] = useMemo(
    () => [
      ...singleTopologyEdges,
      ...singleGroupEdges.filter(
        (edge) => edge?.match_status && edge.match_status !== "MATCHED",
      ),
    ],
    [singleGroupEdges, singleTopologyEdges],
  );

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

  const signatureMsaEdges: any[] = useMemo(
    () => msaEdges.filter((edge) => selectedSignatureGuidSet.has(edge?.guid)),
    [msaEdges, selectedSignatureGuidSet],
  );

  const averageTimelineEdges: any[] = useMemo(
    () => buildAverageTimelineEdges(selectedSignature),
    [selectedSignature],
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
    setError("");
  };

  return (
    <main className="flex flex-col gap-5 p-5 overflow-y-auto">
      <WailsFileDock
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
        </div>
      </AnalyzerOptionsDock>
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
            <TabsTrigger value="profiles">프로파일</TabsTrigger>
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
                    단일 트랜잭션
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant={msaTimelineMode === "average" ? "default" : "outline"}
                    onClick={() => setMsaTimelineMode("average")}
                  >
                    평균 트랜잭션
                  </Button>
                </div>
                {msaTimelineMode === "single" ? (
                  <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs lg:max-w-xl">
                    <span className="font-medium text-foreground/80">
                      분석할 GUID
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
                <p className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-700 dark:text-amber-300">
                  EXTERNAL_CALL / 2PC / CHECK_QUERY 합계가 모두 0입니다. 본문
                  이벤트가 잡히지 않았을 수 있습니다 — "파서 보고서" 탭의
                  파일 오류와 프로파일별 이슈를 확인하고, 필요하면 위 폼에서
                  사용자 분류 패턴을 추가해 다시 분석하세요.
                </p>
              )}

            {msaTimelineMode === "single" ? (
              selectedGuidGroup ? (
                <>
                  <GuidTransactionSummary group={selectedGuidGroup} />
                  <MsaResponseTimeBreakdown
                    groups={[selectedGuidGroup] as any}
                    edges={singleGroupEdges as any}
                  />
                  <MsaTopology
                    title="단일 트랜잭션 호출 토폴로지"
                    edges={singleTopologyEdges as any}
                    rootApplication={selectedGuidGroup?.root_application}
                  />
                  <MsaTimeline
                    title="단일 트랜잭션 타임라인 (호출 시각 순)"
                    edges={singleTimelineEdges as any}
                    mode="overall"
                    rootApplication={selectedGuidGroup?.root_application}
                  />
                </>
              ) : (
                <Card>
                  <CardContent className="p-4 text-xs text-muted-foreground">
                    GUID 기반 MSA 호출 그래프가 없습니다. 원본 프로파일의 GUID
                    또는 EXTERNAL_CALL 매칭 상태를 확인하세요.
                  </CardContent>
                </Card>
              )
            ) : selectedSignature ? (
              <>
                <SignatureAverageSummary signature={selectedSignature} />
                <MsaResponseTimeBreakdown
                  groups={signatureGroups as any}
                  edges={signatureMsaEdges as any}
                />
                <MsaTopology
                  title="평균 트랜잭션 호출 토폴로지"
                  edges={averageTimelineEdges as any}
                  rootApplication={selectedSignature?.root_application}
                />
                <MsaTimeline
                  title="평균 트랜잭션 타임라인 (avg elapsed 기준)"
                  edges={averageTimelineEdges as any}
                  mode="overall"
                  rootApplication={selectedSignature?.root_application}
                />
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
            <div className="flex items-center justify-between gap-3">
              <div className="text-xs text-muted-foreground">
                메인 Caller 서비스가 맨 위. 호출 시각 순으로 가로 막대로 표시.
              </div>
              <label className="flex cursor-pointer select-none items-center gap-1.5 text-xs">
                <input
                  type="checkbox"
                  className="h-3.5 w-3.5"
                  checked={profileTimelineActivated}
                  onChange={(e) =>
                    setProfileTimelineActivated(e.target.checked)
                  }
                />
                활성화 (응답시간 기준 정렬)
              </label>
            </div>
            <MsaTimeline
              title={
                profileTimelineActivated
                  ? "서비스별 타임라인 (응답시간 순)"
                  : "서비스별 타임라인 (호출 시각 순)"
              }
              edges={overallTimelineEdges as any}
              mode="by-service"
              rootApplication={primaryRootApplication}
              useResponseTimeOrder={profileTimelineActivated}
            />
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
