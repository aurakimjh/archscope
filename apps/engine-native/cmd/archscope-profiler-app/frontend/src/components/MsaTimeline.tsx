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

import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { useEffect, useMemo, useRef } from "react";

import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";

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
};

export type MsaTimelineProps = {
  title?: string;
  edges: MsaTimelineEdge[];
  /** Render mode — see file header. */
  mode?: "overall" | "by-service";
  /** Anchors the row order: this service goes on top. */
  rootApplication?: string;
  /** Profile-tab "활성화" toggle. When true the calls inside one
   *  service are repositioned by callee_response_time_ms order
   *  rather than absolute caller_event_start_ms. */
  useResponseTimeOrder?: boolean;
  height?: number;
};

export function MsaTimeline({
  title = "MSA 타임라인",
  edges,
  mode = "overall",
  rootApplication,
  useResponseTimeOrder = false,
  height,
}: MsaTimelineProps): JSX.Element {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);

  const { option, rowCount } = useMemo(() => {
    const items = edges.filter(
      (e) =>
        Number.isFinite(e.external_call_elapsed_ms) &&
        (e.external_call_elapsed_ms ?? 0) > 0,
    );
    if (items.length === 0) {
      return { option: {} as EChartsOption, rowCount: 0 };
    }

    // Compute a min-time anchor so the x-axis is "ms since first
    // call in the trace" — easier to read than absolute ms-since-
    // midnight.
    const t0 = items.reduce(
      (acc, e) => Math.min(acc, e.caller_event_start_ms ?? Infinity),
      Infinity,
    );
    const anchor = Number.isFinite(t0) ? t0 : 0;

    type Bar = {
      row: string;
      start: number;
      end: number;
      label: string;
      color: string;
      tooltip: string;
    };
    const bars: Bar[] = [];
    const rowSet: string[] = [];
    const pushRow = (r: string) => {
      if (!rowSet.includes(r)) rowSet.push(r);
    };

    if (mode === "overall") {
      const sorted = [...items].sort(
        (a, b) =>
          (a.caller_event_start_ms ?? 0) - (b.caller_event_start_ms ?? 0),
      );
      sorted.forEach((e, idx) => {
        const row = `${idx + 1}. ${e.caller_application || "?"} → ${e.callee_application || "?"}`;
        pushRow(row);
        const start = (e.caller_event_start_ms ?? 0) - anchor;
        const elapsed = e.external_call_elapsed_ms ?? 0;
        bars.push({
          row,
          start,
          end: start + elapsed,
          label: `${elapsed} ms`,
          color: edgeColor(e),
          tooltip: tooltipFor(e, start, elapsed),
        });
      });
    } else {
      // "by-service" — group by caller_application. Within a service,
      // bars are positioned either by absolute start (default) or
      // by response-time order ("활성화" mode).
      const byService = new Map<string, MsaTimelineEdge[]>();
      for (const e of items) {
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
        pushRow(svc);
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
              start: cursor,
              end: cursor + elapsed,
              label: `${e.callee_application ?? "?"} ${elapsed} ms`,
              color: edgeColor(e),
              tooltip: tooltipFor(e, cursor, elapsed),
            });
            cursor += elapsed + 4;
          }
        } else {
          for (const e of calls) {
            const start = (e.caller_event_start_ms ?? 0) - anchor;
            const elapsed = e.external_call_elapsed_ms ?? 0;
            bars.push({
              row: svc,
              start,
              end: start + elapsed,
              label: `→ ${e.callee_application ?? "?"} ${elapsed} ms`,
              color: edgeColor(e),
              tooltip: tooltipFor(e, start, elapsed),
            });
          }
        }
      }
    }

    const data = bars.map((b) => ({
      value: [rowSet.indexOf(b.row), b.start, b.end, b.label, b.color, b.tooltip],
      itemStyle: { color: b.color },
    }));

    const opt: EChartsOption = {
      tooltip: {
        formatter: (p: any) => p.value?.[5] ?? "",
      },
      grid: { left: 220, right: 24, top: 24, bottom: 48 },
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
      dataZoom: [
        { type: "slider", height: 14, bottom: 8 },
        { type: "inside" },
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
            const start = api.coord([startVal, rowIdx]);
            const end = api.coord([endVal, rowIdx]);
            const height = api.size([0, 1])[1] * 0.55;
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
                stroke: "#1e293b",
                lineWidth: 0.5,
                opacity: 0.85,
              }),
              textContent: {
                style: {
                  text: label,
                  fontSize: 10,
                  fill: "#0f172a",
                },
              },
              textConfig: {
                position: "right",
              },
            };
          },
          data,
        } as any,
      ],
    };
    return { option: opt, rowCount: rowSet.length };
  }, [edges, mode, rootApplication, useResponseTimeOrder]);

  const finalHeight = height ?? Math.max(220, 60 + rowCount * 28);

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = echarts.init(containerRef.current);
    chartRef.current = chart;
    chart.setOption(option, { notMerge: true });
    const ro = new ResizeObserver(() => chart.resize());
    ro.observe(containerRef.current);
    return () => {
      ro.disconnect();
      chart.dispose();
      if (chartRef.current === chart) chartRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    chartRef.current?.setOption(option, { notMerge: true });
  }, [option]);

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {rowCount === 0 ? (
          <p className="text-xs text-muted-foreground">
            타임라인에 표시할 외부 호출이 없습니다.
          </p>
        ) : (
          <div ref={containerRef} style={{ width: "100%", height: finalHeight }} />
        )}
      </CardContent>
    </Card>
  );
}

function edgeColor(e: MsaTimelineEdge): string {
  if (e.match_status && e.match_status !== "MATCHED") return "#fda4af";
  const gap = e.network_gap_ms ?? 0;
  const total = e.external_call_elapsed_ms ?? 0;
  if (total > 0 && gap / total > 0.3) return "#a78bfa"; // network-heavy
  return "#10b981"; // healthy matched call
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
