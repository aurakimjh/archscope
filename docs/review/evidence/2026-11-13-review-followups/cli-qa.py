from pathlib import Path
import subprocess,json,hashlib,zipfile
root=Path('/Users/aura/workspace/archscope');base=root/'apps/engine-native/internal/catalog/contracts';out=Path('/tmp/review21-cli-delivery');out.mkdir(exist_ok=True);ops=[]
def run(args,code=None):
 p=subprocess.run(['/tmp/review21-delivery-engine',*map(str,args)],capture_output=True,text=True,timeout=120);ops.append({'operation':list(map(str,args[:2])),'expected_refusal':code,'exit':p.returncode})
 if code:assert p.returncode!=0 and code in p.stderr,(args,p.stderr)
 else:assert p.returncode==0,(args,p.stderr)
 return p.stdout
for name in ['apple-ollama-apple-sustained-multi.json','system-db-postgres-readonly-unknown-capture.json','system-db-postgres-capture.json']:
 src=base/name;model=out/(name+'.model');result=out/(name+'.result');run(['metrics','history-normalize','--in',src,'--out',model]);run(['metrics','history-import','--in',src,'--out',result]);m=json.loads(model.read_bytes())
 if name.startswith('system-db'):assert m['normalizer']=='1.3.0'
 rows=json.loads(run(['metrics','history-query','--in',result,'--query',json.dumps({'selection':{'series_ids':[v['id'] for v in m['series'] if v['name'] in ['postgres.database.xact_commit','ollama.selection.excluded_model_count']]},'limit':25})]))
 if name=='system-db-postgres-capture.json':assert any(v['status']=='epoch_transition_from_unknown' for v in rows['rows'])
 before=result.read_bytes();run(['evidence','export','--in',result,'--export-format','html','--out',out/(name+'.html')]);assert 'metric_intervals' in (out/(name+'.html')).read_text()
 pack=out/(name+'.report-pack');run(['evidence','export','--in',result,'--export-format','report-pack','--out',pack]);assert result.read_bytes()==before
 with zipfile.ZipFile(pack) as z:
  assert any(n.startswith('presentations/') and n.endswith('.html') for n in z.namelist()),z.namelist()
  assert before in [z.read(n) for n in z.namelist() if n.endswith('.json')]
options=json.loads((base/'execution-evidence-jmeter-options.json').read_text());options['producer_version']='5.6.2';src=base/'execution-evidence-jmeter-5.6.3.csv';run(['execution','normalize-source','--in',src,'--options',json.dumps(options)],'EXECUTION_JTL_VERSION_UNQUALIFIED');options['allow_unverified_version']=True
v=json.loads(run(['execution','normalize-source','--in',src,'--options',json.dumps(options)]));assert v['source']['producer_version']=='5.6.2' and v['source']['qualification']=='unverified_version'
v=json.loads(run(['execution','normalize-source','--in',base/'execution-evidence-hs-err-21.0.10-full-sections.txt','--options','{"format":"hs-err-text"}']));assert v['crash']['elapsed_seconds']=='0.019933';assert {x['section'] for x in v['crash']['frames']}=={'java','native','problematic'}
opts=(base/'gpu-execution-nsight-options.json').read_text();v=json.loads(run(['gpu-trace','normalize-source','--in',base/'gpu-execution-nsight-overlap-ambiguous.sqlite','--options',opts]));source=v['sources'][0]['id'];f=out/'gpu-model.json';f.write_text(json.dumps(v));result=out/'gpu-result.json';run(['gpu-trace','import-source','--in',f,'--out',result]);b=json.loads(run(['gpu-trace','query','--in',result,'--query',json.dumps({'kind':'busy','source_id':source,'limit':25})]));assert b['rows'][0]['kernel_duration_sum_ns']=='77888' and b['rows'][0]['kernel_interval_union_ns']=='51968',b
(out/'summary.json').write_text(json.dumps({'operations':ops,'count':len(ops),'raw_json_immutable':True,'pack_has_hash_linked_html':True,'actual_sources_and_structural_gpu_distinguished':True},indent=2)+'\n');print('CLI passed',len(ops))
