// MsaTopology — call-graph topology for one or many GUID groups.
//
// Renders an ECharts `graph` series in force-directed layout. Each
// node is a profile (caller or callee), edges are the matched
// external calls. Edge label shows external_call_elapsed_ms so
// users can see where the latency lives. The page picks a single
// GUID group to focus on; all groups can be merged into one big
// topology if the user wants the whole system view.

import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { useEffect, useMemo, useRef } from "react";

import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";

type CallGraphEdge = {
  caller_txid: string;
  caller_application: string;
  callee_txid: string;
  callee_application: string;
  external_call_elapsed_ms?: number;
  callee_response_time_ms?: number;
  network_gap_ms?: number;
  caller_event_start_ms?: number;
  callee_body_start_ms?: number;
};

export type MsaTopologyProps = {
  /** Optional title override; defaults to "MSA topology". */
  title?: string;
  /** All call-graph edges across the result. The component groups
   *  by application and renders one graph; if a GUID filter is
   *  supplied upstream, pass the filtered edge list. */
  edges: CallGraphEdge[];
  /** Optional root application name; the corresponding node is
   *  rendered larger and at the top of the layout. */
  rootApplication?: string;
  height?: number;
};

export function MsaTopology({
  title = "MSA 호출 토폴로지",
  edges,
  rootApplication,
  height = 420,
}: MsaTopologyProps): JSX.Element {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);

  const option = useMemo<EChartsOption>(() => {
    // Aggregate edges by (caller_application -> callee_application).
    // Multiple calls between the same pair fold into one edge with
    // the summed elapsed and the call count for tooltip detail.
    type AggKey = string;
    const agg: Record<
      AggKey,
      {
        from: string;
        to: string;
        count: number;
        totalElapsed: number;
        avgElapsed: number;
        maxElapsed: number;
      }
    > = {};
    const nodeSet: Record<string, { samples: number; calls: number }> = {};
    for (const edge of edges) {
      const from = edge.caller_application || edge.caller_txid || "?";
      const to = edge.callee_application || edge.callee_txid || "?";
      if (!from || !to) continue;
      nodeSet[from] = nodeSet[from] ?? { samples: 0, calls: 0 };
      nodeSet[to] = nodeSet[to] ?? { samples: 0, calls: 0 };
      nodeSet[from].calls += 1;
      const key = `${from}::${to}`;
      const elapsed = Number(edge.external_call_elapsed_ms ?? 0);
      const cur = agg[key];
      if (!cur) {
        agg[key] = {
          from,
          to,
          count: 1,
          totalElapsed: elapsed,
          avgElapsed: elapsed,
          maxElapsed: elapsed,
        };
      } else {
        cur.count += 1;
        cur.totalElapsed += elapsed;
        cur.maxElapsed = Math.max(cur.maxElapsed, elapsed);
        cur.avgElapsed = cur.totalElapsed / cur.count;
      }
    }
    const nodes = Object.entries(nodeSet).map(([name, info]) => {
      const isRoot = !!rootApplication && name === rootApplication;
      return {
        id: name,
        name,
        symbolSize: isRoot ? 60 : Math.min(50, 24 + Math.sqrt(info.calls) * 6),
        itemStyle: {
          color: isRoot ? "#ef4444" : "#0ea5e9",
          borderColor: isRoot ? "#fca5a5" : "#7dd3fc",
          borderWidth: 2,
        },
        label: {
          show: true,
          position: "right" as const,
          formatter: name,
          fontSize: 11,
        },
        category: isRoot ? 0 : 1,
        // Pin root to the top center; let the force layout position
        // the rest. ECharts uses fixed coords when both are present.
        ...(isRoot ? { x: 0, y: -200, fixed: true } : {}),
      };
    });
    const links = Object.values(agg).map((a) => ({
      source: a.from,
      target: a.to,
      value: a.totalElapsed,
      label: {
        show: a.count > 0,
        formatter:
          a.count === 1
            ? `${Math.round(a.avgElapsed).toLocaleString()} ms`
            : `${Math.round(a.avgElapsed).toLocaleString()} ms ×${a.count}`,
        fontSize: 10,
      },
      lineStyle: {
        width: Math.max(1, Math.min(6, Math.log10(a.totalElapsed + 1))),
        color: a.maxElapsed > 1000 ? "#dc2626" : "#94a3b8",
        curveness: 0.15,
      },
    }));

    return {
      tooltip: {
        formatter: (p: any) => {
          if (p.dataType === "edge") {
            return `${p.data.source} → ${p.data.target}<br/>elapsed=${Number(p.data.value).toLocaleString()} ms`;
          }
          return p.data.name;
        },
      },
      legend: { show: false },
      series: [
        {
          type: "graph",
          layout: "force",
          roam: true,
          draggable: true,
          force: {
            repulsion: 220,
            edgeLength: [80, 200],
            gravity: 0.1,
          },
          edgeSymbol: ["none", "arrow"],
          edgeSymbolSize: [0, 8],
          data: nodes,
          links,
          categories: [{ name: "root" }, { name: "service" }],
          emphasis: {
            focus: "adjacency",
          },
        },
      ],
    };
  }, [edges, rootApplication]);

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
        <div ref={containerRef} style={{ width: "100%", height }} />
        {edges.length === 0 && (
          <p className="mt-2 text-xs text-muted-foreground">
            매칭된 외부 호출이 없습니다 — 캡처된 EXTERNAL_CALL이 없거나 매칭 점수
            (containment + 시간 일치)가 모두 임계 미만입니다.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
