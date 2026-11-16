import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtemp,writeFile,readFile,rm} from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import vm from 'node:vm';
import {EventEmitter} from 'node:events';
import {createHash} from 'node:crypto';
import {waitForNativeReceipt} from './lib/native-receipt.mjs';
const valid=JSON.stringify({report:{passed:true},webviewZoom:1});
const missing=()=>{throw Object.assign(new Error('missing'),{code:'ENOENT'});};
function clock(){let time=0;return {isRunning:()=>true,now:()=>time,sleep:async ms=>{time+=ms;},timeoutMs:20,intervalMs:2};}
for(const [name,sequence]of [['complete',[valid]],['missing then complete',[missing,valid]],['empty then complete',['',valid]],['partial then complete',['{','{"report":',valid]]]){
 test(name,async()=>{let index=0;const result=await waitForNativeReceipt('owned',{...clock(),read:async()=>{const value=sequence[Math.min(index++,sequence.length-1)];return typeof value==='function'?value():value;}});assert.equal(result.report.passed,true);});
}
test('permanently invalid JSON times out without passing',async()=>{await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),read:async()=>'{'}),e=>e.code==='NATIVE_RECEIPT_TIMEOUT'&&e.lastRead==='partial-or-invalid');});
test('missing receipt times out without passing',async()=>{await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),read:missing}),e=>e.code==='NATIVE_RECEIPT_TIMEOUT');});
for(const [name,read]of [['missing',missing],['partial',async()=>'{']])test('exit with '+name+' fails promptly',async()=>{const c=clock();await assert.rejects(()=>waitForNativeReceipt('owned',{...c,isRunning:()=>false,read}),e=>e.code==='NATIVE_RECEIPT_EXITED');assert.equal(c.now(),0);});
test('valid final receipt is read even when producer already exited',async()=>{assert((await waitForNativeReceipt('owned',{...clock(),isRunning:()=>false,read:async()=>valid})).report.passed);});
test('explicit failed report stays failed and is not retried',async()=>{let reads=0;const result=await waitForNativeReceipt('owned',{...clock(),read:async()=>{reads++;return '{"report":{"passed":false,"failure":"synthetic"}}';}});assert.equal(result.report.passed,false);assert.equal(reads,1);assert.throws(()=>assert.equal(result.report.passed,true));});
for(const value of ['null','{}','{"report":{"passed":"true"}}'])test('well-formed wrong schema '+value,async()=>{await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),read:async()=>value}),e=>e.code==='NATIVE_RECEIPT_INVALID');});
test('permission errors are not hidden behind retry',async()=>{await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),read:async()=>{throw Object.assign(new Error('denied'),{code:'EACCES'});}}),e=>e.code==='EACCES');});
test('partial actual file can complete on the next bounded poll',async()=>{const root=await mkdtemp(path.join(os.tmpdir(),'archscope-receipt-'));try{const file=path.join(root,'native.json');await writeFile(file,'{');const c=clock();let completed=false;const value=await waitForNativeReceipt(file,{...c,sleep:async ms=>{if(!completed){await writeFile(file,valid);completed=true;}await c.sleep(ms);}});assert(value.report.passed);assert.equal(await readFile(file,'utf8'),valid);}finally{await rm(root,{recursive:true,force:true});}});

for(const code of ['EACCES','EPERM']) test('Windows permission error '+code+' fails immediately',async()=>{
 let reads=0;await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),platform:'win32',read:async()=>{reads++;throw Object.assign(new Error('denied'),{code});}}),e=>e.code===code);assert.equal(reads,1);
});
test('Windows transient sharing violation retries then accepts complete receipt',async()=>{
 let reads=0;const result=await waitForNativeReceipt('owned',{...clock(),platform:'win32',read:async()=>{if(++reads===1)throw Object.assign(new Error('locked'),{code:'EBUSY'});return valid;}});assert(result.report.passed);assert.equal(reads,2);
});
test('Windows permanently busy receipt times out rather than passing',async()=>{
 await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),platform:'win32',read:async()=>{throw Object.assign(new Error('locked'),{code:'EBUSY'});}}),e=>e.code==='NATIVE_RECEIPT_TIMEOUT'&&e.lastRead==='busy');
});
test('Windows exit with busy receipt fails promptly',async()=>{
 await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),platform:'win32',isRunning:()=>false,read:async()=>{throw Object.assign(new Error('locked'),{code:'EBUSY'});}}),e=>e.code==='NATIVE_RECEIPT_EXITED'&&e.lastRead==='busy');
});
test('non-Windows busy read is not classified as Windows publication contention',async()=>{
 await assert.rejects(()=>waitForNativeReceipt('owned',{...clock(),platform:'darwin',read:async()=>{throw Object.assign(new Error('busy'),{code:'EBUSY'});}}),e=>e.code==='EBUSY');
});

