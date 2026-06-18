// MsaMethodHotspots — "메소드 미분류 응답시간" 패널.
//
// Jennifer 프로파일에서 SQL, 외부호출, fetch, Network Prep, 사용자 정의
// 분석 규칙 등으로 식별된 구간을 빼고도 METHOD frame 안에 남는 시간이 있다.
// 엔진이 메소드별 잔여 self-time(= elapsed − 직접 자식 구간 union, 겹침 고려)을
// 계산해 `tables.method_hotspots` 로 내려주면, 이 패널은 현재 드릴다운 스코프
// (txids)에 맞춰 (application, method) 단위로 재집계하여 보여준다.

import { BarChart3, Table2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { useHelpText } from "@/help/helpCatalog";

import { HelpTip } from "./HelpTip";
import { Button } from "./ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";

type MethodHotspotRow = {
  method?: string;
  application?: string;
  txid?: string;
  self_time_ms?: number;
  total_elapsed_ms?: number;
  calls?: number;
  max_self_ms?: number;
  child_method_ms?: number;
  sql_ms?: number;
  external_ms?: number;
  other_ms?: number;
};

type Aggregated = {
  application: string;
  method: string;
  self: number;
  total: number;
  calls: number;
  maxSelf: number;
  childMethod: number;
  sql: number;
  external: number;
  other: number;
};

type AggregationMode = "sum" | "avg";
type ViewMode = "chart" | "table";

type MsaMethodHotspotsProps = {
  /** Raw `tables.method_hotspots` rows (per-profile, per-method). */
  rows: MethodHotspotRow[];
  /** Drilldown scope. When set, only rows whose txid is in the set are
   *  counted (single profile → that profile; group → group Top-N).
   *  Pass null/undefined to include every row. */
  txids?: Set<string> | null;
  title?: string;
  /** How many ranked methods to show before "더 보기". */
  limit?: number;
  /** Initial value mode. Average mode divides totals by sampleCount. */
  defaultAggregationMode?: AggregationMode;
  /** Number of selected transaction samples behind the current scope. */
  sampleCount?: number;
  /** Human-readable current MSA scope. */
  scopeLabel?: string;
};

const CATEGORY = {
  self: { color: "#10b981", label: "미분류 메소드 시간" },
  child_method: { color: "#6366f1", label: "중첩 메소드" },
  sql: { color: "#38bdf8", label: "SQL" },
  external: { color: "#e67e22", label: "외부 호출" },
  other: { color: "#94a3b8", label: "기타" },
} as const;

const KEY_DELIMITER = "\u001f";

function num(v: unknown): number {
  return typeof v === "number" && Number.isFinite(v) ? v : 0;
}

function methodDisplay(signature: string): { name: string; qualifier: string } {
  const trimmed = signature.trim();
  if (!trimmed) return { name: signature, qualifier: "" };
  const paren = trimmed.indexOf("(");
  const head = paren >= 0 ? trimmed.slice(0, paren) : trimmed;
  const normalized = head.replace(/::/g, ".").replace(/[\\/]+/g, ".");
  const parts = normalized.split(".").filter(Boolean);
  if (parts.length === 0) return { name: trimmed, qualifier: "" };
  return {
    name: parts[parts.length - 1],
    qualifier: parts.slice(0, -1).join("."),
  };
}

function formatMs(value: number): string {
  return `${Math.round(value).toLocaleString()}ms`;
}

function formatCount(value: number): string {
  const digits =
    value > 0 && value < 1 ? 2 : Number.isInteger(value) || value >= 10 ? 0 : 1;
  return value.toLocaleString(undefined, { maximumFractionDigits: digits });
}

function formatNumber(value: number): string {
  return Math.round(value).toLocaleString();
}

function aggregate(
  rows: MethodHotspotRow[],
  txids?: Set<string> | null,
): Aggregated[] {
  const map = new Map<string, Aggregated>();
  for (const r of rows ?? []) {
    if (txids && !txids.has(String(r?.txid ?? ""))) continue;
    const method = String(r?.method ?? "");
    if (!method) continue;
    const application = String(r?.application ?? "");
    const key = `${application}${KEY_DELIMITER}${method}`;
    let a = map.get(key);
    if (!a) {
      a = {
        application,
        method,
        self: 0,
        total: 0,
        calls: 0,
        maxSelf: 0,
        childMethod: 0,
        sql: 0,
        external: 0,
        other: 0,
      };
      map.set(key, a);
    }
    a.self += num(r.self_time_ms);
    a.total += num(r.total_elapsed_ms);
    a.calls += num(r.calls);
    a.maxSelf = Math.max(a.maxSelf, num(r.max_self_ms));
    a.childMethod += num(r.child_method_ms);
    a.sql += num(r.sql_ms);
    a.external += num(r.external_ms);
    a.other += num(r.other_ms);
  }
  const out = [...map.values()].filter((a) => a.self > 0);
  out.sort(
    (x, y) =>
      y.self - x.self ||
      y.total - x.total ||
      `${x.application}${x.method}`.localeCompare(`${y.application}${y.method}`),
  );
  return out;
}

function HotspotBreakdownBar({
  a,
  divisor,
  valueLabel,
}: {
  a: Aggregated;
  divisor: number;
  valueLabel: string;
}) {
  const segs: Array<[keyof typeof CATEGORY, number]> = [
    ["self", a.self / divisor],
    ["child_method", a.childMethod / divisor],
    ["sql", a.sql / divisor],
    ["external", a.external / divisor],
    ["other", a.other / divisor],
  ];
  const total = segs.reduce((s, [, v]) => s + Math.max(0, v), 0) || 1;
  return (
    <div className="flex h-2 w-full overflow-hidden rounded-sm bg-muted">
      {segs.map(([k, v]) =>
        v > 0 ? (
          <div
            key={k}
            style={{
              width: `${(v / total) * 100}%`,
              backgroundColor: CATEGORY[k].color,
            }}
            title={`${CATEGORY[k].label} ${valueLabel} ${formatMs(v)}`}
          />
        ) : null,
      )}
    </div>
  );
}

export function MsaMethodHotspots({
  rows,
  txids,
  title = "메소드 미분류 응답시간",
  limit = 10,
  defaultAggregationMode = "sum",
  sampleCount = 1,
  scopeLabel,
}: MsaMethodHotspotsProps) {
  const [expanded, setExpanded] = useState(false);
  const [viewMode, setViewMode] = useState<ViewMode>("chart");
  const [aggregationMode, setAggregationMode] = useState<AggregationMode>(
    defaultAggregationMode,
  );
  const helpText = useHelpText("sectionMsaMethodResidual");
  const agg = useMemo(() => aggregate(rows, txids), [rows, txids]);

  useEffect(() => {
    setAggregationMode(defaultAggregationMode);
  }, [defaultAggregationMode]);

  const normalizedSampleCount = Math.max(1, Math.round(sampleCount || 1));
  const canToggle = normalizedSampleCount > 1;
  const effectiveMode: AggregationMode = canToggle ? aggregationMode : "sum";
  const divisor = effectiveMode === "avg" ? normalizedSampleCount : 1;
  const valueLabel = effectiveMode === "avg" ? "평균" : "합계";
  const shown = expanded ? agg : agg.slice(0, limit);
  const maxSelf = agg.length > 0 ? agg[0].self / divisor : 0;
  const totalSelf = useMemo(
    () => agg.reduce((s, a) => s + a.self / divisor, 0),
    [agg, divisor],
  );
  const scopeSummary = scopeLabel
    ? `${scopeLabel} · ${normalizedSampleCount.toLocaleString()}건`
    : `${normalizedSampleCount.toLocaleString()}건`;

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <CardTitle className="inline-flex items-center gap-2 text-sm">
            {title}
            <HelpTip text={helpText} />
          </CardTitle>
          <div className="flex flex-wrap items-center justify-end gap-2">
            {canToggle ? (
              <div className="flex items-center gap-1 rounded-md border border-border bg-muted/20 p-1">
                <Button
                  type="button"
                  size="sm"
                  variant={effectiveMode === "sum" ? "default" : "outline"}
                  onClick={() => setAggregationMode("sum")}
                  title="선택 범위의 모든 메소드 미분류 시간을 합산"
                >
                  합계
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant={effectiveMode === "avg" ? "default" : "outline"}
                  onClick={() => setAggregationMode("avg")}
                  title="선택된 트랜잭션 표본 수로 나눈 평균 미분류 시간"
                >
                  평균
                </Button>
              </div>
            ) : null}
            <div className="flex items-center gap-1 rounded-md border border-border bg-muted/20 p-1">
              <Button
                type="button"
                size="sm"
                variant={viewMode === "chart" ? "default" : "outline"}
                onClick={() => setViewMode("chart")}
                title="차트 보기"
              >
                <BarChart3 className="h-3.5 w-3.5" />
                차트
              </Button>
              <Button
                type="button"
                size="sm"
                variant={viewMode === "table" ? "default" : "outline"}
                onClick={() => setViewMode("table")}
                title="표 보기"
              >
                <Table2 className="h-3.5 w-3.5" />
                표
              </Button>
            </div>
            <div className="rounded-md border border-border bg-muted/20 px-3 py-2 text-right">
              <div className="text-[10px] font-medium text-muted-foreground">
                {valueLabel} 기준 · {scopeSummary}
              </div>
              <div className="font-mono text-xs tabular-nums text-foreground">
                {agg.length}개 메소드 · 미분류 {valueLabel}{" "}
                {formatMs(totalSelf)}
              </div>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {agg.length === 0 ? (
          <p className="py-3 text-xs text-muted-foreground">
            미분류 메소드 시간이 없습니다 (대부분 SQL·외부 호출·Network
            Prep·사용자 정의 규칙으로 식별되었거나 본문 이벤트에 offset 이
            없습니다).
          </p>
        ) : (
          <>
            {viewMode === "table" ? (
              <div className="overflow-x-auto rounded-md border border-border">
                <table className="min-w-[1120px] w-full text-xs">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-[11px] text-muted-foreground">
                      <th className="px-3 py-2 text-right font-medium">#</th>
                      <th className="px-3 py-2 text-left font-medium">Application</th>
                      <th className="px-3 py-2 text-left font-medium">Method</th>
                      <th className="px-3 py-2 text-right font-medium">Calls</th>
                      <th className="px-3 py-2 text-right font-medium">
                        미분류 {valueLabel} ms
                      </th>
                      <th className="px-3 py-2 text-right font-medium">
                        Total {valueLabel} ms
                      </th>
                      <th className="px-3 py-2 text-right font-medium">
                        Child method {valueLabel} ms
                      </th>
                      <th className="px-3 py-2 text-right font-medium">
                        SQL {valueLabel} ms
                      </th>
                      <th className="px-3 py-2 text-right font-medium">
                        External {valueLabel} ms
                      </th>
                      <th className="px-3 py-2 text-right font-medium">
                        Other {valueLabel} ms
                      </th>
                      <th className="px-3 py-2 text-right font-medium">Max self ms</th>
                      <th className="px-3 py-2 text-right font-medium">Self %</th>
                    </tr>
                  </thead>
                  <tbody>
                    {shown.map((a, i) => {
                      const displaySelf = a.self / divisor;
                      const displayTotal = a.total / divisor;
                      const displayChild = a.childMethod / divisor;
                      const displaySql = a.sql / divisor;
                      const displayExternal = a.external / divisor;
                      const displayOther = a.other / divisor;
                      const displayCalls = a.calls / divisor;
                      const selfRatio = a.total > 0 ? (a.self / a.total) * 100 : 0;
                      return (
                        <tr
                          key={`${a.application}${KEY_DELIMITER}${a.method}`}
                          className="border-b border-border last:border-0"
                        >
                          <td className="px-3 py-2 text-right tabular-nums text-muted-foreground">
                            {i + 1}
                          </td>
                          <td className="max-w-[180px] px-3 py-2 font-mono">
                            <span className="block truncate" title={a.application}>
                              {a.application || "-"}
                            </span>
                          </td>
                          <td className="max-w-[420px] px-3 py-2 font-mono">
                            <span className="block truncate" title={a.method}>
                              {a.method}
                            </span>
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatCount(displayCalls)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(displaySelf)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(displayTotal)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(displayChild)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(displaySql)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(displayExternal)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(displayOther)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {formatNumber(a.maxSelf)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {selfRatio.toFixed(0)}%
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : (
              <>
                <div className="mb-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
                  {Object.entries(CATEGORY).map(([k, c]) => (
                    <span key={k} className="flex items-center gap-1">
                      <span
                        className="inline-block h-2 w-2 rounded-sm"
                        style={{ backgroundColor: c.color }}
                      />
                      {c.label}
                    </span>
                  ))}
                </div>
                <div className="mb-1 hidden items-center gap-2 text-[10px] text-muted-foreground sm:flex">
                  <span className="w-5 shrink-0" />
                  <span className="min-w-0 flex-1">응답시간 구성</span>
                  <span className="w-24 shrink-0 text-center">미분류 시간</span>
                  <span className="w-[8.5rem] shrink-0 text-right">
                    미분류 {valueLabel}
                  </span>
                </div>

                <ol className="space-y-1.5">
                  {shown.map((a, i) => {
                    const displaySelf = a.self / divisor;
                    const selfPct =
                      maxSelf > 0 ? (displaySelf / maxSelf) * 100 : 0;
                    const selfRatio = a.total > 0 ? (a.self / a.total) * 100 : 0;
                    const perCallSelf = a.calls > 0 ? a.self / a.calls : 0;
                    const averageCalls = a.calls / divisor;
                    const method = methodDisplay(a.method);
                    const signatureTitle = `${a.application ? a.application + " · " : ""}${a.method}`;
                    return (
                      <li
                        key={`${a.application}${KEY_DELIMITER}${a.method}`}
                        className="flex items-start gap-2 text-xs"
                        title={signatureTitle}
                      >
                        <span className="w-5 shrink-0 text-right tabular-nums text-muted-foreground">
                          {i + 1}
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex min-w-0 items-center gap-1">
                            {a.application ? (
                              <span className="shrink-0 rounded bg-muted px-1 text-[10px] text-muted-foreground">
                                {a.application}
                              </span>
                            ) : null}
                            <span
                              className="min-w-0 truncate font-medium"
                              title={a.method}
                            >
                              {method.name}
                            </span>
                            <span
                              className="shrink-0 text-[10px] text-muted-foreground"
                              title={
                                effectiveMode === "avg"
                                  ? `총 ${a.calls.toLocaleString()}회 호출`
                                  : undefined
                              }
                            >
                              {effectiveMode === "avg"
                                ? `평균 ${formatCount(averageCalls)}회/트랜잭션`
                                : `총 ${a.calls.toLocaleString()}회`}
                            </span>
                          </div>
                          {method.qualifier ? (
                            <div
                              className="mt-0.5 truncate text-[10px] font-normal text-muted-foreground"
                              title={method.qualifier}
                            >
                              {method.qualifier}
                            </div>
                          ) : null}
                          <div className="mt-1 flex items-center gap-2">
                            <div className="min-w-0 flex-1">
                              <HotspotBreakdownBar
                                a={a}
                                divisor={divisor}
                                valueLabel={valueLabel}
                              />
                            </div>
                            <div className="hidden w-24 shrink-0 sm:block">
                              <div
                                className="h-2 w-full overflow-hidden rounded-sm bg-muted"
                                title={`미분류 ${valueLabel} ${formatMs(displaySelf)}`}
                              >
                                <div
                                  className="h-full rounded-sm"
                                  style={{
                                    width: `${Math.max(selfPct, 2)}%`,
                                    backgroundColor: CATEGORY.self.color,
                                  }}
                                />
                              </div>
                            </div>
                            <span className="w-[8.5rem] shrink-0 text-right tabular-nums">
                              <span className="block font-semibold">
                                {formatMs(displaySelf)}
                              </span>
                              <span className="block text-[10px] text-muted-foreground">
                                {valueLabel} 미분류 · 메소드 내{" "}
                                {selfRatio.toFixed(0)}%
                              </span>
                              {a.calls > 1 ? (
                                <span className="block text-[10px] text-muted-foreground">
                                  호출당 {formatMs(perCallSelf)} · max{" "}
                                  {formatMs(a.maxSelf)}
                                </span>
                              ) : null}
                            </span>
                          </div>
                        </div>
                      </li>
                    );
                  })}
                </ol>
              </>
            )}

            {agg.length > limit ? (
              <button
                type="button"
                onClick={() => setExpanded((v) => !v)}
                className="mt-2 text-xs text-primary hover:underline"
              >
                {expanded ? "접기" : `더 보기 (${agg.length - limit}개)`}
              </button>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  );
}
