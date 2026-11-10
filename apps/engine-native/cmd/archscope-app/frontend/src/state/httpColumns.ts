import {rowHints} from "./httpAnnotations.js";
import type { MessageKey } from "../i18n/messages";
import type { HttpCaptureTransactionRow } from "../bridge/types";

export type HttpColumn = { key: string; labelKey?: MessageKey; label?: string; width: number; visible: boolean };
export const HTTP_COLUMNS: HttpColumn[] = [
  {key:"sequence",labelKey:"httpAnalyzerColSeq",width:56,visible:true},
  {key:"started",labelKey:"httpAnalyzerColStarted",width:112,visible:true},
  {key:"method",labelKey:"httpAnalyzerColMethod",width:72,visible:true},
  {key:"status",labelKey:"httpAnalyzerColStatus",width:64,visible:true},
  {key:"host",labelKey:"httpAnalyzerColHost",width:192,visible:true},
  {key:"path",labelKey:"httpAnalyzerColPath",width:224,visible:true},
  {key:"kind",labelKey:"httpAnalyzerColKind",width:104,visible:true},
  {key:"size",labelKey:"httpAnalyzerColSize",width:88,visible:true},
  {key:"duration",labelKey:"httpAnalyzerColDuration",width:80,visible:true},
  {key:"timeline",labelKey:"httpAnalyzerColTimeline",width:224,visible:true},
  {key:"process",labelKey:"httpAnalyzerColProcess",width:144,visible:true},
  ...[
    ["connection","ConnectId"],["url","URL"],["offset","Offset"],["redirect","Redirect URL"],
    ["sent","Sent"],["bodySize","Body-Size"],["ended","TimeEnd"],["mime","Type"],
    ["starred","★"],["hints","Hints"],["hintList","Hint List"],["comment","Comment"],["caching","Caching"],
    ["content-length","Content-Length"],["ip","IP Address"],["cookie","Cookie"],["set-cookie","Set-Cookie"],
    ["user-agent","User-Agent"],["ua-cpu","UA-CPU"],["soapaction","SOAPAction"],["date","Date"],
    ["server","Server"],["p3p","P3P"],["content-type","Content-Type"],["content-encoding","Content-Encoding"],
    ["transfer-encoding","Transfer-Encoding"],["etag","ETag"],["expires","Expires"],
    ["last-modified","Last-Modified"],["x-powered-by","X-Powered-By"],["stage","Stage"],
    ["fidelity","Fidelity"],["pid","PID"],
  ].map(([key,label]) => ({key,label,width:160,visible:false})),
];

// Persisted preferences are untrusted and may come from an older build.
export function restoreHttpColumns(raw: unknown): HttpColumn[] {
  const configured = Array.isArray(raw) ? raw : [];
  const byKey = new Map(HTTP_COLUMNS.map(column => [column.key,column]));
  const result: HttpColumn[] = [];
  for (const item of configured) {
    if (!item || typeof item.key !== "string") continue;
    const base = byKey.get(item.key);
    if (!base) continue;
    const width = Number.isFinite(item.width) ? Math.max(48,Math.min(800,item.width)) : base.width;
    result.push({...base,width,visible:typeof item.visible==="boolean"?item.visible:base.visible});
    byKey.delete(item.key);
  }
  result.push(...byKey.values());
  if (!result.some(column => column.visible)) result[0].visible=true;
  return result;
}

export function httpColumnValue(row: HttpCaptureTransactionRow,key: string): string | number {
  const requestHeader = ["cookie","user-agent","ua-cpu","soapaction"].includes(key);
  const headers = requestHeader ? row.request?.headers : row.response?.headers;
  switch (key) {
    case "sequence": return row.sequence;
    case "started": return row.started_at;
    case "ended": return row.ended_at;
    case "method": return row.method;
    case "status": return row.status || row.state;
    case "host": return row.host;
    case "path": return row.path + (row.query ? "?" + row.query : "");
    case "url": return row.url;
    case "size": return row.response_bytes;
    case "sent": return row.request_bytes;
    case "bodySize": return row.response?.bodySize ?? -1;
    case "duration": return row.duration_ms;
    case "process": return row.process?.name ?? "—";
    case "pid": return row.process?.pid ?? "—";
    case "connection": return row.connection_id || "—";
    case "redirect": return row.redirect_url || "—";
    case "stage": return row.state;
    case "fidelity": return row.fidelity;
    case "mime": return row.response?.contentType || "—";
    case "hints": return rowHints(row).length;
    case "hintList": return rowHints(row).map(hint=>hint.code).join("; ")||"—";
    case "comment": return row.comment||"—";
    case "starred":return row.starred?"★":"☆";
    case "ip":return "—";
    case "caching": key="cache-control"; break;
  }
  return headers?.filter(header => header.name.toLowerCase()===key).map(header => header.value).join("; ") || "—";
}


export function httpColumnMatches(row: HttpCaptureTransactionRow,key:string,operator:string,needle:string): boolean {
  if (!needle) return true;
  const value=httpColumnValue(row,key);
  if (operator==="gte" || operator==="lte") {
    const number=Number(value);
    const bound=Number(needle);
    return value!=="—" && Number.isFinite(number) && Number.isFinite(bound) && (operator==="gte"?number>=bound:number<=bound);
  }
  const text=String(value).toLowerCase();
  return operator==="equals" ? text===needle.toLowerCase() : text.includes(needle.toLowerCase());
}
