// Windows-owned instrumented test app only; aggregate reads, never native
// acceptance or a heap snapshot. Credentials/URLs/raw identities stay private.
import assert from 'node:assert/strict';
import {readFile,open} from 'node:fs/promises';
import {execFileSync} from 'node:child_process';
import path from 'node:path';
import {connectMemoryProbe,readRendererMemory,verifyOwnedDebugSample} from './lib/renderer-memory-probe.mjs';
assert.equal(process.platform,'win32','native collector requires Windows; library regression is portable');
const options={};for(let i=2;i<process.argv.length;i+=2){assert(['--descriptor','--exe','--port','--target-id','--commit','--out','--samples','--interval-ms'].includes(process.argv[i])&&process.argv[i+1]);options[process.argv[i].slice(2)]=process.argv[i+1];}
for(const key of ['descriptor','exe','port','target-id','commit','out'])assert(options[key],key+' required');
const port=Number(options.port),samples=Number(options.samples??180),interval=Number(options['interval-ms']??5000);
assert(Number.isInteger(port)&&port>0&&port<=65535);assert(Number.isInteger(samples)&&samples>0&&samples<=5000);assert(Number.isInteger(interval)&&interval>=1000&&interval<=60000);
assert(/^[a-zA-Z0-9_-]{1,128}$/.test(options['target-id']));assert(/^[a-f0-9]{7,40}(?:[-+][\w.-]+)?$/.test(options.commit));
const raw=await readFile(path.resolve(options.descriptor));assert(raw.length<=4096);const descriptor=JSON.parse(raw);
assert.equal(descriptor.schemaVersion,1);assert(Number.isSafeInteger(descriptor.pid)&&descriptor.pid>0);assert(/^[a-f0-9]{64}$/.test(descriptor.token));
const endpoint=new URL(descriptor.url);assert.equal(endpoint.protocol,'http:');assert.equal(endpoint.hostname,'127.0.0.1');assert.equal(endpoint.pathname,'/v1/control');assert(!endpoint.username&&!endpoint.password&&!endpoint.search&&!endpoint.hash);
let rootStart='',sequence=0;const identities=new Map();
function processSample(){const args=['-NoProfile','-File',path.resolve('scripts/acceptance/layout/sample-owned-debug-tree.ps1'),'-RootProcessId',String(descriptor.pid),'-ExpectedExecutable',path.resolve(options.exe),'-DebugPort',String(port)];if(rootStart)args.push('-ExpectedStartTime',rootStart);const sample=verifyOwnedDebugSample(JSON.parse(execFileSync('powershell.exe',args,{encoding:'utf8',timeout:20000,windowsHide:true})));rootStart=sample.rootStartTime;return sample;}
processSample();
const capability=await fetch(endpoint,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:'aggregate-memory-owner',command:'capabilities',params:{}}),signal:AbortSignal.timeout(10000),redirect:'error'});
assert.equal(capability.status,200);const payload=await capability.json();assert(payload.ok&&payload.result.buildCommit===options.commit,'owned app source mismatch');
const targetResponse=await fetch(`http://127.0.0.1:${port}/json/list`,{signal:AbortSignal.timeout(5000),redirect:'error'});assert.equal(targetResponse.status,200);const targets=await targetResponse.json();
const target=targets.find(t=>t.id===options['target-id']&&t.type==='page');assert(target,'explicit owned page target missing');const page=new URL(target.url);assert.equal(page.hostname,'wails.localhost','owned embedded Wails page required');assert(['http:','https:','wails:'].includes(page.protocol));const ws=new URL(target.webSocketDebuggerUrl);assert.equal(ws.port,String(port));assert.equal(ws.pathname,'/devtools/page/'+options['target-id']);
const call=await connectMemoryProbe(ws.href);let output;
try{
 output=await open(path.resolve(options.out),'wx',0o600);
 for(let i=0;i<samples;i++){
  const started=Date.now(),owned=processSample(),processCapturedAt=Date.now();const aggregate=await readRendererMemory(call);const finished=Date.now();
  const members=owned.members.map(m=>{if(!identities.has(m.identity)){assert(identities.size<256,'owned process churn limit');identities.set(m.identity,++sequence);}return {ordinal:identities.get(m.identity),role:m.role,privateBytes:m.privateBytes,workingSetBytes:m.workingSetBytes};});
  const record={schemaVersion:1,sourceCommit:options.commit,instrumented:true,nativeAcceptance:false,forcedGC:false,startedEpochMs:started,processCapturedEpochMs:processCapturedAt,finishedEpochMs:finished,sampleSpanMs:finished-started,members,...aggregate};
  assert(Buffer.byteLength(JSON.stringify(record))<=8192);await output.write(JSON.stringify(record)+'\n');
  if(i+1<samples)await new Promise(resolve=>setTimeout(resolve,Math.max(0,interval-(Date.now()-started))));
 }
 console.log(JSON.stringify({completedSamples:samples,scope:'owned instrumented numeric counters only',nativeAcceptance:false}));
}finally{call.close();if(output)await output.close();}
