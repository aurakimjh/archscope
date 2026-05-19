// MsaTimeline — Gantt-style timeline for an MSA call graph.
//
// Two render modes:
//   - "overall": one row per call edge, sorted by caller_event_start_ms.
//     Used in the MSA tab to give a single global view of the trace.
//   - "by-service": one row per service (caller_application). The
//     main caller (root) sits at the top; other services follow in
//     sample order. When `useResponseTimeOrder` is on, calls within
//     a service are positioned by callee_response_time order — the
//     "활성화" toggle in the profile tab.
//
// The chart is built with ECharts `custom` series — each call is a
// horizontal rectangle from (start) to (start + elapsed). Color
// codes the segment (matched / unmatched / 2PC / SQL / network).

import { echarts, type ECharts, type EChartsOption } from "@/charts/echartsCore";
import { HelpTip } from "@/components/HelpTip";
import { getGenericChartHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { useEffect, useMemo, useRef } from "react";

import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import { Button } from "./ui/button";

const DEFAULT_MAX_BARS = 800;
const LABEL_RENDER_LIMIT = 120;
const MAX_TIMELINE_HEIGHT = 760;
const MAX_INDIVIDUAL_TIMELINE_HEIGHT = 1120;
const ROW_HEIGHT = 28;
export type MsaTimelineLayout = "individual" | "grouped";

export type MsaTimelineEdge = {
  caller_application: string;
  callee_application?: string;
  caller_txid?: string;
  callee_txid?: string;
  external_call_elapsed_ms?: number;
  callee_response_time_ms?: number;
  network_gap_ms?: number;
  caller_event_start_ms?: number;
  callee_body_start_ms?: number;
  match_status?: string;
  call_count?: number;
  timeline_kind?: "external_call" | "network_prep";
};

export type MsaTimelineProps = {
  title?: string;
  edges: MsaTimelineEdge[];
  /** Render mode — see file header. */
  mode?: "overall" | "by-service";
  /** Anchors the row order: this service goes on top. */
  rootApplication?: string;
  /** Optional root transaction duration. Rendered as a fixed top-row span. */
  rootDurationMs?: number | null;
  /** Absolute ms-since-midnight of the root START row. Used as x-axis zero. */
  rootStartMs?: number | null;
  /** Profile-tab "활성화" toggle. When true the calls inside one
   *  service are repositioned by callee_response_time_ms order
   *  rather than absolute caller_event_start_ms. */
  useResponseTimeOrder?: boolean;
  layoutToggle?: {
    value: MsaTimelineLayout;
    onChange: (value: MsaTimelineLayout) => void;
  };
  height?: number;
  maxBars?: number;
};

export function MsaTimeline({
  title = "MSA 타임라인",
  edges,
  mode = "overall",
  rootApplication,
  rootDurationMs,
  rootStartMs,
  useResponseTimeOrder = false,
  layoutToggle,
  height,
  maxBars = DEFAULT_MAX_BARS,
}: MsaTimelineProps): JSX.Element {
  const { locale } = useI18n();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ECharts | null>(null);

  const { option, rowCount, visibleCount, totalCount } = useMemo(() => {
    const items = edges.filter(
      (e) =>
        Number.isFinite(e.external_call_elapsed_ms) &&
        (e.external_call_elapsed_ms ?? 0) > 0,
    );
    const rootDuration = Number(rootDurationMs ?? 0);
    const hasRootSpan = Number.isFinite(rootDuration) && rootDuration > 0;
    if (items.length === 0 && !hasRootSpan) {
      return {
        option: {} as EChartsOption,
        rowCount: 0,
        visibleCount: 0,
        totalCount: 0,
      };
    }
    const cappedItems = limitTimelineEdges(items, maxBars, mode);

    // Compute a min-time anchor so the x-axis is "ms since first
    // call in the trace" — easier to read than absolute ms-since-
    // midnight.
    const rootStart = Number(rootStartMs ?? NaN);
    const hasRootStart = Number.isFinite(rootStart) && rootStart > 0;
    const t0 = cappedItems.reduce(
      (acc, e) => Math.min(acc, e.caller_event_start_ms ?? Infinity),
      Infinity,
    );
    const anchor = hasRootStart ? rootStart : Number.isFinite(t0) ? t0 : 0;

    type Bar = {
      row: string;
      start: number;
      end: number;
      label: string;
      color: string;
      tooltip: string;
      rowIndex: number;
      kind: "root" | "call";
    };
    const bars: Bar[] = [];
    const rowSet: string[] = [];
    const rowIndexByName = new Map<string, number>();
    const pushRow = (r: string): number => {
      const existing = rowIndexByName.get(r);
      if (existing != null) return existing;
      const idx = rowSet.length;
      rowSet.push(r);
      rowIndexByName.set(r, idx);
      return idx;
    };
    const pushRootSpan = () => {
      if (!hasRootSpan) return;
      const row = rootApplication ? `최초 서비스 · ${rootApplication}` : "최초 서비스";
      const rowIndex = pushRow(row);
      bars.push({
        row,
        rowIndex,
        start: 0,
        end: rootDuration,
        label: `${rootApplication || "Root"} · ${Math.round(rootDuration).toLocaleString()} ms`,
        color: "#dbeafe",
        tooltip: `${row}<br/>root response = ${Math.round(rootDuration).toLocaleString()} ms`,
        kind: "root",
      });
    };

    pushRootSpan();

    if (mode === "overall") {
      const sorted = [...cappedItems].sort(
        (a, b) =>
          (a.caller_event_start_ms ?? 0) - (b.caller_event_start_ms ?? 0),
      );
      sorted.forEach((e, idx) => {
        const service = e.callee_application || e.caller_application || "?";
        const row = `${idx + 1}. ${service}`;
        const rowIndex = pushRow(row);
        const start = (e.caller_event_start_ms ?? 0) - anchor;
        const elapsed = e.external_call_elapsed_ms ?? 0;
        bars.push({
          row,
          rowIndex,
          start,
          end: start + elapsed,
          label: `${e.caller_application || "?"} → ${e.callee_application || "?"} ${elapsed} ms`,
          color: edgeColor(e),
          tooltip: tooltipFor(e, start, elapsed),
          kind: "call",
        });
      });
    } else {
      // "by-service" — group by caller_application. Within a service,
      // bars are positioned either by absolute start (default) or
      // by response-time order ("활성화" mode).
      const byService = new Map<string, MsaTimelineEdge[]>();
      for (const e of cappedItems) {
        const k = e.caller_application || "?";
        const list = byService.get(k) ?? [];
        list.push(e);
        byService.set(k, list);
      }
      // Order rows: root first, then by total elapsed descending.
      const services = Array.from(byService.keys());
      services.sort((a, b) => {
        if (rootApplication && a === rootApplication) return -1;
        if (rootApplication && b === rootApplication) return 1;
        const aTotal = (byService.get(a) ?? []).reduce(
          (s, e) => s + (e.external_call_elapsed_ms ?? 0),
          0,
        );
        const bTotal = (byService.get(b) ?? []).reduce(
          (s, e) => s + (e.external_call_elapsed_ms ?? 0),
          0,
        );
        return bTotal - aTotal;
      });
      for (const svc of services) {
        const rowIndex = pushRow(svc);
        const calls = byService.get(svc) ?? [];
        if (useResponseTimeOrder) {
          // Response-time mode: bars are stacked left→right in the
          // order of descending callee_response_time_ms — visualises
          // which downstream calls dominated this service's elapsed.
          const ordered = [...calls].sort(
            (a, b) =>
              (b.callee_response_time_ms ?? 0) -
              (a.callee_response_time_ms ?? 0),
          );
          let cursor = 0;
          for (const e of ordered) {
            const elapsed = e.external_call_elapsed_ms ?? 0;
            bars.push({
              row: svc,
              rowIndex,
              start: cursor,
              end: cursor + elapsed,
              label: `${e.callee_application ?? "?"} ${elapsed} ms`,
              color: edgeColor(e),
              tooltip: tooltipFor(e, cursor, elapsed),
              kind: "call",
            });
            cursor += elapsed + 4;
          }
        } else {
          for (const e of calls) {
            const start = (e.caller_event_start_ms ?? 0) - anchor;
            const elapsed = e.external_call_elapsed_ms ?? 0;
            bars.push({
              row: svc,
              rowIndex,
              start,
              end: start + elapsed,
              label: `→ ${e.callee_application ?? "?"} ${elapsed} ms`,
              color: edgeColor(e),
              tooltip: tooltipFor(e, start, elapsed),
              kind: "call",
            });
          }
        }
      }
    }

    const data = bars.map((b) => ({
      value: [b.rowIndex, b.start, b.end, b.label, b.color, b.tooltip, b.kind],
      itemStyle: { color: b.color },
    }));
    const showLabels = bars.length <= LABEL_RENDER_LIMIT;

    const opt: EChartsOption = {
      animation: false,
      tooltip: {
        formatter: (p: any) => p.value?.[5] ?? "",
      },
      grid: { left: 220, right: rowSet.length > 24 ? 44 : 24, top: 24, bottom: 48 },
      xAxis: {
        type: "value",
        name: "ms",
        nameTextStyle: { fontSize: 10 },
        axisLabel: {
          fontSize: 10,
          formatter: (v: number) => `${v} ms`,
        },
      },
      yAxis: {
        type: "category",
        data: rowSet,
        inverse: true,
        axisLabel: { fontSize: 10, width: 200, overflow: "truncate" },
      },
      dataZoom:
        rowSet.length > 24
          ? [
              { type: "slider", xAxisIndex: 0, height: 14, bottom: 8 },
              { type: "inside", xAxisIndex: 0 },
              {
                type: "slider",
                yAxisIndex: 0,
                width: 14,
                right: 4,
                top: 24,
                bottom: 48,
                start: 0,
                end: Math.min(100, (24 / rowSet.length) * 100),
              },
              { type: "inside", yAxisIndex: 0 },
            ]
          : [
              { type: "slider", xAxisIndex: 0, height: 14, bottom: 8 },
              { type: "inside", xAxisIndex: 0 },
            ],
      series: [
        {
          type: "custom",
          encode: { x: [1, 2], y: 0 },
          renderItem: (params: any, api: any) => {
            const rowIdx = api.value(0);
            const startVal = api.value(1);
            const endVal = api.value(2);
            const label = api.value(3) as string;
            const color = api.value(4) as string;
            const kind = api.value(6) as string;
            const start = api.coord([startVal, rowIdx]);
            const end = api.coord([endVal, rowIdx]);
            const isRoot = kind === "root";
            const height = api.size([0, 1])[1] * (isRoot ? 0.82 : 0.55);
            return {
              type: "rect",
              shape: {
                x: start[0],
                y: start[1] - height / 2,
                width: Math.max(2, end[0] - start[0]),
                height,
              },
              style: api.style({
                fill: color,
                stroke: isRoot ? "#2563eb" : "#1e293b",
                lineWidth: isRoot ? 1.2 : 0.5,
                opacity: isRoot ? 0.96 : 0.85,
              }),
              ...(showLabels
                ? {
                    textContent: {
                      style: {
                        text: label,
                        fontSize: 10,
                        fill: isRoot ? "#1e3a8a" : "#0f172a",
                        fontWeight: isRoot ? 700 : 400,
                      },
                    },
                    textConfig: {
                      position: isRoot ? "insideLeft" : "right",
                    },
                  }
                : {}),
            };
          },
          data,
          progressive: 400,
          progressiveThreshold: 800,
        } as any,
      ],
    };
    return {
      option: opt,
      rowCount: rowSet.length,
      visibleCount: cappedItems.length,
      totalCount: items.length,
    };
  }, [edges, maxBars, mode, rootApplication, rootDurationMs, rootStartMs, useResponseTimeOrder]);

  const maxHeight =
    mode === "overall" ? MAX_INDIVIDUAL_TIMELINE_HEIGHT : MAX_TIMELINE_HEIGHT;
  const finalHeight =
    height ?? Math.min(maxHeight, Math.max(220, 60 + rowCount * ROW_HEIGHT));

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = echarts.init(containerRef.current, undefined, {
      renderer: "canvas",
    });
    chartRef.current = chart;
    let resizeFrame = 0;
    const ro = new ResizeObserver(() => {
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      resizeFrame = requestAnimationFrame(() => chart.resize());
    });
    ro.observe(containerRef.current);
    return () => {
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      ro.disconnect();
      chart.dispose();
      if (chartRef.current === chart) chartRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    chartRef.current?.setOption(option, { notMerge: true, lazyUpdate: true });
  }, [option]);

  return (
    <Card>
      <CardHeader className="flex flex-col gap-2 pb-3 sm:flex-row sm:items-center sm:justify-between">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          {title}
          <HelpTip text={getGenericChartHelpText(locale, title)} />
        </CardTitle>
        {layoutToggle && (
          <div className="flex items-center gap-1 rounded-md border border-border bg-muted/20 p-1">
            <Button
              type="button"
              size="sm"
              variant={layoutToggle.value === "individual" ? "default" : "outline"}
              onClick={() => layoutToggle.onChange("individual")}
            >
              개별
            </Button>
            <Button
              type="button"
              size="sm"
              variant={layoutToggle.value === "grouped" ? "default" : "outline"}
              onClick={() => layoutToggle.onChange("grouped")}
            >
              그룹
            </Button>
          </div>
        )}
      </CardHeader>
      <CardContent>
        {rowCount === 0 ? (
          <p className="text-xs text-muted-foreground">
            타임라인에 표시할 외부 호출이 없습니다.
          </p>
        ) : (
          <>
            {visibleCount < totalCount && (
              <p className="mb-2 text-xs text-muted-foreground">
                호출 {visibleCount.toLocaleString()} / {totalCount.toLocaleString()} 표시
              </p>
            )}
            <div ref={containerRef} style={{ width: "100%", height: finalHeight }} />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function MsaTimelineTreemap({
  title = "MSA 트리맵",
  edges,
  rootApplication,
  height = 360,
  maxBars = DEFAULT_MAX_BARS,
}: Pick<
  MsaTimelineProps,
  "title" | "edges" | "rootApplication" | "height" | "maxBars"
>): JSX.Element {
  const { locale } = useI18n();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ECharts | null>(null);

  const { option, visibleCount, totalCount, leafCount } = useMemo(() => {
    const items = edges.filter(
      (e) =>
        Number.isFinite(e.external_call_elapsed_ms) &&
        (e.external_call_elapsed_ms ?? 0) > 0,
    );
    if (items.length === 0) {
      return {
        option: {} as EChartsOption,
        visibleCount: 0,
        totalCount: 0,
        leafCount: 0,
      };
    }

    const cappedItems = limitTimelineEdges(items, maxBars, "by-service");
    const root = buildTreemapRoot(cappedItems, rootApplication);
    const opt: EChartsOption = {
      animation: false,
      tooltip: {
        formatter: (p: any) => {
          const data = p.data ?? {};
          const path = Array.isArray(p.treePathInfo)
            ? p.treePathInfo.map((x: any) => x.name).filter(Boolean).join(" / ")
            : data.name;
          const elapsed = Number(data.value ?? 0);
          const count = Number(data.count ?? 0);
          const gap = Number(data.networkGapMs ?? 0);
          const response = Number(data.responseTimeMs ?? 0);
          const ratio = Number(data.ratioPct ?? 0);
          const lines = [
            path,
            `elapsed = ${Math.round(elapsed).toLocaleString()} ms`,
          ];
          if (ratio > 0) lines.push(`ratio = ${ratio.toFixed(1)}%`);
          if (count > 0) lines.push(`calls = ${count.toLocaleString()}`);
          if (response > 0) {
            lines.push(`callee response = ${Math.round(response).toLocaleString()} ms`);
          }
          if (gap > 0) lines.push(`network gap = ${Math.round(gap).toLocaleString()} ms`);
          if (data.status) lines.push(`status = ${data.status}`);
          return lines.join("<br/>");
        },
      },
      series: [
        {
          type: "treemap",
          roam: false,
          nodeClick: "zoomToNode",
          breadcrumb: { show: false },
          squareRatio: 1.2,
          visibleMin: 8,
          label: {
            show: true,
            position: "inside",
            align: "center",
            verticalAlign: "middle",
            formatter: (p: any) => {
              const count = Number(p.data?.count ?? 0);
              const ratio = Number(p.data?.ratioPct ?? 0);
              const ratioLabel = ratio > 0 ? `${ratio.toFixed(1)}%` : "";
              const countLabel = count > 0 ? `${count.toLocaleString()}건` : "";
              return [p.name, [ratioLabel, countLabel].filter(Boolean).join(" · ")]
                .filter(Boolean)
                .join("\n");
            },
            fontSize: 11,
            color: "#0f172a",
            overflow: "truncate",
          },
          upperLabel: {
            show: false,
          },
          itemStyle: {
            borderColor: "#f8fafc",
            borderWidth: 1,
            gapWidth: 2,
          },
          levels: [
            {
              itemStyle: {
                borderColor: "#e2e8f0",
                borderWidth: 0,
                gapWidth: 4,
              },
            },
            {
              colorSaturation: [0.35, 0.65],
              itemStyle: {
                borderColor: "#f8fafc",
                borderWidth: 2,
                gapWidth: 2,
              },
            },
            {
              colorSaturation: [0.4, 0.75],
              itemStyle: {
                borderColor: "#f8fafc",
                borderWidth: 1,
                gapWidth: 1,
              },
            },
          ],
          data: root.children ?? [],
        } as any,
      ],
    };

    return {
      option: opt,
      visibleCount: cappedItems.length,
      totalCount: items.length,
      leafCount: (root.children ?? []).reduce(
        (sum, node) => sum + (node.children?.length ?? 0),
        0,
      ),
    };
  }, [edges, maxBars, rootApplication]);

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = echarts.init(containerRef.current, undefined, {
      renderer: "canvas",
    });
    chartRef.current = chart;
    let resizeFrame = 0;
    const ro = new ResizeObserver(() => {
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      resizeFrame = requestAnimationFrame(() => chart.resize());
    });
    ro.observe(containerRef.current);
    return () => {
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      ro.disconnect();
      chart.dispose();
      if (chartRef.current === chart) chartRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    chartRef.current?.setOption(option, { notMerge: true, lazyUpdate: true });
  }, [option]);

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          {title}
          <HelpTip text={getGenericChartHelpText(locale, title)} />
        </CardTitle>
        {leafCount > 0 && (
          <p className="text-xs text-muted-foreground">
            사각형 면적은 external call elapsed 합계 기준입니다.
          </p>
        )}
      </CardHeader>
      <CardContent>
        {leafCount === 0 ? (
          <p className="text-xs text-muted-foreground">
            트리맵에 표시할 외부 호출이 없습니다.
          </p>
        ) : (
          <>
            {visibleCount < totalCount && (
              <p className="mb-2 text-xs text-muted-foreground">
                호출 {visibleCount.toLocaleString()} / {totalCount.toLocaleString()} 표시
              </p>
            )}
            <div ref={containerRef} style={{ width: "100%", height }} />
          </>
        )}
      </CardContent>
    </Card>
  );
}

function limitTimelineEdges(
  items: MsaTimelineEdge[],
  maxBars: number,
  mode: "overall" | "by-service",
): MsaTimelineEdge[] {
  if (maxBars <= 0 || items.length <= maxBars) return items;
  if (mode === "overall") {
    const sorted = [...items].sort(
      (a, b) => (a.caller_event_start_ms ?? 0) - (b.caller_event_start_ms ?? 0),
    );
    const out: MsaTimelineEdge[] = [];
    const step = sorted.length / maxBars;
    let lastIndex = -1;
    for (let i = 0; i < maxBars; i += 1) {
      const idx = Math.min(sorted.length - 1, Math.floor(i * step));
      if (idx !== lastIndex) {
        out.push(sorted[idx]);
        lastIndex = idx;
      }
    }
    return out;
  }
  return [...items]
    .sort(
      (a, b) =>
        (b.external_call_elapsed_ms ?? 0) - (a.external_call_elapsed_ms ?? 0),
    )
    .slice(0, maxBars);
}

function edgeColor(e: MsaTimelineEdge): string {
  if (e.timeline_kind === "network_prep" || e.match_status === "NETWORK_PREP") {
    return "#a78bfa";
  }
  if (e.match_status && e.match_status !== "MATCHED") return "#fda4af";
  const gap = e.network_gap_ms ?? 0;
  const total = e.external_call_elapsed_ms ?? 0;
  if (total > 0 && gap / total > 0.3) return "#a78bfa"; // network-heavy
  return "#10b981"; // healthy matched call
}

type TreemapNode = {
  name: string;
  value: number;
  count?: number;
  networkGapMs?: number;
  responseTimeMs?: number;
  ratioPct?: number;
  status?: string;
  itemStyle?: { color: string };
  children?: TreemapNode[];
};

function buildTreemapRoot(
  edges: MsaTimelineEdge[],
  rootApplication?: string,
): TreemapNode {
  const byCaller = new Map<string, Map<string, TreemapNode>>();
  for (const edge of edges) {
    const caller = edge.caller_application || "?";
    const callee = edge.callee_application || "unmatched / unknown";
    const elapsed = edge.external_call_elapsed_ms ?? 0;
    const callerChildren = byCaller.get(caller) ?? new Map<string, TreemapNode>();
    const key = `${caller}\u0000${callee}\u0000${edge.match_status ?? ""}\u0000${edge.timeline_kind ?? ""}`;
    const node = callerChildren.get(key) ?? {
      name: callee,
      value: 0,
      count: 0,
      networkGapMs: 0,
      responseTimeMs: 0,
      status: edge.match_status,
      itemStyle: { color: edgeColor(edge) },
    };
    node.value += elapsed;
    node.count = (node.count ?? 0) + Math.max(1, edge.call_count ?? 1);
    node.networkGapMs = (node.networkGapMs ?? 0) + (edge.network_gap_ms ?? 0);
    node.responseTimeMs =
      (node.responseTimeMs ?? 0) + (edge.callee_response_time_ms ?? 0);
    callerChildren.set(key, node);
    byCaller.set(caller, callerChildren);
  }

  const children = Array.from(byCaller.entries())
    .map(([caller, childMap]) => {
      const serviceChildren = Array.from(childMap.values()).sort(
        (a, b) => b.value - a.value,
      );
      return {
        name: caller,
        value: serviceChildren.reduce((sum, child) => sum + child.value, 0),
        count: serviceChildren.reduce((sum, child) => sum + (child.count ?? 0), 0),
        networkGapMs: serviceChildren.reduce(
          (sum, child) => sum + (child.networkGapMs ?? 0),
          0,
        ),
        responseTimeMs: serviceChildren.reduce(
          (sum, child) => sum + (child.responseTimeMs ?? 0),
          0,
        ),
        children: serviceChildren,
      };
    })
    .sort((a, b) => b.value - a.value);

  const rootName = rootApplication || children[0]?.name || "MSA";
  const rootDirectChildren = children.filter((child) => child.name !== rootName);
  const rootSelf = children.find((child) => child.name === rootName);
  const mergedChildren = rootSelf
    ? [...(rootSelf.children ?? []), ...rootDirectChildren]
    : children;
  const root: TreemapNode = {
    name: rootName,
    value: mergedChildren.reduce((sum, child) => sum + child.value, 0),
    count: mergedChildren.reduce((sum, child) => sum + (child.count ?? 0), 0),
    networkGapMs: mergedChildren.reduce(
      (sum, child) => sum + (child.networkGapMs ?? 0),
      0,
    ),
    responseTimeMs: mergedChildren.reduce(
      (sum, child) => sum + (child.responseTimeMs ?? 0),
      0,
    ),
    children: mergedChildren,
  };
  assignTreemapRatios(root, root.value);
  return root;
}

function assignTreemapRatios(node: TreemapNode, total: number): void {
  node.ratioPct = total > 0 ? (node.value / total) * 100 : 0;
  for (const child of node.children ?? []) {
    assignTreemapRatios(child, total);
  }
}

function tooltipFor(e: MsaTimelineEdge, start: number, elapsed: number): string {
  const lines = [
    `${e.caller_application ?? "?"} → ${e.callee_application ?? "?"}`,
    `start=${start} ms · elapsed=${elapsed} ms`,
  ];
  if (e.callee_response_time_ms != null) {
    lines.push(`callee response = ${e.callee_response_time_ms} ms`);
  }
  if (e.network_gap_ms != null) {
    lines.push(`network gap = ${e.network_gap_ms} ms`);
  }
  if (e.match_status) lines.push(`status = ${e.match_status}`);
  return lines.join("<br/>");
}
