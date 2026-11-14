// Native Windows crash/restart with an uncommitted rolling staging file and orphan blob.
// Public results omit credentials/session IDs. UI observation is acknowledged separately.
// The origin must be an isolated loopback fixture. No system policy is changed.
import { spawn, execFileSync } from 'node:child_process';
import { mkdir, readFile, writeFile, rm, readdir, realpath, stat } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import assert from 'node:assert/strict';

const options = {};
for (let i = 2; i < process.argv.length; i += 2) {
  const key = process.argv[i];
  if (!['--exe', '--out', '--origin'].includes(key) || !process.argv[i + 1]) throw Error('invalid soak option');
  options[key.slice(2)] = process.argv[i + 1];
}
if (process.platform !== 'win32') throw Error('native Windows required');
const origin = new URL(options.origin);
if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash) throw Error('use a loopback-only synthetic HTTP origin');
const exe = path.resolve(options.exe), out = path.resolve(options.out);
await mkdir(out, { recursive: false });
const descriptorPath = path.join(out, '.private-controller.json');
const binarySha256 = createHash('sha256').update(await readFile(exe)).digest('hex');
const report = { schema: 1, platform: 'windows', scope: 'production Wails desktop/controller, synthetic Docker-loopback HTTP, bodies disabled', scenario: 'forced owned-process termination, orphan rolling staging/blob cleanup, exact committed-row preservation', binarySha256, passed: false, requests: 0, pauses: 0, clears: 0, checkpoints: [], nativeUIObservations: 'record separately; controller/liveness is not a renderer correctness claim' };
let child, descriptor, session, sequence = 0;
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
let ownedStore, committedHashes, injected;
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function control(command, params = {}) {
  const response = await fetch(descriptor.url, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${descriptor.token}` }, body: JSON.stringify({ id: `soak-${++sequence}`, command, params }), signal: AbortSignal.timeout(30000) });
  const result = await response.json();
  if (response.status !== 200 || !result.ok) throw Error(`controller ${command} failed`);
  return result.result;
}
function request(proxy, index) {
  return new Promise((resolve, reject) => {
    const req = http.get({ host: '127.0.0.1', port: Number(proxy.split(':').at(-1)), path: `${origin}soak?sequence=${index}`, headers: { Host: origin.host }, agent: false, timeout: 10000 }, response => {
      const chunks = []; response.on('data', c => chunks.push(c)); response.on('error', reject);
      response.on('end', () => { try { if (response.statusCode !== 200 || JSON.parse(Buffer.concat(chunks).toString()).marker !== 'desktop-soak-origin') throw Error('origin response mismatch'); resolve(); } catch (e) { reject(e); } });
    });
    req.on('error', reject); req.on('timeout', () => req.destroy(Error('origin timeout')));
  });
}
async function checkpoint(elapsed) {
  if (child.exitCode !== null) throw Error('desktop process exited');
  const page = await control('list', { sessionId: session.sessionId, limit: 100 });
  if (page.total > 128 || page.items.some(x => new URL(x.transaction.url).origin !== origin.origin || x.transaction.process?.key.pid !== process.pid || x.transaction.process?.attribution !== 'confirmed')) throw Error('retention/privacy/process mismatch');
  const stats = await control('stats', { sessionId: session.sessionId });
  const script = `$p=Get-Process -Id ${child.pid} -ErrorAction Stop; $p | Select-Object WorkingSet64,PrivateMemorySize64,HandleCount,Responding,MainWindowHandle | ConvertTo-Json -Compress`;
  const memory = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-Command', script], { encoding: 'utf8', windowsHide: true }));
  if (!memory.Responding || !memory.MainWindowHandle || memory.PrivateMemorySize64 > 1073741824) throw Error('desktop liveness/private-memory guard failed');
  const { sessionId: _privateID, ...counters } = stats;
  const { MainWindowHandle: _handle, ...memoryCounters } = memory;
  report.checkpoints.push({ elapsedSeconds: elapsed, requests: report.requests, retainedRows: page.total, stats: counters, hasMainWindow: true, ...memoryCounters });
  report.elapsedSeconds = elapsed;
  await writeFile(path.join(out, 'progress.json'), JSON.stringify(report, null, 2));
}
async function launch(){
  descriptor=undefined;
  child=spawn(exe,['--automation-descriptor',descriptorPath],{stdio:'ignore',windowsHide:false});
  for(let i=0;i<120;i++){try{descriptor=JSON.parse(await readFile(descriptorPath,'utf8'));if(descriptor.pid===child.pid)break;}catch{}await delay(250);}
  if(descriptor?.pid!==child.pid)throw Error('desktop readiness failed');
  await delay(15000);
}
try{
  await launch();
  report.buildCommit=(await control('capabilities')).buildCommit;
  const own=(await control('processes')).find(p=>p.key.pid===process.pid&&p.key.startTime);
  if(!own)throw Error('generator identity missing');
  await control('attach',{processes:[own.key]});
  session=await control('start',{listenAddress:'127.0.0.1:0',captureBodies:false,retentionRows:128,liveWindow:128});
  for(let n=0;n<12;n++){await request(session.listenAddress,n);report.requests++;}
  await delay(1500);
  await checkpoint(1.5);
  await writeFile(path.join(out,'.private-original-session.json'),JSON.stringify(session));
  const before=await control('list',{sessionId:session.sessionId,limit:100});
  if(before.total!==12)throw Error('expected12 captured rows');
  ownedStore=await realpath(path.join(out,'.archscope-captures',session.sessionId));
  assert(ownedStore.startsWith((await realpath(out))+path.sep),'isolated store boundary');
  const rolling=path.join(ownedStore,'rolling');
  const committedNames=(await readdir(rolling)).filter(n=>/^\d{20}\.json$/.test(n)).sort();
  assert.equal(committedNames.length,12);
  committedHashes=Object.fromEntries(await Promise.all(committedNames.map(async n=>[n,sha(await readFile(path.join(rolling,n)))])));
  child.kill('SIGKILL');
  while(child.exitCode===null&&child.signalCode===null)await delay(100);
  // Inject only into the verified, stopped, isolated owned store. This does not
  // corrupt a committed record and is not a power-loss/truncated-record test.
  injected=[{file:path.join(ownedStore,'rolling','pending-acceptance-tail'),bytes:Buffer.from('{"transaction":"unfinished"')},
    {file:path.join(ownedStore,'blobs','acceptance-unreferenced-blob'),bytes:Buffer.from('synthetic-orphan-body-not-committed')}];
  for(const item of injected)await writeFile(item.file,item.bytes,{flag:'wx'});
  report.injectedOrphanBytes=injected.reduce((n,x)=>n+x.bytes.length,0);
  report.injectedOrphanArtifacts=injected.map(x=>({kind:x.file.includes('rolling')?'uncommitted rolling staging':'unreferenced synthetic blob',bytes:x.bytes.length,sha256:sha(x.bytes)}));
  await rm(descriptorPath);
  await launch();
  report.nativeRecoveryRequired=true;
  await writeFile(path.join(out,'ready.json'),JSON.stringify({requests:12,buildCommit:report.buildCommit,binarySha256,nativeRecoveryRequired:true},null,2));
  console.log('Restarted owned desktop after forced termination; inspect native recovery banner.');
  for(let i=0;i<600;i++){try{await readFile(path.join(out,'native-observed.flag'));break;}catch{}if(i===599)throw Error('native observation not acknowledged within600s');await delay(1000);}
  const recovered=await control('list',{sessionId:session.sessionId,limit:100});
  if(recovered.total!==12)throw Error('recovered row count mismatch');
  assert.deepEqual(recovered.items.map(x=>x.transaction.id),before.items.map(x=>x.transaction.id));
  for(const [name,digest] of Object.entries(committedHashes))assert.equal(sha(await readFile(path.join(ownedStore,'rolling',name))),digest);
  for(const item of injected){let absent=false;try{await stat(item.file);}catch(e){if(e.code==='ENOENT')absent=true;else throw e;}assert(absent,'orphan must be removed by product recovery');}
  report.committedRecordBytesUnchanged=true;report.recoveredIDsEqual=true;report.injectedOrphansRemoved=true;
  report.committedRecordHashSetSha256=sha(Buffer.from(JSON.stringify(committedHashes)));
  const destination=path.join(out,'recovered.httpsession');
  await control('save',{source:{sessionId:session.sessionId},destination,format:'session',scope:'all',ids:[],selectedId:recovered.items[0].transaction.id});
  const saved=JSON.parse(await readFile(destination,'utf8'));
  const opened=await control('open',{path:destination});
  if(saved.transactions.length!==12||opened.selectedId!==recovered.items[0].transaction.id||JSON.stringify(saved.transactions.map(x=>x.id))!==JSON.stringify(recovered.items.map(x=>x.transaction.id)))throw Error('recovered save/open mismatch');
  report.recoveredRows=12;report.savedSessionSha256=createHash('sha256').update(await readFile(destination)).digest('hex');report.passed=true;
}catch(error){report.failure='Recovery harness failed; details remain in private output';await writeFile(path.join(out,'.private-failure.txt'),String(error));console.log(report.failure);process.exitCode=1;}
finally{await writeFile(path.join(out,'result.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,recoveredRows:report.recoveredRows}));}
// Keep the referenced owned app alive for the native final observation.
