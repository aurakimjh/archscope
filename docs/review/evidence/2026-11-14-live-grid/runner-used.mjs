import {spawn} from 'node:child_process';
import {mkdir,readFile,writeFile,readdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';import path from 'node:path';import assert from 'node:assert/strict';
const base='C:/workspace/archscope/.tmp/live-grid-20261114',arch=process.argv[2]??'amd64',suffix=process.argv[3]??'baseline';
const out=base+'/'+arch+'-'+suffix;await mkdir(out,{recursive:false});const exe=base+'/archscope-live-'+arch+'.exe';
const child=spawn(exe,['--automation-descriptor',out+'/.private-controller.json'],{stdio:'ignore',windowsHide:false});
const delay=ms=>new Promise(r=>setTimeout(r,ms));let descriptor,sequence=0;
for(let n=0;n<120;n++){try{descriptor=JSON.parse(await readFile(out+'/.private-controller.json','utf8'));if(descriptor.pid===child.pid)break;}catch{}await delay(250);}assert.equal(descriptor?.pid,child.pid);
async function control(command,params={}){const r=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:`live-grid-${++sequence}`,command,params}),signal:AbortSignal.timeout(30000)});const v=await r.json();if(!r.ok||!v.ok)throw Error(command+' failed');return v.result;}
await delay(15000);
const workers=[0,1].map(()=>spawn(process.execPath,[base+'/producer.mjs'],{stdio:['ignore','ignore','ignore','ipc'],windowsHide:true}));
await Promise.all(workers.map(w=>new Promise(resolve=>w.once('message',resolve))));
const processes=await control('processes');const targets=workers.map(w=>processes.find(p=>p.key.pid===w.pid&&p.key.startTime));assert(targets.every(Boolean));await control('attach',{processes:targets.map(p=>p.key)});
const session=await control('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:640,liveWindow:64});await writeFile(out+'/.private-session.json',JSON.stringify(session));
const proxyPort=Number(session.listenAddress.split(':').at(-1));
async function request(i){const w=workers[i%2];await new Promise((resolve,reject)=>{const listener=msg=>{if(msg.index!==i)return;w.off('message',listener);msg.ok?resolve():reject(Error('fixture request failed'));};w.on('message',listener);w.send({command:'request',index:i,proxyPort});});}
for(let i=0;i<600;i++){await request(i);if(i===0)await delay(1200);}
for(let n=0;n<100;n++){if((await control('stats')).persisted===600)break;await delay(100);}
let page=await control('list',{limit:100}),rows=[...page.items];while(page.nextCursor){page=await control('list',{limit:100,cursor:page.nextCursor});rows.push(...page.items);}assert.equal(rows.length,600);assert(rows.every(x=>x.transaction.process?.attribution==='confirmed'&&targets.some(p=>p.key.pid===x.transaction.process.key.pid&&p.key.startTime===x.transaction.process.key.startTime)));
const stats=await control('stats');assert.equal(stats.captured,600);assert.equal(stats.persisted,600);assert.equal(stats.dropped,0);
const report={schemaVersion:1,scope:'Real production desktop/controller,2own Node producers,600 synthetic Docker loopback requests,bodiesdisabled,640retention/64backendLiveWindow;nativeUI separate',buildCommit:(await control('capabilities')).buildCommit,binarySha256:createHash('sha256').update(await readFile(exe)).digest('hex'),requests:600,confirmedProducerCounts:targets.map(p=>rows.filter(x=>x.transaction.process.key.pid===p.key.pid).length),archiveURLMatches:60,headerMatches:40,requestHeaderMatches:20,queryMatches:10,hostCounts:{'127.0.0.1':300,localhost:300},methodCounts:{GET:300,POST:300},statusCounts:{200:200,404:200,503:200},mimeCounts:{'application/json':200,'text/plain':200,'text/html':200},stats:(({sessionId,...rest})=>rest)(stats),passed:true};
await writeFile(out+'/seed-report.json',JSON.stringify(report,null,2));await writeFile(out+'/.private-rows.json',JSON.stringify(rows));console.log(JSON.stringify({arch,seeded:600,persisted:600,dropped:0}));
// Explicit test commands are files in this newly owned private directory. They
// produce traffic/control only; no renderer navigation or UI verdict is injected.
let next=600;
for(;;){const names=(await readdir(out)).filter(n=>n.startsWith('command-')&&n.endsWith('.json')).sort();for(const name of names){const done=out+'/'+name+'.done';try{await readFile(done);continue;}catch{}const command=JSON.parse(await readFile(out+'/'+name,'utf8'));let result;
if(command.command==='burst'){for(let n=0;n<command.count;n++)await request(next++);await delay(800);result=await control('stats');result=(({sessionId,...rest})=>rest)(result);}
else if(command.command==='save'){result=await control('save',command.params);}
else if(command.command==='stop'){result=await control('stop');result={state:result.state};}
else if(command.command==='open'){result=await control('open',command.params);result={transactions:result.transactions};}
else if(command.command==='audit'){const values=await control('list',{filter:command.filter,limit:100});result={total:values.total,sequences:values.items.map(x=>x.transaction.sequence)};}
else throw Error('unknown private test command');await writeFile(done,JSON.stringify(result,null,2));}
if(child.exitCode!==null||child.signalCode!==null)break;await delay(250);}
for(const w of workers)w.send({command:'exit'});
