import {spawn} from 'node:child_process';
import {mkdir,readFile,writeFile,readdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';import assert from 'node:assert/strict';
const base='C:/workspace/archscope/.tmp/grid-boundaries-20261114',arch=process.argv[2],stage=process.argv[3],out=base+'/'+arch+'-'+stage;
await mkdir(out);const binary=stage.startsWith('baseline')?base+'/baseline-amd64.exe':base+'/fixed-'+arch+'.exe';
const desktop=spawn(binary,['--automation-descriptor',out+'/.private-controller.json'],{stdio:'ignore',windowsHide:false});
const delay=ms=>new Promise(r=>setTimeout(r,ms));let descriptor,serial=0,workers=[];
for(let n=0;n<120;n++){try{descriptor=JSON.parse(await readFile(out+'/.private-controller.json','utf8'));if(descriptor.pid===desktop.pid)break;}catch{}await delay(250);}assert.equal(descriptor?.pid,desktop.pid);
async function call(command,params={}){const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+descriptor.token},body:JSON.stringify({id:'boundary-'+(++serial),command,params}),signal:AbortSignal.timeout(30000)});const value=await response.json();if(!value.ok)throw Error(command+' failed');return value.result;}
try{
await delay(12000);
workers=[0,1].map(()=>spawn(process.execPath,['C:/workspace/archscope/.tmp/live-grid-20261114/producer.mjs'],{stdio:['ignore','ignore','ignore','ipc'],windowsHide:true}));
await Promise.all(workers.map(w=>new Promise(r=>w.once('message',r))));
const processes=await call('processes'),targets=workers.map(w=>processes.find(p=>p.key.pid===w.pid&&p.key.startTime));assert(targets.every(Boolean));await call('attach',{processes:targets.map(p=>p.key)});
const session=await call('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:128,liveWindow:64});await writeFile(out+'/.private-session.json',JSON.stringify(session));
async function request(i){await new Promise((resolve,reject)=>{const worker=workers[i%2],listener=message=>{if(message.index!==i)return;worker.off('message',listener);message.ok?resolve():reject(Error('fixture request failed'));};worker.on('message',listener);worker.send({command:'request',index:i,proxyPort:Number(session.listenAddress.split(':').at(-1))});});}
for(let i=0;i<120;i++)await request(i);await delay(800);
const page=await call('list',{limit:100});let nextCursor=page.nextCursor;while(nextCursor){const more=await call('list',{limit:100,cursor:nextCursor});page.items.push(...more.items);nextCursor=more.nextCursor;}assert.equal(page.items.length,120);assert(page.items.every(x=>x.transaction.process.attribution==='confirmed'));
await writeFile(out+'/.private-rows.json',JSON.stringify(page.items));
const stats=await call('stats');assert.equal(stats.persisted,120);assert.equal(stats.dropped,0);
await writeFile(out+'/seed.json',JSON.stringify({arch,stage,buildCommit:(await call('capabilities')).buildCommit,binarySha256:createHash('sha256').update(await readFile(binary)).digest('hex'),rows:120,producers:2,confirmed:true,captureBodies:false,retentionRows:128,liveWindow:64,dropped:0},null,2));console.log(JSON.stringify({arch,stage,seeded:120}));
let next=120;
while(desktop.exitCode===null&&desktop.signalCode===null){for(const name of (await readdir(out)).filter(n=>/^command-.*\.json$/.test(n)).sort()){const done=out+'/'+name+'.done';try{await readFile(done);continue;}catch{}const command=JSON.parse(await readFile(out+'/'+name,'utf8'));try{let result;
if(command.command==='burst'){for(let n=0;n<command.count;n++)await request(next++);await delay(800);const {sessionId,...safe}=await call('stats');result=safe;}
else if(command.command==='stop'){result={state:(await call('stop')).state};}
else if(command.command==='export'){const id=page.items.find(x=>x.transaction.sequence===(command.selectedSequence??5))?.transaction.id;result={rows:await call('save',{source:{sessionId:session.sessionId},destination:out+'/'+command.filename,format:'session',scope:'all',selectedId:id})};}
else throw Error('Unknown owned harness command');await writeFile(done,JSON.stringify({ok:true,...result},null,2));
}catch(error){await writeFile(done,JSON.stringify({ok:false,error:String(error)}));}}
await delay(250);}
}finally{for(const worker of workers)worker.kill();if(desktop.exitCode===null&&desktop.signalCode===null)desktop.kill();}
