from pathlib import Path
import json
out=Path('.tmp/windows-final-20261114/ui-fixtures')
p={'startTime':0,'endTime':12000,'nodes':[{'id':1,'callFrame':{'functionName':'(root)'},'children':[2,3]},{'id':2,'callFrame':{'functionName':'renderList','url':'https://example.test/app.js','lineNumber':41}},{'id':3,'callFrame':{'functionName':'layout','url':'https://example.test/layout.js','lineNumber':7}}],'samples':[2,3,2,3,2,3,2,3,2,3,2,3],'timeDeltas':[1000]*12}
(out/'cpu-twelve.cpuprofile').write_text(json.dumps(p),encoding='utf-8')
