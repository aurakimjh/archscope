import {
  exportNodeImage,
  type ImageFormat,
  type ImageMultiplier,
} from "@/lib/exportImage";

export type BatchExportOptions = {
  format?: ImageFormat;
  multiplier?: ImageMultiplier;
  /** Filename prefix prepended to each chart name. */
  prefix?: string;
  /** Optional progress callback. */
  onProgress?: (done: number, total: number, name: string) => void;
};

/**
 * Walk a container DOM tree, locate every chart export target marker, and
 * trigger image downloads in sequence.
 *
 * Pages that want batch export should render charts via `D3ChartFrame` or
 * mark the relevant nodes with `data-chart-export-target` (and an optional
 * `data-chart-export-name` on an ancestor for the filename).
 */
export async function exportChartsInContainer(
  container: HTMLElement,
  options: BatchExportOptions = {},
): Promise<number> {
  const { format = "png", multiplier = 2, prefix, onProgress } = options;
  const targets = Array.from(
    container.querySelectorAll<HTMLElement>("[data-chart-export-target]"),
  );
  for (let i = 0; i < targets.length; i += 1) {
    const target = targets[i];
    const named = target.closest<HTMLElement>("[data-chart-export-name]");
    const name =
      named?.dataset.chartExportName ||
      target.getAttribute("aria-label") ||
      `chart-${i + 1}`;
    const filename = prefix ? `${prefix}-${name}` : name;
    onProgress?.(i, targets.length, filename);
    await exportNodeImage(target, {
      filename,
      format,
      multiplier,
    });
  }
  onProgress?.(targets.length, targets.length, "");
  return targets.length;
}
