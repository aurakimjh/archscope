import * as d3 from "d3";
import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";

import { D3ChartFrame, type D3ChartFrameHandle } from "@/components/charts/D3ChartFrame";
import { useTheme } from "@/components/theme-provider";

export type TimelinePoint = {
  time: Date | number | string;
  value: number;
};

export type TimelineSeries = {
  id: string;
  label: string;
  color?: string;
  data: TimelinePoint[];
  /** When true, fill the area under the line. */
  area?: boolean;
  /** When true, render circle markers at each data point. */
  showPoints?: boolean;
};

export type TimelineEvent = {
  time: Date | number | string;
  label?: string;
  color?: string;
};

export type D3TimelineChartProps = {
  title: string;
  description?: string;
  exportName?: string;
  /** Multiple line series share an x axis (time) and y axis (linear). */
  series: TimelineSeries[];
  /** Optional vertical event markers (e.g. GC events). */
  events?: TimelineEvent[];
  /** Pixel height for the SVG plot area. */
  height?: number;
  /** Y axis label rendered next to the axis. */
  yLabel?: string;
  /** Empty-state copy. */
  emptyLabel?: string;
};

const DEFAULT_COLORS = ["#6366f1", "#06b6d4", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6"];

function toDate(value: Date | number | string): Date {
  if (value instanceof Date) return value;
  if (typeof value === "number") return new Date(value);
  return new Date(value);
}

export const D3TimelineChart = forwardRef<D3ChartFrameHandle, D3TimelineChartProps>(
  function D3TimelineChart(
    {
      title,
      description,
      exportName,
      series,
      events,
      height = 240,
      yLabel,
      emptyLabel = "No data",
    },
    ref,
  ) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const frameRef = useRef<D3ChartFrameHandle | null>(null);
    const [width, setWidth] = useState(0);
    const [hover, setHover] = useState<{ x: number; y: number; time: Date; rows: Array<{ id: string; label: string; value: number; color: string }> } | null>(null);
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

    const layout = useMemo(() => {
      if (!series.length || width <= 0) return null;
      const margin = { top: 10, right: 16, bottom: 26, left: 48 };
      const innerWidth = Math.max(0, width - margin.left - margin.right);
      const innerHeight = Math.max(0, height - margin.top - margin.bottom);

      const allTimes: Date[] = [];
      let yMin = Number.POSITIVE_INFINITY;
      let yMax = Number.NEGATIVE_INFINITY;
      const normSeries = series.map((s, idx) => {
        const points = s.data
          .map((p) => ({ time: toDate(p.time), value: p.value }))
          .filter((p) => Number.isFinite(p.value) && !Number.isNaN(p.time.getTime()))
          .sort((a, b) => a.time.getTime() - b.time.getTime());
        for (const p of points) {
          allTimes.push(p.time);
          if (p.value < yMin) yMin = p.value;
          if (p.value > yMax) yMax = p.value;
        }
        return {
          ...s,
          color: s.color ?? DEFAULT_COLORS[idx % DEFAULT_COLORS.length],
          points,
        };
      });

      if (allTimes.length === 0) return null;

      const xExtent = d3.extent(allTimes) as [Date, Date];
      const x = d3
        .scaleTime()
        .domain(xExtent[0].getTime() === xExtent[1].getTime()
          ? [new Date(xExtent[0].getTime() - 1), new Date(xExtent[1].getTime() + 1)]
          : xExtent)
        .range([0, innerWidth])
        .nice();

      if (yMin === yMax) {
        yMin -= 1;
        yMax += 1;
      }
      if (yMin > 0) yMin = 0;
      const y = d3
        .scaleLinear()
        .domain([yMin, yMax])
        .range([innerHeight, 0])
        .nice();

      const line = d3
        .line<{ time: Date; value: number }>()
        .x((p) => x(p.time))
        .y((p) => y(p.value))
        .curve(d3.curveMonotoneX);

      const area = d3
        .area<{ time: Date; value: number }>()
        .x((p) => x(p.time))
        .y0(y(yMin))
        .y1((p) => y(p.value))
        .curve(d3.curveMonotoneX);

      const tickFormat = d3.timeFormat(
        (xExtent[1].getTime() - xExtent[0].getTime()) > 1000 * 60 * 60 * 24
          ? "%m-%d %H:%M"
          : "%H:%M:%S",
      );
      const xTicks = x.ticks(Math.max(2, Math.min(8, Math.floor(innerWidth / 100)))).map((t) => ({
        x: x(t),
        label: tickFormat(t as Date),
      }));
      const yTicks = y.ticks(Math.max(2, Math.min(6, Math.floor(innerHeight / 40)))).map((t) => ({
        y: y(t),
        label: d3.format(",.2~s")(t as number),
      }));

      const seriesOut = normSeries.map((s) => ({
        ...s,
        path: s.points.length > 0 ? line(s.points) ?? "" : "",
        areaPath: s.area && s.points.length > 0 ? area(s.points) ?? "" : "",
      }));

      const eventsOut = (events ?? [])
        .map((e) => {
          const t = toDate(e.time);
          if (Number.isNaN(t.getTime())) return null;
          return {
            x: x(t),
            label: e.label,
            color: e.color ?? "#ef4444",
          };
        })
        .filter((e): e is NonNullable<typeof e> => Boolean(e));

      return {
        margin,
        innerWidth,
        innerHeight,
        x,
        y,
        xTicks,
        yTicks,
        seriesOut,
        eventsOut,
        bisectTime: d3.bisector<{ time: Date; value: number }, Date>((d) => d.time).left,
      };
    }, [series, events, width, height]);

    const axisColor = resolvedTheme === "dark" ? "#475569" : "#cbd5e1";
    const labelColor = resolvedTheme === "dark" ? "#94a3b8" : "#64748b";

    function handleMove(event: React.MouseEvent<SVGRectElement>): void {
      if (!layout) return;
      const containerRect = containerRef.current?.getBoundingClientRect();
      const svgRect = (event.currentTarget.ownerSVGElement as SVGSVGElement).getBoundingClientRect();
      const localX = event.clientX - svgRect.left - layout.margin.left;
      if (localX < 0 || localX > layout.innerWidth) {
        setHover(null);
        return;
      }
      const time = layout.x.invert(localX);
      const rows = layout.seriesOut
        .map((s) => {
          if (s.points.length === 0) return null;
          const idx = layout.bisectTime(s.points, time);
          const before = s.points[Math.max(0, idx - 1)];
          const after = s.points[Math.min(s.points.length - 1, idx)];
          const closest =
            !before
              ? after
              : !after
              ? before
              : Math.abs(before.time.getTime() - time.getTime()) <
                Math.abs(after.time.getTime() - time.getTime())
              ? before
              : after;
          if (!closest) return null;
          return {
            id: s.id,
            label: s.label,
            value: closest.value,
            color: s.color,
          };
        })
        .filter((r): r is NonNullable<typeof r> => Boolean(r));

      if (containerRect) {
        setHover({
          x: event.clientX - containerRect.left,
          y: event.clientY - containerRect.top,
          time,
          rows,
        });
      }
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
                {layout.yTicks.map((tick) => (
                  <g key={`y-${tick.label}-${tick.y}`} transform={`translate(0, ${tick.y})`}>
                    <line x2={layout.innerWidth} stroke={axisColor} strokeDasharray="2 4" />
                    <text x={-8} dy="0.32em" textAnchor="end" fill={labelColor} fontSize={10}>
                      {tick.label}
                    </text>
                  </g>
                ))}
                {layout.xTicks.map((tick) => (
                  <g key={`x-${tick.label}-${tick.x}`} transform={`translate(${tick.x}, ${layout.innerHeight})`}>
                    <line y2={6} stroke={axisColor} />
                    <text y={18} textAnchor="middle" fill={labelColor} fontSize={10}>
                      {tick.label}
                    </text>
                  </g>
                ))}
                {yLabel && (
                  <text
                    transform={`rotate(-90)`}
                    x={-layout.innerHeight / 2}
                    y={-36}
                    textAnchor="middle"
                    fill={labelColor}
                    fontSize={10}
                  >
                    {yLabel}
                  </text>
                )}

                {layout.eventsOut.map((event, idx) => (
                  <g key={`event-${idx}`} transform={`translate(${event.x}, 0)`}>
                    <line y2={layout.innerHeight} stroke={event.color} strokeOpacity={0.4} strokeDasharray="3 3" />
                  </g>
                ))}

                {layout.seriesOut.map((s) => (
                  <g key={s.id}>
                    {s.areaPath && (
                      <path d={s.areaPath} fill={s.color} fillOpacity={0.12} />
                    )}
                    {s.path && (
                      <path d={s.path} fill="none" stroke={s.color} strokeWidth={1.6} />
                    )}
                    {s.showPoints &&
                      s.points.map((p, i) => (
                        <circle
                          key={`${s.id}-${i}`}
                          cx={layout.x(p.time)}
                          cy={layout.y(p.value)}
                          r={2.4}
                          fill={s.color}
                        />
                      ))}
                  </g>
                ))}

                <rect
                  width={layout.innerWidth}
                  height={layout.innerHeight}
                  fill="transparent"
                  onMouseMove={handleMove}
                  onMouseLeave={() => setHover(null)}
                />

                {hover && (
                  <line
                    x1={layout.x(hover.time)}
                    x2={layout.x(hover.time)}
                    y1={0}
                    y2={layout.innerHeight}
                    stroke={labelColor}
                    strokeOpacity={0.5}
                    strokeDasharray="2 3"
                  />
                )}
              </g>

              <g transform={`translate(${layout.margin.left}, ${height - 6})`}>
                {layout.seriesOut.map((s, idx) => (
                  <g key={`legend-${s.id}`} transform={`translate(${idx * 120}, 0)`}>
                    <rect width={10} height={2} y={-3} fill={s.color} />
                    <text x={14} y={0} fontSize={10} fill={labelColor}>
                      {s.label}
                    </text>
                  </g>
                ))}
              </g>
            </svg>
          )}
          {hover && layout && (
            <div
              className="pointer-events-none absolute z-10 max-w-xs rounded-md border border-border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg"
              style={{
                left: Math.min(hover.x + 12, (containerRef.current?.clientWidth ?? 0) - 220),
                top: Math.max(0, hover.y - 60),
              }}
            >
              <p className="font-mono text-[11px] leading-tight">
                {d3.timeFormat("%Y-%m-%d %H:%M:%S")(hover.time)}
              </p>
              <table className="mt-1 text-[11px]">
                <tbody>
                  {hover.rows.map((row) => (
                    <tr key={row.id}>
                      <td className="pr-2">
                        <span
                          className="mr-1 inline-block h-2 w-2 rounded-sm"
                          style={{ background: row.color }}
                        />
                        {row.label}
                      </td>
                      <td className="text-right tabular-nums">
                        {Number.isFinite(row.value) ? row.value.toLocaleString() : "-"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </D3ChartFrame>
    );
  },
);
