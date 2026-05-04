import * as d3 from "d3";
import { RotateCcw } from "lucide-react";
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from "react";

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
  /** Optional payload echoed back when the event is hovered. */
  payload?: Record<string, unknown>;
};

export type TimelineSelection = { start: Date; end: Date };

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
  /** Enable wheel/drag zoom + brush selection. */
  interactive?: boolean;
  /** Fired when the user finishes brushing a time range; null when cleared. */
  onSelectionChange?: (selection: TimelineSelection | null) => void;
};

const DEFAULT_COLORS = ["#6366f1", "#06b6d4", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6"];
const BRUSH_HEIGHT = 32;

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
      interactive = false,
      onSelectionChange,
    },
    ref,
  ) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const svgRef = useRef<SVGSVGElement | null>(null);
    const zoomLayerRef = useRef<SVGRectElement | null>(null);
    const brushLayerRef = useRef<SVGGElement | null>(null);
    const frameRef = useRef<D3ChartFrameHandle | null>(null);
    const [width, setWidth] = useState(0);
    const [hover, setHover] = useState<{
      x: number;
      y: number;
      time: Date;
      rows: Array<{ id: string; label: string; value: number; color: string }>;
      event?: TimelineEvent;
    } | null>(null);
    const [transform, setTransform] = useState<d3.ZoomTransform>(d3.zoomIdentity);
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

    // Reset transform whenever the series identity changes — old zoom is
    // meaningless against a fresh dataset.
    useEffect(() => {
      setTransform(d3.zoomIdentity);
    }, [series]);

    const baseLayout = useMemo(() => {
      if (!series.length || width <= 0) return null;
      const brushReserve = interactive ? BRUSH_HEIGHT + 8 : 0;
      const margin = { top: 10, right: 16, bottom: 26 + brushReserve, left: 48 };
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
      const baseX = d3
        .scaleTime()
        .domain(
          xExtent[0].getTime() === xExtent[1].getTime()
            ? [new Date(xExtent[0].getTime() - 1), new Date(xExtent[1].getTime() + 1)]
            : xExtent,
        )
        .range([0, innerWidth])
        .nice();

      if (yMin === yMax) {
        yMin -= 1;
        yMax += 1;
      }
      if (yMin > 0) yMin = 0;
      const y = d3.scaleLinear().domain([yMin, yMax]).range([innerHeight, 0]).nice();

      return {
        margin,
        brushReserve,
        innerWidth,
        innerHeight,
        baseX,
        y,
        yMinNice: y.domain()[0],
        normSeries,
        xExtent,
      };
    }, [series, width, height, interactive]);

    const layout = useMemo(() => {
      if (!baseLayout) return null;
      const x = interactive
        ? transform.rescaleX(baseLayout.baseX)
        : baseLayout.baseX;
      const tickFormatBase = d3.timeFormat(
        (baseLayout.xExtent[1].getTime() - baseLayout.xExtent[0].getTime()) >
          1000 * 60 * 60 * 24
          ? "%m-%d %H:%M"
          : "%H:%M:%S",
      );
      const xTicks = x
        .ticks(Math.max(2, Math.min(8, Math.floor(baseLayout.innerWidth / 100))))
        .map((t) => ({ x: x(t), label: tickFormatBase(t as Date) }));
      const yTicks = baseLayout.y
        .ticks(Math.max(2, Math.min(6, Math.floor(baseLayout.innerHeight / 40))))
        .map((t) => ({
          y: baseLayout.y(t as number),
          label: d3.format(",.2~s")(t as number),
        }));
      const line = d3
        .line<{ time: Date; value: number }>()
        .x((p) => x(p.time))
        .y((p) => baseLayout.y(p.value))
        .curve(d3.curveMonotoneX);
      const area = d3
        .area<{ time: Date; value: number }>()
        .x((p) => x(p.time))
        .y0(baseLayout.y(baseLayout.yMinNice))
        .y1((p) => baseLayout.y(p.value))
        .curve(d3.curveMonotoneX);

      const seriesOut = baseLayout.normSeries.map((s) => ({
        ...s,
        path: s.points.length > 0 ? line(s.points) ?? "" : "",
        areaPath: s.area && s.points.length > 0 ? area(s.points) ?? "" : "",
      }));

      const eventsOut = (events ?? [])
        .map((event) => {
          const t = toDate(event.time);
          if (Number.isNaN(t.getTime())) return null;
          return {
            x: x(t),
            time: t,
            label: event.label,
            color: event.color ?? "#ef4444",
            event,
          };
        })
        .filter((event): event is NonNullable<typeof event> => Boolean(event));

      return {
        ...baseLayout,
        x,
        xTicks,
        yTicks,
        seriesOut,
        eventsOut,
        bisectTime: d3.bisector<{ time: Date; value: number }, Date>((d) => d.time)
          .left,
      };
    }, [baseLayout, transform, interactive, events]);

    // Bind d3-zoom on the transparent overlay rect once layout is ready.
    useEffect(() => {
      if (!interactive || !layout || !zoomLayerRef.current) return undefined;
      const overlay = d3.select(zoomLayerRef.current);
      const zoomBehavior = d3
        .zoom<SVGRectElement, unknown>()
        .scaleExtent([1, 80])
        .extent([
          [0, 0],
          [layout.innerWidth, layout.innerHeight],
        ])
        .translateExtent([
          [0, -Infinity],
          [layout.innerWidth, Infinity],
        ])
        .on("zoom", (event) => {
          setTransform(event.transform);
        });
      overlay.call(zoomBehavior);
      return () => {
        overlay.on(".zoom", null);
      };
    }, [interactive, layout]);

    // Bind d3-brushX on a dedicated <g> below the plot area.
    useEffect(() => {
      if (!interactive || !layout || !brushLayerRef.current || !onSelectionChange) {
        return undefined;
      }
      const brushGroup = d3.select(brushLayerRef.current);
      const brush = d3
        .brushX<unknown>()
        .extent([
          [0, 0],
          [layout.innerWidth, BRUSH_HEIGHT],
        ])
        .on("end", (event) => {
          if (!event.sourceEvent) return; // ignore programmatic clears
          if (!event.selection) {
            onSelectionChange(null);
            return;
          }
          const [x0, x1] = event.selection as [number, number];
          const start = layout.x.invert(x0);
          const end = layout.x.invert(x1);
          onSelectionChange({ start, end });
        });
      brushGroup
        .call(brush)
        .selectAll(".overlay")
        .attr("fill", "transparent");
      return () => {
        brushGroup.on(".brush", null);
        brushGroup.selectAll("*").remove();
      };
    }, [interactive, layout, onSelectionChange]);

    const axisColor = resolvedTheme === "dark" ? "#475569" : "#cbd5e1";
    const labelColor = resolvedTheme === "dark" ? "#94a3b8" : "#64748b";

    const handleMove = useCallback(
      (event: React.MouseEvent<SVGRectElement>) => {
        if (!layout) return;
        const containerRect = containerRef.current?.getBoundingClientRect();
        const svgRect = (
          event.currentTarget.ownerSVGElement as SVGSVGElement
        ).getBoundingClientRect();
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

        // Pick the closest event marker (within 6px) so the popup can
        // surface its payload (cause, before/after heap, ...).
        const nearestEvent = layout.eventsOut.reduce<typeof layout.eventsOut[number] | undefined>(
          (closest, event) => {
            if (Math.abs(event.x - localX) > 6) return closest;
            if (!closest) return event;
            return Math.abs(event.x - localX) < Math.abs(closest.x - localX)
              ? event
              : closest;
          },
          undefined,
        );

        if (containerRect) {
          setHover({
            x: event.clientX - containerRect.left,
            y: event.clientY - containerRect.top,
            time,
            rows,
            event: nearestEvent?.event,
          });
        }
      },
      [layout],
    );

    const handleResetZoom = useCallback(() => {
      if (!interactive || !zoomLayerRef.current) return;
      d3.select(zoomLayerRef.current)
        .transition()
        .duration(200)
        .call(d3.zoom<SVGRectElement, unknown>().transform as never, d3.zoomIdentity);
      setTransform(d3.zoomIdentity);
    }, [interactive]);

    const isZoomed = interactive && transform.k !== 1;

    return (
      <D3ChartFrame
        ref={frameRef}
        title={title}
        description={description}
        exportName={exportName}
        actions={
          interactive && isZoomed ? (
            <button
              type="button"
              onClick={handleResetZoom}
              className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            >
              <RotateCcw className="h-3 w-3" />
              Reset zoom
            </button>
          ) : null
        }
      >
        <div ref={containerRef} className="relative w-full bg-card">
          {!layout ? (
            <div
              className="flex items-center justify-center text-sm text-muted-foreground"
              style={{ height }}
            >
              {emptyLabel}
            </div>
          ) : (
            <svg
              ref={svgRef}
              width={width || 0}
              height={height}
              role="img"
              aria-label={title}
              className="block"
            >
              <defs>
                <clipPath id={`plot-${title}`}>
                  <rect
                    width={layout.innerWidth}
                    height={layout.innerHeight}
                    x={0}
                    y={0}
                  />
                </clipPath>
              </defs>
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
                  <g
                    key={`x-${tick.label}-${tick.x}`}
                    transform={`translate(${tick.x}, ${layout.innerHeight})`}
                  >
                    <line y2={6} stroke={axisColor} />
                    <text y={18} textAnchor="middle" fill={labelColor} fontSize={10}>
                      {tick.label}
                    </text>
                  </g>
                ))}
                {yLabel && (
                  <text
                    transform="rotate(-90)"
                    x={-layout.innerHeight / 2}
                    y={-36}
                    textAnchor="middle"
                    fill={labelColor}
                    fontSize={10}
                  >
                    {yLabel}
                  </text>
                )}

                <g clipPath={`url(#plot-${title})`}>
                  {layout.eventsOut.map((event, idx) => (
                    <g key={`event-${idx}`} transform={`translate(${event.x}, 0)`}>
                      <line
                        y2={layout.innerHeight}
                        stroke={event.color}
                        strokeOpacity={0.4}
                        strokeDasharray="3 3"
                      />
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
                </g>

                <rect
                  ref={zoomLayerRef}
                  width={layout.innerWidth}
                  height={layout.innerHeight}
                  fill="transparent"
                  cursor={interactive ? "crosshair" : "default"}
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

                {interactive && (
                  <g
                    ref={brushLayerRef}
                    transform={`translate(0, ${layout.innerHeight + 8})`}
                  />
                )}
              </g>

              <g transform={`translate(${layout.margin.left}, ${height - layout.brushReserve - 6})`}>
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
                left: Math.min(hover.x + 12, (containerRef.current?.clientWidth ?? 0) - 240),
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
              {hover.event && (
                <div className="mt-2 border-t border-border pt-1.5 text-[11px]">
                  <p className="font-semibold">{hover.event.label ?? "Event"}</p>
                  {hover.event.payload && (
                    <ul className="mt-0.5 space-y-0.5 text-muted-foreground">
                      {Object.entries(hover.event.payload)
                        .slice(0, 6)
                        .map(([key, value]) => (
                          <li key={key}>
                            <span className="text-foreground/70">{key}:</span>{" "}
                            <span className="tabular-nums">{String(value)}</span>
                          </li>
                        ))}
                    </ul>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </D3ChartFrame>
    );
  },
);
