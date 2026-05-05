// Canvas-rendered flamegraph for the native profiler app.
//
// Ported from apps/frontend/src/components/charts/CanvasFlameGraph.tsx (T-217)
// — the same d3-hierarchy + d3-partition layout, the same Canvas 2D painter,
// trimmed of the web app's D3ChartFrame / Tailwind / theme-provider stack so
// it can stand alone inside the slim Wails v3 shell. The web component stays
// canonical; future fixes should be ported in both directions.

import { hierarchy, partition, type HierarchyNode } from "d3-hierarchy";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
} from "react";

import { useI18n } from "../i18n/I18nProvider";
import { useTheme } from "../theme/ThemeProvider";

export type FlameGraphNode = {
  name: string;
  value: number;
  children?: FlameGraphNode[];
  category?: string | null;
  color?: string | null;
};

type RenderRect = {
  node: HierarchyNode<FlameGraphNode>;
  x: number;
  y: number;
  width: number;
  height: number;
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

function readableTextColor(hex: string): string {
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

function colorFor(node: HierarchyNode<FlameGraphNode>): string {
  const datum = node.data;
  if (typeof datum.color === "string" && datum.color) return datum.color;
  if (typeof datum.category === "string" && datum.category) return hashColor(datum.category);
  return hashColor(datum.name || "node");
}

export type CanvasFlameGraphProps = {
  data: FlameGraphNode | null | undefined;
  rowHeight?: number;
  exportName?: string;
};

export function CanvasFlameGraph({
  data,
  rowHeight = 22,
  exportName = "archscope-flamegraph",
}: CanvasFlameGraphProps) {
  const { t } = useI18n();
  const { resolvedTheme } = useTheme();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
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
    return hierarchy<FlameGraphNode>(data, (d) => d.children ?? null)
      .sum((d) => (d.children && d.children.length > 0 ? 0 : d.value || 0))
      .sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
  }, [data]);

  const layoutInfo = useMemo(() => {
    if (!root || !root.value || width <= 0) return null;
    const totalRows = root.height + 1;
    const totalHeight = totalRows * rowHeight;
    const partitioned = partition<FlameGraphNode>().size([width, totalHeight]).padding(0)(root);

    let focused = partitioned;
    for (const idx of focusPath) {
      const next = focused.children?.[idx];
      if (!next) break;
      focused = next;
    }
    return { partitioned, focused };
  }, [root, focusPath, width, rowHeight]);

  // Render to canvas whenever layout changes.
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
    const xScale = (value: number) => ((value - fx0) / Math.max(1e-6, fx1 - fx0)) * width;
    const yScale = (value: number) => value - fy0;
    const visibleHeight = partitioned
      .descendants()
      .reduce((acc, node) => Math.max(acc, node.y1 - fy0), rowHeight);

    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(visibleHeight * dpr);
    canvas.style.width = `${width}px`;
    canvas.style.height = `${visibleHeight}px`;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, width, visibleHeight);
    const strokeColor = resolvedTheme === "dark" ? "#1f2937" : "#ffffff";
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
        ctx.fillStyle = readableTextColor(fill);
        ctx.fillText(clipText(node.data.name || "", w), x + 4, y + h / 2);
      }
      rects.push({ node, x, y, width: w, height: h });
    }
    rectsRef.current = rects;
  }, [layoutInfo, width, rowHeight, root, resolvedTheme]);

  const findHit = useCallback((event: ReactMouseEvent<HTMLCanvasElement>): RenderRect | null => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    let found: RenderRect | null = null;
    for (const candidate of rectsRef.current) {
      if (
        x >= candidate.x &&
        x <= candidate.x + candidate.width &&
        y >= candidate.y &&
        y <= candidate.y + candidate.height
      ) {
        // Don't break — descendants come later and should win when they overlap.
        found = candidate;
      }
    }
    return found;
  }, []);

  const handleMouseMove = useCallback(
    (event: ReactMouseEvent<HTMLCanvasElement>) => {
      const found = findHit(event);
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
        value: (found.node.value as number) ?? 0,
        ratio: ((found.node.value as number) ?? 0) / total,
        category: found.node.data.category ?? null,
      });
    },
    [findHit, root],
  );

  const handleClick = useCallback(
    (event: ReactMouseEvent<HTMLCanvasElement>) => {
      const target = findHit(event);
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
    [findHit, layoutInfo],
  );

  const handleReset = useCallback(() => setFocusPath([]), []);

  const handleSavePng = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const dataUrl = canvas.toDataURL("image/png");
    const link = document.createElement("a");
    link.href = dataUrl;
    link.download = `${exportName}.png`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }, [exportName]);

  if (!data) {
    return <p className="muted">{t("flamegraphEmpty")}</p>;
  }

  return (
    <div className="flamegraph">
      <div className="flamegraph-toolbar">
        {focusPath.length > 0 && (
          <button type="button" className="ghost" onClick={handleReset}>
            {t("flamegraphReset")}
          </button>
        )}
        <button type="button" className="ghost" onClick={handleSavePng}>
          {t("flamegraphSavePng")}
        </button>
      </div>
      <div ref={containerRef} className="flamegraph-canvas-host">
        {!root || !root.value ? (
          <div className="flamegraph-empty">{t("flamegraphEmpty")}</div>
        ) : (
          <canvas
            ref={canvasRef}
            onMouseMove={handleMouseMove}
            onMouseLeave={() => setTooltip(null)}
            onClick={handleClick}
            className="flamegraph-canvas"
          />
        )}
        {tooltip && (
          <div
            className="flamegraph-tooltip"
            style={{
              left: Math.min(
                tooltip.x + 12,
                Math.max(0, (containerRef.current?.clientWidth ?? 0) - 280),
              ),
              top: Math.max(0, tooltip.y - 60),
            }}
          >
            <div className="flamegraph-tooltip-name">{tooltip.name}</div>
            <div className="flamegraph-tooltip-meta">
              {tooltip.value.toLocaleString()} samples · {(tooltip.ratio * 100).toFixed(2)}%
              {tooltip.category ? ` · ${tooltip.category}` : ""}
            </div>
          </div>
        )}
      </div>
      <p className="flamegraph-hint">{t("flamegraphHint")}</p>
    </div>
  );
}
