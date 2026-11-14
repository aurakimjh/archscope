import hashlib, json, sys
from pathlib import Path
from urllib.parse import urlsplit
ROOT=Path('C:/workspace/archscope')
sys.path.insert(0,str(ROOT/'scripts/acceptance/layout'))
from verify_p2_windows_readback import compare_transactions,require,read

for arch in ['amd64','386']:
    run=(ROOT/f'.tmp/live-grid-20261114/{arch}-fixed').resolve()
    store=Path(read(run/'.private-session.json')['storePath']).resolve()
    require(store.is_relative_to(run/'.archscope-captures'),'store escaped isolated run')
    source=[read(p)['transaction'] for p in sorted((store/'rolling').glob('*.json'))]
    file=run/'live-after.httpsession';doc=read(file)
    annotations=read(store/'annotations.json')
    result=compare_transactions(source,doc['transactions'],annotations,doc['selectedId'])
    require(result['rows']==640,'unexpected retained row count')
    notes=[tx for tx in doc['transactions'] if tx.get('starred') or tx.get('comment')]
    require(len(notes)==1 and notes[0]['id']==doc['selectedId'],'expected selected retained annotation')
    require(notes[0].get('starred') is True and bool(notes[0].get('comment')),'nonempty star/comment required')
    require(all(tx['process']['attribution']=='confirmed' and tx['statusCode']==[200,404,503][int(urlsplit(tx['url']).path.rsplit('/',1)[-1])%3] for tx in doc['transactions']),'unexpected fixture rows')
    result.update(schemaVersion=2,scope='existing Windows live640row private store/export recheck; no new UI action',arch=arch,
        passed=True,nonemptyRetainedAnnotations=1,selectedAnnotationPreserved=True,
        fileSha256=hashlib.sha256(file.read_bytes()).hexdigest(),fileBytes=file.stat().st_size,
        retainedIDsSha256=hashlib.sha256(json.dumps(sorted(tx['id'] for tx in doc['transactions'])).encode()).hexdigest())
    out=ROOT/f'docs/review/evidence/2026-11-14-t613-readback-v2/live640-{arch}.json'
    with out.open('x',encoding='utf-8',newline='\n') as stream:
        stream.write(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'arch':arch,'rows':640,'annotationsMatch':True,'nonemptyRetainedAnnotations':1}))
