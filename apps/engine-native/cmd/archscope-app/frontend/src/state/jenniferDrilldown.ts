import type { AnalysisResult } from "../bridge/types";

export type JenniferDrilldownSelection = {
  mode: "single" | "average";
  guid?: string;
  txid?: string;
  signature_hash?: string;
  application?: string;
};
export type MsaDrilldownScope = {
  group: any;
  profileRows: any[];
  timelineEdges: any[];
  topologyEdges: any[];
  serviceNetworkEdges: any[];
  slowSqlRows: any[];
  networkTimeByTxid: Map<string, number>;
  selectedValue: string;
  selectedApplication: string;
  selectedTxid?: string;
  isWholeTransaction: boolean;
};

// Wire adaptation only. Selection, traversal and costs belong to Go.
export function readJenniferDrilldown(result: AnalysisResult): MsaDrilldownScope[] {
  const contract = result.metadata.drilldown as any;
  if (result.type !== "jennifer_profile" || contract?.version !== "1.0.0" ||
      !Array.isArray(result.series.drilldown_scopes)) throw new Error("Unsupported Jennifer drilldown contract");
  return (result.series.drilldown_scopes as any[]).map((scope) => {
    for (const key of ["profile_rows", "timeline_edges", "topology_edges", "service_network_edges", "slow_sql_rows"]) {
      if (!Array.isArray(scope[key])) throw new Error(`Invalid Jennifer drilldown ${key}`);
    }
    if (!scope.group || typeof scope.is_whole_transaction !== "boolean") throw new Error("Invalid Jennifer drilldown scope");
    return {
      group: scope.group, profileRows: scope.profile_rows, timelineEdges: scope.timeline_edges,
      topologyEdges: scope.topology_edges, serviceNetworkEdges: scope.service_network_edges,
      slowSqlRows: scope.slow_sql_rows,
      networkTimeByTxid: new Map(Object.entries(scope.network_time_by_txid ?? {}) as [string, number][]),
      selectedValue: scope.selected_value, selectedApplication: scope.selected_application,
      selectedTxid: scope.selected_txid || undefined, isWholeTransaction: scope.is_whole_transaction,
    };
  });
}

// Equal selectors in a new source must not reuse or export an old projection.
export function isCurrentJenniferProjection(snapshot: { source: unknown; key: string } | null, source: unknown, key: string): boolean {
  return snapshot !== null && snapshot.source === source && snapshot.key === key;
}
