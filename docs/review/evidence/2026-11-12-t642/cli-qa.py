import pathlib,subprocess,json,hashlib,decimal
root=pathlib.Path(__file__).resolve().parents[4];f=root/'apps/engine-native/internal/catalog/contracts';out=pathlib.Path('/tmp/archscope-t642-cli-qa');out.mkdir(exist_ok=True);[p.unlink() for p in out.iterdir() if p.is_file() and (p.name.startswith('export.') or p.name=='visual-spec.json')];engine='/tmp/archscope-t642-engine';reports=[]
def run(args,dest=None,fail=False):
 p=subprocess.run([engine,*args],text=True,capture_output=True);reports.append({'args':args,'exit':p.returncode,'stderr':p.stderr[:2000],'stdout_sha256':hashlib.sha256(p.stdout.encode()).hexdigest()})
 if fail:assert p.returncode!=0,(args,p.stdout);return
 assert p.returncode==0,(args,p.stderr)
 if dest:(out/dest).write_text(p.stdout)

 if not p.stdout.strip():return None
 try:return json.loads(p.stdout)
 except json.JSONDecodeError:return p.stdout
for name,raw in [('k6','k6-1.3.0.json'),('llmperf','llmperf-summary.json'),('jmeter','jmeter-5.6.3.csv'),('crash','hs-err-21.0.10.txt')]:
 opts=str(f/('execution-evidence-'+name+'-options.json'));source=str(f/('execution-evidence-'+raw));args=['--in',source,'--options-file',opts]
 run(['execution','inspect-source',*args],name+'-inventory.json');run(['execution','normalize-source',*args],name+'-model.json');run(['execution','import-source','--in',str(out/(name+'-model.json'))],name+'-result.json')
 for kind in (['metrics','contexts','sources'] if name!='crash' else ['crash','crash_frames']):run(['execution','query','--in',str(out/(name+'-result.json')),'--query',json.dumps({'kind':kind,'limit':25})],name+'-page-'+kind+'.json')
run(['execution','query','--in',str(out/'jmeter-result.json'),'--query','{"kind":"samples","limit":1}'],'jtl-samples.json')
# Deliberately synthetic latency-only mutation of the measured export, not a second measured run.
raw=json.loads((f/'execution-evidence-k6-1.3.0.json').read_text(),parse_float=decimal.Decimal)
for key,value in raw['metrics']['http_req_duration']['values'].items():raw['metrics']['http_req_duration']['values'][key]=float(decimal.Decimal(str(value))*2)
# Other measured float fields retain their original JSON numeric values for this test.
raw=json.loads(json.dumps(raw,default=float));(out/'target-raw.json').write_text(json.dumps(raw))
run(['execution','import-source','--in',str(out/'target-raw.json'),'--options-file',str(f/'execution-evidence-k6-options.json')],'target-result.json')
comparison=run(['execution','compare','--in',str(out/'k6-result.json'),'--target',str(out/'target-result.json'),'--query','{"limit":25}'],'comparison.json');assert comparison['status']=='compatible_declarations' and any(v['name']=='request_latency' and v['difference']!='0' for v in comparison['rows'])
model=json.loads((out/'target-model.json').read_text()) if (out/'target-model.json').exists() else json.loads((out/'k6-model.json').read_text());model['context']['client_environment_sha256']='unknown';(out/'unknown-model.json').write_text(json.dumps(model));run(['execution','import-source','--in',str(out/'unknown-model.json')],'unknown-result.json');unknown=run(['execution','compare','--in',str(out/'k6-result.json'),'--target',str(out/'unknown-result.json')],'incompatible.json');assert unknown['status']=='incompatible' and unknown['rows']==[]
crash=run(['execution','compare','--in',str(out/'k6-result.json'),'--target',str(out/'crash-result.json')]);assert crash['status']=='incompatible'
first=run(['execution','query','--in',str(out/'llmperf-result.json'),'--query','{"kind":"metrics","limit":1}']);run(['execution','query','--in',str(out/'llmperf-result.json'),'--query',json.dumps({'kind':'metrics','limit':1,'cursor':first['next']})]);run(['execution','query','--in',str(out/'llmperf-result.json'),'--query',json.dumps({'kind':'metrics','limit':1,'metric_name':'ttft','cursor':first['next']})],fail=True)
for fmt in ['json','csv','html','report-pack']:
 path=out/('export.'+('zip' if fmt=='report-pack' else fmt));p=subprocess.run([engine,'evidence','export','--in',str(out/'k6-result.json'),'--export-format',fmt,'--out',str(path)],capture_output=True,text=True);assert p.returncode==0,p.stderr;reports.append({'operation':'common export '+fmt,'exit':p.returncode});assert b'SYNTHETIC_BODY_NOT_RETAINED' not in path.read_bytes()
