import {Call,type CancellablePromise} from "@wailsio/runtime";
import type {Cancelable} from "@/state/resultProviders";
export type Inventory={result_sha256?:string;source_sha256?:string;kind:string;source:Record<string,string>;context:Record<string,unknown>|null;metric_count:number;sample_count:number;crash_frames:number;comparison_unavailable_reasons:string[];warnings:string[]};
export type Query={kind:string;metric_name?:string;group_sha256?:string;limit:number;cursor?:string};
export type Page={result_sha256:string;query_sha256:string;rows:Record<string,unknown>[];next:string;total:number};
export type Comparison={version:string;baseline_result_sha256:string;target_result_sha256:string;baseline_source_sha256:string;target_source_sha256:string;context_sha256:string;status:string;reasons:string[];rows:Record<string,unknown>[];next:string;total:number;query_sha256:string};
const call=<T>(name:string,...args:unknown[]):CancellablePromise<T>=>Call.ByName(`main.EngineService.${name}`,...args);
function parse<T>(request:CancellablePromise<string>):Cancelable<T>{const result=request.then(raw=>JSON.parse(raw) as T) as Cancelable<T>;result.cancel=()=>request.cancel();return result;}
export const inspect=(path:string,options:string):Cancelable<Inventory>=>parse(call("InspectExecutionSource",path,options));
export const normalize=(path:string,options:string,digest:string,output:string):Cancelable<Inventory>=>parse(call("NormalizeExecutionSource",path,options,digest,output));
export const describe=(handle:string):Cancelable<Inventory>=>parse(call("DescribeExecutionEvidence",handle));
export const query=(handle:string,digest:string,q:Query):Cancelable<Page>=>parse(call("QueryExecutionEvidence",handle,digest,JSON.stringify(q)));
export const compare=(left:string,right:string,leftSHA:string,rightSHA:string,q:{cursor?:string;limit:number}):Cancelable<Comparison>=>parse(call("CompareExecutionEvidence",left,right,leftSHA,rightSHA,JSON.stringify(q)));
export const exportPage=(left:string,right:string,leftSHA:string,rightSHA:string,q:{cursor?:string;limit:number},output:string):Cancelable<Comparison>=>parse(call("ExportExecutionComparison",left,right,leftSHA,rightSHA,JSON.stringify(q),output));
