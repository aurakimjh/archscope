// MsaResponseTimeBreakdown — decomposes the main caller's response
// time into the categories the user reasons about when targeting an
// optimisation:
//
//   root_response = SQL + CheckQuery + 2PC + Fetch + NetworkCall +
//                   UnprofiledExternalCall + NetworkPrep + ConnAcquire +
//                   MethodTime
//
// MethodTime is the residual — what's left after subtracting all
// the categorised time. A high MethodTime ratio means the trace
// spent most of its time inside non-categorised application code;
// a high SQL or NetworkCall ratio points the user at the obvious
// optimisation target without making them stare at raw tables.
//
// Two views ship in this component:
//   1) Aggregate bar across all GUID groups (single bar, percentages
//      per category) — gives "where did the time go overall".
//   2) Per-group bars (one bar per GUID, sorted by descending root
//      response time) — surfaces which trace is dragging the avg.
//
// External-call drill-down tables live in the profile verification tab;
// this component stays focused on the response-time composition bars.

import { useMemo, useState } from "react";

import { HelpTip } from "@/components/HelpTip";
import { getGenericChartHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";

import { Button } from "./ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";

type GroupMetrics = {
  guid?: string;
  root_application?: string;
  root_response_time_ms?: number | null;
  metrics?: {
    response_time_breakdown?: {
      root_response_time_ms?: number;
      sql_execute_ms?: number;
      check_query_ms?: number;
      two_pc_ms?: number;
      fetch_ms?: number;
      network_call_ms?: number;
      unprofiled_external_call_ms?: number;
      network_prep_ms?: number;
      servlet_dispatch_ms?: number;
      connection_acquire_ms?: number;
      custom_slices?: Array<{
        id?: string;
        label?: string;
        group?: SliceGroupId | string;
        source?: string;
        value_ms?: number;
        count?: number;
      }>;
      method_time_ms?: number;
      method_time_ratio?: number;
      coverage?: number;
      negative_method_time?: boolean;
    };
  };
};

type AggregationMode = "sum" | "avg";

type MsaResponseTimeBreakdownProps = {
  groups: GroupMetrics[];
  /** Initial aggregation mode for the top bar. Defaults to "sum".
   *  AVG divides each metric by the number of contributing groups so
   *  files with very different root response times don't drown each
   *  other out — useful when comparing distributions side by side. */
  defaultAggregationMode?: AggregationMode;
};

type SliceGroupId = "database" | "external" | "internal";
type SliceKey = string;

type SliceDef = {
  key: SliceKey;
  label: string;
  group: SliceGroupId;
  color: string;
  hint?: string;
};

const SLICE_DEFS: SliceDef[] = [
  { key: "sql_execute_ms", label: "SQL", group: "database", color: "#2563eb", hint: "DB 쿼리 수행 누적" },
  { key: "check_query_ms", label: "Check Query", group: "database", color: "#06b6d4", hint: "select 1 같은 헬스체크" },
  { key: "two_pc_ms", label: "2PC / XA", group: "database", color: "#f59e0b", hint: "분산트랜잭션 prepare/commit" },
  { key: "fetch_ms", label: "Fetch", group: "database", color: "#fbbf24", hint: "ResultSet fetch" },
  { key: "connection_acquire_ms", label: "Conn Acquire", group: "database", color: "#ef4444", hint: "DB 풀 대기" },
  { key: "network_call_ms", label: "Network", group: "external", color: "#10b981", hint: "외부호출 - callee 응답시간 (실제 망 시간)" },
  { key: "unprofiled_external_call_ms", label: "External Call", group: "external", color: "#14b8a6", hint: "callee profile 없이 caller EXTERNAL_CALL elapsed만 확인된 외부호출" },
  { key: "network_prep_ms", label: "Network Prep", group: "external", color: "#a78bfa", hint: "sendToService 등 wrapper의 비-네트워크 부분" },
  { key: "servlet_dispatch_ms", label: "Servlet Dispatch", group: "internal", color: "#f97316", hint: "service() 등 콜리 프레임워크 dispatch self-time (업무 메소드 제외)" },
  { key: "method_time_ms", label: "Method", group: "internal", color: "#94a3b8", hint: "잔여 메소드 수행시간 (전체 - 위 합계)" },
];

const GROUP_ORDER: SliceGroupId[] = ["database", "external", "internal"];

const GROUP_LABELS: Record<SliceGroupId, string> = {
  database: "Database",
  external: "External",
  internal: "Internal",
};

const GROUP_COLORS: Record<SliceGroupId, string> = {
  database: "#2563eb",
  external: "#059669",
  internal: "#ea580c",
};

const CUSTOM_COLORS = [
  "#7c3aed",
  "#db2777",
  "#0f766e",
  "#b45309",
  "#475569",
  "#be123c",
  "#0891b2",
  "#4f46e5",
];

export function MsaResponseTimeBreakdown({
  groups,
  defaultAggregationMode = "sum",
}: MsaResponseTimeBreakdownProps): JSX.Element {
  const { locale } = useI18n();
  const helpText = getGenericChartHelpText(locale, "응답시간 구성");
  const [aggregationMode, setAggregationMode] = useState<AggregationMode>(
    defaultAggregationMode,
  );
  const {
    totalRoot,
    totalCovered,
    aggregateBase,
    aggregateGroups,
    visibleSlices,
    sliceDefs,
    perGroup,
    sampleCount,
  } = useMemo(() => {
      const customDefs = new Map<string, SliceDef>();
      // First pass: SUMS across all groups. AVG is derived later by
      // dividing each total by sampleCount so files with very
      // different response times don't drown each other out.
      const sumTotals = SLICE_DEFS.reduce<Record<string, number>>((acc, def) => {
        acc[def.key] = 0;
        return acc;
      }, {});
      let sumTotalRoot = 0;
      let nextSampleCount = 0;
      for (const g of groups) {
        const bd = g.metrics?.response_time_breakdown;
        if (!bd) continue;
        nextSampleCount += 1;
        sumTotalRoot += Number(bd.root_response_time_ms ?? 0);
        for (const def of SLICE_DEFS) {
          sumTotals[def.key] += Math.max(0, Number((bd as any)[def.key] ?? 0));
        }
        for (const slice of bd.custom_slices ?? []) {
          const key = customSliceKey(slice);
          const value = Math.max(0, Number(slice?.value_ms ?? 0));
          if (!customDefs.has(key)) {
            customDefs.set(key, {
              key,
              label: String(slice?.label || "Custom"),
              group: normalizeGroup(slice?.group),
              color: CUSTOM_COLORS[customDefs.size % CUSTOM_COLORS.length],
              hint: `사용자 정의 카드${slice?.source ? ` · ${slice.source}` : ""}`,
            });
            sumTotals[key] = 0;
          }
          sumTotals[key] += value;
        }
      }
      const divisor =
        aggregationMode === "avg" && nextSampleCount > 0 ? nextSampleCount : 1;
      const nextTotalRoot = sumTotalRoot / divisor;
      const nextTotals: Record<string, number> = {};
      for (const key of Object.keys(sumTotals)) {
        nextTotals[key] = sumTotals[key] / divisor;
      }
      const nextSliceDefs = [...SLICE_DEFS, ...customDefs.values()];
      const nextTotalCovered = nextSliceDefs.reduce(
        (sum, def) => sum + Number(nextTotals[def.key] ?? 0),
        0,
      );
      const nextAggregateBase = Math.max(nextTotalRoot, nextTotalCovered, 1);
      const nextVisibleSlices = nextSliceDefs.map((def) => ({
        ...def,
        value: Number(nextTotals[def.key] ?? 0),
      })).filter((slice) => slice.value > 0);
      const groupMap = new Map<
        SliceGroupId,
        {
          id: SliceGroupId;
          value: number;
          slices: Array<(typeof nextVisibleSlices)[number]>;
        }
      >();
      for (const id of GROUP_ORDER) {
        groupMap.set(id, { id, value: 0, slices: [] });
      }
      for (const slice of nextVisibleSlices) {
        const group = groupMap.get(slice.group);
        if (!group) continue;
        group.value += slice.value;
        group.slices.push(slice);
      }

      // Per-group sorted by root response time desc.
      const nextPerGroup = groups
        .map((g) => {
          const bd = g.metrics?.response_time_breakdown;
          const root = Number(bd?.root_response_time_ms ?? 0);
          return { g, bd, root };
        })
        .filter((r) => r.bd && r.root > 0)
        .sort((a, b) => b.root - a.root);

      return {
        totalRoot: nextTotalRoot,
        totalCovered: nextTotalCovered,
        aggregateBase: nextAggregateBase,
        aggregateGroups: GROUP_ORDER.map((id) => groupMap.get(id)!).filter(
          (group) => group.value > 0,
        ),
        visibleSlices: nextVisibleSlices,
        sliceDefs: nextSliceDefs,
        perGroup: nextPerGroup,
        sampleCount: nextSampleCount,
      };
    }, [aggregationMode, groups]);

  if (totalRoot === 0 && totalCovered === 0 && perGroup.length === 0) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="inline-flex items-center gap-2 text-sm">
            응답시간 구성
            <HelpTip text={helpText} />
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-xs text-muted-foreground">
            root response_time_ms 가 비어있어 분해할 수 없습니다 — 헤더의
            RESPONSE_TIME 필드를 확인하세요.
          </p>
        </CardContent>
      </Card>
    );
  }

  const canToggle = sampleCount > 1;
  const effectiveMode: AggregationMode = canToggle ? aggregationMode : "sum";
  const summaryLabel =
    effectiveMode === "avg"
      ? `Avg Root RT / Avg Categorised · n=${sampleCount}`
      : "Root RT / Categorised";
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <CardTitle className="inline-flex items-center gap-2 text-sm">
            응답시간 구성
            <HelpTip text={helpText} />
          </CardTitle>
          <div className="flex flex-wrap items-center justify-end gap-2">
            {canToggle && (
              <div className="flex items-center gap-1 rounded-md border border-border bg-muted/20 p-1">
                <Button
                  type="button"
                  size="sm"
                  variant={effectiveMode === "sum" ? "default" : "outline"}
                  onClick={() => setAggregationMode("sum")}
                  title="모든 그룹의 시간을 합산해 비율을 봅니다 — 전체 비용 분포에 적합"
                >
                  합계
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant={effectiveMode === "avg" ? "default" : "outline"}
                  onClick={() => setAggregationMode("avg")}
                  title="그룹당 평균으로 봅니다 — 응답시간 편차가 큰 파일들을 균등 비교할 때 적합"
                >
                  평균
                </Button>
              </div>
            )}
            <div className="rounded-md border border-border bg-muted/20 px-3 py-2 text-right">
              <div className="text-[10px] font-medium text-muted-foreground">
                {summaryLabel}
              </div>
              <div className="font-mono text-xs tabular-nums text-foreground">
                {formatMs(totalRoot)} · {formatMs(totalCovered)}
              </div>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {/* Aggregate */}
        <section>
          <GroupedTimelineBar
            groups={aggregateGroups}
            slices={visibleSlices}
            base={aggregateBase}
          />
          <GroupedLegend groups={aggregateGroups} base={aggregateBase} />
        </section>

        {/* Per-group */}
        {perGroup.length > 0 && (
          <section>
            <header className="mb-1.5 text-xs font-semibold">
              GUID 그룹별 (root 응답시간 큰 순서)
            </header>
            <div className="flex flex-col gap-1.5">
              {perGroup.slice(0, 25).map(({ g, bd, root }, idx) => {
                if (!bd) return null;
                const slices: Record<string, number> = {};
                for (const def of SLICE_DEFS) {
                  slices[def.key] = Math.max(0, Number((bd as any)[def.key] ?? 0));
                }
                for (const slice of bd.custom_slices ?? []) {
                  slices[customSliceKey(slice)] = Math.max(
                    0,
                    Number(slice?.value_ms ?? 0),
                  );
                }
                return (
                  <GroupRow
                    key={g.guid ?? `${g.root_application ?? "group"}-${idx}`}
                    label={
                      g.root_application
                        ? `${g.root_application} · …${String(g.guid ?? "").slice(-8)}`
                        : `…${String(g.guid ?? "").slice(-12)}`
                    }
                    root={root}
                    slices={slices}
                    sliceDefs={sliceDefs}
                    negativeMethod={!!bd.negative_method_time}
                  />
                );
              })}
            </div>
          </section>
        )}

      </CardContent>
    </Card>
  );
}
function GroupedTimelineBar({
  groups,
  slices,
  base,
}: {
  groups: Array<{
    id: SliceGroupId;
    value: number;
    slices: Array<{
      key: SliceKey;
      label: string;
      group: SliceGroupId;
      color: string;
      hint?: string;
      value: number;
    }>;
  }>;
  slices: Array<{
    key: string;
    label: string;
    group: SliceGroupId;
    color: string;
    hint?: string;
    value: number;
  }>;
  base: number;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex h-9 overflow-hidden">
        {groups.map((group) => {
          const widthPct = pct(group.value, base);
          const color = GROUP_COLORS[group.id];
          const label = GROUP_LABELS[group.id];
          return (
            <div
              key={group.id}
              className="relative h-9 border-l border-border/70 px-1 first:border-l-0"
              style={{ width: `${widthPct.toFixed(2)}%` }}
              title={`${label}: ${formatMs(group.value)} · ${widthPct.toFixed(1)}%`}
            >
              <div
                className="absolute inset-x-1 bottom-0 h-3 rounded-t-sm border-l border-r border-t"
                style={{ borderColor: color }}
                aria-hidden
              />
              {widthPct >= 12 ? (
                <div
                  className="truncate text-center text-[10px] font-semibold leading-4"
                  style={{ color }}
                >
                  {label} · {formatMs(group.value)} · {widthPct.toFixed(1)}%
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
      <div className="flex h-7 overflow-hidden rounded border border-border bg-muted/30">
        {slices.map((slice) => {
          const widthPct = pct(slice.value, base);
          return (
            <div
              key={slice.key}
              className="flex min-w-[2px] items-center justify-center overflow-hidden border-r border-background/70 text-[10px] font-medium text-white last:border-r-0"
              style={{
                width: `${widthPct.toFixed(2)}%`,
                backgroundColor: slice.color,
              }}
              title={`${slice.label}: ${formatMs(slice.value)} · ${widthPct.toFixed(1)}%${slice.hint ? ` - ${slice.hint}` : ""}`}
            >
              {widthPct >= 12 ? (
                <span className="truncate px-1">{slice.label}</span>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function GroupedLegend({
  groups,
  base,
}: {
  groups: Array<{
    id: SliceGroupId;
    value: number;
    slices: Array<{
      key: SliceKey;
      label: string;
      group: SliceGroupId;
      color: string;
      hint?: string;
      value: number;
    }>;
  }>;
  base: number;
}): JSX.Element {
  return (
    <div className="mt-3 grid gap-3 text-[11px] lg:grid-cols-3">
      {groups.map((group) => {
        const groupPct = pct(group.value, base);
        const groupColor = GROUP_COLORS[group.id];
        return (
          <section
            key={group.id}
            className="min-w-0 rounded-md border border-border bg-background/70 p-2.5"
          >
            <div className="mb-2 flex min-w-0 items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-1.5">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: groupColor }}
                  aria-hidden
                />
                <span className="truncate font-semibold text-foreground/85">
                  {GROUP_LABELS[group.id]}
                </span>
              </div>
              <span className="shrink-0 font-mono tabular-nums text-muted-foreground">
                {formatMs(group.value)} · {groupPct.toFixed(1)}%
              </span>
            </div>
            <ul className="flex flex-col gap-1.5">
              {group.slices.map((slice) => {
                const slicePct = pct(slice.value, base);
                return (
                  <li
                    key={slice.key}
                    className="flex min-w-0 items-center gap-1.5"
                    title={`${slice.label}: ${formatMs(slice.value)} · ${slicePct.toFixed(1)}%${slice.hint ? ` - ${slice.hint}` : ""}`}
                  >
                    <span
                      className="h-2.5 w-2.5 shrink-0 rounded-sm"
                      style={{ backgroundColor: slice.color }}
                      aria-hidden
                    />
                    <span className="min-w-0 flex-1 truncate text-foreground/75">
                      {slice.label}
                    </span>
                    <span className="shrink-0 font-mono tabular-nums text-muted-foreground">
                      {formatMs(slice.value)} · {slicePct.toFixed(1)}%
                    </span>
                  </li>
                );
              })}
            </ul>
          </section>
        );
      })}
    </div>
  );
}

function formatMs(value: number): string {
  return `${Math.round(value).toLocaleString()} ms`;
}

function pct(value: number, base: number): number {
  return base > 0 ? (value / base) * 100 : 0;
}

function GroupRow({
  label,
  root,
  slices,
  sliceDefs,
  negativeMethod,
}: {
  label: string;
  root: number;
  slices: Record<string, number>;
  sliceDefs: SliceDef[];
  negativeMethod: boolean;
}): JSX.Element {
  const covered = sliceDefs.reduce((s, d) => s + (slices[d.key] ?? 0), 0);
  const base = Math.max(root, covered, 1);
  return (
    <div className="flex items-center gap-2">
      <div className="w-44 truncate font-mono text-[11px]" title={label}>
        {label}
      </div>
      <div
        className="flex h-4 flex-1 overflow-hidden rounded bg-muted/30"
        title={formatMs(root)}
      >
        {sliceDefs.map((def) => {
          const v = slices[def.key];
          if (!v) return null;
          const widthPct = pct(v, base);
          return (
            <div
              key={def.key}
              style={{ width: `${widthPct}%`, backgroundColor: def.color }}
              title={`${def.label}: ${formatMs(v)}`}
            />
          );
        })}
      </div>
      <div className="w-24 text-right font-mono text-[11px] tabular-nums">
        {formatMs(root)}
      </div>
      {negativeMethod && (
        <span
          className="rounded bg-amber-500/20 px-1.5 py-0.5 text-[10px] text-amber-700 dark:text-amber-300"
          title="categorised time exceeds root response time — likely missing data"
        >
          neg
        </span>
      )}
    </div>
  );
}

function customSliceKey(slice: { id?: string; label?: string }): string {
  return `custom:${String(slice?.id || slice?.label || "custom")}`;
}

function normalizeGroup(group: unknown): SliceGroupId {
  if (group === "database" || group === "external" || group === "internal") {
    return group;
  }
  return "internal";
}
