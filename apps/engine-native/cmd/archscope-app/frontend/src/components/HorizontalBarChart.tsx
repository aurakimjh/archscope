// ─────────────────────────────────────────────────────────────────────
// [한글] HorizontalBarChart.tsx — 의존성 없는 가벼운 가로 막대 차트.
//
// 책임/목적: BarRow[] 를 받아 div 기반 막대 트랙을 렌더. d3-scale 없이
// 단순 (value / max) * 100% 비율로 폭 계산. timeline segment 와
// execution breakdown 양쪽에서 재사용되도록 데이터 모양을 일부러 좁게
// 잡았습니다 (label + value + 선택적 ratio).
// ─────────────────────────────────────────────────────────────────────
// Lightweight SVG horizontal-bar chart. No d3 dependency — we already pay
// for d3-hierarchy in the flamegraph but not d3-scale/d3-axis here. The
// data shape is intentionally narrow so it serves both the timeline view
// (segment label + sample count) and the execution breakdown (category
// label + sample count) without a full chart abstraction.

export type BarRow = {
  key: string;
  label: string;
  value: number;
  ratio?: number; // 0..100, optional pre-computed percent
  detail?: string;
};

export type HorizontalBarChartProps = {
  rows: BarRow[];
  emptyLabel?: string;
  /** Optional max value override; defaults to the largest row value. */
  maxValue?: number;
  /** Suffix shown after the value (e.g. "samples", "ms"). */
  valueSuffix?: string;
  /** Number of fraction digits for ratio display (default 1). */
  ratioFractionDigits?: number;
};

const PALETTE = [
  "#6366f1",
  "#22c55e",
  "#f59e0b",
  "#06b6d4",
  "#a855f7",
  "#ec4899",
  "#14b8a6",
  "#ef4444",
  "#84cc16",
  "#0ea5e9",
];

function colorFor(index: number): string {
  return PALETTE[index % PALETTE.length];
}

export function HorizontalBarChart({
  rows,
  emptyLabel = "No data",
  maxValue,
  valueSuffix = "",
  ratioFractionDigits = 1,
}: HorizontalBarChartProps) {
  if (!rows.length) {
    return <p className="muted">{emptyLabel}</p>;
  }
  const total = rows.reduce((acc, row) => acc + row.value, 0) || 1;
  const max = maxValue ?? Math.max(...rows.map((row) => row.value), 1);

  return (
    <div className="bar-chart">
      {rows.map((row, idx) => {
        const ratio = row.ratio != null ? row.ratio : (row.value / total) * 100;
        const widthPct = (row.value / max) * 100;
        return (
          <div key={row.key} className="bar-row">
            <div className="bar-label" title={row.label}>
              {row.label}
            </div>
            <div className="bar-track">
              <div
                className="bar-fill"
                style={{
                  width: `${widthPct.toFixed(2)}%`,
                  background: colorFor(idx),
                }}
              />
            </div>
            <div className="bar-value">
              <span className="bar-count">
                {row.value.toLocaleString()}
                {valueSuffix ? ` ${valueSuffix}` : ""}
              </span>
              {row.detail && <span className="bar-detail">{row.detail}</span>}
              <span className="bar-ratio">{ratio.toFixed(ratioFractionDigits)}%</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
