// Aggregate-only diagnostic CDP reads. Never collect garbage, prepare leak
// detection, take a heap/DOM snapshot, evaluate scripts or return page payloads.
import assert from 'node:assert/strict';
export const memoryProbeMethods=Object.freeze(['Runtime.getHeapUsage','Memory.getDOMCounters','Performance.getMetrics']);
const numeric=(value)=>typeof value==='number' && Number.isFinite(value) && value>=0?value:null;
export function verifyOwnedDebugSample(sample){
 assert.equal(sample.debugPortOwned,true,'debug listener must belong to the owned process tree');
 assert.equal(sample.responding,true);assert.equal(sample.hasMainWindow,true);
 assert(sample.rootStartTime && sample.members.length>0);
 assert(sample.members.some(m=>m.identity===sample.debugListenerIdentity),'listener start identity is not an owned member');
 assert.equal(sample.members.filter(m=>m.role==='app').length,1);
 assert.equal(sample.members.filter(m=>m.role==='renderer').length,1,'one owned renderer is required for unambiguous correlation');
 return sample;
}
export async function readRendererMemory(call){
 const results={},unavailable=[];
 for(const method of memoryProbeMethods){
  try{results[method]=await call(method);}catch(error){
   if(error?.code!==-32601)throw new Error('Aggregate CDP read failed: '+method);
   results[method]=null;unavailable.push(method);
  }
 }
 const heap=results['Runtime.getHeapUsage'],dom=results['Memory.getDOMCounters'];
 const metrics=new Map((results['Performance.getMetrics']?.metrics??[]).map(m=>[m.name,m.value]));
 // Explicit projection drops unknown strings, URLs, headers, IDs and protocol errors.
 const result={heap:heap?{used:numeric(heap.usedSize),total:numeric(heap.totalSize),embedder:numeric(heap.embedderHeapUsedSize),backingStorage:numeric(heap.backingStorageSize)}:null,
  dom:dom?{documents:numeric(dom.documents),nodes:numeric(dom.nodes),listeners:numeric(dom.jsEventListeners)}:null,
  performance:Object.fromEntries(['JSHeapUsedSize','JSHeapTotalSize','Nodes','Documents','JSEventListeners','TaskDuration','LayoutCount'].map(k=>[k,numeric(metrics.get(k))])),unavailable};
 assert(Buffer.byteLength(JSON.stringify(result))<=4096,'aggregate record too large');return result;
}
export async function connectMemoryProbe(url,{timeoutMs=5000,WebSocketImpl=globalThis.WebSocket}={}){
 const endpoint=new URL(url);assert.equal(endpoint.protocol,'ws:');assert.equal(endpoint.hostname,'127.0.0.1');
 assert(!endpoint.username&&!endpoint.password&&!endpoint.search&&!endpoint.hash);assert(endpoint.pathname.startsWith('/devtools/page/'));
 const socket=new WebSocketImpl(endpoint.href),pending=new Map();let sequence=0,closed=false;
 const fail=()=>{closed=true;for(const p of pending.values()){clearTimeout(p.timer);p.reject(new Error('Aggregate CDP connection closed'));}pending.clear();};
 await new Promise((resolve,reject)=>{const timer=setTimeout(()=>{socket.close();reject(new Error('CDP open timeout'));},timeoutMs);socket.addEventListener('open',()=>{clearTimeout(timer);resolve();},{once:true});socket.addEventListener('error',()=>{clearTimeout(timer);reject(new Error('CDP open failed'));},{once:true});});
 socket.addEventListener('close',fail);socket.addEventListener('error',fail);
 socket.addEventListener('message',event=>{
  if(typeof event.data!=='string'||Buffer.byteLength(event.data)>65536){fail();socket.close();return;}
  let message;try{message=JSON.parse(event.data);}catch{fail();socket.close();return;}
  const p=pending.get(message.id);if(!p)return;pending.delete(message.id);clearTimeout(p.timer);
  if(message.error)p.reject(Object.assign(new Error('Aggregate method unavailable'),{code:message.error.code}));else p.resolve(message.result);
 });
 const call=(method)=>{assert(memoryProbeMethods.includes(method),'non-read-only memory method refused');assert(!closed,'aggregate connection closed');return new Promise((resolve,reject)=>{const id=++sequence;const timer=setTimeout(()=>{pending.delete(id);reject(new Error('Aggregate CDP read timeout'));},timeoutMs);pending.set(id,{resolve,reject,timer});try{socket.send(JSON.stringify({id,method,params:{}}));}catch{clearTimeout(timer);pending.delete(id);reject(new Error('Aggregate CDP send failed'));}});};
 call.close=()=>{fail();socket.close();};return call;
}
