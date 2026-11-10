import type { HttpCaptureTransactionRow } from "@/bridge/types";
export const HTTP_BUILDER_COPY = "archscope:http-builder-copy";
export const HTTP_BUILDER_DRAG = "application/x-archscope-http-request";
export type BuilderCopy = {
  row: HttpCaptureTransactionRow;
  sessionId?: string;
};
export function copyToHttpBuilder(value: BuilderCopy): void {
  window.dispatchEvent(new CustomEvent(HTTP_BUILDER_COPY, { detail: value }));
}
const managed = new Set([
  "content-length",
  "transfer-encoding",
  "connection",
  "proxy-connection",
  "proxy-authorization",
  "upgrade",
  "trailer",
  "te",
]);
export function builderHeaders(row: HttpCaptureTransactionRow): string {
  return row.request.headers
    .filter((header) => !managed.has(header.name.toLowerCase()))
    .map((header) => `${header.name}: ${header.value}`)
    .join("\n");
}
export function parseBuilderHeaders(
  text: string,
): { name: string; value: string }[] {
  return text
    .split(/\r?\n/)
    .filter((line) => line.trim())
    .map((line, index) => {
      const colon = line.indexOf(":");
      if (colon <= 0) throw new Error(`Header ${index + 1}: name: value`);
      return {
        name: line.slice(0, colon).trim(),
        value: line.slice(colon + 1).trim(),
      };
    });
}
