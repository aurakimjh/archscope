import {Call,type CancellablePromise} from "@wailsio/runtime";
import type {Cancelable} from "@/state/resultProviders";
export type RuntimeInventory={result_sha256?:string;source_sha256?:string;sources:Record<string,string>[];processes:Record<string,string|null>[];entities:{id:string;kind:string;name:string;runtime_id:string;native_id:string;mapping_basis:string}[];entity_count:number;truncated:boolean;observations:number;plan_nodes:number;profile_refs:number;warnings:string[]};
export type RuntimeQuery={kind:string;entity_id?:string;stack_id?:string;frame_id?:string;cpu_observation_id?:string;stack_observation_id?:string;cursor?:string;limit:number};
export type RuntimePage={result_sha256:string;query_sha256:string;total:number;next:string;rows:Record<string,unknown>[]};
const call=<T>(name:string,...args:unknown[]):CancellablePromise<T>=>Call.ByName(`main.EngineService.${name}`,...args);
function parse<T>(request:CancellablePromise<string>):Cancelable<T>{const result=request.then(raw=>JSON.parse(raw) as T) as Cancelable<T>;result.cancel=()=>request.cancel();return result;}
export const inspect=(path:string,options:string):Cancelable<RuntimeInventory>=>parse(call("InspectRuntimeSource",path,options));
export const normalize=(path:string,options:string,digest:string,output:string):Cancelable<RuntimeInventory>=>parse(call("NormalizeRuntimeSource",path,options,digest,output));
export const merge=(paths:string[],output:string):Cancelable<RuntimeInventory>=>parse(call("MergeRuntimeModels",JSON.stringify(paths),output));
export const describe=(handle:string):Cancelable<RuntimeInventory>=>parse(call("DescribeRuntimeEvidence",handle));
export const query=(handle:string,digest:string,q:RuntimeQuery):Cancelable<RuntimePage>=>parse(call("QueryRuntimeEvidence",handle,digest,JSON.stringify(q)));

export const allocation=(handle:string,digest:string,entity:string,source:string,metric:string):CancellablePromise<string>=>call("MaterializeRuntimeAllocation",handle,digest,entity,source,metric);
