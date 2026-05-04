import * as d3 from "d3";
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
import type { FlameGraphNode } from "@/components/charts/D3FlameGraph";
import { useTheme } from "@/components/theme-provider";
import { downloadDataUrl } from "@/lib/exportImage";

/**
 * Canvas-rendered flamegraph (T-217).
 *
 * Designed for large flamegraphs (≥10k frames) where the SVG renderer
 * starts struggling. Layout uses the same `d3-hierarchy` + `d3-partition`
 * pipeline as :mod:`D3FlameGraph` so the visual reads identically; the
 * only difference is the painter — every rect is drawn via Canvas 2D so
 * thousands of nodes stay buttery-smooth.
 *
 * Image export bypasses `html-to-image` and uses `canvas.toDataURL()`
 * directly, which both is faster and produces a pixel-perfect PNG of the
 * exact frame the user is looking at.
 */
export type CanvasFlameGraphProps = {
  title: string;
  description?: string;
  exportName?: string;
  data: FlameGraphNode | null | undefined;
  /** Pixel height of each row. */
  rowHeight?: number;
  /** Empty-state copy. */
  emptyLabel?: string;
};

const FALLBACK_PALETTE = [
  "#ef4444",
  "#f97316",
  "#f59e0b",
  "#eab308",
  "#84cc16",
  "#22c55e",
  "#10b981",
  "#14b8a6",
  "#06b6d4",
  "#0ea5e9",
  "#3b82f6",
  "#6366f1",
  "#8b5cf6",
  "#a855f7",
  "#d946ef",
  "#ec4899",
];

function hashColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i += 1) {
    h = (h * 31 + name.charCodeAt(i)) | 0;
  }
  return FALLBACK_PALETTE[Math.abs(h) % FALLBACK_PALETTE.length];
}

