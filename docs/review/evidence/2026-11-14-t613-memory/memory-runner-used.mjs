// Production native process, authenticated control and OS resource observations.
// No GC/heap manipulation, DOM injection, system policy or user-store changes.
import { spawn, execFileSync } from 'node:child_process';
import { mkdir,readFile,writeFile,appendFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import { performance } from 'node:perf_hooks';
import { waitForCapturePersistence,clearDesktopCapture,finalizeDesktopCapture } from './lib/desktop-soak-checks.mjs';
import { assessMemoryPlateau,memoryPlateauPolicy } from './lib/desktop-memory-checks.mjs';

const args={};
for(let i=2;i<process.argv.length;i+=2){if(!['--exe','--out','--origin'].includes(process.argv[i])||!process.argv[i+1])throw Error('invalid memory run option');args[process.argv[i].slice(2)]=process.argv[i+1];}
if(process.platform!=='win32')throw Error('native Windows required');
const exe=path.resolve(args.exe),out=path.resolve(args.out),origin=new URL(args.origin);
if(origin.protocol!=='http:'||origin.hostname!=='127.0.0.1'||origin.pathname!=='/'||origin.search||origin.hash||origin.username||origin.password)throw Error('loopback-only synthetic origin required');
await mkdir(out,{recursive:false});
const descriptorPath=path.join(out,'.private-controller.json'),begin=performance.now();
const report={schemaVersion:1,scope:'Windows production explicit-proxy HTTP, bodies disabled; fixed repeated load and matched idle; not universal leak-free acceptance',
  binarySha256:createHash('sha256').update(await readFile(exe)).digest('hex'),passed:false,functionalPassed:false,
  protocol:{baselineIdleSeconds:120,warmupCycles:2,measuredCycles:8,requestsPerCycle:512,idleSeconds:180,finalizedIdleSeconds:300,sampleIntervalSeconds:15},
  screeningPolicy:memoryPlateauPolicy,requests:0,cycles:[],samples:[],nativeUIObservations:'record separately; no UI correctness inferred from counters'};
let child,descriptor,session,command=0,rootStartTime='',phase='launch',logBytes=0;
const identities=new Map(),delay=ms=>new Promise(r=>setTimeout(r,ms));
const controller=async(name,params={})=>{
  const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:`memory-${++command}`,command:name,params}),signal:AbortSignal.timeout(30000)});
  const result=await response.json();if(response.status!==200||!result.ok)throw Error(`controller ${name} failed`);return result.result;
};
const send=index=>new Promise((resolve,reject)=>{
  const req=http.get({host:'127.0.0.1',port:Number(session.listenAddress.split(':').at(-1)),path:`${origin}soak?sequence=${index}`,headers:{Host:origin.host},agent:false,timeout:10000},res=>{
    const chunks=[];res.on('data',c=>chunks.push(c));res.on('error',reject);res.on('end',()=>{try{if(res.statusCode!==200||JSON.parse(Buffer.concat(chunks)).marker!=='desktop-soak-origin')throw Error('origin mismatch');resolve();}catch(e){reject(e);}});
  });req.on('error',reject);req.on('timeout',()=>req.destroy(Error('origin timeout')));
});
async function traffic(count){const urls=[];for(let i=0;i<count;i++){urls.push(`${origin}soak?sequence=${report.requests}`);await send(report.requests);report.requests++;if(i%4===3)await delay(25);}return urls;}
async function publish(){report.elapsedSeconds=(performance.now()-begin)/1000;report.phase=phase;await writeFile(path.join(out,'progress.json'),JSON.stringify(report,null,2));}
async function sample(label){
  if(child.exitCode!==null||child.signalCode!==null)throw Error('owned desktop exited');
  const params=['-NoProfile','-File',path.resolve('scripts/acceptance/layout/sample-owned-process-tree.ps1'),'-RootProcessId',String(child.pid),'-ExpectedExecutable',exe];
  if(rootStartTime)params.push('-ExpectedStartTime',rootStartTime);
  const raw=JSON.parse(execFileSync('powershell.exe',params,{encoding:'utf8',windowsHide:true,timeout:20000}));
  rootStartTime=raw.rootStartTime;
  if(!raw.responding||!raw.hasMainWindow||raw.members.filter(p=>p.role==='app').length!==1)throw Error('owned process liveness mismatch');
  const members=raw.members.map(({identity,...values})=>{
    if(!identities.has(identity))identities.set(identity,identities.size+1);
    return {ordinal:identities.get(identity),...values};
  });
  const root=members.find(p=>p.role==='app');
  const point={label,elapsedSeconds:(performance.now()-begin)/1000,members,
    treePrivateBytes:members.reduce((s,p)=>s+p.privateBytes,0),rootPrivateBytes:root.privateBytes,rootHandles:root.handles,rootThreads:root.threads};
  report.samples.push(point);await publish();return point;
}
async function idle(seconds,label){phase=label;const until=performance.now()+seconds*1000;while(performance.now()<until){await sample(label);await delay(Math.min(15000,Math.max(0,until-performance.now())));}return sample(label+'-end');}
async function retained(){
  const rows=[];let cursor;
  do{const page=await controller('list',{sessionId:session.sessionId,limit:100,...(cursor?{cursor}:{})});rows.push(...page.items);cursor=page.nextCursor;}while(cursor&&rows.length<=128);
  if(rows.length!==128||rows.some(x=>new URL(x.transaction.url).origin!==origin.origin||x.transaction.process?.key.pid!==process.pid||x.transaction.process?.attribution!=='confirmed'||x.transaction.statusCode!==200||x.transaction.state!=='complete'))throw Error('retained identity/status bound mismatch');
  return rows;
}
try{
  child=spawn(exe,['--automation-descriptor',descriptorPath],{stdio:['ignore','pipe','pipe'],windowsHide:false});
  const log=chunk=>{const kept=chunk.subarray(0,Math.max(0,(256<<10)-logBytes));logBytes+=kept.length;if(kept.length)appendFile(path.join(out,'.private-desktop.log'),kept).catch(()=>{});};
  child.stdout.on('data',log);child.stderr.on('data',log);let launchError;child.on('error',e=>{launchError=e;});
  for(let i=0;i<120;i++){if(launchError)throw launchError;if(child.exitCode!==null)throw Error('desktop exited before readiness');try{descriptor=JSON.parse(await readFile(descriptorPath));break;}catch{}await delay(250);}
  if(descriptor?.pid!==child.pid||descriptor?.schemaVersion!==1||!/^[a-f0-9]{64}$/.test(descriptor.token))throw Error('invalid owned descriptor');
  const endpoint=new URL(descriptor.url);if(endpoint.protocol!=='http:'||endpoint.hostname!=='127.0.0.1'||endpoint.pathname!=='/v1/control'||endpoint.username||endpoint.password||endpoint.search||endpoint.hash)throw Error('invalid controller endpoint');
  await delay(15000);report.buildCommit=(await controller('capabilities')).buildCommit;
  const own=(await controller('processes')).find(p=>p.key.pid===process.pid&&p.key.startTime);if(!own)throw Error('producer identity unavailable');
  await controller('attach',{processes:[own.key]});
  session=await controller('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:128,liveWindow:128});
  await writeFile(path.join(out,'.private-session.json'),JSON.stringify(session));
  await idle(120,'baseline-running-idle');
  for(let cycle=1;cycle<=10;cycle++){
    phase=`cycle-${cycle}-load`;
    const checked=await clearDesktopCapture(controller,session.sessionId,()=>traffic(4));
    await traffic(508);
    const stats=await waitForCapturePersistence(()=>controller('stats',{sessionId:session.sessionId}));
    if(stats.generation!==cycle||stats.captured!==512||stats.persisted!==512)throw Error('cycle capture/persistence mismatch');
    await retained();const loaded=await sample(phase);
    const settled=await idle(180,`cycle-${cycle}-running-idle`);
    const {sessionId:_privateID,...counters}=stats;
    report.cycles.push({cycle,warmup:cycle<=2,requests:512,clear:checked,stats:counters,loaded,settled});
    await publish();console.log(JSON.stringify({cycle,warmup:cycle<=2,requests:report.requests,treePrivateMiB:settled.treePrivateBytes/1048576,rootHandles:settled.rootHandles}));
  }
  report.memoryAssessment=assessMemoryPlateau(report.cycles.filter(c=>!c.warmup).map(c=>c.settled));
  phase='finalize';Object.assign(report,await finalizeDesktopCapture(controller,session.sessionId,path.join(out,'final.httpsession')));
  report.functionalPassed=true;await idle(300,'finalized-open-file-idle');
  report.passed=report.memoryAssessment.withinBudget;
}catch(error){report.failure='Memory harness failed; details remain private';report.failurePhase=phase;await writeFile(path.join(out,'.private-failure.txt'),String(error));process.exitCode=1;}
finally{
  if(session&&descriptor&&child?.exitCode===null){try{await controller('stop',{sessionId:session.sessionId});}catch{}}
  report.elapsedSeconds=(performance.now()-begin)/1000;
  report.desktopAliveAtEnd=child?.exitCode===null&&child?.signalCode===null;
  report.cleanupRequired='After native observation stop only private-descriptor PID after exact executable ownership check.';
  await writeFile(path.join(out,'result.json'),JSON.stringify(report,null,2));
  console.log(JSON.stringify({passed:report.passed,functionalPassed:report.functionalPassed,phase,cycles:report.cycles.length,assessment:report.memoryAssessment?.classification,failure:report.failure}));
}
// Keep the owned app referenced for final native inspection, as with the soak runner.