// Execute the current wrapper's real assertions with controlled process/I/O.
// This exercises Windows-specific orchestration without claiming native execution.
const runner = (await readFile(new URL('./verify-windows-ribbon.mjs', import.meta.url), 'utf8')).replace(/^import .*;\r?\n/gm, '');
async function runWrapper(kind) {
 let time=0, reads=0, polls=0;
 const writes=[];
 const pe=Buffer.alloc(128); pe.writeUInt32LE(64,0x3c); pe.writeUInt16LE(0x8664,68);
 function spawn() {
  const child=new EventEmitter(); child.stdout=new EventEmitter(); child.stderr=new EventEmitter();
  child.exitCode=kind==='exited'?0:null; child.signalCode=null; child.kill=()=>{child.exitCode=0;};
  return child;
 }
 async function read(file) {
  if(file.endsWith('.exe')) return pe;
  if(file.endsWith('verify-ribbon-native.js')) return Buffer.from('synthetic script');
  reads++;
  if(kind==='partial' && reads===1) return '{';
  if(kind==='missing' && reads===1) return missing();
  if(kind==='busy' && reads===1) throw Object.assign(new Error('locked'),{code:'EBUSY'});
  if(kind==='invalid' || kind==='exited') return '{';
  const zoom=file.includes(`${path.sep}wide${path.sep}`)?1:1.25;
  return JSON.stringify({webviewZoom:kind==='wrong-zoom'?2:zoom,osScreenScaleFactor:1,report:{passed:kind!=='failed',failure:'synthetic failure',locales:(kind==='wrong-locales'?['en']:['en','ko','ja','zh-CN','zh-TW']).map(locale=>({locale,selected:'true',menuRows:1,ribbonHeight:122})),themes:[{},{},{}],viewport:{pixelRatio:zoom}}});
 }
 const context={assert,spawn,mkdir:async()=>{},readFile:read,writeFile:async(file,body)=>writes.push(JSON.parse(body)),appendFile:async()=>{},createHash,path,performance:{now:()=>time},setTimeout:(callback,ms)=>{time+=ms;queueMicrotask(callback);},process:{platform:'win32',argv:['node','runner','/owned/app.exe','/owned/output'],env:{}},console:{log(){}},
  waitForNativeReceipt:(file,options)=>waitForNativeReceipt(file,{...options,platform:'win32',read,now:()=>time,timeoutMs:20,intervalMs:2,sleep:async ms=>{polls++;time+=ms;}})};
 let failure;
 try {await vm.runInThisContext('(async ({assert,spawn,mkdir,readFile,writeFile,appendFile,createHash,path,performance,setTimeout,process,console,waitForNativeReceipt})=>{'+runner+'\n})')(context);} catch(error) {failure=error;}
 return {reads,polls,passed:writes.at(-1)?.passed===true,failure};
}
for(const kind of ['complete','partial','missing','busy']) test('actual runner accepts '+kind+' only after complete reports',async()=>{
 const result=await runWrapper(kind); assert.equal(result.failure,undefined); assert.equal(result.passed,true); assert.equal(result.reads,kind==='complete'?3:4);
});
for(const kind of ['invalid','exited','failed','wrong-zoom','wrong-locales']) test('actual runner retains failure for '+kind,async()=>{
 const result=await runWrapper(kind); assert(result.failure); assert.equal(result.passed,false);
 if(kind==='invalid') {assert.equal(result.failure.code,'NATIVE_RECEIPT_TIMEOUT');assert.equal(result.polls,10);}
 if(kind==='exited') assert.equal(result.failure.code,'NATIVE_RECEIPT_EXITED');
 if(kind==='failed'||kind.startsWith('wrong-')) assert.equal(result.failure.code,'ERR_ASSERTION');
});
