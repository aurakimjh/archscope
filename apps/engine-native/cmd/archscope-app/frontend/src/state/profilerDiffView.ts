import type { FlameGraphNode } from "./flamegraphLayout";

type DiffNode = { name?: string; samples?: number; metadata?: unknown; children?: DiffNode[] };
const finiteWeight = (value: unknown): value is number => typeof value === "number" && Number.isFinite(value) && value >= 0;

// The legacy integer samples field is scaled geometry, and can be bounded on
// 32bit hosts. Canonical a/b weights are authoritative for display and layout.
export function adaptProfilerDiffFlame(node: DiffNode | null | undefined): FlameGraphNode | null {
  if (!node) return null;
  const meta = node.metadata && typeof node.metadata === "object" ? node.metadata as Record<string, unknown> : {};
  const value = finiteWeight(meta.a) && finiteWeight(meta.b)
    ? Math.max(meta.a, meta.b)
    : finiteWeight(node.samples) ? node.samples / 1000000 : 0;
  const delta = typeof meta.delta === "number" && Number.isFinite(meta.delta) ? meta.delta : 0;
  return {
    name: node.name ?? "", value, category: null,
    color: delta > 0.001 ? "#ef4444" : delta < -0.001 ? "#22c55e" : "#94a3b8",
    children: (node.children ?? []).map(adaptProfilerDiffFlame).filter((child): child is FlameGraphNode => child !== null),
  };
}
