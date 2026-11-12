#!/usr/bin/env python3
"""Build/check deterministic release resources using only this checkout."""
import hashlib, io, json, pathlib, sys, zipfile
root=pathlib.Path(__file__).resolve().parents[1]
app=root/'apps/engine-native'; dest=app/'internal/distribution/assets'
outputs={}
for lang in ('en','ko'):
 for name in ('OFFLINE_PACKS','INPUT_ASSISTANCE','SYSTEM_EVIDENCE','INFERENCE_EVIDENCE','VISUAL_ARTIFACTS','METRIC_HISTORY'):
  outputs[f'docs/{lang}/{name}.md']=(root/f'docs/{lang}/{name}.md').read_bytes()
files={}
for p in sorted((app/'internal/catalog/contracts').iterdir()):
 if p.is_file():files['catalog/'+p.name]=p.read_bytes()
for p in sorted((app/'internal/packs/assets').iterdir()):
 if p.is_file():files['packs/'+p.name]=p.read_bytes()
# Each *registered* canonical importer has a standalone synthetic case. Legacy
# facades outside this registry retain their independent regression suites.
cases=[{'importer':'server.tomcat','input':'catalog/server.log','options':{},'summary':{'total_events':'3'}}, {'importer':'profile.set-json','input':'catalog/profile-set.json','options':{'profile_ids':['worker-a','worker-b'],'metric_id':'duration'},'summary':{}}, {'importer':'data.synthetic-metrics','input':'packs/fixture-input.jsonl','options':{},'summary':{'total_samples':'2'}}]
cases.append({'importer':'profile.source-set-json','input':'catalog/profile-source-set.json','options':{'profile_ids':['profile-0','profile-1'],'metric_id':'sampled-seconds'},'summary':{}})
metric_model=json.loads(files['catalog/metric-history-set.json'])
cases.append({'importer':'metrics.history-json','input':'catalog/metric-history-set.json','options':{'series_ids':[metric_model['series'][0]['id']]},'summary':{'point_count':'2'}})
cases.append({'importer':'system.evidence-json','input':'catalog/system-evidence-set.json','options':{},'summary':{'sample_records':'1'}})
cases.append({'importer':'inference.evidence-json','input':'catalog/inference-evidence-set.json','options':{},'summary':{'entity_count':'6','signal_count':'24'}})
# Keep names aligned with the signed bootstrap pack's human-readable fixtures.
for c in cases:
 if c['importer']=='data.synthetic-metrics':
  candidates=[n for n in files if n.startswith('packs/') and n.endswith('input.jsonl')]
  assert len(candidates)==1,candidates
  c['input']=candidates[0]
manifest={'version':'1.0.0','synthetic':True,'files':{n:hashlib.sha256(b).hexdigest() for n,b in sorted(files.items())},'cases':cases}
files['manifest.json']=(json.dumps(manifest,ensure_ascii=False,sort_keys=True,indent=2)+'\n').encode()
buf=io.BytesIO()
with zipfile.ZipFile(buf,'w',compression=zipfile.ZIP_STORED) as z:
 for name,b in sorted(files.items()):
  i=zipfile.ZipInfo(name,date_time=(2026,11,11,0,0,0));i.create_system=3;i.compress_type=zipfile.ZIP_STORED;i.external_attr=0o100644<<16;z.writestr(i,b)
outputs['fixtures.zip']=buf.getvalue()
outputs['fixtures.sha256']=(hashlib.sha256(buf.getvalue()).hexdigest()+'\n').encode()
for n,b in outputs.items():
 p=dest/n
 if '--check' in sys.argv:
  if not p.exists() or p.read_bytes()!=b:raise SystemExit('stale/missing offline resource: '+str(p))
 else:p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(b)
notices=json.loads((dest/'licenses/manifest.json').read_text())
if notices['go_sum_sha256']!=hashlib.sha256((app/'go.sum').read_bytes()).hexdigest() or notices['npm_lock_sha256']!=hashlib.sha256((app/'cmd/archscope-app/frontend/package-lock.json').read_bytes()).hexdigest():
 raise SystemExit('stale dependency notices: run scripts/build-offline-notices.py')
print('offline resources verified' if '--check' in sys.argv else 'offline resources built')
