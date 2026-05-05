import * as d3 from "d3";
import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";

import { D3ChartFrame, type D3ChartFrameHandle } from "@/components/charts/D3ChartFrame";
import { useTheme } from "@/components/theme-provider";

export type BarDatum = {
  id?: string;
  label: string;
  /** Optional full label shown in the hover tooltip when the visible label is
   *  truncated (e.g. simple class name visible, full FQN on hover). */
  fullLabel?: string;
  value: number;
  color?: string;
};

export type D3BarChartProps = {
  title: string;
  description?: string;
  exportName?: string;
  data: BarDatum[];
  /** "vertical" (column) or "horizontal" bars. Default: vertical. */
  orientation?: "vertical" | "horizontal";
  height?: number;
  /** When true, sort by value descending. */
  sort?: boolean;
  valueLabel?: string;
  emptyLabel?: string;
};

const DEFAULT_COLORS = ["#6366f1", "#06b6d4", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6"];

type Margin = { top: number; right: number; bottom: number; left: number };

type VerticalLayout = {
  orientation: "vertical";
  margin: Margin;
  innerWidth: number;
  innerHeight: number;
  band: d3.ScaleBand<string>;
  linear: d3.ScaleLinear<number, number>;
};

type HorizontalLayout = {
  orientation: "horizontal";
  margin: Margin;
  innerWidth: number;
  innerHeight: number;
  band: d3.ScaleBand<string>;
  linear: d3.ScaleLinear<number, number>;
};

type Layout = VerticalLayout | HorizontalLayout;

export const D3BarChart = forwardRef<D3ChartFrameHandle, D3BarChartProps>(
  function D3BarChart(
    {
      title,
      description,
      exportName,
      data,
      orientation = "vertical",
      height = 240,
      sort = false,
      valueLabel,
      emptyLabel = "No data",
    },
    ref,
  ) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const frameRef = useRef<D3ChartFrameHandle | null>(null);
    const [width, setWidth] = useState(0);
    const { resolvedTheme } = useTheme();

    useImperativeHandle(ref, () => ({
      get exportTarget() {
        return frameRef.current?.exportTarget ?? null;
      },
    }));

    useEffect(() => {
      const node = containerRef.current;
      if (!node) return undefined;
      const observer = new ResizeObserver((entries) => {
        const entry = entries[0];
        if (entry) setWidth(Math.max(0, Math.floor(entry.contentRect.width)));
      });
      observer.observe(node);
      setWidth(Math.max(0, Math.floor(node.getBoundingClientRect().width)));
      return () => observer.disconnect();
    }, []);

    const sorted = useMemo(() => {
      const cleaned = data.filter((d) => Number.isFinite(d.value));
      if (!sort) return cleaned;
      return [...cleaned].sort((a, b) => b.value - a.value);
    }, [data, sort]);

    const layout = useMemo<Layout | null>(() => {
      if (sorted.length === 0 || width <= 0) return null;
      const margin: Margin =
        orientation === "horizontal"
          ? { top: 10, right: 32, bottom: 24, left: 110 }
          : { top: 10, right: 16, bottom: 36, left: 48 };
      const innerWidth = Math.max(0, width - margin.left - margin.right);
      const innerHeight = Math.max(0, height - margin.top - margin.bottom);

      let valueMin = Math.min(0, d3.min(sorted, (d) => d.value) ?? 0);
      let valueMax = d3.max(sorted, (d) => d.value) ?? 0;
      if (valueMin === valueMax) {
        valueMax += 1;
      }

      const ids = sorted.map((d, idx) => d.id ?? `${idx}`);

      if (orientation === "horizontal") {
        const band = d3.scaleBand<string>().domain(ids).range([0, innerHeight]).padding(0.18);
        const linear = d3
          .scaleLinear()
          .domain([valueMin, valueMax])
          .range([0, innerWidth])
          .nice();
        return { orientation, margin, innerWidth, innerHeight, band, linear };
      }

      const band = d3.scaleBand<string>().domain(ids).range([0, innerWidth]).padding(0.18);
      const linear = d3
        .scaleLinear()
        .domain([valueMin, valueMax])
        .range([innerHeight, 0])
        .nice();
      return { orientation, margin, innerWidth, innerHeight, band, linear };
    }, [sorted, width, height, orientation]);

    const axisColor = resolvedTheme === "dark" ? "#475569" : "#cbd5e1";
    const labelColor = resolvedTheme === "dark" ? "#94a3b8" : "#64748b";

    function colorFor(d: BarDatum, idx: number): string {
      return d.color ?? DEFAULT_COLORS[idx % DEFAULT_COLORS.length];
    }

    return (
      <D3ChartFrame ref={frameRef} title={title} description={description} exportName={exportName}>
        <div ref={containerRef} className="relative w-full bg-card">
          {!layout ? (
            <div className="flex items-center justify-center text-sm text-muted-foreground" style={{ height }}>
              {emptyLabel}
            </div>
          ) : (
            <svg width={width || 0} height={height} role="img" aria-label={title} className="block">
              <g transform={`translate(${layout.margin.left}, ${layout.margin.top})`}>
                {layout.orientation === "vertical" ? (
                  <VerticalBars
                    layout={layout}
                    sorted={sorted}
                    axisColor={axisColor}
                    labelColor={labelColor}
                    valueLabel={valueLabel}
                    colorFor={colorFor}
                  />
                ) : (
                  <HorizontalBars
                    layout={layout}
                    sorted={sorted}
                    axisColor={axisColor}
                    labelColor={labelColor}
                    colorFor={colorFor}
                  />
                )}
              </g>
            </svg>
          )}
        </div>
      </D3ChartFrame>
    );
  },
);

