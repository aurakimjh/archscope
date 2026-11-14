import {mkdir,readFile,writeFile,stat,readdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import assert from 'node:assert/strict';
import path from 'node:path';
const [folder,output]=process.argv.slice(2),out=path.resolve(output);await mkdir(out,{recursive:false});
const d=JSON.parse(await readFile(folder+'/.private-controller.json'));
const sha=b=>createHash('sha256').update(b).digest('hex');
async function call(command,params,fail=false){const response=await fetch(d.url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${d.token}`},body:JSON.stringify({id:crypto.randomUUID(),command,params}),signal:AbortSignal.timeout(60000)});const r=await response.json();assert.equal(r.ok,!fail);return r;}
const source=path.resolve('.tmp/windows-final-20261114/unicode-notes.httpsession'),original=await readFile(source),doc=JSON.parse(original);
const report={schemaVersion:1,scope:'Actual owned Windows desktop controller; not every export dialog/native viewer',passed:false,binarySource:'adfc9544 plus two-line imported-comment post-redaction limit fix',cases:[]};
try {
  for(const kind of ['truncated','duplicate-id','redaction-growth']) {
    const bad=structuredClone(doc);bad.transactions=bad.transactions.slice(0,1);
    if(kind==='duplicate-id')bad.transactions.push(structuredClone(bad.transactions[0]));
    if(kind==='redaction-growth')bad.transactions[0].comment='password=x '.repeat(300);
    const file=path.join(out,kind+'.httpsession'),raw=kind==='truncated'?'{"type":':JSON.stringify(bad);await writeFile(file,raw);
    const rejected=await call('open',{path:file},true);assert.equal(sha(await readFile(file)),sha(raw));
    report.cases.push({case:kind,rejected:true,sourceBytesUnchanged:true,reason:rejected.error});
  }
  const ids=[doc.transactions[17].id,doc.transactions[0].id,doc.transactions[31].id];
  for(const scope of ['all','filtered'])for(const [format,ext] of Object.entries({session:'httpsession',har:'har',native:'halog',xml:'xml',xml_compact:'compact.xml',urls:'urls.txt',text:'text.txt',html:'html',xls:'xls',responses:'responses'})) {
    const destination=path.join(out,scope+'-'+format+'.'+ext),count=scope==='all'?32:3;
    if(format==='responses')await mkdir(destination);
    const receipt={format,scope,rows:count};
    let exportSource=source;
    if(format==='native'){
      const refused=await call('save',{source:{path:source},destination,format,scope,ids:scope==='filtered'?ids:[],selectedId:ids[1]},true);
      assert.match(refused.error,/HALOG_ENCODING.*codepage 949/);await assert.rejects(stat(destination));
      const compatible=structuredClone(doc);compatible.transactions[0].comment='native-compatible-note';
      exportSource=path.join(out,'native-compatible.httpsession');await writeFile(exportSource,JSON.stringify(compatible));
      receipt.unrepresentableUnicodeRefusedWithoutOutput=true;receipt.nativeRetryUsesDeclaredCP949CompatibleComment=true;
    }
    const saved=await call('save',{source:{path:exportSource},destination,format,scope,ids:scope==='filtered'?ids:[],selectedId:ids[1]});assert.equal(saved.result,count);
    if(format==='responses'){const files=await readdir(destination);assert.equal(files.length,1);receipt.outputDirectoryCount=files.length;}
    else {const raw=await readFile(destination);assert(raw.length>0);receipt.bytes=raw.length;receipt.sha256=sha(raw);}
    if(['session','har','xml','xml_compact','native'].includes(format)) {
      const opened=(await call('open',{path:destination})).result;assert.equal(opened.transactions,count);receipt.reopenedRows=count;
      if(format!=='native'){assert.equal(opened.selectedId,ids[1]);receipt.selectionPreserved=true;}
      if(['session','har','xml'].includes(format)) {
        const proof=path.join(out,scope+'-'+format+'-readback.httpsession');await call('save',{source:{path:destination},destination:proof,format:'session',scope:'all',ids:[],selectedId:ids[1]});
        const actual=JSON.parse(await readFile(proof)),wanted=scope==='all'?doc.transactions:ids.map(id=>doc.transactions.find(tx=>tx.id===id));
        if(format==='xml'){
          let nullCookies=0;for(let i=0;i<actual.transactions.length;i++)for(const message of ['request','response']){
            if(wanted[i][message].cookies?.length===0&&actual.transactions[i][message].cookies===null){actual.transactions[i][message].cookies=[];nullCookies++;}
          }
          receipt.xmlEmptyCookieSlicePolicy={version:'1.0.0',nullVersusEmptyArrayFields:nullCookies};
        }
        assert.deepEqual(actual.transactions,wanted);receipt.allTransactionFieldsAndAnnotationsMatchDeclaredPolicy=true;
      }
      if(format==='xml_compact'){const proof=path.join(out,scope+'-compact-readback.httpsession');await call('save',{source:{path:destination},destination:proof,format:'session',scope:'all',ids:[],selectedId:ids[1]});const rows=JSON.parse(await readFile(proof)).transactions;assert(rows.every(tx=>tx.request.bodyStorage==='export_omitted'&&tx.response.bodyStorage==='export_omitted'));receipt.compactOmissionExplicit=true;}
    }
    report.cases.push(receipt);
  }
  const rejectedPath=path.join(out,'must-not-exist.httpsession');
  await call('save',{source:{path:source},destination:rejectedPath,format:'session',scope:'filtered',ids:['absent'],selectedId:''},true);
  await assert.rejects(stat(rejectedPath));report.staleFilteredIDRefusedWithoutOutput=true;
  assert.equal(sha(await readFile(source)),sha(original));report.originalSourceUnchanged=true;
  await call('open',{path:source});report.validRetryOpened32=true;report.passed=true;
}catch(e){await writeFile(out+'/.private-failure.txt',String(e));report.failure='File matrix failed; details remain private';process.exitCode=1;}
await writeFile(out+'/result.json',JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,cases:report.cases.length,failure:report.failure}));
