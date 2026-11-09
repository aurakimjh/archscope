import { hierarchy, type HierarchyRectangularNode } from "d3-hierarchy";

export type FlameGraphNode = {
  name: string;
  value: number;
  children?: FlameGraphNode[];
  category?: string | null;
  color?: string | null;
};

const weight = (value: number) => Number.isFinite(value) && value > 0 ? value : 0;

// Wire values are inclusive. d3.sum expects self weights, including those
// on internal nodes; counting leaves alone discards stacks that end early.
export function buildFlameHierarchy(data: FlameGraphNode) {
  return hierarchy(data, (d) => d.children ?? null)
    .sum((d) => Math.max(0, weight(d.value) - (d.children ?? []).reduce((sum, child) => sum + weight(child.value), 0)))
    .sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
}

export function flameSearchCoverage(scopeRoot: HierarchyRectangularNode<FlameGraphNode>, query: string) {
  const basisSamples = Number(scopeRoot.value ?? 0);
  const basisWidth = Math.max(1e-6, scopeRoot.x1 - scopeRoot.x0);
  const matches = scopeRoot.descendants().filter((n) => n.data.name.toLowerCase().includes(query.toLowerCase()));
  const intervals = matches.map((n) => ({start: Math.max(scopeRoot.x0, n.x0), end: Math.min(scopeRoot.x1, n.x1)}))
    .filter((i) => i.end > i.start).sort((a, b) => a.start === b.start ? b.end - a.end : a.start - b.start);
  let coveredWidth = 0;
  let coveredEnd = -Infinity;
  for (const interval of intervals) {
    if (interval.end <= coveredEnd) continue;
    coveredWidth += interval.end - Math.max(interval.start, coveredEnd);
    coveredEnd = interval.end;
  }
  const ratio = basisSamples > 0 ? Math.max(0, Math.min(1, coveredWidth / basisWidth)) : 0;
  return { frameCount: matches.length, matchedSamples: basisSamples * ratio, ratio, basisSamples };
}
