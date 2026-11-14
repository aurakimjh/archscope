from pathlib import Path
import json,csv,zipfile,xml.etree.ElementTree as ET,hashlib
p=Path('.tmp/windows-final-20261114/workspace-exports')
j=json.loads((p/'access-native.json').read_text(encoding='utf-8'))
assert j['summary']['total_requests']==120 and j['summary']['avg_response_ms']==315.5
h=(p/'access-native.html').read_text(encoding='utf-8')
assert '315.5' in h and '1500' in h and '120' in h
c=(p/'access-native.csv').read_text(encoding='utf-8-sig');assert 'api_count,120' in c and 'api_p95_response_ms,1500' in c
files=list((p/'access-tables').glob('*.csv'));assert len(files)==18
r=list(csv.DictReader((p/'access-tables/series_requests_per_minute.csv').open()));assert r[0]['value']=='120'
r=list(csv.DictReader((p/'access-tables/series_p95_response_time_per_minute.csv').open()));assert r[0]['value']=='1500' and r[0]['sample_count']=='120'
with zipfile.ZipFile(p/'access-native.pptx') as z:
    assert z.testzip() is None
    slides=[n for n in z.namelist() if n.startswith('ppt/slides/slide') and n.endswith('.xml')]
    texts=[' '.join(ET.fromstring(z.read(n)).itertext()) for n in slides]
    assert any('120' in s for s in texts)
    metric_lines=[t.text for t in ET.fromstring(z.read('ppt/slides/slide2.xml')).iter('{http://schemas.openxmlformats.org/drawingml/2006/main}t') if t.text and t.text.split(': ',1)[0] in j['summary']]
    assert len(metric_lines)==10
    for line in metric_lines:
        key,value=line.split(': ',1);expected=j['summary'][key]
        if isinstance(expected,(int,float)): assert float(value)==expected
        elif expected is None: assert value=='None'
        else: assert value==str(expected)
receipt={'scope':'One owned nativeAccess result; all5ExportCenter modes. Package/readback, not allOffice rendering.','summary':{'requests':120,'avg_ms':315.5,'p95_ms':1500},'csvFiles':len(files),'pptxSlides':len(slides),'pptxSummaryLimit':10,'pptxDisplayedMetrics':metric_lines,'files':[{'name':str(f.relative_to(p)).replace(chr(92),'/'),'bytes':f.stat().st_size,'sha256':hashlib.sha256(f.read_bytes()).hexdigest()} for f in sorted(p.rglob('*')) if f.is_file()]}
Path('docs/review/evidence/2026-11-14-windows-final/workspace-export-readback.json').write_text(json.dumps(receipt,indent=2)+'\n',encoding='utf-8')
print(json.dumps({'result':'PASS','csvFiles':len(files),'pptxSlides':len(slides),'pptxSummaryLimit':10,'pptxDisplayedMetrics':metric_lines}))
