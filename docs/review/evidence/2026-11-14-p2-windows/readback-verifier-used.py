"""Independent Windows P2 receipt readback; raw IDs/credentials stay private."""
import argparse,json,hashlib
from pathlib import Path
from urllib.parse import urlsplit
base=Path('C:/workspace/archscope/.tmp/p2-windows-20261114')
public=Path('C:/workspace/archscope/docs/review/evidence/2026-11-14-p2-windows')
def read(p):return json.loads(p.read_text(encoding='utf-8-sig'))
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def normalized(tx):
 out={k:v for k,v in tx.items() if k not in ('starred','comment')}
 url=urlsplit(tx['url']);out.update(host=url.netloc,query=url.query)
 return out
def verify(kind,arch):
 folder=base/(kind+'-'+arch);file=folder/('final.httpsession')
 doc=read(file);rows=doc['transactions'];assert len({tx['id'] for tx in rows})==len(rows)>0
 session=read(folder/'.private-session.json');store=Path(session['storePath'])
 source=[read(p)['transaction'] for p in (store/'rolling').glob('*.json')]
 assert {tx['id']:normalized(tx) for tx in source}=={tx['id']:normalized(tx) for tx in rows}
 assert all(tx['statusCode']==200 and tx['state']=='complete' and tx['process']['attribution']=='confirmed' for tx in rows)
 selected=next(tx for tx in rows if tx['id']==doc['selectedId'])
 common={'arch':arch,'scope':kind,'rows':len(rows),'allIdsUniqueAndEqual':True,'allOtherFieldsEqualAfterPortableURLNormalization':True,'selectedSequence':selected['sequence'],'selectedURL':selected['url'],'fileSha256':sha(file),'fileBytes':file.stat().st_size,'idSetSha256':hashlib.sha256(json.dumps(sorted(tx['id'] for tx in rows)).encode()).hexdigest(),'retention':doc['retention'],'passed':True}
 if kind=='query':
  seed=read(folder/'seed.json');assert seed['requests']==len(rows)==4
  assert [tx['url'] for tx in rows]==seed['retainedURLs'] and selected['sequence']==2
  assert selected['url'].endswith('/lookup?next=/detail?id=7&keep=2')
  assert all(not tx.get('query') for tx in read(folder/'.private-rows.json'))
  done=read(folder/'command-02-stop.json.done');assert done['state']=='finalized' and done['stats']['persisted']==done['stats']['captured']==4
  common.update(separateLiveQueryEmpty=True,retainedURLSpellingExact=True)
 else:
  result=read(folder/'result.json');assert result['schema']==4 and result['passed'] and result['requestedSeconds']==130 and result['elapsedSeconds']>=130
  assert result['pauses']>=2 and result['clears']==0
  for point in result['checkpoints']:
   stats=point['stats'];assert stats['state']=='running' and stats['captured']==stats['persisted'] and stats['dropped']==0
   assert point['checkedRows']==point['retainedRows']<=128 and point['hasMainWindow'] and point['Responding']
  stats=result['finalStats'];assert stats['state']=='finalized' and stats['captured']==stats['persisted'] and stats['dropped']==0
  assert result['savedRows']==len(rows)==128 and result['savedSessionSha256']==sha(file)
  assert doc['retention']['persisted']==stats['persisted'] and doc['retention']['retained']==128
  assert selected['id']==rows[0]['id'] and len({(tx['process']['key']['pid'],tx['process']['key']['startTime']) for tx in rows})==1
  assert all(urlsplit(tx['url']).netloc=='127.0.0.1:11510' and urlsplit(tx['url']).path=='/soak' for tx in rows)
  common.update(elapsedSeconds=result['elapsedSeconds'],requests=result['requests'],pauses=result['pauses'],clears=0,checkpoints=len(result['checkpoints']),finalState=stats['state'],finalCountersEqual=True)
 return common
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('kind',choices=['query','soak']);parser.add_argument('arch',choices=['amd64','386']);args=parser.parse_args()
 value=verify(args.kind,args.arch);(public/('readback-'+args.kind+'-'+args.arch+'.json')).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n',encoding='utf-8');print(json.dumps({'kind':args.kind,'arch':args.arch,'rows':value['rows'],'passed':True}))
