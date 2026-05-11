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

import { useMemo } from "react";

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
      connection_acquire_ms?: number;
      method_time_ms?: number;
      method_time_ratio?: number;
      coverage?: number;
      negative_method_time?: boolean;
    };
  };
};

type MsaResponseTimeBreakdownProps = {
  groups: GroupMetrics[];
  edges?: any[];
};

const SLICE_DEFS: {
  key: keyof NonNullable<NonNullable<GroupMetrics["metrics"]>["response_time_breakdown"]>;
  label: string;
  color: string;
  hint?: string;
}[] = [
  { key: "sql_execute_ms", label: "SQL", color: "#3b82f6", hint: "DB 쿼리 수행 누적" },
  { key: "check_query_ms", label: "Check Query", color: "#06b6d4", hint: "select 1 같은 헬스체크" },
  { key: "two_pc_ms", label: "2PC / XA", color: "#f59e0b", hint: "분산트랜잭션 prepare/commit" },
  { key: "fetch_ms", label: "Fetch", color: "#fbbf24", hint: "ResultSet fetch" },
  { key: "network_call_ms", label: "Network", color: "#10b981", hint: "외부호출 - callee 응답시간 (실제 망 시간)" },
  { key: "unprofiled_external_call_ms", label: "Unprofiled Call", color: "#14b8a6", hint: "callee profile 없이 caller EXTERNAL_CALL elapsed만 확인된 외부호출" },
  { key: "network_prep_ms", label: "Network Prep", color: "#a78bfa", hint: "sendToService 등 wrapper의 비-네트워크 부분" },
  { key: "connection_acquire_ms", label: "Conn Acquire", color: "#ef4444", hint: "DB 풀 대기" },
  { key: "method_time_ms", label: "Method", color: "#94a3b8", hint: "잔여 메소드 수행시간 (전체 - 위 합계)" },
];

export function MsaResponseTimeBreakdown({
  groups,
}: MsaResponseTimeBreakdownProps): JSX.Element {
  const { totals, totalRoot, totalCovered, aggregateBase, perGroup } =
    useMemo(() => {
      // Aggregate across all groups.
      const nextTotals = SLICE_DEFS.reduce<Record<string, number>>((acc, def) => {
        acc[def.key] = 0;
        return acc;
      }, {});
      let nextTotalRoot = 0;
      for (const g of groups) {
        const bd = g.metrics?.response_time_breakdown;
        if (!bd) continue;
        nextTotalRoot += Number(bd.root_response_time_ms ?? 0);
        for (const def of SLICE_DEFS) {
          nextTotals[def.key] += Number((bd as any)[def.key] ?? 0);
        }
      }
      const nextTotalCovered =
        nextTotals.sql_execute_ms +
        nextTotals.check_query_ms +
        nextTotals.two_pc_ms +
        nextTotals.fetch_ms +
        nextTotals.network_call_ms +
        nextTotals.unprofiled_external_call_ms +
        nextTotals.network_prep_ms +
        nextTotals.connection_acquire_ms +
        nextTotals.method_time_ms;

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
        totals: nextTotals,
        totalRoot: nextTotalRoot,
        totalCovered: nextTotalCovered,
        aggregateBase: nextTotalCovered > 0 ? nextTotalCovered : 1,
        perGroup: nextPerGroup,
      };
    }, [groups]);

  if (totalRoot === 0 && perGroup.length === 0) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">응답시간 구성</CardTitle>
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

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">
          응답시간 구성 (전체 합계 + GUID 그룹별)
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          root 응답시간 = SQL + CheckQuery + 2PC + Fetch + Network + Unprofiled +
          Prep + Conn + Method (잔여). 비중이 큰 카테고리가 우선 개선 대상입니다.
        </p>
      </CardHeader>
      <CardContent className="flex flex-col gap-5">
        {/* Aggregate */}
        <section>
          <header className="mb-1.5 flex items-center justify-between text-xs">
            <span className="font-semibold">전체</span>
            <span className="font-mono text-muted-foreground">
              root 합계 {totalRoot.toLocaleString()} ms · 분해 합계{" "}
              {totalCovered.toLocaleString()} ms
            </span>
          </header>
          <BreakdownBar totals={totals} base={aggregateBase} />
          <Legend totals={totals} base={aggregateBase} />
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
                  slices[def.key] = Number((bd as any)[def.key] ?? 0);
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
function BreakdownBar({
  totals,
  base,
}: {
  totals: Record<string, number>;
  base: number;
}): JSX.Element {
  return (
    <div className="flex h-6 overflow-hidden rounded border border-border bg-muted/30">
      {SLICE_DEFS.map((def) => {
        const v = totals[def.key];
        if (!v) return null;
        const pct = (v / base) * 100;
        return (
          <div
            key={def.key}
            style={{ width: `${pct}%`, backgroundColor: def.color }}
            title={`${def.label}: ${Math.round(v).toLocaleString()} ms (${pct.toFixed(1)}%) — ${def.hint ?? ""}`}
          />
        );
      })}
    </div>
  );
}

function Legend({
  totals,
  base,
}: {
  totals: Record<string, number>;
  base: number;
}): JSX.Element {
  return (
    <ul className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] sm:grid-cols-4">
      {SLICE_DEFS.map((def) => {
        const v = totals[def.key];
        const pct = base > 0 ? (v / base) * 100 : 0;
        return (
          <li
            key={def.key}
            className="flex items-center gap-1.5"
            title={def.hint}
          >
            <span
              className="inline-block h-2.5 w-2.5 rounded-sm"
              style={{ backgroundColor: def.color }}
              aria-hidden
            />
            <span className="text-foreground/70">{def.label}</span>
            <span className="ml-auto font-mono tabular-nums text-muted-foreground">
              {Math.round(v).toLocaleString()} ms · {pct.toFixed(1)}%
            </span>
          </li>
        );
      })}
    </ul>
  );
}

function GroupRow({
  label,
  root,
  slices,
  negativeMethod,
}: {
  label: string;
  root: number;
  slices: Record<string, number>;
  negativeMethod: boolean;
}): JSX.Element {
  const covered = SLICE_DEFS.reduce((s, d) => s + (slices[d.key] ?? 0), 0);
  const base = covered > 0 ? covered : 1;
  return (
    <div className="flex items-center gap-2">
      <div className="w-44 truncate font-mono text-[11px]" title={label}>
        {label}
      </div>
      <div
        className="flex h-4 flex-1 overflow-hidden rounded bg-muted/30"
        title={`${root.toLocaleString()} ms`}
      >
        {SLICE_DEFS.map((def) => {
          const v = slices[def.key];
          if (!v) return null;
          const pct = (v / base) * 100;
          return (
            <div
              key={def.key}
              style={{ width: `${pct}%`, backgroundColor: def.color }}
              title={`${def.label}: ${Math.round(v).toLocaleString()} ms`}
            />
          );
        })}
      </div>
      <div className="w-24 text-right font-mono text-[11px] tabular-nums">
        {root.toLocaleString()} ms
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
