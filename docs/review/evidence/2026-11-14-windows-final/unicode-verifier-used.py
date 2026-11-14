import copy,hashlib,json,sys
from pathlib import Path
base=Path('.tmp/windows-final-20261114')
source=base/'import-amd64/synthetic-32.httpsession'
saved=base/'unicode-notes.httpsession'
a=json.loads(source.read_text(encoding='utf-8'));b=json.loads(saved.read_text(encoding='utf-8'))
assert len(a['transactions'])==len(b['transactions'])==32
expected=copy.deepcopy(a['transactions']);expected[0]['comment']='주석 조합 경계: 가😀 e\u0301'
sys.path.insert(0,str(Path('scripts/acceptance/layout').resolve()))
from verify_p2_windows_readback import compare_transactions
comparison=compare_transactions(a['transactions'],b['transactions'],{expected[0]['id']:{'starred':True,'comment':expected[0]['comment']}},b['selectedId'])
assert b['selectedId']==a['selectedId']
assert len(expected[0]['comment'].encode())==33
report={'passed':True,'rows':32,'selectedIdMatches':True,'comparison':comparison,'commentUTF8Bytes':33,
 'sourceSha256':hashlib.sha256(source.read_bytes()).hexdigest(),'savedSha256':hashlib.sha256(saved.read_bytes()).hexdigest()}
(Path('docs/review/evidence/2026-11-14-windows-final')/'unicode-readback.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
print(report)
