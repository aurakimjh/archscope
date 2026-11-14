from pathlib import Path
import zipfile,xml.etree.ElementTree as E,json,hashlib
p=Path('.tmp/windows-final-20261114/workspace-exports')
a=(p/'access-stable-01.pptx').read_bytes();b=(p/'access-stable-02.pptx').read_bytes();assert a==b
j=json.loads((p/'access-native.json').read_text(encoding='utf-8'))
with zipfile.ZipFile(p/'access-stable-01.pptx') as z:
 lines=[v.text for v in E.fromstring(z.read('ppt/slides/slide2.xml')).iter('{http://schemas.openxmlformats.org/drawingml/2006/main}t')][1:]
 keys=[line.split(': ',1)[0] for line in lines[:10]]
 assert keys==sorted(j['summary'])[:10]
 assert lines[-1].startswith(str(len(j['summary'])-10)+' additional metrics omitted')
r={'build':'519051ef-plus-report-diff','result':'PASS','identicalRepeatedBytes':True,'bytes':len(a),'sha256':hashlib.sha256(a).hexdigest(),'summaryKeys':keys,'notice':lines[-1],'scope':'NativeExportCenter two outputs; sorted subset and omission, not Office rendering'}
Path('docs/review/evidence/2026-11-14-windows-final/pptx-stable-native.json').write_text(json.dumps(r,indent=2)+'\n',encoding='utf-8')
print(json.dumps(r))
