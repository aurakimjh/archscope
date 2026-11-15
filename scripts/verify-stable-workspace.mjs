// Invoked by the releasequalification Go test against its owned synthetic server.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import path from 'node:path';
const [moduleFile,out]=process.argv.slice(2);
const workspace=await import(pathToFileURL(path.resolve(moduleFile)).href);
const {exportWorkspaceResult}=await import(pathToFileURL(path.join(path.dirname(path.resolve(moduleFile)),'workspaceExport.js')).href);
const url=new URL(process.env.ARCHSCOPE_STABLE_TEST_URL);assert.equal(url.hostname,'127.0.0.1');
const pending=new Set();
async function rpc(command,...args){
 const response=await fetch(url,{method:'POST',headers:{Authorization:`Bearer ${process.env.ARCHSCOPE_STABLE_TEST_TOKEN}`,'Content-Type':'application/json'},body:JSON.stringify({command,args}),signal:AbortSignal.timeout(15000)});
 const data=await response.json();assert.equal(response.status,200,data.error);return data.value;
}
function tracked(promise){pending.add(promise);promise.finally(()=>pending.delete(promise));return promise;}
async function settle(){while(pending.size)await Promise.all([...pending]);}
workspace.configureCasePersistence({
 create:async title=>JSON.parse(await rpc('create',title)),get:async id=>JSON.parse(await rpc('get',id)),
 save:async(handle,request)=>JSON.parse(await rpc('save',handle,request)),
 reopen:async(caseID,resultID)=>{const owned=JSON.parse(await rpc('reopen',caseID,resultID));return {handle:owned.view.handle,leaseID:owned.lease.id,type:owned.view.type,summaryJSON:owned.view.summaryJSON};},
 acquire:handle=>tracked(rpc('acquire',handle)),release:id=>tracked(rpc('release',id)),
});
workspace.clearWorkspaceResults();
const inputs=await rpc('inputs'),saved=[];
for(const [name,input] of Object.entries(inputs)){
 const entry=workspace.addWorkspaceResult({result:JSON.parse(input.raw),backendResultHandle:input.handle,title:`synthetic-${name}`});await settle();
 const ref=await workspace.saveWorkspaceCase(entry.id);assert.deepEqual(await workspace.saveWorkspaceCase(entry.id),ref);
 const manifest=JSON.parse(await rpc('get',ref.case_id));assert.equal(manifest.results.length,1);
 saved.push({name,ref,raw:input.raw});
}
workspace.clearWorkspaceResults();await settle();await rpc('restart');
let exports=0;
for(const {name,ref,raw} of saved){
 const entry=await workspace.reopenWorkspaceCase(ref.case_id,ref.result_id);await settle();
 assert.equal(entry.saved_case.result_sha256,ref.result_sha256);
 // Reopened renderer entry is only a display projection; the exact JSON must
 // come from the backend, including integers beyond JavaScript precision.
 assert.deepEqual(entry.result.summary,{});
 assert(entry.summary_preview.length>0,'reopened bounded display summary missing');
 for(const mode of ['json','html','csv']){
  let result;
  await exportWorkspaceResult(entry,mode,`${name}.${mode}`,{backend:async(handle,file,format)=>{result=await rpc('export',handle,file,format);},legacy:new Proxy({},{get(){throw new Error('opaque entry used legacy exporter');}})});
  assert.equal(typeof result,'string');assert(result.length>0);
  if(mode==='json')assert.equal(result,raw);
  exports++;
 }
 await assert.rejects(()=>exportWorkspaceResult(entry,'pptx','refused.pptx',{backend:()=>assert.fail('unsupported format reached backend'),legacy:{}}),/UNSUPPORTED/);
}
assert.equal(workspace.getActiveWorkspaceEntry().saved_case.result_id,saved.at(-1).ref.result_id);
const active=workspace.getActiveWorkspaceEntry();workspace.removeWorkspaceResult(active.id);await settle();
assert.equal(workspace.getWorkspaceEntry(active.id),null);assert(workspace.getActiveWorkspaceEntry());
workspace.clearWorkspaceResults();await settle();assert.equal(workspace.getActiveWorkspaceEntry(),null);
for(const {ref} of saved)assert.equal(JSON.parse(await rpc('get',ref.case_id)).results.length,1);
assert(saved.some(x=>x.raw.includes('9007199254740993')));
const report={passed:true,scope:'Headless actual renderer state and Wails services through synthetic loopback test transport; no native UI acceptance',families:saved.map(x=>x.name),cases:saved.length,exports,exactJSONPreservedAcrossFreshBackend:true,saveIdempotent:true,opaqueExportRouting:true,unsupportedPptxRefused:true,independentRemovalAndLeaseCleanup:true,caseDataSurvivesWorkspaceClear:true,sourceResultHashes:saved.map(x=>({family:x.name,sha256:createHash('sha256').update(x.raw).digest('hex')}))};
await writeFile(out,JSON.stringify(report,null,2)+'\n');console.log(`PASS ${saved.length} real saved cases / ${exports} exports / cache-loss restart`);
