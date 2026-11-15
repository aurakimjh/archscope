// Read-only passive telemetry around actual backend opens. Diagnostic build only;
// this rapid sequence does not qualify the ordinary36-cycle memory budget.
import {readFile,writeFile} from 'node:fs/promises';
import assert from 'node:assert/strict';
import path from 'node:path';
const [profile,fixtures,output]=process.argv.slice(2);
const descriptor=JSON.parse(await readFile(profile+'/.private-controller.json'));
assert.equal(new URL(descriptor.url).hostname,'127.0.0.1');
const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function call(command,params={}) {
 const response=await fetch(descriptor.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${descriptor.token}`},body:JSON.stringify({id:crypto.randomUUID(),command,params}),signal:AbortSignal.timeout(60000)});
 const result=await response.json();assert.equal(result.ok,true,command);return result.result;
}
async function observed(predicate) {
 const deadline=Date.now()+20000;
 while(Date.now()<deadline) {
  try {
   const report=JSON.parse(await readFile(profile+'/layout.json'));
   const value=report.report.views.at(-1).workspaceRetention;
   if(value&&predicate(value))return value;
  } catch(error) {if(error.code!=='ENOENT'&&!(error instanceof SyntaxError))throw error;}
  await delay(100);
 }
 throw Error('Passive workspace readiness timed out');
}
const result={schemaVersion:1,scope:'Instrumented actual Windows renderer owner/reference counts; no native memory-budget claim',forcedGC:false,passed:false,samples:[]};
try {
 result.build=(await call('capabilities')).buildCommit;
 await observed(v=>v.entries===0);
 for(let index=0;index<108;index++) {
  const rows=[4096,512,32][index%3];
  const opened=await call('open',{path:path.resolve(fixtures,`synthetic-${rows}.httpsession`)});
  assert.equal(opened.transactions,rows);
  const expected=Math.min(index+1,80);
  const value=await observed(v=>v.entries===expected&&v.activeRows===rows);
  assert.equal(value.fingerprintedHttpEntries,expected);
  assert.equal(value.uniqueTransactionArrays,Math.min(index+1,3));
  assert.equal(value.immutableArrays,Math.min(index+1,3));
  result.samples.push({open:index+1,rows,...value});
 }
 const final=result.samples.at(-1);
 assert.equal(final.entries,80);assert.equal(final.logicalRows,121184);
 assert.equal(final.uniqueRetainedRows,4640);assert.equal(final.uniqueTransactionArrays,3);
 result.passed=true;
} catch(error) {result.failure=String(error);process.exitCode=1}
await writeFile(output,JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify({passed:result.passed,opens:result.samples.length,last:result.samples.at(-1),failure:result.failure}));
