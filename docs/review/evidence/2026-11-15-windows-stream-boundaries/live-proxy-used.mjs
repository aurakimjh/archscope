import {readFile,writeFile,mkdir} from 'node:fs/promises';
import http from 'node:http';
import {createHash} from 'node:crypto';
import path from 'node:path';
import assert from 'node:assert/strict';
const [folder,output]=process.argv.slice(2),out=path.resolve(output);
await mkdir(out,{recursive:false});
const descriptor=JSON.parse(await readFile(folder+'/.private-controller.json'));
let session;
async function call(command,params={}) {
 const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:crypto.randomUUID(),command,params}),signal:AbortSignal.timeout(60000)});
 const result=await response.json();assert.equal(result.ok,true,command);return result.result;
}
const report={schemaVersion:1,scope:'Actual Windows explicit proxy to owned Docker; current SEC10 gate, no dump-policy or trust changes',passed:false};
try {
 const capabilities=await call('capabilities');
 report.build=capabilities.buildCommit;
 report.gate={available:capabilities.contract.bodyCaptureAvailable,state:capabilities.contract.bodyCaptureGate};
 if(!report.gate.available){report.gated=true;report.positiveBodyVerified=false;}
 else {
  assert.equal(report.gate.state,'sec10_verified');
  const producer=(await call('processes')).find(p=>p.key.pid===process.pid&&p.key.startTime);assert(producer);
  await call('attach',{processes:[producer.key]});
  session=await call('start',{listenAddress:'127.0.0.1:0',captureBodies:true,maxBodyBytes:4096,retentionRows:128,liveWindow:128});
  assert.equal(session.bodyCaptureGate,'sec10_verified');assert.equal(session.maxBodyBytes,4096);
  await writeFile(out+'/.private-session.json',JSON.stringify(session));
  report.clientWire={};
  for(const endpoint of ['identity','x-gzip','stacked','binary','binary-gzip']) {
   await new Promise((resolve,reject)=>{
    const request=http.request({host:'127.0.0.1',port:Number(session.listenAddress.split(':').at(-1)),path:'http://127.0.0.1:11516/'+endpoint,headers:{Host:'127.0.0.1:11516','Accept-Encoding':'gzip'},agent:false,timeout:10000},response=>{assert.equal(response.statusCode,200);const hash=createHash('sha256');let bytes=0;response.on('data',chunk=>{bytes+=chunk.length;hash.update(chunk)});response.on('end',()=>{report.clientWire[endpoint]={bytes,sha256:hash.digest('hex'),encoding:response.headers['content-encoding']??''};resolve()});response.on('error',reject)});
    request.on('error',reject);request.on('timeout',()=>request.destroy(Error('timeout')));request.end();
   });
  }
  assert.equal((await call('stop',{sessionId:session.sessionId})).state,'finalized');
  const stats=await call('stats',{sessionId:session.sessionId});
  assert.equal(stats.captured,5);assert.equal(stats.persisted,5);assert.equal(stats.dropped,0);
  const page=await call('list',{sessionId:session.sessionId,limit:100});assert.equal(page.items.length,5);
  const rows=[];
  for(const item of page.items){const detail=await call('detail',{sessionId:session.sessionId,id:item.transaction.id});rows.push(detail.transaction??detail)}
  await writeFile(out+'/.private-rows.json',JSON.stringify(rows));
  report.responses=rows.map(tx=>({path:tx.path,state:tx.state,bodySize:tx.response.bodySize,bodyDecoded:tx.response.bodyDecoded,bodyStorage:tx.response.bodyStorage,bodyEncoding:tx.response.bodyEncoding??'',previewBytes:Buffer.byteLength(tx.response.bodyPreview??''),codingHeaders:tx.response.headers.filter(h=>h.name.toLowerCase()==='content-encoding').map(h=>h.value)}));
  report.summary=await call('summary',{sessionId:session.sessionId});
  const at=route=>rows.find(tx=>new URL(tx.url).pathname===route);
  assert.equal(at('/identity').response.bodyPreview,'{"marker":"identity-proof"}');
  assert.equal(at('/x-gzip').response.bodyPreview,'{"marker":"identity-proof"}');
  for(const route of ['/binary','/binary-gzip']) {
   assert.equal(at(route).response.bodyStorage,'truncated');assert.equal(at(route).response.bodyDecoded,4096);assert(!at(route).response.bodyPreview);assert(!at(route).response.imagePreview);
  }
  assert.equal(at('/stacked').response.bodyStorage,'encoded');assert.equal(at('/stacked').response.bodyDecoded,-1);assert.equal(at('/stacked').response.bodyEncoding,'gzip, br');
  assert.equal(report.summary.compressionKnown,2);assert.equal(report.summary.compressionSaved,-20);
  const saved=path.join(out,'live-binary.httpsession');
  await call('save',{source:{sessionId:session.sessionId},destination:saved,format:'session',scope:'all',ids:[],selectedId:at('/binary-gzip').id});
  const file=JSON.parse(await readFile(saved));assert.equal(file.transactions.length,5);assert.equal(file.selectedId,at('/binary-gzip').id);
  await call('open',{path:saved});
  report.positiveBodyVerified=true;report.zeroDrops=true;report.savedReopened=true;report.passed=true;
 }
} catch(error) {report.failure='Actual live proxy assertions failed; details remain private';await writeFile(out+'/.private-failure.txt',String(error));process.exitCode=1}
finally {
 if(session)try{await call('stop',{sessionId:session.sessionId})}catch{}
 await writeFile(out+'/result.json',JSON.stringify(report,null,2)+'\n');
 console.log(JSON.stringify({passed:report.passed,gated:report.gated,positiveBodyVerified:report.positiveBodyVerified,failure:report.failure}));
}
