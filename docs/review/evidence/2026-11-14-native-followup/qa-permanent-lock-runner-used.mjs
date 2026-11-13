import { readFile, writeFile, stat } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import http from 'node:http';
import path from 'node:path';
const base='C:/workspace/archscope/.tmp/native-followup-20261114';
const descriptor=JSON.parse(await readFile(base+'/qa/.private-controller.json','utf8'));
let sequence=0;
const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function control(command,params={}) {
  const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:`lock-qa-${++sequence}`,command,params}),signal:AbortSignal.timeout(30000)});
  const value=await response.json();if(!response.ok||!value.ok)throw Error(command+' refused');return value.result;
}
const own=(await control('processes')).find(p=>p.key.pid===process.pid&&p.key.startTime);
if(!own)throw Error('Own producer identity unavailable');
await control('attach',{processes:[own.key]});
const session=await control('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:128,liveWindow:128});
await writeFile(base+'/qa/.private-lock-session.json',JSON.stringify(session));
async function request(index) {
  await new Promise((resolve,reject)=>{const req=http.get({host:'127.0.0.1',port:Number(session.listenAddress.split(':').at(-1)),path:`http://127.0.0.1:11507/soak?sequence=${index}`,headers:{Host:'127.0.0.1:11507'},agent:false,timeout:10000},response=>{const chunks=[];response.on('data',c=>chunks.push(c));response.on('error',reject);response.on('end',()=>{try{if(response.statusCode!==200||JSON.parse(Buffer.concat(chunks)).marker!=='desktop-soak-origin')throw Error('Origin mismatch');resolve();}catch(e){reject(e);}});});req.on('error',reject);req.on('timeout',()=>req.destroy(Error('timeout')));});
}
for(let i=0;i<4;i++)await request(i);
for(let i=0;i<50;i++){if((await control('stats')).persisted===4)break;await delay(100);}
const manifest=path.join(base,'qa','.archscope-captures',session.sessionId,'manifest.json');await stat(manifest);
const ready=base+'/qa/.private-lock-ready';
const helper=spawn('powershell.exe',['-NoProfile','-NonInteractive','-File',base+'/hold-manifest.ps1','-Manifest',manifest,'-Ready',ready],{stdio:'ignore',windowsHide:true});
for(let i=0;i<50;i++){try{await stat(ready);break;}catch{await delay(100);}}
await stat(ready);
await request(4);
let stats;
for(let i=0;i<80;i++){stats=await control('stats');if(stats.state==='recoverable')break;await delay(100);}
await new Promise(resolve=>helper.exitCode!==null?resolve():helper.once('exit',resolve));
const events=await control('events',{after:0,limit:100});
await writeFile(base+'/qa/.private-lock-events.json',JSON.stringify(events,null,2));
const errors=events.items.filter(x=>x.kind==='error');
if(stats.state!=='recoverable'||stats.captured!==5||stats.persisted!==4||!errors.some(x=>/manifest.json/.test(x.data.message)&&/Access is denied/.test(x.data.message)))throw Error('Expected permanent-lock failure was not observed');
const rows=await control('list',{sessionId:session.sessionId,limit:100});if(rows.total!==4)throw Error('Committed rows changed');
const report={scenario:'Controlled permanent Windows manifest read-sharing lock in isolated QA app; long-run apps untouched',lockHoldSeconds:3,passed:true,state:stats.state,captured:stats.captured,persisted:stats.persisted,retained:rows.total,captureErrorEvents:errors.length,expectedError:'Manifest atomic replacement denied after bounded retry; four previously committed rows preserved',nativeUI:'Record separately'};
await writeFile(base+'/qa-permanent-lock.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report));
