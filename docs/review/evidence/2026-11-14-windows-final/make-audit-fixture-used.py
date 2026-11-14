from pathlib import Path
import json,copy
p=json.loads(Path('apps/engine-native/internal/parsers/lighthouse/testdata/populated_report.json').read_text(encoding='utf-8'))
p['categories']['performance']['score']=None
p['audits']['largest-contentful-paint']={'id':'largest-contentful-paint','title':'Largest Contentful Paint','score':None,'scoreDisplayMode':'error','errorMessage':'Synthetic interrupted LCP audit'}
p['runtimeError']={'code':'SYNTHETIC_INTERRUPTION','message':'Owned partial recording fixture'}
Path('.tmp/windows-final-20261114/ui-fixtures/lighthouse-partial.json').write_text(json.dumps(p),encoding='utf-8')
