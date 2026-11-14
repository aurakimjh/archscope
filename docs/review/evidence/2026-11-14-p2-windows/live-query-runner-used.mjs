import {spawn} from 'node:child_process';import http from 'node:http';
import {mkdir,readFile,writeFile,readdir} from 'node:fs/promises';import {createHash} from 'node:crypto';import assert from 'node:assert/strict';
const base='C:/workspace/archscope/.tmp/p2-windows-20261114',arch=process.argv[2],out=base+'/query-'+arch,exe=base+'/archscope-p2-'+arch+'.exe';
await mkdir(out);const child=spawn(exe,['--automation-descriptor',out+'/.private-controller.json'],{stdio:'ignore',windowsHide:false});
const delay=ms=>new Promise(r=>setTimeout(r,ms));let descriptor,serial=0;
for(let n=0;n<120;n++){try{descriptor=JSON.parse(await readFile(out+'/.private-controller.json','utf8'));if(descriptor.pid===child.pid)break;}catch{}await delay(250);}assert.equal(descriptor?.pid,child.pid);
async function call(command,params={}){const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+descriptor.token},body:JSON.stringify({id:'p2-query-'+(++serial),command,params}),signal:AbortSignal.timeout(30000)});const value=await response.json();if(!value.ok)throw Error(command+' failed');return value.result;}
try {
 await delay(12000);const own=(await call('processes')).find(p=>p.key.pid===process.pid&&p.key.startTime);assert(own);await call('attach',{processes:[own.key]});
 const session=await call('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:128,liveWindow:128});await writeFile(out+'/.private-session.json',JSON.stringify(session));
 const urls=['/lookup?next=/detail?id=3&keep=1','/lookup?next=/detail?id=7&keep=2','/repeat?value=??tail?&raw=%3F%26&raw=%3f','/plain?x=1'].map(p=>'http://127.0.0.1:11510'+p);
 for(const url of urls)await new Promise((resolve,reject)=>{const req=http.get({host:'127.0.0.1',port:Number(session.listenAddress.split(':').at(-1)),path:url,headers:{Host:'127.0.0.1:11510'},agent:false},res=>{const chunks=[];res.on('data',c=>chunks.push(c));res.on('error',reject);res.on('end',()=>{try{assert.equal(res.statusCode,200);assert.equal(JSON.parse(Buffer.concat(chunks)).marker,'desktop-soak-origin');resolve();}catch(e){reject(e)}})});req.on('error',reject)});
 await delay(500);const page=await call('list',{limit:100});assert.equal(page.total,4);const rows=page.items.map(x=>x.transaction);assert(rows.every((tx,i)=>tx.url===urls[i]&&!tx.query&&tx.process?.key.pid===process.pid&&tx.process.attribution==='confirmed'));
 await writeFile(out+'/.private-rows.json',JSON.stringify(rows));const {sessionId,...stats}=await call('stats');assert.equal(stats.captured,4);assert.equal(stats.persisted,4);assert.equal(stats.dropped,0);
 await writeFile(out+'/seed.json',JSON.stringify({schemaVersion:1,arch,buildCommit:(await call('capabilities')).buildCommit,binarySha256:createHash('sha256').update(await readFile(exe)).digest('hex'),captureBodies:false,requests:4,confirmedOwnProducer:true,separateQueryEmpty:true,retainedURLs:urls,stats,passed:true},null,2));console.log(JSON.stringify({arch,seeded:4,separateQueryEmpty:true}));
 while(child.exitCode===null&&child.signalCode===null){for(const name of (await readdir(out)).filter(n=>/^command-.*\.json$/.test(n))){const done=out+'/'+name+'.done';try{await readFile(done);continue;}catch{}const command=JSON.parse(await readFile(out+'/'+name,'utf8'));let result;
 if(command.command==='export'){result={rows:await call('save',{source:{sessionId:session.sessionId},destination:out+'/final.httpsession',format:'session',scope:'all',selectedId:rows[1].id})};}
 else if(command.command==='stop'){const stopped=await call('stop',{sessionId:session.sessionId});const {sessionId,...safe}=await call('stats',{sessionId:session.sessionId});result={state:stopped.state,stats:safe};assert.equal(stopped.state,'finalized');assert.equal(safe.persisted,safe.captured);}
 else throw Error('Unknown test command');await writeFile(done,JSON.stringify(result,null,2));}await delay(250);}
} finally {if(child.exitCode===null&&child.signalCode===null)child.kill();}
