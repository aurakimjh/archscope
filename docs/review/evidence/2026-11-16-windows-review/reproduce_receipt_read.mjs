// Source-bound runner reproduction with controlled I/O; no Windows/native execution.
import assert from 'node:assert/strict';
import {readFile as actualRead,writeFile as actualWrite} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {EventEmitter} from 'node:events';
import path from 'node:path';
import vm from 'node:vm';
import {execFileSync} from 'node:child_process';
const reviewedRef='635512cfdfe7673f8a255cd80fae1dcbbc0e16ba';
const source=execFileSync('git',['show',reviewedRef+':scripts/verify-windows-ribbon.mjs'],{encoding:'utf8'});
const body=source.replace(/^import .*;\r?\n/gm,'');
const script=execFileSync('git',['show',reviewedRef+':scripts/verify-ribbon-native.js']);
const pe=Buffer.alloc(128);pe.writeUInt32LE(64,0x3c);pe.writeUInt16LE(0x8664,68);
async function scenario(kind){
 let reads=0,completeOffered=false,time=0;const writes=[];
 function spawn(){const child=new EventEmitter();child.stdout=new EventEmitter();child.stderr=new EventEmitter();child.exitCode=null;child.signalCode=null;child.kill=()=>{child.exitCode=0;};return child;}
 async function readFile(file){
  if(file.endsWith('.exe'))return pe;
  if(file.endsWith('verify-ribbon-native.js'))return script;
  reads++;
  if(kind==='partial-then-complete' && reads===1)return Buffer.from('{');
  if(kind==='not-yet-created' && reads===1)throw Object.assign(new Error('not created'),{code:'ENOENT'});
  completeOffered=true;const zoom=file.includes('/wide/')?1:1.25;
  return Buffer.from(JSON.stringify({webviewZoom:zoom,osScreenScaleFactor:1,report:{passed:true,locales:['en','ko','ja','zh-CN','zh-TW'].map(locale=>({locale,selected:'true',menuRows:1,ribbonHeight:122})),themes:[{},{},{}],viewport:{pixelRatio:zoom}}}));
 }
 const context={assert,spawn,mkdir:async()=>{},readFile,writeFile:async(file,text)=>writes.push(JSON.parse(text)),appendFile:async()=>{},createHash,path,performance:{now:()=>time++},setTimeout:callback=>queueMicrotask(callback),process:{platform:'win32',argv:['node','runner','/owned/app.exe','/owned/new-output'],env:{}},console:{log(){}}};
 let failure;
 try{await vm.runInNewContext('(async()=>{'+body+'\n})()',context);}catch(error){failure={name:error.name,message:error.message};}
 return {kind,reads,completeOffered,passed:writes.at(-1)?.passed===true,failure};
}
const complete=await scenario('complete');const delayed=await scenario('not-yet-created');const partial=await scenario('partial-then-complete');
assert(complete.passed && delayed.passed);
assert(!partial.passed && partial.failure.name==='SyntaxError' && partial.reads===1 && !partial.completeOffered);
const result={scope:'Actual newly added ribbon runner in a VM with controlled Windows process/filesystem I/O; not a new native failure or change to received PASS receipts',sourceSha256:createHash('sha256').update(source).digest('hex'),producer:'layout_audit.go writes final JSON with os.WriteFile, not atomic publication',complete,delayed,partial,finding:'WREV3-01',priority:'P3',conclusion:'ENOENT is retried, but an existing not-yet-complete JSON receipt fails immediately before the next valid read'};
await actualWrite(process.argv[2]??'/tmp/archscope-receipt-reproduction.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result));