function pickReadableTextColor(hex: string): string {
  const m = hex.match(/^#([0-9a-f]{6})$/i);
  if (!m) return "#0f172a";
  const value = parseInt(m[1], 16);
  const r = (value >> 16) & 0xff;
  const g = (value >> 8) & 0xff;
  const b = value & 0xff;
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance > 0.6 ? "#0f172a" : "#ffffff";
}

function clipText(label: string, available: number): string {
  const maxChars = Math.max(1, Math.floor((available - 8) / 6.5));
  if (label.length <= maxChars) return label;
  return `${label.slice(0, Math.max(1, maxChars - 1))}…`;
}

type RenderRect = {
  node: d3.HierarchyNode<FlameGraphNode>;
  x: number;
  y: number;
  width: number;
  height: number;
};

export const CanvasFlameGraph = forwardRef<D3ChartFrameHandle, CanvasFlameGraphProps>(
  function CanvasFlameGraph(
    { title, description, exportName, data, rowHeight = 22, emptyLabel = "No data" },
    ref,
  ) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const canvasRef = useRef<HTMLCanvasElement | null>(null);
    const frameRef = useRef<D3ChartFrameHandle | null>(null);
    const rectsRef = useRef<RenderRect[]>([]);
    const [width, setWidth] = useState(0);
    const [focusPath, setFocusPath] = useState<number[]>([]);
    const [tooltip, setTooltip] = useState<{
      x: number;
      y: number;
      name: string;
      value: number;
      ratio: number;
      category: string | null;
    } | null>(null);
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

    const root = useMemo(() => {
      if (!data) return null;
      const hierarchy = d3
        .hierarchy<FlameGraphNode>(data, (d) => d.children ?? null)
        .sum((d) =>
          d.children && d.children.length > 0 ? 0 : d.value || 0,
        )
        .sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
      return hierarchy as d3.HierarchyNode<FlameGraphNode> & { value: number };
    }, [data]);

    const layoutInfo = useMemo(() => {
      if (!root || !root.value || width <= 0) return null;
      const totalRows = root.height + 1;
      const totalHeight = totalRows * rowHeight;
      const partition = d3
        .partition<FlameGraphNode>()
        .size([width, totalHeight])
        .padding(0);
      const partitioned = partition(root);

      let focused = partitioned;
      for (const idx of focusPath) {
        const next = focused.children?.[idx];
        if (!next) break;
        focused = next;
      }

      return { partitioned, focused, totalRows };
    }, [root, focusPath, width, rowHeight]);

    function colorFor(node: d3.HierarchyNode<FlameGraphNode>): string {
      const datum = node.data;
      if (typeof datum.color === "string" && datum.color) return datum.color;
      if (typeof datum.category === "string" && datum.category) {
        return hashColor(datum.category);
      }
      return hashColor(datum.name || "node");
    }

    // Render to canvas whenever layout or theme changes.
    useEffect(() => {
      const canvas = canvasRef.current;
      if (!canvas || !layoutInfo) {
        rectsRef.current = [];
        return;
      }
      const { partitioned, focused } = layoutInfo;
      const dpr = Math.max(1, window.devicePixelRatio || 1);
      const fx0 = focused.x0;
      const fx1 = focused.x1;
      const fy0 = focused.y0;
      const xScale = (value: number) =>
        ((value - fx0) / Math.max(1e-6, fx1 - fx0)) * width;
      const yScale = (value: number) => value - fy0;
      const visibleHeight = partitioned
        .descendants()
        .reduce(
          (acc, node) => Math.max(acc, node.y1 - fy0),
          rowHeight,
        );

      canvas.width = Math.floor(width * dpr);
      canvas.height = Math.floor(visibleHeight * dpr);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${visibleHeight}px`;

      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, width, visibleHeight);
      const strokeColor = resolvedTheme === "dark" ? "#1f2937" : "#ffffff";
      const totalRoot = (root!.value as number) || 1;
      const rects: RenderRect[] = [];

      ctx.font = '11px ui-monospace, SFMono-Regular, Menlo, monospace';
      ctx.textBaseline = "middle";

      for (const node of partitioned.descendants()) {
        if (node === partitioned) continue;
        const x = xScale(node.x0);
        const x1 = xScale(node.x1);
        const y = yScale(node.y0);
        const w = Math.max(0, x1 - x);
        const h = Math.max(0, node.y1 - node.y0 - 1);
        if (y < 0 || w < 0.5) continue;

        const fill = colorFor(node);
        ctx.fillStyle = fill;
        ctx.fillRect(x, y, w, h);
        ctx.strokeStyle = strokeColor;
        ctx.lineWidth = 0.5;
        ctx.strokeRect(x + 0.25, y + 0.25, Math.max(0, w - 0.5), Math.max(0, h - 0.5));

        if (w > 28) {
          ctx.fillStyle = pickReadableTextColor(fill);
          ctx.fillText(clipText(node.data.name || "", w), x + 4, y + h / 2);
        }
        rects.push({ node, x, y, width: w, height: h });
      }

      rectsRef.current = rects;
    }, [layoutInfo, width, rowHeight, resolvedTheme, root]);

    const handleMouseMove = useCallback(
      (event: React.MouseEvent<HTMLCanvasElement>) => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const rect = canvas.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        let found: RenderRect | null = null;
        // Linear scan in render order; for ≤O(50k) rects this is well under
        // a millisecond per move event in modern Chrome/Safari.
        for (const candidate of rectsRef.current) {
          if (
            x >= candidate.x &&
            x <= candidate.x + candidate.width &&
            y >= candidate.y &&
            y <= candidate.y + candidate.height
          ) {
            found = candidate;
            // Don't break — descendants come later in the iteration order
            // and should win when overlapping (they have smaller height
            // ranges so the inner-most rect is the latest one we touch).
          }
        }
        if (!found || !root) {
          setTooltip(null);
          return;
        }
        const total = (root.value as number) || 1;
        const containerRect = containerRef.current?.getBoundingClientRect();
        if (!containerRect) return;
        setTooltip({
          x: event.clientX - containerRect.left,
          y: event.clientY - containerRect.top,
          name: found.node.data.name || "",
          value: found.node.value as number,
          ratio: ((found.node.value as number) ?? 0) / total,
          category: found.node.data.category ?? null,
        });
      },
      [root],
    );

    const handleClick = useCallback(
      (event: React.MouseEvent<HTMLCanvasElement>) => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const rect = canvas.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        let target: RenderRect | null = null;
        for (const candidate of rectsRef.current) {
          if (
            x >= candidate.x &&
            x <= candidate.x + candidate.width &&
            y >= candidate.y &&
            y <= candidate.y + candidate.height
          ) {
            target = candidate;
          }
        }
        if (!target || !layoutInfo) return;
        const ancestors = target.node.ancestors().reverse();
        const path: number[] = [];
        for (let i = 1; i < ancestors.length; i += 1) {
          const parent = ancestors[i - 1];
          const child = ancestors[i];
          const idx = parent.children?.indexOf(child) ?? -1;
          if (idx < 0) break;
          path.push(idx);
        }
        setFocusPath(path);
      },
      [layoutInfo],
    );

    const handleReset = useCallback(() => setFocusPath([]), []);

    const handleCanvasExport = useCallback(() => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const dataUrl = canvas.toDataURL("image/png");
      const filename = (exportName ?? title).trim() || "archscope-flamegraph";
      downloadDataUrl(dataUrl, `${filename}.png`);
    }, [exportName, title]);

    return (
      <D3ChartFrame
        ref={frameRef}
        title={title}
        description={description}
        exportName={exportName}
        bodyClassName="bg-card"
        actions={
          <>
            {focusPath.length > 0 && (
              <button
                type="button"
                onClick={handleReset}
                className="rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              >
                Reset zoom
              </button>
            )}
            <button
              type="button"
              onClick={handleCanvasExport}
              className="rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              title="Save canvas as PNG"
            >
              Save PNG
            </button>
          </>
        }
      >
        <div ref={containerRef} className="relative w-full overflow-x-auto bg-card">
          {!root || !root.value ? (
            <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
              {emptyLabel}
            </div>
          ) : (
            <canvas
              ref={canvasRef}
              onMouseMove={handleMouseMove}
              onMouseLeave={() => setTooltip(null)}
              onClick={handleClick}
              className="block cursor-pointer"
            />
          )}
          {tooltip && (
            <div
              className="pointer-events-none absolute z-10 max-w-xs rounded-md border border-border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg"
              style={{
                left: Math.min(tooltip.x + 12, (containerRef.current?.clientWidth ?? 0) - 240),
                top: Math.max(0, tooltip.y - 60),
              }}
            >
              <p className="font-mono text-[11px] leading-tight break-all">
                {tooltip.name}
              </p>
              <p className="mt-1 text-[10px] text-muted-foreground tabular-nums">
                {tooltip.value.toLocaleString()} samples · {(tooltip.ratio * 100).toFixed(2)}%
                {tooltip.category ? ` · ${tooltip.category}` : ""}
              </p>
            </div>
          )}
        </div>
      </D3ChartFrame>
    );
  },
);
