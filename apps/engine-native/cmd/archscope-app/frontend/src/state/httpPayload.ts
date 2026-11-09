import type { CaptureHTTPMessage } from "../bridge/types";

export type HttpParameter = {
  name: string;
  value: string;
  rawName: string;
  rawValue: string;
  decodeError: boolean;
  filename?: string;
  contentType?: string;
};

function decode(value: string): { value: string; failed: boolean } {
  try { return { value: decodeURIComponent(value.replace(/\+/g, " ")), failed: false }; }
  catch { return { value, failed: true }; }
}

// Keep order, empty values and repeated names. The encoded spelling is
// necessary when diagnosing double encoding or malformed percent escapes.
export function httpParameters(raw: string): HttpParameter[] {
  if (!raw) return [];
  return raw.split("&").map((pair) => {
    const split = pair.indexOf("=");
    const rawName = split < 0 ? pair : pair.slice(0, split);
    const rawValue = split < 0 ? "" : pair.slice(split + 1);
    const name = decode(rawName);
    const value = decode(rawValue);
    return { name: name.value, value: value.value, rawName, rawValue, decodeError: name.failed || value.failed };
  });
}

export function requestQuery(query: string | undefined, url: string): string {
  return query || url.split("?")[1]?.split("#")[0] || "";
}

export type FormInspection = { fields: HttpParameter[]; state: "available" | "unavailable" | "unsupported" | "limited" };
const MAX_VIEW_BYTES = 1024 * 1024;

export function inspectForm(message: CaptureHTTPMessage | undefined): FormInspection {
  if (!message || !["inline", "truncated"].includes(message.bodyStorage) || message.bodyPreview === undefined) {
    return { fields: [], state: "unavailable" };
  }
  const body = message.bodyPreview;
  if (body.length > MAX_VIEW_BYTES) return { fields: [], state: "limited" };
  const mime = (message.contentType ?? "").toLowerCase();
  if (mime.startsWith("application/x-www-form-urlencoded")) {
    return { fields: httpParameters(body), state: message.bodyStorage === "truncated" ? "limited" : "available" };
  }
  if (!mime.startsWith("multipart/form-data")) return { fields: [], state: "unsupported" };
  const boundary = /boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(message.contentType ?? "");
  const marker = boundary?.[1] ?? boundary?.[2];
  if (!marker || marker.length > 200) return { fields: [], state: "unavailable" };
  const parts = body.split("--" + marker).slice(1);
  const fields: HttpParameter[] = [];
  for (const part of parts.slice(0, 128)) {
    if (part.startsWith("--")) break;
    const split = part.indexOf("\r\n\r\n");
    if (split < 0) continue;
    const headers = part.slice(0, split);
    const disposition = /^content-disposition:\s*form-data;([^\r\n]*)/im.exec(headers)?.[1] ?? "";
    const name = /(?:^|;)\s*name="([^"]*)"/i.exec(disposition)?.[1];
    if (name === undefined) continue;
    const value = part.slice(split + 4).replace(/\r\n$/, "");
    fields.push({
      name, value, rawName: name, rawValue: value, decodeError: false,
      filename: /(?:^|;)\s*filename="([^"]*)"/i.exec(disposition)?.[1],
      contentType: /^content-type:\s*([^\r\n]*)/im.exec(headers)?.[1],
    });
  }
  return { fields, state: parts.length > 129 || message.bodyStorage === "truncated" ? "limited" : "available" };
}

export function inspectJson(message: CaptureHTTPMessage | undefined): { value?: unknown; state: "available" | "unavailable" | "invalid" | "limited" } {
  if (!message || message.bodyPreview === undefined || !["inline", "truncated"].includes(message.bodyStorage)) {
    return { state: "unavailable" };
  }
  if (message.bodyPreview.length > MAX_VIEW_BYTES) return { state: "limited" };
  try {
    return { value: JSON.parse(message.bodyPreview), state: message.bodyStorage === "truncated" ? "limited" : "available" };
  } catch { return { state: message.bodyStorage === "truncated" ? "limited" : "invalid" }; }
}
