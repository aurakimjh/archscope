#!/usr/bin/env python3
"""Observe a prebuilt Linux CLI in a network-none container, including failures.

Prepare the observer image before disconnecting: python:3.12-alpine + apk strace.
This gate covers the CLI process tree, not a desktop WebView or Windows host.
"""
import argparse,json,pathlib,re,subprocess,tempfile
p=argparse.ArgumentParser();p.add_argument('--engine',required=True);p.add_argument('--image',default='archscope-t632-observer');p.add_argument('--out',required=True);a=p.parse_args()
engine=pathlib.Path(a.engine).resolve();out=pathlib.Path(a.out).resolve();out.mkdir(parents=True,exist_ok=False)
program=r'''
import json,os,pathlib,re,socket,subprocess,zipfile
out=pathlib.Path('/evidence');bundle=pathlib.Path('/clean-install')
def run(args):
 r=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
 if r.returncode:raise RuntimeError(str(args)+': '+r.stderr)
 return r.stdout
run(['/input/engine','offline','build','--engine','/input/engine','--out',str(bundle)])
# Only the new bundle and synthetic corpus remain visible to the tested process.
with zipfile.ZipFile(bundle/'resources/fixtures.zip') as z:
 for name in ('packs/input.jsonl','catalog/metric-history-set.json','catalog/metric-history-sdk-python-1.38.0.otlp.json','catalog/system-evidence-perf-script.txt','catalog/system-evidence-perf-capture.json','catalog/inference-evidence-set.json','catalog/inference-evidence-nvidia-set.json','catalog/inference-evidence-nvidia-query.csv','catalog/inference-evidence-nvidia-capture.json','catalog/llm-trace-sdk-python-1.38.0.otlp.json','catalog/llm-trace-neutral-options.json','catalog/runtime-evidence-set.json','catalog/runtime-evidence-python-3.14.4.json','catalog/runtime-evidence-python-options.json','catalog/execution-evidence-k6-1.3.0.json','catalog/execution-evidence-k6-options.json','catalog/execution-evidence-set.json','catalog/execution-evidence-hs-err-21.0.10.txt','catalog/execution-evidence-llmperf-summary.json','catalog/execution-evidence-llmperf-options.json','catalog/execution-evidence-jmeter-5.6.3.csv','catalog/execution-evidence-jmeter-options.json','catalog/apple-ollama-capture.json','catalog/apple-ollama-metric-model.json','catalog/execution-evidence-jmeter-1001.csv','catalog/gpu-execution-set.json','catalog/gpu-execution-nsight-sanitized.sqlite','catalog/gpu-execution-nsight-options.json','catalog/gpu-execution-pytorch-rank-0.json','catalog/gpu-execution-pytorch-rank-0-options.json','catalog/gpu-execution-nsight-summary.csv','catalog/gpu-execution-nsight-summary-options.json','catalog/system-db-postgres-capture.json','catalog/system-db-postgres-model.json','catalog/system-db-linux-ss-capture.json','catalog/system-db-linux-ss-model.json'):z.extract(name,'/sources')
model=json.loads(pathlib.Path('/sources/catalog/metric-history-set.json').read_text())
selection={'series_ids':[model['series'][0]['id']]}
inference=json.loads(pathlib.Path('/sources/catalog/inference-evidence-nvidia-set.json').read_text())
signal=inference['signals'][0]
gpu=json.loads(pathlib.Path('/sources/catalog/gpu-execution-set.json').read_text())
gpu_source=next(v['id'] for v in gpu['sources'] if v['producer']=='nsight-systems')
gpu_query=json.dumps({'kind':'events','source_id':gpu_source,'limit':1})
cli=str(bundle/'archscope-engine');base=[cli,'--offline-root',str(bundle)]
commands=[['offline','fixtures'],['evidence','formats','--pack-store','/state/packs'],['evidence','import','--pack-store','/state/packs','--importer','data.synthetic-metrics','--in','/sources/packs/input.jsonl'],['packs','list','--store','/state/packs'],['evidence','discover','--pack-store','/state/packs','--in','/sources/packs/input.jsonl'],['evidence','collection-kit','--out','/evidence/manual-collection.zip']]
commands.extend([
 ['metrics','history-inspect','--in','/sources/catalog/metric-history-sdk-python-1.38.0.otlp.json'],
 ['metrics','history-normalize','--in','/sources/catalog/metric-history-sdk-python-1.38.0.otlp.json','--out','/evidence/normalized.json'],
 ['evidence','import','--importer','metrics.history-json','--in','/sources/catalog/metric-history-set.json','--options',json.dumps(selection),'--out','/evidence/history-result.json'],
 ['metrics','history-query','--in','/evidence/history-result.json','--query',json.dumps({'selection':selection,'limit':1})],
 ['visual','create-spec','--in','/evidence/history-result.json','--query',json.dumps({'kind':'provider_table','provider':'generic:summary','offset':0,'limit':25}),'--out','/evidence/visual-spec.json'],
 ['visual','validate','--in','/evidence/visual-spec.json'],
 ['system','inspect-source','--in','/sources/catalog/system-evidence-perf-script.txt','--format','perf-script','--capture-file','/sources/catalog/system-evidence-perf-capture.json'],
 ['system','normalize-source','--in','/sources/catalog/system-evidence-perf-script.txt','--format','perf-script','--capture-file','/sources/catalog/system-evidence-perf-capture.json','--out','/evidence/system-model.json'],
 ['evidence','import','--in','/evidence/system-model.json','--importer','system.evidence-json','--options','{}','--out','/evidence/system-result.json'],
 ['system','project-profile','--in','/evidence/system-result.json','--metric-id','period/cpu-clock:HG','--out','/evidence/system-profile.json'],
 ['profile-set','tree','--in','/evidence/system-profile.json'],
 ['inference','inspect-source','--in','/sources/catalog/inference-evidence-nvidia-query.csv','--format','nvidia-smi-csv','--capture-file','/sources/catalog/inference-evidence-nvidia-capture.json'],
 ['inference','normalize-source','--in','/sources/catalog/inference-evidence-nvidia-query.csv','--format','nvidia-smi-csv','--capture-file','/sources/catalog/inference-evidence-nvidia-capture.json','--out','/evidence/inference-model.json'],
 ['evidence','import','--in','/evidence/inference-model.json','--importer','inference.evidence-json','--options','{}','--out','/evidence/inference-result.json'],
 ['inference','query-metric','--in','/evidence/inference-result.json','--signal-id',signal['id']],
 ['inference','open-metrics','--in','/evidence/inference-result.json','--signal-id',signal['id'],'--out','/evidence/inference-metric.json'],
 ['metrics','history-query','--in','/evidence/inference-metric.json','--query',json.dumps({'selection':{'series_ids':[signal['series_id']]},'limit':1})],
 ['inference','import-source','--in','/sources/catalog/inference-evidence-set.json','--out','/evidence/serving-result.json'],

])
commands.extend([
 ['trace','llm-inspect','--in','/sources/catalog/llm-trace-sdk-python-1.38.0.otlp.json','--options-file','/sources/catalog/llm-trace-neutral-options.json'],
 ['trace','llm-normalize','--in','/sources/catalog/llm-trace-sdk-python-1.38.0.otlp.json','--options-file','/sources/catalog/llm-trace-neutral-options.json','--out','/evidence/llm-model.json'],
 ['evidence','import','--in','/evidence/llm-model.json','--importer','trace.llm-json','--options','{}','--out','/evidence/llm-result.json'],
 ['trace','llm-query','--in','/evidence/llm-result.json','--query','{"kind":"attempts","limit":1}'],
])
commands.extend([
 ['runtime','inspect-source','--in','/sources/catalog/runtime-evidence-python-3.14.4.json','--options-file','/sources/catalog/runtime-evidence-python-options.json'],
 ['runtime','normalize-source','--in','/sources/catalog/runtime-evidence-python-3.14.4.json','--options-file','/sources/catalog/runtime-evidence-python-options.json','--out','/evidence/runtime-model.json'],
 ['runtime','import-source','--in','/sources/catalog/runtime-evidence-set.json','--out','/evidence/runtime-result.json'],
 ['runtime','query','--in','/evidence/runtime-result.json','--query','{"kind":"frames","limit":1}'],
])
commands.extend([
 ['execution','inspect-source','--in','/sources/catalog/execution-evidence-k6-1.3.0.json','--options-file','/sources/catalog/execution-evidence-k6-options.json'],
 ['execution','normalize-source','--in','/sources/catalog/execution-evidence-k6-1.3.0.json','--options-file','/sources/catalog/execution-evidence-k6-options.json','--out','/evidence/execution-model.json'],
 ['execution','import-source','--in','/evidence/execution-model.json','--out','/evidence/execution-result.json'],
 ['execution','query','--in','/evidence/execution-result.json','--query','{"kind":"metrics","limit":1}'],
 ['execution','compare','--in','/evidence/execution-result.json','--target','/evidence/execution-result.json','--query','{"limit":1}'],
 ['execution','import-source','--in','/sources/catalog/execution-evidence-hs-err-21.0.10.txt','--options','{"format":"hs-err-text"}','--out','/evidence/crash-result.json'],
 ['execution','query','--in','/evidence/crash-result.json','--query','{"kind":"crash_frames","limit":1}'],
 ['execution','import-source','--in','/sources/catalog/execution-evidence-llmperf-summary.json','--options-file','/sources/catalog/execution-evidence-llmperf-options.json','--out','/evidence/llm-benchmark-result.json'],
 ['execution','query','--in','/evidence/llm-benchmark-result.json','--query','{"kind":"metrics","limit":1}'],
 ['execution','import-source','--in','/sources/catalog/execution-evidence-jmeter-5.6.3.csv','--options-file','/sources/catalog/execution-evidence-jmeter-options.json','--out','/evidence/jtl-result.json'],
 ['execution','query','--in','/evidence/jtl-result.json','--query','{"kind":"samples","limit":1}'],
])
commands.extend([
 ['metrics','history-inspect','--in','/sources/catalog/apple-ollama-capture.json'],
 ['metrics','history-normalize','--in','/sources/catalog/apple-ollama-capture.json','--out','/evidence/apple-model.json'],
 ['metrics','history-import','--in','/sources/catalog/apple-ollama-capture.json','--out','/evidence/apple-result.json'],
 ['execution','import-source','--in','/sources/catalog/execution-evidence-jmeter-1001.csv','--options-file','/sources/catalog/execution-evidence-jmeter-options.json','--out','/evidence/jtl-compact-result.json'],
 ['execution','query','--in','/evidence/jtl-compact-result.json','--query','{"kind":"distributions","limit":1}'],
])
commands.extend([
 ['gpu-trace','inspect-source','--in','/sources/catalog/gpu-execution-pytorch-rank-0.json','--options-file','/sources/catalog/gpu-execution-pytorch-rank-0-options.json'],
 ['gpu-trace','normalize-source','--in','/sources/catalog/gpu-execution-nsight-sanitized.sqlite','--options-file','/sources/catalog/gpu-execution-nsight-options.json','--out','/evidence/gpu-model.json'],
 ['gpu-trace','import-source','--in','/sources/catalog/gpu-execution-set.json','--out','/evidence/gpu-result.json'],
 ['gpu-trace','query','--in','/evidence/gpu-result.json','--query',gpu_query],
 ['gpu-trace','query','--in','/evidence/gpu-result.json','--query','{"kind":"busy","limit":1}'],
 ['gpu-trace','export-svg','--in','/evidence/gpu-result.json','--query',gpu_query,'--out','/evidence/gpu-selected.svg'],
 ['gpu-trace','import-source','--in','/sources/catalog/gpu-execution-nsight-summary.csv','--options-file','/sources/catalog/gpu-execution-nsight-summary-options.json','--out','/evidence/gpu-summary-result.json'],
 ['gpu-trace','query','--in','/evidence/gpu-summary-result.json','--query','{"kind":"summaries","limit":1}'],
])
# Actual manually collected database/socket inputs reuse the metric pipeline.
for stem,name in [('postgres','postgres.database.xact_commit'),('linux-ss','linux.ss.tcp.established')]:
 source='/sources/catalog/system-db-'+stem+'-capture.json'
 model_path='/sources/catalog/system-db-'+stem+'-model.json'
 dbmodel=json.loads(pathlib.Path(model_path).read_text())
 ids=[v['id'] for v in dbmodel['series'] if v['name']==name]
 selected={'series_ids':ids}
 commands.extend([
  ['metrics','history-inspect','--in',source],
  ['metrics','history-normalize','--in',source,'--out','/evidence/'+stem+'-normalized.json'],
  ['metrics','history-import','--in',source,'--out','/evidence/'+stem+'-result.json'],
  ['metrics','history-query','--in','/evidence/'+stem+'-result.json','--query',json.dumps({'selection':selected,'limit':1})],
  ['evidence','import','--in',model_path,'--importer','metrics.history-json','--options',json.dumps(selected),'--out','/evidence/'+stem+'-selected.json'],
  ['evidence','export','--in','/evidence/'+stem+'-result.json','--export-format','csv','--out','/evidence/'+stem+'.csv']
 ])
reports=[]
for i,args in enumerate(commands):
 trace=out/f'operation-{i}.strace';report=out/f'operation-{i}.json'
 result=run(['strace','-f','-qq','-e','trace=network','-o',str(trace)]+base+['--offline-report',str(report)]+args)
 (out/f'operation-{i}.output.json').write_text(result)
 lines=[l for l in trace.read_text().splitlines() if re.search(r'AF_INET6?|sockaddr_in|inet_addr\(|inet_pton\(',l)]
 if lines:raise RuntimeError('network attempt, even if failed: '+str(lines))
 counts=json.loads(report.read_text())['denied_attempts']
 if any(counts.values()):raise RuntimeError('app-owned network attempt: '+str(counts))
 reports.append({'operation':args[:2],'inet_syscalls':len(lines),'app_denied_attempts':counts})
# Explicitly invoked shipped manual helper is separate from automatic CLI intake.
trace=out/'manual-subset.strace'
manual=['strace','-f','-qq','-e','trace=network','-o',str(trace),'python3',str(bundle/'resources/tools/subset-nsight-sqlite.py'),'--in','/sources/catalog/gpu-execution-nsight-sanitized.sqlite','--from-ns','80000000','--to-ns','80500000','--out','/evidence/manual-subset.sqlite']
run(manual)
lines=[l for l in trace.read_text().splitlines() if re.search(r'AF_INET6?|sockaddr_in|inet_addr\(|inet_pton\(',l)]
receipt=json.loads((out/'manual-subset.sqlite.receipt.json').read_text())
if lines or receipt['retained_rows']!=19:raise RuntimeError('manual subset helper scope failed')
reports.append({'operation':['manual-helper','nsight-subset'],'inet_syscalls':0,'app_denied_attempts':{},'manual_only':True})
# Explicit installed-tool conversion remains denied offline before tool execution.
trace=out/'conversion-denied.strace';report=out/'conversion-denied.json'
args=['strace','-f','-qq','-e','trace=network','-o',str(trace)]+base+['--offline-report',str(report),'jfr','convert','--in','/sources/packs/input.jsonl','--cli','/must-not-execute-jfr','--out','/evidence/must-not-publish.json']
refused=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
if refused.returncode==0 or 'OFFLINE_OPERATION_DENIED' not in refused.stderr:raise RuntimeError('offline converter refusal missing: '+refused.stderr)
lines=[l for l in trace.read_text().splitlines() if re.search(r'AF_INET6?|sockaddr_in|inet_addr\(|inet_pton\(',l)]
counts=json.loads(report.read_text())['denied_attempts']
if lines or counts!={'converter.jfr':1} or (out/'must-not-publish.json').exists():raise RuntimeError('offline converter guard failed')
reports.append({'operation':['jfr','convert'],'expected_refusal':'OFFLINE_OPERATION_DENIED','inet_syscalls':0,'app_denied_attempts':counts})
# Observer must detect a failed connection: no Internet is ever enabled here.
positive=out/'positive-control.strace'
run(['strace','-f','-qq','-e','trace=network','-o',str(positive),'python3','-c',"import socket; s=socket.socket(); s.settimeout(.2); print(s.connect_ex(('192.0.2.1',443))); s.close()"])
positive_lines=[l for l in positive.read_text().splitlines() if 'AF_INET' in l]
if not any('connect(' in l for l in positive_lines):raise RuntimeError('observer positive control failed')
summary={'scope':'Linux CLI process trees and explicitly invoked manual Python subset helper; no desktop/Windows outbound claim','network':'none','clean_install':True,'no_sibling_checkout':True,'operations':reports,'failed_network_positive_control_inet_syscalls':len(positive_lines)}
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))
'''
with tempfile.TemporaryDirectory(prefix='archscope-offline-observer-') as d:
 script=pathlib.Path(d)/'observe.py';script.write_text(program)
 subprocess.run(['docker','run','--rm','--network','none','--cap-add','SYS_PTRACE','--security-opt','seccomp=unconfined','-v',str(engine)+':/input/engine:ro','-v',str(script)+':/observe.py:ro','-v',str(out)+':/evidence',a.image,'python3','/observe.py'],check=True)
