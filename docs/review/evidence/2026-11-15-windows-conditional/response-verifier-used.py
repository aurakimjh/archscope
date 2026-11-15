from pathlib import Path
import json,hashlib
p=Path('.tmp/windows-conditional-20261115/response-matrix.httpsession')
d=json.loads(p.read_text());r={t['path']:t for t in d['transactions']}
a=r['/gzip'];assert a['state']=='complete' and a['response']['bodySize']==46 and a['response']['bodyDecoded']==26 and json.loads(a['response']['bodyPreview'])=={'marker':'gzip-boundary'}
a=r['/encoded'];assert a['response']['bodyStorage']=='encoded' and a['response']['bodyDecoded']==-1 and not a['response'].get('bodyPreview')
a=r['/short'];assert a['state']=='failed' and a['error']=='unexpected EOF' and a['response']['bodySize']==12 and a['response']['bodyPreview']=='short'
a=r['/large'];assert a['state']=='complete' and a['response']['bodyStorage']=='truncated' and a['response']['bodySize']==1048634 and a['response']['bodyPreview'].encode()==b'builder-large-prefix:'+b'x'*(1048576-21)
a=r['/error'];assert a['statusCode']==503 and a['state']=='complete' and json.loads(a['response']['bodyPreview'])=={'marker':'deliberate-503'}
a=r['/delay'];assert a['totalMs']>=1900 and a['state']=='complete'
receipt={'passed':True,'execution':'Actual Windows386 Go requestbuilder transport against owned Docker; not all requests sent by UI','sourceCommit':'f51e8fe3','sessionSha256':hashlib.sha256(p.read_bytes()).hexdigest(),'cases':[{k:t[k] for k in ['path','statusCode','state','totalMs']}|{'error':t.get('error'),'response':{k:v for k,v in t['response'].items() if k!='bodyPreview'},'retainedPreviewBytes':len(t['response'].get('bodyPreview','').encode())} for t in d['transactions']]}
Path('docs/review/evidence/2026-11-15-windows-conditional/response-matrix-readback.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')
print('PASS: six independent response-boundary readbacks')
