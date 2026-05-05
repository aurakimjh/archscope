import { ChevronDown, ChevronRight } from "lucide-react";
import { useMemo, useState } from "react";

import {
  applyDisplayOptions,
  type FlameDisplayOptions,
  type FlameGraphNode,
} from "@/components/charts/D3FlameGraph";

export type FlameTreeTableProps = {
  data: FlameGraphNode | null | undefined;
  /** Optional cap so very deep trees don't render thousands of rows by default. */
  initialDepth?: number;
  /** Show a self-sample column (exclusive samples). */
  showSelf?: boolean;
  emptyLabel?: string;
  highlightPattern?: string;
  displayOptions?: FlameDisplayOptions;
  labels: {
    frame: string;
    samples: string;
    self: string;
    ratio: string;
  };
};

/**
 * Hierarchical expandable table view of a flame tree, sorted by samples
 * descending at every level. Shows inclusive samples + optional self
 * (exclusive) samples + share of root.
 */
export function FlameTreeTable({
  data,
  initialDepth = 2,
  showSelf = true,
  emptyLabel = "No data",
  highlightPattern,
  displayOptions,
  labels,
}: FlameTreeTableProps): JSX.Element {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(["root"]));
  const total = data?.value ?? 0;

  const highlightRe = useMemo(() => {
    if (!highlightPattern) return null;
    try {
      return new RegExp(highlightPattern, "i");
    } catch {
      return null;
    }
  }, [highlightPattern]);

  if (!data || total <= 0) {
    return (
      <div className="rounded-md border border-border bg-card p-3 text-xs text-muted-foreground">
        {emptyLabel}
      </div>
    );
  }

  function toggle(key: string): void {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  const rows: Array<{ node: FlameGraphNode; key: string; depth: number; self: number; matched: boolean }> = [];

  function walk(node: FlameGraphNode, key: string, depth: number, parentVisible: boolean): void {
    const childTotal = (node.children ?? []).reduce((acc, c) => acc + c.value, 0);
    const self = Math.max(0, node.value - childTotal);
    const visible = parentVisible;
    if (visible) {
      const matched = highlightRe ? highlightRe.test(node.name) : false;
      rows.push({ node, key, depth, self, matched });
    }
    const open = expanded.has(key) || depth < initialDepth;
    if (!open) return;
    const children = (node.children ?? []).slice().sort((a, b) => b.value - a.value);
    for (let i = 0; i < children.length; i += 1) {
      const child = children[i];
      walk(child, `${key}/${i}-${child.name}`, depth + 1, visible);
    }
  }

  walk(data, "root", 0, true);

  return (
    <div className="overflow-auto rounded-md border border-border bg-card">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
            <th className="px-3 py-2 text-left font-medium">{labels.frame}</th>
            <th className="w-[100px] px-3 py-2 text-right font-medium">{labels.samples}</th>
            {showSelf ? (
              <th className="w-[80px] px-3 py-2 text-right font-medium">{labels.self}</th>
            ) : null}
            <th className="w-[80px] px-3 py-2 text-right font-medium">{labels.ratio}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ node, key, depth, self, matched }) => {
            const hasChildren = (node.children?.length ?? 0) > 0;
            const open = expanded.has(key) || depth < initialDepth;
            const ratio = total > 0 ? (node.value / total) * 100 : 0;
            const display = applyDisplayOptions(node.name, displayOptions);
            return (
              <tr
                key={key}
                className={`border-b border-border last:border-0 ${matched ? "bg-amber-100/40 dark:bg-amber-900/20" : ""}`}
              >
                <td className="px-3 py-1.5 font-mono">
                  <div
                    className="flex items-center gap-1"
                    style={{ paddingLeft: `${depth * 14}px` }}
                  >
                    {hasChildren ? (
                      <button
                        type="button"
                        className="flex h-4 w-4 items-center justify-center rounded text-muted-foreground hover:bg-muted"
                        onClick={() => toggle(key)}
                      >
                        {open ? (
                          <ChevronDown className="h-3 w-3" />
                        ) : (
                          <ChevronRight className="h-3 w-3" />
                        )}
                      </button>
                    ) : (
                      <span className="inline-block w-4" aria-hidden />
                    )}
                    <span className="truncate" title={node.name}>
                      {display}
                    </span>
                  </div>
                </td>
                <td className="px-3 py-1.5 text-right tabular-nums">
                  {node.value.toLocaleString()}
                </td>
                {showSelf ? (
                  <td className="px-3 py-1.5 text-right tabular-nums text-muted-foreground">
                    {self > 0 ? self.toLocaleString() : "—"}
                  </td>
                ) : null}
                <td className="px-3 py-1.5 text-right tabular-nums">{ratio.toFixed(2)}%</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
