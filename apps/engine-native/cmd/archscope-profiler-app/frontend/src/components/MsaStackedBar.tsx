// MsaStackedBar — per-GUID-group stacked bar showing the cumulative
// time each transaction spent across the MSA categories the user
// asked for (2PC / CHECK_QUERY / SQL / EXTERNAL_CALL / NETWORK_PREP /
// NETWORK_GAP). Each bar is one GUID group; widths are proportional
// to total time so the eye can compare contributors across groups.
//
// This is the "summary" panel in the Swimlane + Summary stacked-bar
// layout the user requested. The full per-call swimlane requires
// per-event offset data threaded through the API and lands in a
// follow-up; the stacked bar already answers "where is the time
// going" for any given GUID without that plumbing.

type Group = {
  guid?: string;
  root_application?: string;
  metrics?: Record<string, number | string>;
};

type Slice = {
  key: string;
  label: string;
  value: number;
  color: string;
};

const SLICE_DEFS: { key: string; label: string; color: string }[] = [
  { key: "total_sql_execute_ms", label: "SQL", color: "#3b82f6" },
  { key: "total_check_query_ms", label: "CHECK_QUERY", color: "#06b6d4" },
  { key: "total_two_pc_ms", label: "2PC / XA", color: "#f59e0b" },
  { key: "total_external_call_cumulative_ms", label: "EXTERNAL_CALL", color: "#10b981" },
  { key: "total_network_gap_cumulative_ms", label: "NETWORK_GAP", color: "#a78bfa" },
  { key: "total_connection_acquire_ms", label: "CONN_ACQUIRE", color: "#ef4444" },
];

export type MsaStackedBarProps = {
  groups: Group[];
  /** Limits how many groups the chart renders. Older groups
   *  (descending total) take precedence. */
  maxGroups?: number;
};

export function MsaStackedBar({ groups, maxGroups = 20 }: MsaStackedBarProps) {
  const rows = groups
    .map((g) => {
      const m = g.metrics ?? {};
      const slices: Slice[] = SLICE_DEFS.map((def) => ({
        key: def.key,
        label: def.label,
        value: Number(m[def.key] ?? 0),
        color: def.color,
      }));
      const total = slices.reduce((s, x) => s + x.value, 0);
      return { group: g, slices, total };
    })
    .filter((r) => r.total > 0)
    .sort((a, b) => b.total - a.total)
    .slice(0, maxGroups);

  if (rows.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        합계가 0인 그룹뿐입니다 — MSA 캡처가 비어있는지 파서 보고서 탭에서
        확인하세요.
      </p>
    );
  }

  const maxTotal = Math.max(...rows.map((r) => r.total));

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-3 text-xs">
        {SLICE_DEFS.map((d) => (
          <span key={d.key} className="inline-flex items-center gap-1.5">
            <span
              className="inline-block h-3 w-3 rounded-sm"
              style={{ backgroundColor: d.color }}
            />
            {d.label}
          </span>
        ))}
      </div>
      <div className="flex flex-col gap-1.5">
        {rows.map(({ group, slices, total }) => {
          const widthPct = maxTotal > 0 ? (total / maxTotal) * 100 : 0;
          const guid = group.guid ?? "";
          return (
            <div key={guid} className="flex items-center gap-2">
              <div className="w-36 truncate font-mono text-[11px]" title={guid}>
                {group.root_application
                  ? `${group.root_application} · …${guid.slice(-8)}`
                  : `…${guid.slice(-12)}`}
              </div>
              <div
                className="relative h-5 flex-1 overflow-hidden rounded bg-muted/40"
                title={`Total ${total.toLocaleString()} ms`}
              >
                <div
                  className="flex h-full"
                  style={{ width: `${widthPct}%` }}
                >
                  {slices.map((s) => {
                    if (s.value <= 0) return null;
                    const sliceWidth = (s.value / total) * 100;
                    return (
                      <div
                        key={s.key}
                        style={{
                          width: `${sliceWidth}%`,
                          backgroundColor: s.color,
                        }}
                        title={`${s.label}: ${s.value.toLocaleString()} ms`}
                      />
                    );
                  })}
                </div>
              </div>
              <div className="w-24 text-right font-mono text-[11px] tabular-nums">
                {total.toLocaleString()} ms
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
