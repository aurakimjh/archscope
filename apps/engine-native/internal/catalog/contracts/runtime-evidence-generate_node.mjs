// Node.js 25.9.0의 실제 getReport/perf_hooks/worker API를 사용한 비식별 합성 export.
import fs from 'node:fs';import path from 'node:path';import {fileURLToPath} from 'node:url';import {Worker} from 'node:worker_threads';import {monitorEventLoopDelay,performance} from 'node:perf_hooks';
if(process.version!=='v25.9.0')throw Error('pinned Node.js 25.9.0 required');process.report.excludeEnv=true;
const base=path.dirname(fileURLToPath(import.meta.url));
function clean(report){return {header:{reportVersion:report.header.reportVersion,nodejsVersion:report.header.nodejsVersion,processId:'600',threadId:String(report.header.threadId)},javascriptStack:{stack:(report.javascriptStack?.stack??[]).map(s=>s.replaceAll(base,'/synthetic').replaceAll(process.cwd(),'/synthetic-cwd'))},nativeStack:(report.nativeStack??[]).map(v=>({symbol:v.symbol})),libuv:(report.libuv??[]).map(v=>({type:v.type,is_active:v.is_active})),workers:[]};}
const worker=new Worker(`const {parentPort}=require('node:worker_threads');process.report.excludeEnv=true;parentPort.postMessage(process.report.getReport());`,{eval:true});
const workerReport=await new Promise((resolve,reject)=>{worker.once('message',resolve);worker.once('error',reject)});await worker.terminate();
const histogram=monitorEventLoopDelay({resolution:10});histogram.enable();const elu=performance.eventLoopUtilization();const from=process.hrtime.bigint();await new Promise(resolve=>setTimeout(resolve,40));const to=process.hrtime.bigint();const measured=performance.eventLoopUtilization(elu);histogram.disable();
const report=clean(process.report.getReport());report.workers=[clean(workerReport)];
const finite=n=>Number.isFinite(n)?n.toString():null;
const value={schema:'node-runtime-export.v1',report,time_ns:'1000000000',event_loop:[{thread_id:'0',from_ns:'1000000000',to_ns:(1000000000n+to-from).toString(),delay_mean_ns:finite(histogram.mean),delay_p95_ns:finite(histogram.percentile(95)),delay_max_ns:finite(histogram.max),utilization_ratio:finite(measured.utilization)}]};
fs.writeFileSync(path.join(base,'node-25.9.0.json'),JSON.stringify(value,null,2)+'\n');
