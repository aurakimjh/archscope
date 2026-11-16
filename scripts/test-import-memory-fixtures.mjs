import assert from 'node:assert/strict';
import {test} from 'node:test';
import {createHash} from 'node:crypto';
import {importMemoryDocuments} from './lib/import-memory-fixtures.mjs';
const seed={id:'seed',request:{bodyStorage:'omitted'},response:{bodyStorage:'omitted'},extra:{nested:true}};
const digest=value=>createHash('sha256').update(JSON.stringify(value)).digest('hex');
test('default workload preserves the historical generator byte for byte',()=>{
  const before=JSON.stringify(seed);
  const expected=[4096,512,32].map(size=>({name:`synthetic-${size}.httpsession`,doc:{type:'archscope-http-session',schemaVersion:1,scope:'all',selectedId:`import-memory-${size}-0`,transactions:Array.from({length:size},(_,i)=>({
    ...structuredClone(seed),id:`import-memory-${size}-${i}`,connectionId:`synthetic-${i}`,sequence:i+1,
    url:`http://127.0.0.1:11514/import?size=${size}&row=${i}`,host:'127.0.0.1:11514',path:'/import',query:`size=${size}&row=${i}`,
    starred:i===0,comment:i===0?'메모리 재열기 synthetic note':'',
    process:{key:{pid:4242,startTime:'2026-11-14T00:00:00Z'},name:'synthetic-fixture',attribution:'confirmed'},
  }))}}));
  assert.equal(JSON.stringify(importMemoryDocuments(seed)),JSON.stringify(expected));
  assert.equal(JSON.stringify(seed),before);
});
for(const [scenario,rows] of [['unique',512],['body-rich',128]]) test(`${scenario}: 108 distinct documents with coherent rows and unchanged seed`,()=>{
  const before=JSON.stringify(seed),files=importMemoryDocuments(seed,scenario);
  assert.equal(files.length,108);assert.equal(new Set(files.map(f=>f.name)).size,108);
  // Full transaction hashes independently rule out accidental exact-row sharing.
  assert.equal(new Set(files.map(f=>digest(f.doc.transactions))).size,108);
  const ids=new Set();
  for(const {doc} of files){
    assert.equal(doc.transactions.length,rows);assert.equal(doc.selectedId,doc.transactions[0].id);
    for(const [i,tx] of doc.transactions.entries()){
      assert(!ids.has(tx.id));ids.add(tx.id);assert.equal(tx.starred,i===0);
      assert.equal(Boolean(tx.comment),i===0);
      for(const side of ['request','response']){
        const m=tx[side];
        if(scenario==='unique'){assert.equal(m.bodyStorage,'omitted');continue;}
        assert.equal(m.bodyStorage,'inline');assert.equal(m.contentType,'text/plain');
        const length=Buffer.byteLength(m.bodyPreview);
        assert(length>1024);assert.equal(m.bodySize,length);assert.equal(m.bodyDecoded,length);assert.equal(m.transferSize,length);
        assert.equal(m.headers.find(h=>h.name==='Content-Length').value,String(length));
        assert.equal(m.headers.find(h=>h.name==='Content-Type').value,'text/plain');
        assert.equal(m.bodyRef,undefined);assert.equal(m.omittedReason,undefined);
      }
    }
  }
  assert.equal(JSON.stringify(seed),before);
});
test('invalid workload and cycle counts are refused',()=>{
  assert.throws(()=>importMemoryDocuments(seed,'other'));assert.throws(()=>importMemoryDocuments(seed,'unique',0));
});
