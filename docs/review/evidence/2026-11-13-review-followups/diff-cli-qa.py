from pathlib import Path
import subprocess,json
from fractions import Fraction
r=Path('/Users/aura/workspace/archscope');b=r/'apps/engine-native/internal/catalog/contracts';out=Path('/tmp/review21-diff-cli-delivery');out.mkdir(exist_ok=True);ops=[]
def run(args):
 p=subprocess.run(['/tmp/review21-delivery-engine',*map(str,args)],capture_output=True,text=True,timeout=60);assert p.returncode==0,(args,p.stderr);ops.append(args[:2]);return json.loads(p.stdout) if p.stdout.strip().startswith('{') else None
raw=json.loads((b/'profile-set.json').read_text());raw['profiles'][0]['samples'][0]['weight']='0.01';raw['profiles'][1]['samples'][0]['weight']='0.30';model=out/'target-model.json';model.write_text(json.dumps(raw));options=json.dumps({'profile_ids':['worker-a','worker-b'],'metric_id':'duration'})
for stem,path in [('base',b/'profile-set.json'),('target',model)]:run(['evidence','import','--importer','profile.set-json','--in',path,'--options',options,'--out',out/(stem+'.json')])
for order in ['delta_abs','delta','name']:
 q={'kind':'profile_diff','offset':0,'limit':25}
 if order!='delta_abs':q['sort']=order
 v=run(['visual','create-spec','--in',out/'base.json','--target-in',out/'target.json','--query',json.dumps(q)])
 (out/(order+'.json')).write_text(json.dumps(v,indent=2)+'\n')
 assert v['view']['query']['sort']==order and v['view']['query']['omit_zero_delta'] is True,v.keys()
 rows=v['view']['rows'];keys=[]
 for row in rows:
  delta=Fraction(row[3]['text']);assert delta!=0;ref=row[5]['text'];node=ref[ref.index('node-'):] if 'node-' in ref else ref
  keys.append((-abs(delta),row[0]['text'],node) if order=='delta_abs' else (-delta,row[0]['text'],node) if order=='delta' else (row[0]['text'],node))
 assert keys==sorted(keys),(order,keys)
 print(order,'exact rational sort verified',len(rows))
(out/'summary.json').write_text(json.dumps({'operations':ops,'nonzero_comparison':True,'all_three_sorts_and_node_ties_verified_with_exact_rationals':True},indent=2)+'\n')
