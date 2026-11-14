import json,hashlib
from pathlib import Path
from urllib.parse import urlsplit
base=Path('C:/workspace/archscope/.tmp/grid-boundaries-20261114')
public=Path('C:/workspace/archscope/docs/review/evidence/2026-11-14-grid-boundaries')
def read(p):return json.loads(p.read_text(encoding='utf-8-sig'))
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def without_notes(tx):
 out={k:v for k,v in tx.items() if k not in ('starred','comment')}
 parsed=urlsplit(tx['url']);out.update(host=parsed.netloc,query=parsed.query)
 return out
reports=[]
for run,filename,numbers,comment,starred,missing in [
 ('amd64-fixed','before-target-eviction.httpsession',range(1,121),'retry-boundary',True,False),
 ('amd64-fixed','after-target-eviction.httpsession',range(13,141),None,False,True),
 ('386-fixed','boundary-notes.httpsession',range(1,121),'password=[REDACTED]',False,False)]:
 folder=base/run;session=read(folder/'.private-session.json');store=Path(session['storePath']);file=folder/filename;doc=read(file)
 if missing:source=[read(p)['transaction'] for p in (store/'rolling').glob('*.json')]
 else:source=[r['transaction'] for r in read(folder/'.private-rows.json')]
 expected={tx['id']:without_notes(tx) for tx in source}
 actual={tx['id']:without_notes(tx) for tx in doc['transactions']}
 assert expected==actual and len(actual)==len(numbers)
 assert sorted(tx['sequence'] for tx in doc['transactions'])==list(numbers)
 noted=[tx for tx in doc['transactions'] if tx.get('comment') or tx.get('starred')]
 if missing:assert not noted and not doc.get('selectedId')
 else:
  note=next(tx for tx in doc['transactions'] if tx['sequence']==5)
  assert len(noted)==1 and note['comment']==comment and note.get('starred',False)==starred and doc['selectedId']==note['id']
 reports.append({'run':run,'filename':filename,'sha256':sha(file),'bytes':file.stat().st_size,'rows':len(actual),'sequences':[min(numbers),max(numbers)],'allIdsUniqueAndEqual':True,'allOtherFieldsEqualAfterPortableURLNormalization':True,'selectedSequence':None if missing else 5,'annotatedRows':len(noted),'retention':doc['retention'],'passed':True})
for run in ['amd64-fixed','386-fixed']:
 store=Path(read(base/run/'.private-session.json')['storePath']);assert not list(store.glob('.annotations-*'))
(public/'export-readback.json').write_text(json.dumps({'scope':'Independent data readback, not native UI verdict; host/query portable normalization explicitly applied','exports':reports,'stagingFilesAfterRuns':0},ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
print('PASS:3exports,uniqueIDs,selection,annotationandallotherfieldsafterdeclaredURLnormalization')
