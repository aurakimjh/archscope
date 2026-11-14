// Owned production store/export failure probes; no user store or machine policy.
import {spawn,execFileSync} from 'node:child_process';
import {mkdir,readFile,writeFile,readdir,realpath,rename,stat} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import http from 'node:http';
import assert from 'node:assert/strict';
const options={};for(let i=2;i<process.argv.length;i+=2)options[process.argv[i].slice(2)]=process.argv[i+1];
const exe=await realpath(options.exe),out=path.resolve(options.out),origin=new URL(options.origin);
assert.equal(process.platform,'win32');assert.equal(origin.origin,'http://127.0.0.1:11512');assert.equal(origin.pathname,'/');
await mkdir(out,{recursive:false});const descriptorPath=path.join(out,'.private-controller.json');
const sha=b=>createHash('sha256').update(b).digest('hex'),delay=ms=>new Promise(r=>setTimeout(r,ms));
const report={schemaVersion:1,scope:'Native production Windows, synthetic HTTP bodies disabled; owned committed-row corruption and export refusal/retry',binarySha256:sha(await readFile(exe)),passed:false};
let child,descriptor,session,sequence=0;
async function call(command,params={},expectedFailure=false){
  const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:`fault-${++sequence}`,command,params}),signal:AbortSignal.timeout(30000)});
  const value=await response.json();
  if(expectedFailure){assert.equal(value.ok,false,'product must reject the injected failure');return;}
  assert.equal(value.ok,true,`controller ${command} must succeed`);return value.result;
}
async function launch(){
  descriptor=undefined;
  child=spawn(exe,['--automation-descriptor',descriptorPath],{stdio:'ignore',windowsHide:false});
  for(let i=0;i<120;i++){assert.equal(child.exitCode,null,'owned child exited before readiness');try{const next=JSON.parse(await readFile(descriptorPath));if(next.pid===child.pid){descriptor=next;break;}}catch{}await delay(250);}
  assert.equal(descriptor?.pid,child.pid);await delay(8000);
  report.buildCommit=(await call('capabilities')).buildCommit;
}
async function stopOwned(){
  execFileSync('powershell.exe',['-NoProfile','-File',path.resolve('scripts/acceptance/layout/sample-owned-process-tree.ps1'),'-RootProcessId',String(child.pid),'-ExpectedExecutable',exe],{windowsHide:true});
  const exited=new Promise(resolve=>child.once('exit',resolve));child.kill();await exited;
  // Forced termination leaves the O_EXCL controller descriptor behind. Preserve
  // only this stopped child/token's descriptor before relaunching the profile.
  const retired=JSON.parse(await readFile(descriptorPath));assert.equal(retired.pid,child.pid);assert.equal(retired.token,descriptor.token);
  await rename(descriptorPath,path.join(out,`.private-retired-controller-${child.pid}.json`));
}
async function attach(){const own=(await call('processes')).find(p=>p.key.pid===process.pid&&p.key.startTime);assert(own);await call('attach',{processes:[own.key]});}
async function start(){await attach();session=await call('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:128,liveWindow:128});}
async function traffic(count){
  for(let i=0;i<count;i++)await new Promise((resolve,reject)=>{
    const request=http.get({host:'127.0.0.1',port:Number(session.listenAddress.split(':').at(-1)),path:`${origin}soak?sequence=${i}`,headers:{Host:origin.host},agent:false},response=>{response.resume();response.on('end',()=>response.statusCode===200?resolve():reject(Error('origin mismatch')));});request.on('error',reject);
  });
}
async function stopped(count){assert.equal((await call('stop',{sessionId:session.sessionId})).state,'finalized');const stats=await call('stats',{sessionId:session.sessionId});assert.equal(stats.captured,count);assert.equal(stats.persisted,count);assert.equal(stats.dropped,0);}
async function acknowledge(){await writeFile(path.join(out,'ready-corrupt.json'),JSON.stringify({nativeRequired:true,phase:'corrupt-committed-record',buildCommit:report.buildCommit}));console.log('Ready: inspect corrupt-store native startup');for(let i=0;i<600;i++){try{await stat(path.join(out,'native-corrupt-observed.flag'));return;}catch{}await delay(1000);}throw Error('native observation not acknowledged');}
try{
  await launch();await start();await traffic(6);await stopped(6);
  const initial=session;await writeFile(path.join(out,'.private-session.json'),JSON.stringify(initial));
  const store=await realpath(initial.storePath),ownedRoot=await realpath(path.join(out,'.archscope-captures'));
  assert(store.startsWith(ownedRoot+path.sep));
  const rolling=path.join(store,'rolling'),names=(await readdir(rolling)).filter(n=>/^\d{20}\.json$/.test(n)).sort();assert.equal(names.length,6);
  const original=new Map(await Promise.all(names.map(async n=>[n,await readFile(path.join(rolling,n))])));
  const hashes=()=>Promise.all(names.map(async n=>sha(await readFile(path.join(rolling,n)))));
  const beforeHashes=await hashes();const page=await call('list',{sessionId:initial.sessionId,limit:100});
  const selectedId=page.items[0].transaction.id,destination=path.join(out,'retry.httpsession');
  await mkdir(destination);await writeFile(path.join(destination,'sentinel.txt'),'preserve existing directory');
  const exportParams={source:{sessionId:initial.sessionId},destination,format:'session',scope:'all',ids:[],selectedId};
  await call('save',exportParams,true);assert.deepEqual(await hashes(),beforeHashes);
  assert.equal(await readFile(path.join(destination,'sentinel.txt'),'utf8'),'preserve existing directory');
  const preserved=path.join(out,'preserved-existing-directory');assert(preserved.startsWith(out+path.sep)&&destination.startsWith(out+path.sep));await rename(destination,preserved);
  await call('save',exportParams);assert.equal((await call('open',{path:destination})).selectedId,selectedId);
  const exported=JSON.parse(await readFile(destination));assert.equal(exported.transactions.length,6);
  assert.deepEqual(exported.transactions.map(x=>x.id),page.items.map(x=>x.transaction.id));
  report.exportRefusalPreservedRecords=true;report.existingDestinationPreserved=true;report.explicitExportRetryAndSelection=true;
  report.exportSha256=sha(await readFile(destination));
  await stopOwned();
  const corrupt=path.join(rolling,names[0]);await writeFile(path.join(out,'.private-record-backup'),original.get(names[0]));
  const broken=Buffer.from('{"seq":');await writeFile(corrupt,broken);
  await launch();await call('list',{sessionId:initial.sessionId,limit:100},true);
  assert.equal(sha(await readFile(corrupt)),sha(broken));
  for(const n of names.slice(1))assert.equal(sha(await readFile(path.join(rolling,n))),sha(original.get(n)));
  report.committedCorruptionRejectedWithoutTruncation=true;report.otherCommittedBytesPreserved=true;
  await acknowledge();
  // A corrupt historical store must not corrupt a distinct new capture.
  await start();await traffic(4);await stopped(4);report.newCaptureIndependentOfCorruptStore=true;
  await stopOwned();await writeFile(corrupt,original.get(names[0]));
  assert.deepEqual(await hashes(),beforeHashes);report.exactBackupRestored=true;
  await launch();assert.equal((await call('list',{sessionId:initial.sessionId,limit:100})).total,6);await call('open',{path:destination});
  assert.deepEqual(await hashes(),beforeHashes);report.restoredStoreAndExportReadable=true;
  report.committedHashSetSha256=sha(Buffer.from(JSON.stringify(beforeHashes)));report.passed=true;
}catch(error){report.failure='Failure probe failed; details remain private';await writeFile(path.join(out,'.private-failure.txt'),String(error));process.exitCode=1;}
finally{report.desktopAliveAtEnd=child?.exitCode===null&&child?.signalCode===null;await writeFile(path.join(out,'result.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,failure:report.failure}));}
