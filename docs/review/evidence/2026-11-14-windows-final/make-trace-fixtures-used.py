from pathlib import Path
import json,copy
base=json.loads(Path('examples/traces/sample-jaeger-query.json').read_text(encoding='utf-8'))
out=Path('.tmp/windows-final-20261114/ui-fixtures')
traces=[]
for i in range(12):
    trace=copy.deepcopy(base['data'][0]); trace['traceID']=f'native-trace-{i:02d}'
    for span in trace['spans']:
        span['traceID']=trace['traceID'];span['startTime']+=i*1_000_000
        for ref in span.get('references',[]):ref['traceID']=trace['traceID']
    traces.append(trace)
(out/'trace-twelve.json').write_text(json.dumps({'data':traces,'total':12}),encoding='utf-8')
orphan=copy.deepcopy(base)
orphan['data'][0]['spans'][1]['references'][0]['spanID']='absent-parent'
(out/'trace-orphan.json').write_text(json.dumps(orphan),encoding='utf-8')
print('Jaeger12traces/36spans,240ms each; separate3span orphan-parent input')
