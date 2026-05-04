import * as d3 from "d3";
import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";

import { D3ChartFrame, type D3ChartFrameHandle } from "@/components/charts/D3ChartFrame";
import { useTheme } from "@/components/theme-provider";
import { cn } from "@/lib/utils";

export type FlameGraphNode = {
  name: string;
  value: number;
  category?: string | null;
  color?: string | null;
  children?: FlameGraphNode[] | null;
};

export type D3FlameGraphProps = {
  /** Translated chart title used by the wrapping card. */
  title: string;
  /** Translated description (optional). */
  description?: string;
  /** Filename prefix for image export. */
  exportName?: string;
  /** Hierarchical flame data: leaves drive sample counts. */
  data: FlameGraphNode | null | undefined;
  /** Pixel height of each row. */
  rowHeight?: number;
  /** Optional cap for the rendered depth (rows beyond this are hidden). */
  maxDepth?: number;
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

function cloneSafe(node: FlameGraphNode | null | undefined): FlameGraphNode | null {
  if (!node) return null;
  return {
    name: node.name,
    value: node.value,
    category: node.category ?? null,
    color: node.color ?? null,
    children: Array.isArray(node.children)
      ? node.children.map((child) => cloneSafe(child)).filter((c): c is FlameGraphNode => Boolean(c))
      : null,
  };
}

export const D3FlameGraph = forwardRef<D3ChartFrameHandle, D3FlameGraphProps>(
  function D3FlameGraph(
    { title, description, exportName, data, rowHeight = 22, maxDepth = 60, emptyLabel = "No data" },
    ref,
  ) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const svgRef = useRef<SVGSVGElement | null>(null);
    const frameRef = useRef<D3ChartFrameHandle | null>(null);
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

    const cleaned = useMemo(() => cloneSafe(data ?? null), [data]);

    const root = useMemo(() => {
      if (!cleaned) return null;
      const hierarchy = d3
        .hierarchy<FlameGraphNode>(cleaned, (d) => d.children ?? null)
        .sum((d) => (d.children && d.children.length > 0 ? 0 : d.value || 0))
        .sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
      return hierarchy as d3.HierarchyNode<FlameGraphNode> & { value: number };
    }, [cleaned]);

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

    const layoutResult = useMemo(() => {
      if (!root || !root.value || width <= 0) return null;
      const depth = Math.min(maxDepth, root.height + 1);
      const totalHeight = depth * rowHeight;
      const partition = d3.partition<FlameGraphNode>().size([width, totalHeight]).padding(0);
      const partitioned = partition(root);

      let focused = partitioned;
      for (const idx of focusPath) {
        const next = focused.children?.[idx];
        if (!next) break;
        focused = next;
      }

      const fx0 = focused.x0;
      const fx1 = focused.x1;
      const fy0 = focused.y0;
      const xScale = (value: number) =>
        ((value - fx0) / Math.max(1e-6, fx1 - fx0)) * width;
      const yScale = (value: number) => value - fy0;

      const visibleHeight =
        partitioned.descendants().reduce((acc, d) => Math.max(acc, d.y1 - fy0), 0) || rowHeight;

      return { partitioned, focused, xScale, yScale, totalHeight: visibleHeight };
    }, [root, width, rowHeight, maxDepth, focusPath]);

    function colorFor(node: d3.HierarchyNode<FlameGraphNode>): string {
      const datum = node.data;
      if (typeof datum.color === "string" && datum.color) return datum.color;
      if (typeof datum.category === "string" && datum.category) {
        return hashColor(datum.category);
      }
      return hashColor(datum.name || "node");
    }

    function rootValue(): number {
      return (root && (root.value as number)) || 0;
    }

    function nodeRatio(node: d3.HierarchyNode<FlameGraphNode>): number {
      const total = rootValue();
      if (!total) return 0;
      return (node.value as number) / total;
    }

    function handleNodeClick(node: d3.HierarchyNode<FlameGraphNode>): void {
      if (!root) return;
      const ancestors = node.ancestors().reverse();
      // ancestors[0] is the root; collect child indexes from each subsequent ancestor
      const path: number[] = [];
      for (let i = 1; i < ancestors.length; i += 1) {
        const parent = ancestors[i - 1];
        const child = ancestors[i];
        const idx = parent.children?.indexOf(child) ?? -1;
        if (idx < 0) break;
        path.push(idx);
      }
      setFocusPath(path);
    }

    function handleReset(): void {
      setFocusPath([]);
    }

    return (
      <D3ChartFrame
        ref={frameRef}
        title={title}
        description={description}
        exportName={exportName}
        bodyClassName="bg-card"
        actions={
          focusPath.length > 0 ? (
            <button
              type="button"
              onClick={handleReset}
              className="rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            >
              Reset zoom
            </button>
          ) : null
        }
      >
        <div ref={containerRef} className="relative w-full overflow-x-auto bg-card">
          {!root || !root.value ? (
            <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
              {emptyLabel}
            </div>
          ) : (
            <svg
              ref={svgRef}
              width={width || 0}
              height={layoutResult?.totalHeight || rowHeight}
              role="img"
              aria-label={title}
              className="block"
            >
              {layoutResult &&
                layoutResult.partitioned.descendants().map((node) => {
                  if (node === layoutResult.partitioned) return null;
                  const x = layoutResult.xScale(node.x0);
                  const x1 = layoutResult.xScale(node.x1);
                  const y = layoutResult.yScale(node.y0);
                  const w = Math.max(0, x1 - x);
                  const h = Math.max(0, node.y1 - node.y0 - 1);
                  if (w < 0.5) return null;
                  if (y < 0) return null;
                  const fill = colorFor(node);
                  const ratio = nodeRatio(node);
                  const label = node.data.name || "";
                  return (
                    <g
                      key={
                        // hierarchical id: depth+x0 should be unique per layout
                        `${node.depth}-${node.x0.toFixed(3)}`
                      }
                      transform={`translate(${x},${y})`}
                      onClick={() => handleNodeClick(node)}
                      onMouseEnter={(event) => {
                        const rect = (event.currentTarget as SVGGElement)
                          .ownerSVGElement?.getBoundingClientRect();
                        const containerRect = containerRef.current?.getBoundingClientRect();
                        if (rect && containerRect) {
                          setTooltip({
                            x: event.clientX - containerRect.left,
                            y: event.clientY - containerRect.top,
                            name: label,
                            value: node.value as number,
                            ratio,
                            category: node.data.category ?? null,
                          });
                        }
                      }}
                      onMouseMove={(event) => {
                        const containerRect = containerRef.current?.getBoundingClientRect();
                        if (containerRect && tooltip) {
                          setTooltip({
                            ...tooltip,
                            x: event.clientX - containerRect.left,
                            y: event.clientY - containerRect.top,
                          });
                        }
                      }}
                      onMouseLeave={() => setTooltip(null)}
                      className="cursor-pointer"
                    >
                      <rect
                        width={w}
                        height={h}
                        fill={fill}
                        stroke={resolvedTheme === "dark" ? "#1f2937" : "#ffffff"}
                        strokeWidth={0.5}
                        rx={2}
                      />
                      {w > 28 && (
                        <text
                          x={4}
                          y={h / 2}
                          dominantBaseline="middle"
                          fontSize={11}
                          fontFamily="ui-monospace, SFMono-Regular, Menlo, monospace"
                          fill={pickReadableTextColor(fill)}
                          style={{ pointerEvents: "none", userSelect: "none" }}
                        >
                          {clipText(label, w)}
                        </text>
                      )}
                    </g>
                  );
                })}
            </svg>
          )}
          {tooltip && (
            <div
              className={cn(
                "pointer-events-none absolute z-10 max-w-xs rounded-md border border-border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg",
              )}
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
