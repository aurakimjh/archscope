// MsaTopology — call-graph topology for one or many GUID groups.
//
// Renders an ECharts `graph` series. Nodes are placed on latency
// bands derived from network_gap_ms, so single-digit internal calls
// remain close while gateway/external calls with double-digit gaps
// are pushed into separate columns.

import { echarts, type ECharts, type EChartsOption } from "@/charts/echartsCore";
import { useEffect, useMemo, useRef } from "react";

import { getGenericChartHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { HelpTip } from "./HelpTip";
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

type LatencyGroup = {
  key: string;
  label: string;
  index: number;
  color: string;
};

const LATENCY_GROUPS: LatencyGroup[] = [
  { key: "near_0_4_ms", label: "0-4 ms", index: 0, color: "#0ea5e9" },
  { key: "internal_5_9_ms", label: "5-9 ms", index: 1, color: "#22c55e" },
  {
    key: "cross_service_10_19_ms",
    label: "10-19 ms",
    index: 2,
    color: "#84cc16",
  },
  {
    key: "gateway_external_20_49_ms",
    label: "20-49 ms",
    index: 3,
    color: "#f59e0b",
  },
  { key: "remote_50_99_ms", label: "50-99 ms", index: 4, color: "#f97316" },
  {
    key: "slow_remote_ge_100_ms",
    label: ">=100 ms",
    index: 5,
    color: "#dc2626",
  },
];

export function MsaTopology({
  title = "MSA 호출 토폴로지",
  edges,
  rootApplication,
  height = 420,
}: MsaTopologyProps): JSX.Element {
  const { locale } = useI18n();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ECharts | null>(null);
  const helpText = getGenericChartHelpText(locale, title);

  const { option, latencyGroups } = useMemo<{
    option: EChartsOption;
    latencyGroups: LatencyGroup[];
  }>(() => {
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
        totalCalleeResponse: number;
        totalNetworkGap: number;
        networkSamples: number;
        avgElapsed: number;
        avgCalleeResponse: number;
        avgNetworkGap: number;
        maxElapsed: number;
        maxNetworkGap: number;
      }
    > = {};
    const nodeSet: Record<
      string,
      {
        calls: number;
        inbound: number;
        outbound: number;
        inboundGroupIndex: number;
      }
    > = {};
    for (const edge of edges) {
      const from = edge.caller_application || edge.caller_txid || "?";
      const to = edge.callee_application || edge.callee_txid || "?";
      if (!from || !to) continue;
      nodeSet[from] = nodeSet[from] ?? {
        calls: 0,
        inbound: 0,
        outbound: 0,
        inboundGroupIndex: -1,
      };
      nodeSet[to] = nodeSet[to] ?? {
        calls: 0,
        inbound: 0,
        outbound: 0,
        inboundGroupIndex: -1,
      };
      nodeSet[from].calls += 1;
      nodeSet[from].outbound += 1;
      nodeSet[to].inbound += 1;
      const key = `${from}::${to}`;
      const elapsed = Number(edge.external_call_elapsed_ms ?? 0);
      const calleeResponse = Number(edge.callee_response_time_ms ?? 0);
      const networkGap = networkGapValue(edge);
      const cur = agg[key];
      if (!cur) {
        agg[key] = {
          from,
          to,
          count: 1,
          totalElapsed: elapsed,
          totalCalleeResponse: calleeResponse,
          totalNetworkGap: networkGap ?? 0,
          networkSamples: networkGap == null ? 0 : 1,
          avgElapsed: elapsed,
          avgCalleeResponse: calleeResponse,
          avgNetworkGap: networkGap ?? 0,
          maxElapsed: elapsed,
          maxNetworkGap: networkGap ?? 0,
        };
      } else {
        cur.count += 1;
        cur.totalElapsed += elapsed;
        cur.totalCalleeResponse += calleeResponse;
        if (networkGap != null) {
          cur.totalNetworkGap += networkGap;
          cur.networkSamples += 1;
          cur.maxNetworkGap = Math.max(cur.maxNetworkGap, networkGap);
        }
        cur.maxElapsed = Math.max(cur.maxElapsed, elapsed);
        cur.avgElapsed = cur.totalElapsed / cur.count;
        cur.avgCalleeResponse = cur.totalCalleeResponse / cur.count;
        cur.avgNetworkGap =
          cur.networkSamples > 0 ? cur.totalNetworkGap / cur.networkSamples : 0;
      }
    }
    const aggregatedLinks = Object.values(agg).map((a) => ({
      ...a,
      avgNetworkGap:
        a.networkSamples > 0 ? a.totalNetworkGap / a.networkSamples : 0,
      group: classifyNetworkLatency(
        a.networkSamples > 0 ? a.totalNetworkGap / a.networkSamples : 0,
      ),
    }));
    for (const link of aggregatedLinks) {
      nodeSet[link.to].inboundGroupIndex = Math.max(
        nodeSet[link.to].inboundGroupIndex,
        link.group.index,
      );
    }

    const levels = estimateNodeLevels(
      Object.keys(nodeSet),
      aggregatedLinks,
      rootApplication,
    );
    const levelsByIndex = new Map<number, string[]>();
    for (const name of Object.keys(nodeSet)) {
      const level = levels.get(name) ?? 0;
      const names = levelsByIndex.get(level) ?? [];
      names.push(name);
      levelsByIndex.set(level, names);
    }
    for (const names of levelsByIndex.values()) {
      names.sort((a, b) => {
        if (rootApplication && a === rootApplication) return -1;
        if (rootApplication && b === rootApplication) return 1;
        const ac = nodeSet[a].calls + nodeSet[a].inbound + nodeSet[a].outbound;
        const bc = nodeSet[b].calls + nodeSet[b].inbound + nodeSet[b].outbound;
        if (ac !== bc) return bc - ac;
        return a.localeCompare(b);
      });
    }
    const maxLevel = Math.max(0, ...Array.from(levelsByIndex.keys()));
    const columnWidth = 190;
    const rowHeight = 82;

    const nodes = Object.entries(nodeSet).map(([name, info]) => {
      const isRoot = !!rootApplication && name === rootApplication;
      const group =
        info.inboundGroupIndex >= 0
          ? LATENCY_GROUPS[
              Math.min(info.inboundGroupIndex, LATENCY_GROUPS.length - 1)
            ]
          : LATENCY_GROUPS[0];
      const level = levels.get(name) ?? 0;
      const peers = levelsByIndex.get(level) ?? [name];
      const row = Math.max(0, peers.indexOf(name));
      const x = level * columnWidth - (maxLevel * columnWidth) / 2;
      const y = (row - (peers.length - 1) / 2) * rowHeight;
      return {
        id: name,
        name,
        symbolSize: isRoot ? 60 : Math.min(50, 24 + Math.sqrt(info.calls) * 6),
        x,
        y,
        itemStyle: {
          color: isRoot ? "#ef4444" : group.color,
          borderColor: isRoot ? "#fca5a5" : group.color,
          borderWidth: 2,
        },
        label: {
          show: true,
          position: "right" as const,
          formatter: name,
          fontSize: 11,
        },
        category: isRoot ? 0 : 1,
      };
    });
    const links = aggregatedLinks.map((a) => {
      const width = Math.max(1.5, Math.min(7, 1.2 + a.group.index * 0.8));
      return {
        source: a.from,
        target: a.to,
        value: a.totalElapsed,
        avgElapsed: a.avgElapsed,
        avgCalleeResponse: a.avgCalleeResponse,
        avgNetworkGap: a.avgNetworkGap,
        maxNetworkGap: a.maxNetworkGap,
        count: a.count,
        groupLabel: a.group.label,
        label: {
          show: a.count > 0,
          formatter:
            a.count === 1
              ? `net ${formatMs(a.avgNetworkGap)}`
              : `net ${formatMs(a.avgNetworkGap)} ×${a.count}`,
          fontSize: 10,
        },
        lineStyle: {
          width,
          color: a.group.color,
          curveness: 0.08 + a.group.index * 0.025,
          opacity: 0.9,
        },
      };
    });
    const usedLatencyGroups = Array.from(
      new Map(
        aggregatedLinks.map((link) => [link.group.key, link.group]),
      ).values(),
    ).sort((a, b) => a.index - b.index);

    return {
      latencyGroups: usedLatencyGroups,
      option: {
        animation: false,
        tooltip: {
          formatter: (p: any) => {
            if (p.dataType === "edge") {
              return [
                `${p.data.source} → ${p.data.target}`,
                `network avg=${formatMs(Number(p.data.avgNetworkGap ?? 0))}`,
                `network max=${formatMs(Number(p.data.maxNetworkGap ?? 0))}`,
                `elapsed avg=${formatMs(Number(p.data.avgElapsed ?? 0))}`,
                `callee avg=${formatMs(Number(p.data.avgCalleeResponse ?? 0))}`,
                `group=${p.data.groupLabel ?? "—"}`,
              ].join("<br/>");
            }
            return p.data.name;
          },
        },
        legend: { show: false },
        series: [
          {
            type: "graph",
            layout: "none",
            roam: true,
            draggable: true,
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
      },
    };
  }, [edges, rootApplication]);

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = echarts.init(containerRef.current);
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
    chartRef.current?.setOption(option, { notMerge: true });
  }, [option]);

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          {title}
          <HelpTip text={helpText} />
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div ref={containerRef} style={{ width: "100%", height }} />
        {latencyGroups.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5 text-[11px]">
            {latencyGroups.map((group) => (
              <span
                key={group.key}
                className="rounded border border-border px-1.5 py-0.5 font-mono tabular-nums"
                style={{ color: group.color }}
              >
                {group.label}
              </span>
            ))}
          </div>
        )}
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