type BarSubProps = {
  sorted: BarDatum[];
  axisColor: string;
  labelColor: string;
  colorFor: (d: BarDatum, idx: number) => string;
};

function VerticalBars({
  layout,
  sorted,
  axisColor,
  labelColor,
  valueLabel,
  colorFor,
}: BarSubProps & { layout: VerticalLayout; valueLabel?: string }): JSX.Element {
  const ticks = layout.linear.ticks(
    Math.max(2, Math.min(6, Math.floor(layout.innerHeight / 40))),
  );
  const baseline = layout.linear(0);
  return (
    <>
      {ticks.map((tick) => (
        <g key={`y-${tick}`} transform={`translate(0, ${layout.linear(tick)})`}>
          <line x2={layout.innerWidth} stroke={axisColor} strokeDasharray="2 4" />
          <text x={-8} dy="0.32em" textAnchor="end" fill={labelColor} fontSize={10}>
            {d3.format(",.2~s")(tick)}
          </text>
        </g>
      ))}
      {sorted.map((d, idx) => {
        const id = d.id ?? `${idx}`;
        const x = layout.band(id);
        if (x === undefined) return null;
        const w = layout.band.bandwidth();
        const yVal = layout.linear(d.value);
        const top = Math.min(baseline, yVal);
        const h = Math.abs(baseline - yVal);
        return (
          <g key={id} transform={`translate(${x}, 0)`}>
            <rect width={w} y={top} height={h} fill={colorFor(d, idx)} rx={3}>
              <title>
                {d.fullLabel ?? d.label}: {d.value.toLocaleString()}
              </title>
            </rect>
            <text
              x={w / 2}
              y={layout.innerHeight + 14}
              textAnchor="middle"
              fontSize={10}
              fill={labelColor}
            >
              {clipText(d.label, w)}
              <title>{d.fullLabel ?? d.label}</title>
            </text>
          </g>
        );
      })}
      {valueLabel && (
        <text
          transform="rotate(-90)"
          x={-layout.innerHeight / 2}
          y={-36}
          textAnchor="middle"
          fill={labelColor}
          fontSize={10}
        >
          {valueLabel}
        </text>
      )}
    </>
  );
}

function HorizontalBars({
  layout,
  sorted,
  axisColor,
  labelColor,
  colorFor,
}: BarSubProps & { layout: HorizontalLayout }): JSX.Element {
  const ticks = layout.linear.ticks(
    Math.max(2, Math.min(6, Math.floor(layout.innerWidth / 80))),
  );
  const baseline = layout.linear(0);
  return (
    <>
      {ticks.map((tick) => (
        <g key={`x-${tick}`} transform={`translate(${layout.linear(tick)}, 0)`}>
          <line y2={layout.innerHeight} stroke={axisColor} strokeDasharray="2 4" />
          <text y={layout.innerHeight + 14} textAnchor="middle" fill={labelColor} fontSize={10}>
            {d3.format(",.2~s")(tick)}
          </text>
        </g>
      ))}
      {sorted.map((d, idx) => {
        const id = d.id ?? `${idx}`;
        const y = layout.band(id);
        if (y === undefined) return null;
        const h = layout.band.bandwidth();
        const xVal = layout.linear(d.value);
        const left = Math.min(baseline, xVal);
        const w = Math.abs(baseline - xVal);
        return (
          <g key={id} transform={`translate(0, ${y})`}>
            <rect x={left} y={0} width={w} height={h} fill={colorFor(d, idx)} rx={3}>
              <title>
                {d.fullLabel ?? d.label}: {d.value.toLocaleString()}
              </title>
            </rect>
            <text
              x={-8}
              y={h / 2}
              dy="0.32em"
              textAnchor="end"
              fontSize={10}
              fill={labelColor}
            >
              {clipText(d.label, layout.margin.left - 16)}
              <title>{d.fullLabel ?? d.label}</title>
            </text>
            <text x={left + w + 6} y={h / 2} dy="0.32em" fontSize={10} fill={labelColor}>
              {d3.format(",.2~s")(d.value)}
            </text>
          </g>
        );
      })}
    </>
  );
}

function clipText(label: string, available: number): string {
  const maxChars = Math.max(2, Math.floor(available / 6.5));
  if (label.length <= maxChars) return label;
  return `${label.slice(0, Math.max(2, maxChars - 1))}…`;
}
