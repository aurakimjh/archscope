// ─────────────────────────────────────────────────────────────────────
// [한글] lib/batchExport.ts — 컨테이너 DOM 안의 모든 차트를 한 번에
//   이미지(PNG 등)로 일괄 저장.
//
// 책임/목적:
//   - 페이지가 chartsContainerRef 를 넘겨 주면 그 안의
//     `[data-chart-export-target]` 노드를 모두 찾아 순차 export.
//   - 파일명은 가장 가까운 `[data-chart-export-name]` 조상의 dataset 값
//     또는 aria-label, 마지막으로 chart-N fallback.
//   - prefix 옵션으로 페이지별 접두사(예: "dashboard", "gc-log") 부여.
//
// 데이터 흐름:
//   페이지 "Save all charts" 버튼 → exportChartsInContainer → for-loop
//   exportNodeImage 호출.
//
// 페이지 통합:
//   - 새 차트 컴포넌트를 추가할 때는 ChartPanel/D3ChartFrame 처럼
//     export marker 두 속성을 부여해야 일괄 저장에 포함됩니다.
// ─────────────────────────────────────────────────────────────────────
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
