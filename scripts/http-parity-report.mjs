import {readFile} from 'node:fs/promises';
export const contract=JSON.parse(await readFile(new URL('./http-standalone-parity-contract.json',import.meta.url),'utf8'));
export function createReport({platform,sourceCommit,binarySha256,buildCommit}){
 return {schemaVersion:1,platform,sourceCommit,binarySha256,buildCommit,nativeWindowVerified:false,cases:contract.cases.map(item=>({id:item.id,task:item.task,status:'UNVERIFIED',evidenceType:item.evidence,reason:'Not observed by this run',artifacts:[]}))};
}
export function assessReport(report){
 const errors=[];if(report.schemaVersion!==1)errors.push('unsupported report schema');
 if(!/^[a-f0-9]{40}$/.test(report.sourceCommit??'')||report.buildCommit!==report.sourceCommit)errors.push('binary/source commit is unverified');
 if(!/^[a-f0-9]{64}$/.test(report.binarySha256??''))errors.push('binary hash missing');
 const byId=new Map();for(const row of report.cases??[]){if(byId.has(row.id))errors.push(`duplicate case ${row.id}`);byId.set(row.id,row)}
 for(const item of contract.cases){const row=byId.get(item.id);if(!row){errors.push(`missing case ${item.id}`);continue};if(!['PASS','FAIL','UNVERIFIED','NOT_RUN'].includes(row.status))errors.push(`invalid verdict ${item.id}`);
  if(row.status==='PASS'){if(report.platform!=='win32')errors.push(`Windows evidence required for ${item.id}`);if(row.evidenceType!==item.evidence)errors.push(`wrong evidence type ${item.id}`);if(!row.artifacts?.length||row.artifacts.some(value=>!(/^[a-f0-9]{64}$/).test(value)))errors.push(`artifact hash required for ${item.id}`);if(item.evidence==='native_observation'&&!report.nativeWindowVerified)errors.push(`native window unverified for ${item.id}`)}
 }
 for(const id of byId.keys())if(!contract.cases.some(item=>item.id===id))errors.push(`unknown case ${id}`);
 const failed=(report.cases??[]).some(row=>row.status==='FAIL');const complete=contract.cases.every(item=>byId.get(item.id)?.status==='PASS');
 return {status:errors.length||failed?'FAIL':complete?'PASS':'UNVERIFIED',errors};
}
export function setCase(report,id,status,reason,artifacts=[]){const row=report.cases.find(row=>row.id===id);if(!row)throw new Error(`unknown case ${id}`);Object.assign(row,{status,reason,artifacts})}