function networkGapValue(edge: CallGraphEdge): number | undefined {
  if (edge.network_gap_ms == null) return undefined;
  const value = Number(edge.network_gap_ms);
  return Number.isFinite(value) ? Math.max(0, value) : undefined;
}

function classifyNetworkLatency(ms: number): LatencyGroup {
  if (ms < 5) return LATENCY_GROUPS[0];
  if (ms < 10) return LATENCY_GROUPS[1];
  if (ms < 20) return LATENCY_GROUPS[2];
  if (ms < 50) return LATENCY_GROUPS[3];
  if (ms < 100) return LATENCY_GROUPS[4];
  return LATENCY_GROUPS[5];
}

function latencyStep(group: LatencyGroup): number {
  if (group.index <= 1) return 1;
  return group.index;
}

function estimateNodeLevels(
  nodes: string[],
  links: Array<{ from: string; to: string; group: LatencyGroup }>,
  rootApplication?: string,
): Map<string, number> {
  const inbound = new Map<string, number>();
  for (const node of nodes) inbound.set(node, 0);
  for (const link of links) inbound.set(link.to, (inbound.get(link.to) ?? 0) + 1);

  const roots =
    rootApplication && nodes.includes(rootApplication)
      ? [rootApplication]
      : nodes.filter((node) => (inbound.get(node) ?? 0) === 0);
  const effectiveRoots =
    roots.length > 0
      ? roots
      : [...nodes]
          .sort((a, b) => outboundCount(b, links) - outboundCount(a, links))
          .slice(0, 1);
  const rootSet = new Set(effectiveRoots);
  const levels = new Map<string, number>();
  for (const root of effectiveRoots) levels.set(root, 0);

  const maxLevel = Math.max(1, nodes.length * 2);
  const sortedLinks = [...links].sort((a, b) => {
    const al = levels.get(a.from) ?? Number.MAX_SAFE_INTEGER;
    const bl = levels.get(b.from) ?? Number.MAX_SAFE_INTEGER;
    if (al !== bl) return al - bl;
    if (a.group.index !== b.group.index) return a.group.index - b.group.index;
    return `${a.from}->${a.to}`.localeCompare(`${b.from}->${b.to}`);
  });
  for (let i = 0; i < nodes.length; i += 1) {
    let changed = false;
    for (const link of sortedLinks) {
      const fromLevel = levels.get(link.from);
      if (fromLevel == null || rootSet.has(link.to)) continue;
      const next = Math.min(maxLevel, fromLevel + latencyStep(link.group));
      if (next > (levels.get(link.to) ?? -1)) {
        levels.set(link.to, next);
        changed = true;
      }
    }
    if (!changed) break;
  }
  for (const node of nodes) {
    if (!levels.has(node)) levels.set(node, 0);
  }
  return levels;
}

function outboundCount(
  node: string,
  links: Array<{ from: string; to: string }>,
): number {
  return links.reduce((count, link) => count + (link.from === node ? 1 : 0), 0);
}

function formatMs(value: number): string {
  if (!Number.isFinite(value)) return "—";
  return `${Math.round(value).toLocaleString()} ms`;
}
