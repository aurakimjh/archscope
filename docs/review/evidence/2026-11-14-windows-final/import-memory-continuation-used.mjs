// Declared follow-up: preserve the36-cycle verdict, use the same owned amd64 app,
// sixteen more cycles, eight additional warmup then eight measured; same budgets.
import {readFile,writeFile} from 'node:fs/promises';
import {execFileSync} from 'node:child_process';
import assert from 'node:assert/strict';
import {performance} from 'node:perf_hooks';
import {assessMemoryPlateau} from '../../scripts/lib/desktop-memory-checks.mjs';
const base='C:/workspace/archscope/.tmp/windows-final-20261114/import-saturated-amd64';
const previous=JSON.parse(await readFile(base+'/result.json')),d=JSON.parse(await readFile(base+'/.private-controller.json'));
assert.equal(previous.cycles.length,36);assert.equal(previous.buildCommit,'8985736f793fa02511f3d4d371d66fafc50733f7');
const exe='C:/workspace/archscope/.tmp/t613-memory-20261114/archscope-fault-fixed-amd64.exe';
const report={schemaVersion:1,scope:'Same owned amd64 app continuation; original36cycle FAIL unchanged',protocol:{priorCycles:36,additionalCycles:16,warmupCycles:8,openSizes:[4096,512,32],settleSeconds:3,idleSeconds:15},cycles:[],passed:false};
const delay=ms=>new Promise(r=>setTimeout(r,ms)),begin=performance.now(),ids=new Map();
let rootStart='';
async function sample(){const args=['-NoProfile','-File','C:/workspace/archscope/scripts/acceptance/layout/sample-owned-process-tree.ps1','-RootProcessId',String(d.pid),'-ExpectedExecutable',exe];if(rootStart)args.push('-ExpectedStartTime',rootStart);const raw=JSON.parse(execFileSync('powershell.exe',args,{windowsHide:true,encoding:'utf8'}));rootStart=raw.rootStartTime;assert(raw.responding&&raw.hasMainWindow);const members=raw.members.map(({identity,...v})=>{if(!ids.has(identity))ids.set(identity,ids.size+1);return {ordinal:ids.get(identity),...v};});const root=members.find(p=>p.role==='app');return {elapsedSeconds:(performance.now()-begin)/1000,members,treePrivateBytes:members.reduce((s,p)=>s+p.privateBytes,0),rootPrivateBytes:root.privateBytes,rootHandles:root.handles,rootThreads:root.threads};}
async function open(size){const r=await fetch(d.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${d.token}`},body:JSON.stringify({id:crypto.randomUUID(),command:'open',params:{path:base+`/synthetic-${size}.httpsession`}}),signal:AbortSignal.timeout(60000)});const value=await r.json();assert.equal(value.ok,true);assert.equal(value.result.transactions,size);assert.equal(value.result.selectedId,`import-memory-${size}-0`);}
try {await sample();for(let c=1;c<=16;c++){for(const size of [4096,512,32]){await open(size);await delay(3000);}await delay(15000);report.cycles.push({cycle:c,warmup:c<=8,settled:await sample()});await writeFile(base+'/continuation-progress.json',JSON.stringify(report,null,2));console.log(JSON.stringify({cycle:c,treeMiB:report.cycles.at(-1).settled.treePrivateBytes/1048576}));}
report.memoryAssessment=assessMemoryPlateau(report.cycles.slice(8).map(c=>c.settled));const sets=report.cycles.slice(8).map(c=>c.settled.members.map(p=>p.ordinal).sort().join(','));report.stableMeasuredProcessIdentities=sets.every(x=>x===sets[0]);report.passed=report.memoryAssessment.withinBudget&&report.stableMeasuredProcessIdentities;if(!report.passed)process.exitCode=1;
}catch(e){report.failure='Continuation failed; private detail';await writeFile(base+'/.private-continuation-error.txt',String(e));process.exitCode=1;}
finally{report.elapsedSeconds=(performance.now()-begin)/1000;await writeFile(base+'/continuation.json',JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,cycles:report.cycles.length}));}
