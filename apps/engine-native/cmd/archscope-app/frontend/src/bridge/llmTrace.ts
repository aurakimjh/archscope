import {Call,type CancellablePromise} from "@wailsio/runtime";
import type {Cancelable} from "@/state/resultProviders";
export type LLMInventory={result_sha256?:string;source_sha256:string;semantic_profile:string;trace_namespace:string;privacy_policy:string;span_count:number;link_count:number;event_count:number;context_count:number;contexts_truncated:boolean;contexts:Record<string,string>[];warnings:string[]};
export type LLMQuery={kind:string;trace_id?:string;record_id?:string;request_namespace?:string;logical_request_id?:string;cursor?:string;limit:number};
export type LLMPage={version:string;result_sha256:string;query_sha256:string;total:number;next:string;rows:Record<string,unknown>[]};
const call=<T>(name:string,...args:unknown[]):CancellablePromise<T>=>Call.ByName(`main.EngineService.${name}`,...args);
function parse<T>(request:CancellablePromise<string>):Cancelable<T>{const result=request.then(raw=>JSON.parse(raw) as T) as Cancelable<T>;result.cancel=()=>request.cancel();return result;}
export const inspect=(path:string,options:string):Cancelable<LLMInventory>=>parse(call("InspectLLMTraceSource",path,options));
export const normalize=(path:string,options:string,digest:string,output:string):Cancelable<LLMInventory>=>parse(call("NormalizeLLMTraceSource",path,options,digest,output));
export const describe=(handle:string):Cancelable<LLMInventory>=>parse(call("DescribeLLMTrace",handle));
export const query=(handle:string,digest:string,q:LLMQuery):Cancelable<LLMPage>=>parse(call("QueryLLMTrace",handle,digest,JSON.stringify(q)));
