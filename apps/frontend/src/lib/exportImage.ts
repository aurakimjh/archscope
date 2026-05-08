// ─────────────────────────────────────────────────────────────────────
// [한글] lib/exportImage.ts — 차트/카드 DOM 을 PNG/JPEG/SVG 로 export 하는
//   공용 헬퍼와 프리셋 정의.
//
// 책임/목적:
//   - ECharts 인스턴스가 있으면 native getDataURL(이미지 품질 최고)을 우선.
//   - SVG/Canvas 혼합 D3 차트는 html-to-image 로 DOM 캡처(toPng/toJpeg/toSvg).
//   - downloadDataUrl 로 a[download] 트릭을 사용해 즉시 저장 다이얼로그.
//
// 데이터 흐름:
//   ChartPanel/D3ChartFrame → exportNodeImage(node, options) → 다운로드.
//
// 프리셋(saveImagePresets):
//   - PNG @1x, PNG @2x(고해상도), SVG, JPEG 등의 기본 포맷을 메뉴에 노출.
// ─────────────────────────────────────────────────────────────────────
/**
 * Image-export helpers for the chart system.
 *
 * Strategies:
 *  - ECharts panels: prefer the native `getDataURL` (lossless, picks up the
 *    chart's current theme/background and accepts a pixel ratio).
 *  - D3/Recharts/SVG output: rasterize via `html-to-image` which handles
 *    inline styles and computed CSS variables.
 *  - Arbitrary DOM nodes (e.g. report cards): same as above.
 */
import type { ECharts } from "echarts";
import { toJpeg, toPng, toSvg } from "html-to-image";

export type ImageMultiplier = 1 | 2 | 3;
export type ImageFormat = "png" | "jpeg" | "svg";

export type ExportImageOptions = {
  filename: string;
  multiplier?: ImageMultiplier;
  format?: ImageFormat;
  background?: string;
};

const DEFAULT_BACKGROUND = "#ffffff";

export function downloadDataUrl(dataUrl: string, filename: string): void {
  const link = document.createElement("a");
  link.href = dataUrl;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}

function safeFilename(name: string, extension: string): string {
  const base = name.trim().replace(/\s+/g, "_") || "archscope";
  return base.endsWith(`.${extension}`) ? base : `${base}.${extension}`;
}

/**
 * Export an ECharts instance to PNG/SVG using its native renderer.
 * SVG is only available when the chart was initialised with `renderer: "svg"`.
 */
export function exportEchartsImage(
  chart: ECharts,
  options: ExportImageOptions,
): void {
  const { filename, multiplier = 2, format = "png", background = DEFAULT_BACKGROUND } = options;
  if (format === "jpeg") {
    throw new Error("ECharts native export does not support JPEG. Pass an HTMLElement instead.");
  }
  const dataUrl = chart.getDataURL({
    type: format,
    backgroundColor: background,
    pixelRatio: multiplier,
  });
  downloadDataUrl(dataUrl, safeFilename(filename, format));
}

/**
 * Rasterize an arbitrary DOM node (including SVG/charts) at the requested
 * resolution multiplier and trigger a download.
 */
export async function exportNodeImage(
  node: HTMLElement | SVGElement,
  options: ExportImageOptions,
): Promise<void> {
  const { filename, multiplier = 2, format = "png", background = DEFAULT_BACKGROUND } = options;
  const opts = {
    pixelRatio: multiplier,
    backgroundColor: background,
    cacheBust: true,
  } as const;

  let dataUrl: string;
  if (format === "png") {
    dataUrl = await toPng(node as HTMLElement, opts);
  } else if (format === "jpeg") {
    dataUrl = await toJpeg(node as HTMLElement, { ...opts, quality: 0.95 });
  } else {
    dataUrl = await toSvg(node as HTMLElement, opts);
  }
  downloadDataUrl(dataUrl, safeFilename(filename, format));
}

export type SaveImagePreset = {
  id: string;
  format: ImageFormat;
  multiplier: ImageMultiplier;
  labelKey:
    | "saveImagePng1x"
    | "saveImagePng2x"
    | "saveImagePng3x"
    | "saveImageJpeg2x"
    | "saveImageSvg";
};

export const saveImagePresets: SaveImagePreset[] = [
  { id: "png-1x", format: "png", multiplier: 1, labelKey: "saveImagePng1x" },
  { id: "png-2x", format: "png", multiplier: 2, labelKey: "saveImagePng2x" },
  { id: "png-3x", format: "png", multiplier: 3, labelKey: "saveImagePng3x" },
  { id: "jpeg-2x", format: "jpeg", multiplier: 2, labelKey: "saveImageJpeg2x" },
  { id: "svg", format: "svg", multiplier: 1, labelKey: "saveImageSvg" },
];
