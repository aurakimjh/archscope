// Full-value equivalence/ownership check, separate from native memory screening.
import {readFile,writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
import path from 'node:path';
import assert from 'node:assert/strict';
const [beforeFile,fixedFile,modelDirectory,output]=process.argv.slice(2);
const before=await import(pathToFileURL(path.resolve(beforeFile)).href);
const fixed=await import(pathToFileURL(path.resolve(fixedFile)).href);
const templates=await Promise.all([4096,512,32].map(n=>readFile(path.join(modelDirectory,`model-${n}.json`),'utf8').then(JSON.parse)));
const ids=[[],[]],digests=[createHash('sha256'),createHash('sha256')];
let checked=0;
for(let index=0;index<108;index++) {
 const input=structuredClone(templates[index%3]);input.created_at=`analysis-${index}`;
 const a=before.addWorkspaceResult({result:structuredClone(input),title:`card-${index}`,httpTransactionRowsFingerprint:input.metadata.workspace_transaction_rows_fingerprint});
 const b=fixed.addWorkspaceResult({result:structuredClone(input),title:`card-${index}`,httpTransactionRowsFingerprint:input.metadata.workspace_transaction_rows_fingerprint});
 assert.deepEqual(b.result,a.result);assert.equal(b.created_at,`analysis-${index}`);
 ids[0].push(a.id);ids[1].push(b.id);checked++;
 if(global.gc)global.gc();
}
const retained=ids.map((list,i)=>list.map(id=>(i?fixed:before).getWorkspaceEntry(id)).filter(Boolean));
assert.equal(retained[0].length,80);assert.equal(retained[1].length,80);
for(let i=0;i<80;i++) {
 assert.deepEqual(retained[1][i].result,retained[0][i].result);
 assert.equal(retained[1][i].title,retained[0][i].title);
 for(let j=0;j<2;j++)digests[j].update(JSON.stringify(retained[j][i].result)+'\n');
}
const hashes=digests.map(h=>h.digest('hex'));assert.equal(hashes[0],hashes[1]);
const rows=retained[1].map(e=>e.result.tables.transactions);
assert.equal(new Set(rows).size,3);assert(rows.every(a=>Object.isFrozen(a)&&Object.isFrozen(a[0])));
before.clearWorkspaceResults();fixed.clearWorkspaceResults();
assert(ids[0].every(id=>before.getWorkspaceEntry(id)===null));assert(ids[1].every(id=>fixed.getWorkspaceEntry(id)===null));
await writeFile(output,JSON.stringify({passed:true,scope:'Full value/card metadata equivalence and registry ownership, not native memory qualification',importsCompared:checked,retainedCardsCompared:80,allRetainedResultFieldsSha256:hashes[0],uniqueImmutableTransactionArrays:3,clearReleasesAllEntries:true},null,2)+'\n');
console.log('PASS108imports/80retained full values and metadata;3immutable arrays;clear releases registry entries');
