// Isolated retained-object attribution. Explicit GC is not native leak or T-662 evidence.
import assert from 'node:assert/strict';
import {readFile,writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import path from 'node:path';
const [moduleFile,modelDir,scenario,out]=process.argv.slice(2);
assert.equal(typeof global.gc,'function');
const manifest=JSON.parse(await readFile(path.join(modelDir,'manifest.json'),'utf8'));
assert.equal(manifest.cycles,36);assert.equal(manifest.historyCapacity,80);
assert(['repeated','unique','body-rich','unshared-control'].includes(scenario));
const files=manifest.scenarios[scenario==='unshared-control'?'repeated':scenario];
const workspace=await import(pathToFileURL(path.resolve(moduleFile)).href);
const ids=[],samples=[];let current=0;
async function collect(){global.gc();await new Promise(resolve=>setImmediate(resolve));global.gc();return process.memoryUsage();}
function counts(){const entries=ids.map(id=>workspace.getWorkspaceEntry(id)).filter(Boolean);const arrays=entries.map(e=>e.result.tables.transactions),unique=[...new Set(arrays)];return {entries:entries.length,logicalRows:arrays.reduce((n,a)=>n+a.length,0),distinctArrays:unique.length,uniqueRows:unique.reduce((n,a)=>n+a.length,0),immutableArrays:unique.filter(Object.isFrozen).length};}
async function open(){
 const raw=await readFile(path.join(modelDir,files[current%files.length]),'utf8');
 const result=JSON.parse(raw);result.created_at=`synthetic-import-${current}`;
 const entry=workspace.addWorkspaceResult({result,title:`synthetic-${current}`,httpTransactionRowsFingerprint:scenario==='unshared-control'?undefined:result.metadata.workspace_transaction_rows_fingerprint});
 assert.deepEqual(entry.result,result);
 assert.equal(Object.isFrozen(result.tables.transactions),false);
 ids.push(entry.id);current++;
}
workspace.clearWorkspaceResults();const before=await collect();
for(let cycle=1;cycle<=36;cycle++){
 for(let i=0;i<3;i++)await open();
 const memory=await collect();samples.push({cycle,...counts(),heapUsed:memory.heapUsed,rss:memory.rss,external:memory.external});
}
const final=samples.at(-1);assert.equal(final.entries,80);
if(scenario==='repeated'){assert.equal(final.distinctArrays,3);assert.equal(final.uniqueRows,4640);assert.equal(final.immutableArrays,3);}
else {assert.equal(final.distinctArrays,80);if(scenario!=='unshared-control')assert.equal(final.immutableArrays,80);}
workspace.clearWorkspaceResults();const afterClear=await collect();assert.equal(counts().entries,0);assert(ids.every(id=>workspace.getWorkspaceEntry(id)===null));
const measured=samples.slice(28),slopes=[];
for(let a=0;a<measured.length;a++)for(let b=a+1;b<measured.length;b++)slopes.push((measured[b].heapUsed-measured[a].heapUsed)/(measured[b].cycle-measured[a].cycle));
slopes.sort((a,b)=>a-b);const slope=(slopes[13]+slopes[14])/2;
// Predeclared retained-heap screening limits for this isolated process only.
assert(afterClear.heapUsed-before.heapUsed<16*1048576,'retained heap after clear exceeded 16MiB');
assert(slope<4*1048576,'post-capacity retained-heap slope exceeded 4MiB/cycle');
const report={passed:true,scope:'Actual compiled renderer workspace with actual backend projections; isolated explicit-GC attribution only',node:process.version,scenario,forcedGC:true,nativeAcceptance:false,naturalPressureEvidence:false,protocol:{cycles:36,warmupCycles:28,measuredCycles:8,historyCapacity:80,heapAfterClearBudgetMiB:16,saturatedHeapSlopeBudgetMiBPerCycle:4},before,samples,final,afterClear,afterClearHeapDelta:afterClear.heapUsed-before.heapUsed,measuredHeapSlopeBytesPerCycle:slope,registryEmptyAfterClear:true};
await writeFile(out,JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({scenario,passed:true,arrays:final.distinctArrays,uniqueRows:final.uniqueRows,heapMiB:final.heapUsed/1048576,afterClearDeltaMiB:report.afterClearHeapDelta/1048576}));
