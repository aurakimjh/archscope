import hashlib,json,sys
from pathlib import Path
root=Path('C:/workspace/archscope');sys.path.insert(0,str(root/'scripts/acceptance/layout'))
from verify_p2_windows_readback import compare_transactions,require,read
public=root/'docs/review/evidence/2026-11-14-t613-failures'
for arch in ['amd64','386']:
    run=(root/f'.tmp/t613-memory-20261114/fault-{arch}-fixed').resolve()
    report=read(run/'result.json');require(report['passed'],'probe incomplete')
    store=Path(read(run/'.private-session.json')['storePath']).resolve()
    require(store.is_relative_to(run/'.archscope-captures'),'unowned store')
    raw=(run/'retry.httpsession').read_bytes();doc=json.loads(raw)
    source=[read(p)['transaction'] for p in sorted((store/'rolling').glob('*.json'))]
    annotation_path=store/'annotations.json';annotations=read(annotation_path) if annotation_path.exists() else {}
    checked=compare_transactions(source,doc['transactions'],annotations,doc['selectedId'])
    require(checked['rows']==6 and report['exportSha256']==hashlib.sha256(raw).hexdigest(),'retry export mismatch')
    require(doc['selectedId']==doc['transactions'][0]['id'],'selection mismatch')
    checked.update(schemaVersion=2,arch=arch,passed=True,scope='Restored committed rows and successful export after existing-directory refusal; all fields checked under declared policy',fileSha256=hashlib.sha256(raw).hexdigest(),fileBytes=len(raw),selectedSequence=doc['transactions'][0]['sequence'])
    with (public/f'export-{arch}.json').open('x',encoding='utf-8',newline='\n') as file:file.write(json.dumps(checked,ensure_ascii=False,indent=2)+'\n')
    print({'arch':arch,'rows':6,'allFieldsAndAnnotationsMatch':True})
