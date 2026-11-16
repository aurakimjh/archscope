// Synthetic offline import workloads. No live capture, credentials or network I/O.
import assert from 'node:assert/strict';

export function importMemoryDocuments(seed, scenario = 'repeated', cycles = 36) {
  assert(['repeated','unique','body-rich'].includes(scenario));
  assert(Number.isSafeInteger(cycles) && cycles > 0);
  assert(seed?.request?.bodyStorage === 'omitted' && seed?.response?.bodyStorage === 'omitted');
  const sizes = scenario === 'repeated' ? [4096,512,32] : Array(cycles*3).fill(scenario === 'unique' ? 512 : 128);
  return sizes.map((size,open) => {
    const transactions=Array.from({length:size},(_,i)=>({
      ...structuredClone(seed),id:`import-memory-${size}-${i}`,connectionId:`synthetic-${i}`,sequence:i+1,
      url:`http://127.0.0.1:11514/import?size=${size}&row=${i}`,host:'127.0.0.1:11514',path:'/import',query:`size=${size}&row=${i}`,
      starred:i===0,comment:i===0?'메모리 재열기 synthetic note':'',
      process:{key:{pid:4242,startTime:'2026-11-14T00:00:00Z'},name:'synthetic-fixture',attribution:'confirmed'},
    }));
    if(scenario !== 'repeated') for(const [i,tx] of transactions.entries()) {
      tx.id=`import-memory-${scenario}-${open}-${i}`;
      tx.comment=i===0?`메모리 재열기 synthetic note ${scenario} ${open}`:'';
      if(scenario==='body-rich') for(const side of ['request','response']) {
        const bodyPreview=`synthetic-${side}-${open}-${i}-`+'x'.repeat(1024);
        const bytes=Buffer.byteLength(bodyPreview);
        const headers=[{name:'Content-Type',value:'text/plain'},{name:'Content-Length',value:String(bytes)}];
        tx[side]={headers,cookies:[],headerSize:headers.reduce((n,h)=>n+Buffer.byteLength(`${h.name}: ${h.value}\r\n`),0),
          bodySize:bytes,bodyDecoded:bytes,transferSize:bytes,contentType:'text/plain',bodyStorage:'inline',bodyPreview};
      }
    }
    const doc={type:'archscope-http-session',schemaVersion:1,scope:'all',selectedId:transactions[0].id,transactions};
    return {name:scenario==='repeated'?`synthetic-${size}.httpsession`:`synthetic-${scenario}-${String(open).padStart(3,'0')}.httpsession`,doc};
  });
}
