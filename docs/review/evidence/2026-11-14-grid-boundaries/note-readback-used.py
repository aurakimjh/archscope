import sys,json,hashlib
from pathlib import Path
root=Path('C:/workspace/archscope');run=root/'.tmp/grid-boundaries-20261114'/sys.argv[1]
source=json.loads((run/'.private-session.json').read_text(encoding='utf-8'))
notes=json.loads((Path(source['storePath'])/'annotations.json').read_text(encoding='utf-8'))
rows=json.loads((run/'.private-rows.json').read_text(encoding='utf-8'));ident=next(r['transaction']['id'] for r in rows if r['transaction']['sequence']==5)
expect={'ascii':'a'*4096,'hangul':'가'*1365+'a','emoji':'😀'*1024,'redacted':'password=[REDACTED]','retry':'retry-boundary','empty':''}[sys.argv[2]]
assert notes[ident]['comment']==expect
public=root/'docs/review/evidence/2026-11-14-grid-boundaries/note-readback.json'
values=json.loads(public.read_text(encoding='utf-8')) if public.exists() else []
values.append({'run':sys.argv[1],'checkpoint':sys.argv[3],'expectedCase':sys.argv[2],'utf8Bytes':len(expect.encode()),'commentSha256':hashlib.sha256(expect.encode()).hexdigest(),'starred':notes[ident]['starred'],'annotationCount':len(notes),'exactMatch':True})
public.write_text(json.dumps(values,ensure_ascii=False,indent=2)+'\n',encoding='utf-8');print('PASS: '+sys.argv[3])
