// Isolated Node retained-object experiment. Forced GC here is diagnostic only;
// these measurements NEVER qualify the ordinary Windows production screen.
import {readFile,writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import assert from 'node:assert/strict';
import path from 'node:path';
const [moduleFile,modelDirectory,output]=process.argv.slice(2);
assert.equal(typeof global.gc,'function');
const workspace=await import(pathToFileURL(path.resolve(moduleFile)).href);
const templates=await Promise.all([4096,512,32].map(size=>readFile(path.join(modelDirectory,`model-${size}.json`),'utf8').then(JSON.parse)));
workspace.clearWorkspaceResults();global.gc();
const before=process.memoryUsage(),ids=[],points=[];
for(let cycle=1;cycle<=36;cycle++) {
 for(const template of templates) {
  const result=structuredClone(template);result.created_at=new Date().toISOString();
  const entry=workspace.addWorkspaceResult({result,title:`diagnostic-${ids.length}`,httpTransactionRowsFingerprint:result.metadata.workspace_transaction_rows_fingerprint});ids.push(entry.id);
 }
 global.gc();
 const retained=ids.map(id=>workspace.getWorkspaceEntry(id)).filter(Boolean);
 const arrays=retained.map(entry=>entry.result.tables.transactions);
 const unique=[...new Set(arrays)];
 points.push({cycle,entries:retained.length,uniqueTransactionArrays:unique.length,logicalRows:arrays.reduce((n,a)=>n+a.length,0),uniqueRetainedRows:unique.reduce((n,a)=>n+a.length,0),...process.memoryUsage()});
}
assert.equal(points.at(-1).entries,80);
const final=points.at(-1);
workspace.clearWorkspaceResults();global.gc();
const report={scope:'Isolated actual workspace module with real backend projections; explicit GC for retained-object attribution only',node:process.version,forcedGC:true,nativeAcceptance:false,before,points,final};
await writeFile(output,JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({entries:final.entries,logicalRows:final.logicalRows,uniqueTransactionArrays:final.uniqueTransactionArrays,uniqueRetainedRows:final.uniqueRetainedRows,heapUsedMiB:final.heapUsed/1048576}));
