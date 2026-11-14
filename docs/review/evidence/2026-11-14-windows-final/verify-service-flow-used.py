import json, hashlib
from pathlib import Path
out=Path('docs/review/evidence/2026-11-14-windows-final')
p=Path('C:/Users/aurak/Downloads/archscope-service-flow-2026-11-14T17-15-17-024Z.json')
m=Path('C:/Users/aurak/Downloads/archscope-service-flow-2026-11-14T17-16-15-091Z.mmd')
r=json.loads(p.read_text(encoding='utf-8-sig'))
assert r['type']=='service_flow' and r['summary']['edge_count']==1 and r['summary']['total_call_count']==12
edge=r['tables']['edges'][0]
assert edge['caller']=='checkout-service' and edge['callee']=='inventory-service'
assert edge['call_count']==12 and edge['avg_latency_ms']==90 and edge['max_latency_ms']==90 and edge['error_count']==0
assert len(r['source_files'])==1 and r['source_files'][0].endswith('trace-twelve.json')
s=m.read_text(encoding='utf-8-sig')
assert 'sequenceDiagram' in s and 'checkout-service' in s and 'inventory-service' in s and '12 calls, avg 90 ms' in s
files=[]
for src,name in [(p,'service-flow-export.json'),(m,'service-flow-export.mmd')]:
    b=src.read_bytes(); (out/name).write_bytes(b); files.append({'file':name,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()})
(out/'service-flow-readback.json').write_text(json.dumps({'passed':True,'buildCommit':'e50b0f48-plus-chart-csv','edge':edge,'files':files},ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
print('PASS: native Service Flow JSON and Mermaid numeric/provenance readback')
