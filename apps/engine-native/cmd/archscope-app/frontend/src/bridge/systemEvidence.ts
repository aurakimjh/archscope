import {Call,type CancellablePromise} from "@wailsio/runtime";
import type {Cancelable} from "@/state/resultProviders";
export type SystemInventory={source_sha256:string;result_sha256?:string;format:string;tool:string;tool_version:string;records:string;entities:{id:string;pid:string;tid:string;process_start_ns:string|null}[];projections:{id:string;kind:string;unit:string}[];warnings:string[]};
const call=<T>(name:string,...args:unknown[]):CancellablePromise<T>=>Call.ByName(`main.EngineService.${name}`,...args);
function parse<T>(request:CancellablePromise<string>):Cancelable<T>{const value=request.then(raw=>JSON.parse(raw) as T) as Cancelable<T>;value.cancel=()=>request.cancel();return value;}
export const inspect=(path:string,options:string):Cancelable<SystemInventory>=>parse(call("InspectSystemSource",path,options));
export const normalize=(path:string,options:string,digest:string,output:string):Cancelable<SystemInventory>=>parse(call("NormalizeSystemSource",path,options,digest,output));
export const describe=(handle:string):Cancelable<SystemInventory>=>parse(call("DescribeSystemEvidence",handle));
export const project=(handle:string,digest:string,metric:string,entities:string[]):CancellablePromise<string>=>call("MaterializeSystemProfile",handle,digest,metric,JSON.stringify(entities));
export const openOriginal=(caseID:string,revision:string,resultID:string,resultSHA:string,sourceID:string,sourceSHA:string,eventID:string):Cancelable<{source_sha256:string;lines:{number:string;raw_start:string;raw_end:string;text:string}[]}>=>parse(call("OpenSystemEvidence",caseID,revision,resultID,resultSHA,sourceID,sourceSHA,eventID));
export type HistogramPage={result_sha256:string;event_id:string;source_sha256:string;source_ref:string;definition:{name:string;unit:string;population:string;aggregation:string;reset_id:string;from_ns:string|null;to_ns:string|null};key:string;offset:number;next:number;total:number;retained_count:string;rows:{lower:string|null;upper:string|null;count:string;width:number}[];warnings:string[]};
export const histogram=(handle:string,digest:string,event:string,offset:number):Cancelable<HistogramPage>=>parse(call("QuerySystemHistogram",handle,digest,event,offset,25));
