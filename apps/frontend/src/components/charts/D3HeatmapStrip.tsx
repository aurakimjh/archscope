import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useTheme } from "@/components/theme-provider";

export type HeatmapBucket = {
  index: number;
  time: string;
  count: number;
};

export type HeatmapStripProps = {
  /** Bucketed events from the engine. */
  buckets: HeatmapBucket[];
  /** Bucket size in seconds — shown in the legend so users can read absolute densities. */
  bucketSeconds: number;
  /** Maximum count across buckets — used to scale color intensity. */
  maxCount: number;
  /** Recording start (ISO 8601) shown above the strip. */
  startTime?: string | null;
  /** Recording end (ISO 8601). */
  endTime?: string | null;
  /** Pixel height of the strip. */
  height?: number;
  /** Fired with the start/end ISO timestamps when the user drags a selection
   *  (or null when cleared). */
  onSelectionChange?: (range: { start: string; end: string } | null) => void;
  /** Empty-state copy. */
  emptyLabel?: string;
};

/**
 * 1D wall-clock density strip — one row of cells, color intensity ∝ event
 * count per bucket. Drag to select a sub-range, click empty area to clear.
 *
 * Inspired by async-profiler's heatmap (which is 2D for 24-h recordings);
 * for typical archscope inputs (seconds–minutes) a strip is plenty.
 */
export function D3HeatmapStrip({
  buckets,
  bucketSeconds,
  maxCount,
  startTime,
  endTime,
  height = 56,
  onSelectionChange,
  emptyLabel = "No data",
}: HeatmapStripProps): JSX.Element {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [width, setWidth] = useState(0);
  const [hover, setHover] = useState<{ x: number; bucket: HeatmapBucket } | null>(null);
  const [drag, setDrag] = useState<{ x0: number; x1: number } | null>(null);
  const dragStartRef = useRef<number | null>(null);
  const { resolvedTheme } = useTheme();

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
    if (width <= 0 || buckets.length === 0) return null;
    const cellWidth = width / buckets.length;
    return { cellWidth };
  }, [width, buckets.length]);

  const colorFor = useCallback(
    (count: number) => {
      if (maxCount <= 0 || count <= 0) {
        return resolvedTheme === "dark" ? "#1f2937" : "#f1f5f9";
      }
      const intensity = Math.min(1, count / maxCount);
      // Heat ramp: pale yellow → orange → red → dark red.
      const stops = [
        { t: 0.0, c: [254, 243, 199] },
        { t: 0.25, c: [253, 186, 116] },
        { t: 0.5, c: [251, 113, 133] },
        { t: 0.75, c: [220, 38, 38] },
        { t: 1.0, c: [127, 29, 29] },
      ];
      let lo = stops[0];
      let hi = stops[stops.length - 1];
      for (let i = 1; i < stops.length; i += 1) {
        if (intensity <= stops[i].t) {
          hi = stops[i];
          lo = stops[i - 1];
          break;
        }
      }
      const span = hi.t - lo.t || 1;
      const k = (intensity - lo.t) / span;
      const r = Math.round(lo.c[0] + (hi.c[0] - lo.c[0]) * k);
      const g = Math.round(lo.c[1] + (hi.c[1] - lo.c[1]) * k);
      const b = Math.round(lo.c[2] + (hi.c[2] - lo.c[2]) * k);
      return `rgb(${r}, ${g}, ${b})`;
    },
    [maxCount, resolvedTheme],
  );

  const finalize = useCallback(() => {
    const rect = drag;
    dragStartRef.current = null;
    setDrag(null);
    if (!rect || !layout) return;
    const x0 = Math.min(rect.x0, rect.x1);
    const x1 = Math.max(rect.x0, rect.x1);
    if (x1 - x0 < 4) {
      onSelectionChange?.(null);
      return;
    }
    const i0 = Math.max(0, Math.floor(x0 / layout.cellWidth));
    const i1 = Math.min(buckets.length - 1, Math.floor(x1 / layout.cellWidth));
    const start = buckets[i0]?.time ?? null;
    const endIso =
      buckets[i1] != null
        ? new Date(
            new Date(buckets[i1].time).getTime() + bucketSeconds * 1000,
          ).toISOString()
        : null;
    if (start && endIso) onSelectionChange?.({ start, end: endIso });
  }, [drag, layout, buckets, bucketSeconds, onSelectionChange]);

  if (buckets.length === 0 || !layout) {
    return (
      <div className="rounded-md border border-border bg-card p-3 text-xs text-muted-foreground">
        {emptyLabel}
      </div>
    );
  }

  const dragX = drag
    ? { x: Math.min(drag.x0, drag.x1), w: Math.abs(drag.x1 - drag.x0) }
    : null;

  return (
    <div className="rounded-md border border-border bg-card p-3">
      <div className="mb-1.5 flex items-center justify-between text-[11px] text-muted-foreground">
        <span>{startTime ? formatLocal(startTime) : "—"}</span>
        <span>
          {bucketSeconds}s buckets · max {maxCount}
        </span>
        <span>{endTime ? formatLocal(endTime) : "—"}</span>
      </div>
      <div ref={containerRef} className="relative w-full" style={{ height }}>
        <svg
          width={width || 0}
          height={height}
          className="block cursor-crosshair select-none"
          role="img"
          onMouseDown={(event) => {
            if (event.button !== 0) return;
            const rect = (event.currentTarget as SVGSVGElement).getBoundingClientRect();
            const x = event.clientX - rect.left;
            dragStartRef.current = x;
            setDrag({ x0: x, x1: x });
            event.preventDefault();
          }}
          onMouseMove={(event) => {
            const rect = (event.currentTarget as SVGSVGElement).getBoundingClientRect();
            const x = event.clientX - rect.left;
            const idx = Math.min(
              buckets.length - 1,
              Math.max(0, Math.floor(x / layout.cellWidth)),
            );
            setHover({ x, bucket: buckets[idx] });
            if (dragStartRef.current !== null) {
              setDrag({ x0: dragStartRef.current, x1: x });
            }
          }}
          onMouseUp={finalize}
          onMouseLeave={() => {
            setHover(null);
            if (dragStartRef.current !== null) finalize();
          }}
        >
          {buckets.map((bucket, i) => (
            <rect
              key={bucket.index}
              x={i * layout.cellWidth}
              y={0}
              width={Math.max(1, layout.cellWidth - 0.5)}
              height={height}
              fill={colorFor(bucket.count)}
            />
          ))}
          {dragX && (
            <rect
              x={dragX.x}
              y={0}
              width={dragX.w}
              height={height}
              fill="#6366f1"
              fillOpacity={0.15}
              stroke="#6366f1"
              strokeWidth={1}
              pointerEvents="none"
            />
          )}
        </svg>
        {hover && (
          <div
            className="pointer-events-none absolute z-10 rounded-md border border-border bg-popover px-2 py-1 text-[11px] text-popover-foreground shadow-lg"
            style={{
              left: Math.min(hover.x + 8, width - 220),
              top: -4,
            }}
          >
            <div className="font-mono">{formatLocal(hover.bucket.time)}</div>
            <div className="text-muted-foreground tabular-nums">
              {hover.bucket.count.toLocaleString()} events
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function formatLocal(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
