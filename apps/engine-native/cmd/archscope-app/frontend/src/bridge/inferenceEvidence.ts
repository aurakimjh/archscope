import {Call,type CancellablePromise} from "@wailsio/runtime";
import type {Cancelable} from "@/state/resultProviders";
export type InferenceSignal={id:string;series_id:string;semantic:string;entity_id:string;context_id?:string;boundary:string;population:string;statistical_form:string;conversion:string;declared_metadata?:{unit:string;kind:string;temporality:string;monotonic:boolean;start_ns?:string;expected_step_ns?:string};source_calculation?:string};
export type InferenceInventory={result_sha256?:string;source_sha256?:string;entities:{id:string;kind:string;host_id:string;boot_id:string;device_uuid?:string;mig_uuid?:string;pid?:string;process_start_ns?:string;rank?:string}[];signals:InferenceSignal[];contexts:Record<string,string|null>[];links:{from:string;to:string;kind:string;ref:{source_id:string;position:string}}[];unmapped_series:number;metric_points:number;events:number;warnings:string[]};
export type MetricPage={result_sha256:string;total:number;next:string;rows:{point_id:string;start_ns:string|null;end_ns:string|null;value:string|null;unit:string;delta:string|null;rate:string|null;rate_meaning:string;status:string;gap:boolean;observed_histogram?:unknown;histogram?:unknown;refs:{source_id:string;position:string}[]}[]};
const call=<T>(name:string,...args:unknown[]):CancellablePromise<T>=>Call.ByName(`main.EngineService.${name}`,...args);
function parse<T>(request:CancellablePromise<string>):Cancelable<T>{const result=request.then(raw=>JSON.parse(raw) as T) as Cancelable<T>;result.cancel=()=>request.cancel();return result;}
export const inspect=(path:string,options:string):Cancelable<InferenceInventory>=>parse(call("InspectInferenceSource",path,options));
export const normalize=(path:string,options:string,digest:string,output:string):Cancelable<InferenceInventory>=>parse(call("NormalizeInferenceSource",path,options,digest,output));
export const describe=(handle:string):Cancelable<InferenceInventory>=>parse(call("DescribeInferenceEvidence",handle));
export const query=(handle:string,digest:string,signal:string,cursor:string):Cancelable<MetricPage>=>parse(call("QueryInferenceMetric",handle,digest,signal,cursor,25));
export const materialize=(handle:string,digest:string,signals:string[]):CancellablePromise<string>=>call("MaterializeInferenceMetrics",handle,digest,JSON.stringify(signals));
