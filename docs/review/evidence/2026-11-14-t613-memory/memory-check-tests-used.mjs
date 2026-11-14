import assert from 'node:assert/strict';
import { test } from 'node:test';
import { assessMemoryPlateau } from './lib/desktop-memory-checks.mjs';
const flat = () => Array.from({length:8},() => ({treePrivateBytes:400*1048576,rootPrivateBytes:80*1048576,rootHandles:600,rootThreads:20}));
test('matched flat samples meet the declared budgets',()=>assert.equal(assessMemoryPlateau(flat()).withinBudget,true));
test('cache-like fluctuations do not imply monotonic growth',()=>{
  const points=flat();for(let i=0;i<8;i++)points[i].treePrivateBytes+=(i%3)*1048576;
  assert.equal(assessMemoryPlateau(points).withinBudget,true);
});
for(const [key,growth] of [['treePrivateBytes',8*1048576],['rootPrivateBytes',2*1048576],['rootHandles',4],['rootThreads',1]]) {
  test(`sustained ${key} growth cannot pass a peak-only memory guard`,()=>{
    const points=flat();points.forEach((p,i)=>p[key]+=i*growth);
    assert.equal(assessMemoryPlateau(points).withinBudget,false);
  });
}
test('large retained step fails even with a shallow median slope',()=>{
  const points=flat();points.slice(4).forEach(p=>p.treePrivateBytes+=100*1048576);
  assert.equal(assessMemoryPlateau(points).withinBudget,false);
});
test('missing and invalid samples cannot yield a memory verdict',()=>{
  assert.throws(()=>assessMemoryPlateau(flat().slice(1)));
  const points=flat();points[2].rootHandles=NaN;assert.throws(()=>assessMemoryPlateau(points));
});
