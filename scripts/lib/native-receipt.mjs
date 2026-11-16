// Compatibility with older non-atomic native producers. Never treat malformed,
// missing, timed-out or explicitly failed receipts as a passing report.
import {readFile} from 'node:fs/promises';
import {performance} from 'node:perf_hooks';
export async function waitForNativeReceipt(file,{isRunning,timeoutMs=310000,intervalMs=250,read=readFile,now=()=>performance.now(),sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms))}={}){
 if(typeof isRunning!=='function'||!Number.isFinite(timeoutMs)||timeoutMs<=0||!Number.isFinite(intervalMs)||intervalMs<=0)throw new TypeError('bounded native receipt wait configuration required');
 const deadline=now()+timeoutMs;let lastRead='missing';
 while(now()<deadline){
  let body;
  try{body=await read(file,'utf8');}
  catch(error){if(error.code!=='ENOENT')throw error;lastRead='missing';}
  if(body!==undefined){
   let parsed;
   try{parsed=JSON.parse(body);}catch(error){if(!(error instanceof SyntaxError))throw error;lastRead='partial-or-invalid';}
   if(parsed!==undefined){
    if(!parsed || typeof parsed!=='object' || !parsed.report || typeof parsed.report.passed!=='boolean')throw Object.assign(new Error('Native receipt schema is invalid'),{code:'NATIVE_RECEIPT_INVALID'});
    return parsed; // Caller must still reject report.passed=false and wrong fields.
   }
  }
  if(!isRunning())throw Object.assign(new Error('Native app exited without a complete valid receipt'),{code:'NATIVE_RECEIPT_EXITED',lastRead});
  await sleep(Math.min(intervalMs,Math.max(0,deadline-now())));
 }
 throw Object.assign(new Error('Native receipt did not complete before timeout'),{code:'NATIVE_RECEIPT_TIMEOUT',lastRead});
}
