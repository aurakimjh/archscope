import type {HttpCaptureTransactionRow} from "../bridge/types";
import {httpProcessKey,availablePerspectives,buildWaterfall} from "./httpAnalyzer.js";
export type HttpSummary={total:number;completed:number;pending:number;sent:{bytes:number;unknown:number};received:{bytes:number;unknown:number};decoded:{bytes:number;unknown:number};compressionSaved:number;compressionKnown:number;connections:number;connectionsUnknown:number;dns:number;dnsUnknown:number;elapsedMs:number;timeKnown:boolean;statusCounts:Record<string,number>;phases:{perspective:string;name:string;ms:number;known:number;unknown:number;notApplicable:number}[]};
export function summarizeHttpRows(rows:HttpCaptureTransactionRow[]):HttpSummary{
 const out:HttpSummary={total:rows.length,completed:0,pending:0,sent:{bytes:0,unknown:0},received:{bytes:0,unknown:0},decoded:{bytes:0,unknown:0},compressionSaved:0,compressionKnown:0,connections:0,connectionsUnknown:0,dns:0,dnsUnknown:0,elapsedMs:0,timeKnown:false,statusCounts:{},phases:[]};
 const connections=new Set<string>(),dns=new Set<string>();let first=Infinity,last=-Infinity;
 for(const row of rows){if(["complete","failed","aborted"].includes(row.state))out.completed++;else out.pending++;
  for(const [sum,value] of [[out.sent,row.request.bodySize],[out.received,row.response.bodySize],[out.decoded,row.response.bodyDecoded]] as const){if(Number.isFinite(value)&&value>=0)sum.bytes+=value;else sum.unknown++}
  if(row.response.bodyDecoded>=0&&row.response.bodySize>=0){out.compressionKnown++;out.compressionSaved+=row.response.bodyDecoded-row.response.bodySize}
  if(row.connection_id)connections.add(row.connection_id);else out.connectionsUnknown++;
  const group=row.status>=200&&row.status<600?`${Math.floor(row.status/100)}xx`:"other";out.statusCounts[group]=(out.statusCounts[group]??0)+1;
  const start=Date.parse(row.started_at),end=Date.parse(row.ended_at);if(Number.isFinite(start)){first=Math.min(first,start);if(Number.isFinite(end))last=Math.max(last,end);else if(row.duration_ms>=0&&Number.isFinite(row.duration_ms))last=Math.max(last,start+row.duration_ms)}
  const phases=row.timings.proxyUpstream??row.timings.importedHar;if(!phases||!["known","not_applicable"].includes(phases.dns.state))out.dnsUnknown++;else if(phases.dns.state==="known"&&phases.dns.ms>0)dns.add(row.connection_id||row.id);
 }
 out.connections=connections.size;out.dns=dns.size;out.timeKnown=Number.isFinite(first)&&Number.isFinite(last)&&last>=first;out.elapsedMs=out.timeKnown?last-first:0;
 for(const perspective of ["clientProxy","proxyInternal","proxyUpstream","importedHar"] as const)for(const name of ["blocked","dns","connect","tls","send","wait","receive"] as const){const phase={perspective,name,ms:0,known:0,unknown:0,notApplicable:0};for(const row of rows){const value=row.timings[perspective]?.[name];if(value?.state==="not_applicable")phase.notApplicable++;else if(value?.state==="known"&&Number.isFinite(value.ms)&&value.ms>=0){phase.ms+=value.ms;phase.known++}else phase.unknown++}out.phases.push(phase)}
 return out;
}
// Each process has its own zero point. Missing timestamps never become epoch 0.
export function relativeProcessRows(rows:HttpCaptureTransactionRow[]){
 const first=new Map<string,number>(),last=new Map<string,number>();
 for(const row of rows){const start=Date.parse(row.started_at);if(!Number.isFinite(start))continue;const key=httpProcessKey(row);first.set(key,Math.min(first.get(key)??Infinity,start));last.set(key,Math.max(last.get(key)??-Infinity,start+Math.max(0,row.duration_ms||0)))}
 return rows.map(row=>{const key=httpProcessKey(row),base=first.get(key),end=last.get(key),start=Date.parse(row.started_at);return {row,offsetMs:base!==undefined&&Number.isFinite(start)?start-base:null,spanMs:base!==undefined&&end!==undefined?end-base:null,waterfall:buildWaterfall(row,availablePerspectives(row)[0]??null)}});
}
