import json,subprocess,pathlib,hashlib
root=pathlib.Path(__file__).resolve().parents[4];fixtures=root/'apps/engine-native/internal/catalog/contracts';out=pathlib.Path('/tmp/archscope-t641-cli-qa');out.mkdir(exist_ok=True);engine='/tmp/archscope-t641-engine';reports=[]
def run(args,dest=None,fail=False):
 p=subprocess.run([engine,*args],capture_output=True,text=True);reports.append({'args':args,'exit':p.returncode,'stderr':p.stderr[:1500]})
 if fail:
  assert p.returncode!=0,(args,p.stdout);return
 assert p.returncode==0,(args,p.stderr)
 if dest:(out/dest).write_text(p.stdout)
 return json.loads(p.stdout)
for name,raw,opts in [('python','python-3.14.4.json','python-options.json'),('node','node-25.9.0.json','node-options.json'),('rust','rust-1.94.1-backtrace.txt','rust-options.json'),('thread','jstack.txt','thread-options.json'),('cpu','ps-l.csv','cpu-options.json'),('plan','postgres-actual.json','postgres-options.json')]:
 args=['--in',str(fixtures/('runtime-evidence-'+raw)),'--options-file',str(fixtures/('runtime-evidence-'+opts))]
 run(['runtime','inspect-source',*args],name+'-inventory.json');run(['runtime','normalize-source',*args],name+'-model.json')
run(['runtime','merge-models','--inputs',','.join(str(out/(n+'-model.json')) for n in ['python','node','rust','thread','cpu','plan'])],'merged.json')
result=run(['runtime','import-source','--in',str(fixtures/'runtime-evidence-set.json')],'result.json');source=out/'result.json';inventory=run(['runtime','inspect-source','--in',str(fixtures/'runtime-evidence-set.json')],'inventory.json')
queries={};pages={}
for kind in ['observations','allocation','task_snapshot','thread_snapshot','cpu','gc','event_loop','handle_snapshot','panic_snapshot','entities','processes','sources','frames','stacks','symbol_evidence','relations','profile_refs','plan_nodes']:
 q={'kind':kind,'limit':25};page=run(['runtime','query','--in',str(source),'--query',json.dumps(q)],'page-'+kind+'.json');pages[kind]=page
cpu=pages['cpu']['rows'][0]['id'];thread=next(v['id'] for v in pages['thread_snapshot']['rows'] if v['observed_state']=='RUNNABLE');q={'kind':'cpu_link','limit':25,'cpu_observation_id':cpu,'stack_observation_id':thread};pages['cpu_link']=run(['runtime','query','--in',str(source),'--query',json.dumps(q)],'page-cpu_link.json');assert pages['cpu_link']['rows'][0]['status']=='qualified_context'
alloc=next(v for v in pages['allocation']['rows'] if v['metric']=='live_bytes');run(['runtime','allocation-profile','--in',str(source),'--entity-id',alloc['entity_id'],'--source-id',alloc['ref']['source_id'],'--metric','live_bytes'],'allocation-result.json')
for metric in ['live_bytes','live_blocks']:run(['runtime','allocation-profile','--in',str(source),'--entity-id',alloc['entity_id'],'--source-id',alloc['ref']['source_id'],'--metric',metric])
run(['runtime','query','--in',str(source),'--query','{"kind":"cpu_link","limit":1}'],fail=True)
run(['runtime','import-source','--in',str(fixtures/'runtime-evidence-ps-l.csv')],fail=True)
q={'kind':'observations','limit':1};page=run(['runtime','query','--in',str(source),'--query',json.dumps(q)]);q['cursor']=page['next'];run(['runtime','query','--in',str(source),'--query',json.dumps(q)])
inventory['result_sha256']=pages['observations']['result_sha256']
(out/'ui-data.json').write_text(json.dumps({'inventory':inventory,'pages':pages,'allocation':alloc}))
(out/'summary.json').write_text(json.dumps({'operations':reports,'scope':'actual local CLI; synthetic/sanitized fixtures'},indent=2))
print(len(reports),'CLI operations passed, including explicit refusals')
