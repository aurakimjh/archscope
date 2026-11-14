import json,zipfile,hashlib,html
from pathlib import Path
root=Path('C:/Users/aurak/Downloads')
out=Path('docs/review/evidence/2026-11-14-windows-final')
p=root/'archscope-evidence-pack-2026-11-14T17-18-59-605Z.json'
r=json.loads(p.read_text(encoding='utf-8-sig'))
assert r['card_count']==3 and len(r['cards'])==3
cards={c['source_kind']:c for c in r['cards']}
assert set(cards)=={'service_edge','slo_violation','timeline_event'}
assert cards['service_edge']['payload']['call_count']==12 and cards['service_edge']['payload']['avg_latency_ms']==90
assert cards['slo_violation']['payload']['actual']==33.33 and cards['slo_violation']['payload']['threshold']==10
assert cards['timeline_event']['payload']['payload']['evidence']['cross_service_traces']==12
names=['archscope-evidence-pack-2026-11-14T17-18-59-605Z.json','archscope-evidence-report-2026-11-14T17-19-18-186Z.html','archscope-report-pack-2026-11-14T17-19-34-945Z.html','archscope-report-pack-2026-11-14T17-19-50-457Z.zip']
receipts=[]
for name in names:
    f=root/name;b=f.read_bytes(); extra={}
    if name.endswith('.html'):
        s=b.decode('utf-8-sig')
        assert '<!doctype html>' in s.lower()
        for c in cards.values(): assert html.escape(c['title']) in s
    if name.endswith('.zip'):
        with zipfile.ZipFile(f) as z:
            assert z.testzip() is None
            assert json.loads(z.read('evidence-cards.json'))==r['cards']
            pack=json.loads(z.read('report-pack.json'))
            assert pack['artifacts']['service_flow']['summary']['total_call_count']==12
            extra={'entries':z.namelist()}
    receipts.append({'file':name,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest(),**extra})
(out/'board-three-owned-cards.json').write_bytes(p.read_bytes())
(out/'board-export-readback.json').write_text(json.dumps({'passed':True,'buildCommit':'e50b0f48-plus-chart-csv','scope':'3 own cards;7 preexisting visual artifacts preserved; raw report ZIP private','files':receipts},ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
print('PASS: 3 card payloads, evidence HTML/JSON, Report Pack HTML/ZIP exact card readback')
