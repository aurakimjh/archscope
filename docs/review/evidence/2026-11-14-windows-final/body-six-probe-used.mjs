import {readFile,writeFile,mkdir} from 'node:fs/promises';
import http from 'node:http';import path from 'node:path';import assert from 'node:assert/strict';
const [folder,output]=process.argv.slice(2),out=path.resolve(output);await mkdir(out,{recursive:false});
const d=JSON.parse(await readFile(folder+'/.private-controller.json'));let session;
async function call(command,params={},failure=false){const r=await fetch(d.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${d.token}`},body:JSON.stringify({id:crypto.randomUUID(),command,params}),signal:AbortSignal.timeout(60000)});const value=await r.json();assert.equal(value.ok,!failure,`controller ${command}`);return value;}
const report={schemaVersion:1,scope:'Owned synthetic HTTP explicit proxy; actual SEC10 gate; no trust/security UI changes',passed:false};
try {
  const cap=(await call('capabilities')).result;report.buildCommit=cap.buildCommit;report.contract=cap.contract;
  const producer=(await call('processes')).result.find(p=>p.key.pid===process.pid&&p.key.startTime);assert(producer);await call('attach',{processes:[producer.key]});
  if(!cap.contract.bodyCaptureAvailable){const refused=await call('start',{listenAddress:'127.0.0.1:0',captureBodies:true},true);report.gatedRefusal=true;report.refusal=refused.error;report.positiveBodyVerified=false;report.passed=true;}
  else {
    assert.equal(cap.contract.bodyCaptureGate,'sec10_verified');session=(await call('start',{listenAddress:'127.0.0.1:0',captureBodies:true,retentionRows:128,liveWindow:128})).result;
    await writeFile(out+'/.private-session.json',JSON.stringify(session));assert.equal(session.bodyCaptureGate,'sec10_verified');
    const requests=[{method:'POST',url:'/body',type:'application/json',body:'{"value":"retained-body-proof 한글","password":"synthetic-body-proof"}'},{method:'POST',url:'/echo',type:'application/x-www-form-urlencoded',body:'x=one&x=two&note=post-body-proof&password=synthetic-form-proof'},{method:'GET',url:'/large',body:''},{method:'GET',url:'/html',body:''}];
    requests.push({method:'GET',url:'/image',body:''},{method:'GET',url:'/gzip',body:''});
    for(const input of requests)await new Promise((resolve,reject)=>{const req=http.request({host:'127.0.0.1',port:Number(session.listenAddress.split(':').at(-1)),path:'http://127.0.0.1:11514'+input.url,method:input.method,headers:{Host:'127.0.0.1:11514','Content-Length':Buffer.byteLength(input.body),...(input.type?{'Content-Type':input.type}:{}),...(input.url==='/gzip'?{'Accept-Encoding':'gzip'}:{}),...(input.url==='/body'?{Cookie:'same=first; same=second'}:{})},agent:false,timeout:10000},res=>{res.resume();res.on('end',()=>{assert.equal(res.statusCode,200);resolve();});});req.on('error',reject);req.on('timeout',()=>req.destroy(Error('timeout')));req.end(input.body);});
    assert.equal((await call('stop',{sessionId:session.sessionId})).result.state,'finalized');
    const stats=(await call('stats',{sessionId:session.sessionId})).result;assert.equal(stats.captured,6);assert.equal(stats.persisted,6);assert.equal(stats.dropped,0);
    const page=(await call('list',{sessionId:session.sessionId,limit:100})).result;assert.equal(page.items.length,6);
    const rows=[];for(const item of page.items){const detail=(await call('detail',{sessionId:session.sessionId,id:item.transaction.id})).result;rows.push(detail.transaction??detail);}
    await writeFile(out+'/.private-rows.json',JSON.stringify(rows));
    // Durable completion order can differ from request sequence for a large body.
    const at=route=>rows.find(tx=>new URL(tx.url).pathname===route);
    const json=at('/body'),form=at('/echo'),large=at('/large'),html=at('/html');
    assert(json.request.bodyPreview.includes('retained-body-proof'));assert(!json.request.bodyPreview.includes('synthetic-body-proof'));assert(json.request.bodyPreview.includes('[REDACTED]'));
    assert(form.request.bodyPreview.includes('post-body-proof'));assert(!form.request.bodyPreview.includes('synthetic-form-proof'));
    assert.equal(json.request.cookies.filter(c=>c.name==='same').length,2);assert.equal(json.response.cookies.filter(c=>c.name==='same').length,2);
    assert(json.request.cookies.every(c=>c.redacted)&&json.response.cookies.every(c=>c.redacted));
    assert.equal(json.response.headers.filter(h=>h.name.toLowerCase()==='set-cookie').length,2);
    assert.equal(large.response.bodyStorage,'truncated');assert.equal(Buffer.byteLength(large.response.bodyPreview),cap.contract.maxBodyBytes);assert(large.response.bodySize>cap.contract.maxBodyBytes);
    assert(html.response.bodyPreview.includes('Inert fixture'));
    const raster=at('/image').response.imagePreview;assert.equal(raster.width,1);assert.equal(raster.height,1);assert.equal(raster.mime,'image/png');assert(raster.base64.length>0);
    assert.equal(at('/gzip').response.bodyPreview,'{"marker":"compressed-body-proof"}');
    const searches=[];for(const [field,query,count] of [['post','post-body-proof',1],['body','retained-body-proof',1],['body','Inert fixture',1],['cookies','same',1]]){const found=(await call('list',{sessionId:session.sessionId,limit:100,filter:{query,searchFields:[field]}})).result;assert.equal(found.total,count);searches.push({field,query,total:found.total});}
    const saved=path.join(out,'body.httpsession');await call('save',{source:{sessionId:session.sessionId},destination:saved,format:'session',scope:'all',ids:[],selectedId:html.id});
    const exported=JSON.parse(await readFile(saved));assert.equal(exported.transactions.length,6);assert.equal(exported.selectedId,html.id);
    assert(!JSON.stringify(exported).includes('synthetic-body-proof'));assert(!JSON.stringify(exported).includes('synthetic-form-proof'));
    await call('open',{path:saved});report.positiveBodyVerified=true;report.requests=6;report.zeroDrops=true;report.requestResponseBodiesRetainedAndSecretsMasked=true;report.duplicateRequestResponseCookieCounts=[2,2];report.duplicateSetCookieHeaders=2;report.largeBody={storage:large.response.bodyStorage,retainedBytes:Buffer.byteLength(large.response.bodyPreview),transferredBytes:large.response.bodySize};report.searches=searches;report.savedSelectedHTML=true;report.raster={width:raster.width,height:raster.height,mime:raster.mime};report.gzipDecodedExactly=true;report.passed=true;
  }
}catch(e){report.failure='Body probe failed; details private';await writeFile(out+'/.private-failure.txt',String(e));process.exitCode=1;}
finally{if(session)try{await call('stop',{sessionId:session.sessionId});}catch{}await writeFile(out+'/result.json',JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,positiveBodyVerified:report.positiveBodyVerified,gatedRefusal:report.gatedRefusal,failure:report.failure}));}