run(['visual','create-spec','--in',str(out/'k6-result.json'),'--query','{"kind":"provider_table","provider":"execution:metrics","columns":["name","statistic","value","unit","population","statistical_policy"],"offset":0,"limit":5}','--out',str(out/'visual-spec.json')]);run(['visual','validate','--in',str(out/'visual-spec.json')])
# Actual raw faulthandler and JDK inputs independently exercise R18 fixes.
context={'producer':'CPython','producer_version':'3.14.4','format':'thread-dump','thread_format':'python_faulthandler','clock_id':'unknown','process':{'id':'synthetic-python','host_id':'synthetic-host','boot_id':'synthetic-boot','pid_namespace':'synthetic-ns','pid':'500','process_start_ns':None,'runtime':'Python'}}
fh=run(['runtime','import-source','--in',str(f/'runtime-evidence-faulthandler-3.14.4.txt'),'--options',json.dumps(context)],'r18-python.json');assert fh['summary']['thread_snapshot_observations']=='3'
a=str(f/'runtime-evidence-jdk21-blocked-1.txt');b=str(f/'runtime-evidence-jdk21-blocked-2.txt');args=['thread-dump','analyze-multi','--in',a,'--in',b,'--threshold','2'];unqualified=run(args,'r18-unqualified.json');assert unqualified['metadata']['diagnostics']['warning_count']>0
qualified=run([*args,'--correlation-context-file',str(f/'runtime-evidence-jdk21-correlation-context.json')],'r18-qualified.json');assert qualified['summary']['persistent_blocked_threads']==1
run(['execution','import-source','--in',str(f/'execution-evidence-k6-1.3.0.json'),'--options','{"format":"k6-handle-summary-json","producer_version":"1.3.0"}'],fail=True)
run(['execution','query','--in',str(out/'k6-result.json'),'--query','{"kind":"contexts","metric_name":"requests","limit":1}'],fail=True)
# Actual CLI pages/DTOs used by the separate React/mock-IPC browser harness.
inv=json.loads((out/'k6-inventory.json').read_text());page=json.loads((out/'k6-page-metrics.json').read_text());inv['result_sha256']=page['result_sha256'];target_sha=hashlib.sha256((out/'target-result.json').read_bytes()).hexdigest();unknown_sha=hashlib.sha256((out/'unknown-result.json').read_bytes()).hexdigest();target_inv=dict(inv,result_sha256=target_sha);unknown_inv=dict(inv,result_sha256=unknown_sha)
(out/'ui-data.json').write_text(json.dumps({'inventory':inv,'target_inventory':target_inv,'unknown_inventory':unknown_inv,'page':page,'comparison':comparison,'incompatible':unknown}))
(out/'summary.json').write_text(json.dumps({'scope':'actual local CLI; pinned/sanitized inputs; target latency mutation is synthetic','operations':reports,'count':len(reports)},indent=2)+'\n');print(len(reports),'CLI operations passed including expected refusals; R18 actual producer regressions passed')
