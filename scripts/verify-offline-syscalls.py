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
 for name in ('packs/input.jsonl','catalog/metric-history-set.json','catalog/metric-history-sdk-python-1.38.0.otlp.json','catalog/system-evidence-perf-script.txt','catalog/system-evidence-perf-capture.json','catalog/inference-evidence-set.json','catalog/inference-evidence-nvidia-set.json','catalog/inference-evidence-nvidia-query.csv','catalog/inference-evidence-nvidia-capture.json','catalog/llm-trace-sdk-python-1.38.0.otlp.json','catalog/llm-trace-neutral-options.json'):z.extract(name,'/sources')
model=json.loads(pathlib.Path('/sources/catalog/metric-history-set.json').read_text())
selection={'series_ids':[model['series'][0]['id']]}
inference=json.loads(pathlib.Path('/sources/catalog/inference-evidence-nvidia-set.json').read_text())
signal=inference['signals'][0]
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
summary={'scope':'Linux CLI process trees only; no desktop/Windows outbound claim','network':'none','clean_install':True,'no_sibling_checkout':True,'operations':reports,'failed_network_positive_control_inet_syscalls':len(positive_lines)}
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))
'''
with tempfile.TemporaryDirectory(prefix='archscope-offline-observer-') as d:
 script=pathlib.Path(d)/'observe.py';script.write_text(program)
 subprocess.run(['docker','run','--rm','--network','none','--cap-add','SYS_PTRACE','--security-opt','seccomp=unconfined','-v',str(engine)+':/input/engine:ro','-v',str(script)+':/observe.py:ro','-v',str(out)+':/evidence',a.image,'python3','/observe.py'],check=True)
