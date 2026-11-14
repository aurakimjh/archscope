// Explicitly owned Windows desktop; file open/import memory screening, not leak proof.
import { spawn, execFileSync } from 'node:child_process';
import { mkdir, readFile, writeFile, appendFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { assessMemoryPlateau, memoryPlateauPolicy } from './lib/desktop-memory-checks.mjs';

const args = {};
for (let i=2;i<process.argv.length;i+=2) {
  assert(['--exe','--out','--fixture'].includes(process.argv[i]) && process.argv[i+1], 'invalid option');
  args[process.argv[i].slice(2)] = process.argv[i+1];
}
assert.equal(process.platform,'win32');
const exe=path.resolve(args.exe), out=path.resolve(args.out), fixture=path.resolve(args.fixture);
await mkdir(out,{recursive:false});
const sha=raw=>createHash('sha256').update(raw).digest('hex');
const source=await readFile(fixture), seed=JSON.parse(source).transactions[0];
assert(seed && seed.request.bodyStorage==='omitted' && seed.response.bodyStorage==='omitted');
const files=[];
for (const size of [4096,512,32]) {
  const transactions=Array.from({length:size},(_,i)=>({
    ...structuredClone(seed),id:`import-memory-${size}-${i}`,connectionId:`synthetic-${i}`,sequence:i+1,
    url:`http://127.0.0.1:11514/import?size=${size}&row=${i}`,host:'127.0.0.1:11514',path:'/import',query:`size=${size}&row=${i}`,
    starred:i===0,comment:i===0?'메모리 재열기 synthetic note':'',
    process:{key:{pid:4242,startTime:'2026-11-14T00:00:00Z'},name:'synthetic-fixture',attribution:'confirmed'},
  }));
  const doc={type:'archscope-http-session',schemaVersion:1,scope:'all',selectedId:transactions[0].id,transactions};
  const file=path.join(out,`synthetic-${size}.httpsession`);
  const raw=JSON.stringify(doc);await writeFile(file,raw);files.push({path:file,rows:size,selectedId:doc.selectedId,sha256:sha(raw)});
}
const report={schemaVersion:1,scope:'Windows production file-open/import; synthetic body-omitted32/512/4096rows; no universal leak-free claim',
  binarySha256:sha(await readFile(exe)),sourceFixtureSha256:sha(source),passed:false,
  protocol:{cycles:10,warmupCycles:2,openSizes:[4096,512,32],settleAfterOpenSeconds:3,matchedSmallFileIdleSeconds:15},
  screeningPolicy:memoryPlateauPolicy,files:files.map(({path:_private,...publicFields})=>publicFields),cycles:[],samples:[]};
const begin=performance.now(), descriptorPath=path.join(out,'.private-controller.json'),identities=new Map();
let child,descriptor,rootStartTime='',command=0;
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function call(name,params={}) {
  const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},
    body:JSON.stringify({id:`import-${++command}`,command:name,params}),signal:AbortSignal.timeout(60000)});
  const result=await response.json();assert.equal(response.status,200);assert.equal(result.ok,true,`controller ${name} refused`);return result.result;
}
async function sample(label) {
  assert.equal(child.exitCode,null);assert.equal(child.signalCode,null);
  const params=['-NoProfile','-File',path.resolve('scripts/acceptance/layout/sample-owned-process-tree.ps1'),'-RootProcessId',String(child.pid),'-ExpectedExecutable',exe];
  if(rootStartTime)params.push('-ExpectedStartTime',rootStartTime);
  const raw=JSON.parse(execFileSync('powershell.exe',params,{encoding:'utf8',windowsHide:true,timeout:20000}));
  rootStartTime=raw.rootStartTime;assert(raw.responding && raw.hasMainWindow);
  const members=raw.members.map(({identity,...metrics})=>{if(!identities.has(identity))identities.set(identity,identities.size+1);return {ordinal:identities.get(identity),...metrics};});
  const roots=members.filter(p=>p.role==='app');assert.equal(roots.length,1);const root=roots[0];
  const point={label,elapsedSeconds:(performance.now()-begin)/1000,members,treePrivateBytes:members.reduce((sum,p)=>sum+p.privateBytes,0),
    rootPrivateBytes:root.privateBytes,rootHandles:root.handles,rootThreads:root.threads};
  report.samples.push(point);return point;
}
try {
  child=spawn(exe,['--automation-descriptor',descriptorPath],{stdio:['ignore','pipe','pipe'],windowsHide:false});
  let bytes=0;const log=chunk=>{const kept=chunk.subarray(0,Math.max(0,262144-bytes));bytes+=kept.length;if(kept.length)appendFile(path.join(out,'.private-desktop.log'),kept).catch(()=>{});};
  child.stdout.on('data',log);child.stderr.on('data',log);let launchError;child.on('error',e=>{launchError=e;});
  for(let i=0;i<120;i++){if(launchError)throw launchError;assert.equal(child.exitCode,null);try{descriptor=JSON.parse(await readFile(descriptorPath));if(descriptor.pid===child.pid)break;}catch{}await delay(250);}
  assert.equal(descriptor?.pid,child.pid);assert.equal(descriptor.schemaVersion,1);assert.match(descriptor.token,/^[a-f0-9]{64}$/);
  const endpoint=new URL(descriptor.url);assert.equal(endpoint.protocol,'http:');assert.equal(endpoint.hostname,'127.0.0.1');assert.equal(endpoint.pathname,'/v1/control');
  assert(!endpoint.username && !endpoint.password && !endpoint.search && !endpoint.hash);
  await delay(10000);report.buildCommit=(await call('capabilities')).buildCommit;
  for(let cycle=1;cycle<=10;cycle++) {
    const opens=[];
    for(const file of files) {
      const opened=await call('open',{path:file.path});assert.equal(opened.transactions,file.rows);assert.equal(opened.selectedId,file.selectedId);
      opens.push({rows:file.rows,selectedIdMatches:true});await delay(3000);
    }
    await delay(15000);const settled=await sample(`cycle-${cycle}-small-file-idle`);
    report.cycles.push({cycle,warmup:cycle<=2,opens,settled});
    await writeFile(path.join(out,'progress.json'),JSON.stringify(report,null,2));
    console.log(JSON.stringify({cycle,treePrivateMiB:settled.treePrivateBytes/1048576,rootHandles:settled.rootHandles}));
  }
  report.memoryAssessment=assessMemoryPlateau(report.cycles.slice(2).map(c=>c.settled));
  const stableSets=report.cycles.slice(2).map(c=>c.settled.members.map(p=>p.ordinal).sort((a,b)=>a-b).join(','));
  report.stableMeasuredProcessIdentities=stableSets.every(s=>s===stableSets[0]);
  for(const file of files)assert.equal(sha(await readFile(file.path)),file.sha256);
  report.sourceFilesUnchanged=true;report.functionalPassed=true;
  report.passed=report.memoryAssessment.withinBudget && report.stableMeasuredProcessIdentities;
  if(!report.passed)process.exitCode=1;
} catch(error) {
  report.failure='Import memory harness failed; details remain private';await writeFile(path.join(out,'.private-failure.txt'),String(error));process.exitCode=1;
} finally {
  report.elapsedSeconds=(performance.now()-begin)/1000;report.desktopAliveAtEnd=child?.exitCode===null && child?.signalCode===null;
  report.cleanupRequired='Stop only the descriptor-owned app after native UI observation and executable/start ownership verification.';
  await writeFile(path.join(out,'result.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,cycles:report.cycles.length,failure:report.failure}));
}
