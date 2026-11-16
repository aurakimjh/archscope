"""Correlate read-only native diagnostic counters with process-private samples."""
import argparse
import hashlib
import json
from pathlib import Path
import statistics

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--private',type=Path,required=True)
parser.add_argument('--public',type=Path,required=True)
args=parser.parse_args()
run=args.private/'body-rich-amd64-diagnostic'
raw=(run/'result.json').read_bytes();report=json.loads(raw)
assert report['instrumented'] and report['nativeAcceptance'] is False
assert report['scenario']=='body-rich' and report['functionalPassed'] and report['sourceFilesUnchanged']
assert report['protocol']==dict(cycles=36,warmupCycles=28,workspaceCapacity=80,openSizes=[128]*3,settleAfterOpenSeconds=3,postCycleIdleSeconds=15)
assert len(report['cycles'])==len(report['samples'])==36
layout_raw=(run/'layout.json').read_bytes();layout=json.loads(layout_raw)
memory_raw=(run/'layout.json.memory.jsonl').read_bytes()
lines=memory_raw.splitlines()
assert 8<=len(lines)<=5000 and all(len(line)<=4096 for line in lines)
records=[json.loads(line) for line in lines]
views=[dict(workspaceRetention=v['counters'],memoryDiagnosticEpochMs=v['epochMs']) for v in records]
assert views,'instrumented getter must actually be observed'
views.sort(key=lambda v:v['memoryDiagnosticEpochMs'])
protocol=json.loads((args.public/'native-retention-diagnostic-protocol.json').read_text(encoding='utf-8'))
assert protocol['declaredBeforeRun'] and protocol['forcedGC'] is False and protocol['nativeAcceptance'] is False
binary=(args.private/'archscope-retention-diagnostic-amd64.exe').read_bytes()
assert hashlib.sha256(binary).hexdigest()==protocol['binarySha256']==report['binarySha256']
matched=[]
for p in report['samples'][28:]:
    candidates=[v for v in views if v['memoryDiagnosticEpochMs']<=p['epochMs']]
    assert candidates,'missing preceding renderer observation'
    v=candidates[-1];lag=(p['epochMs']-v['memoryDiagnosticEpochMs'])/1000
    counters=v['workspaceRetention']
    renderer,=[m for m in p['members'] if m['role']=='renderer']
    matched.append(dict(cycle=int(p['label'].split('-')[1]),lagSeconds=lag,counters=counters,
        rendererPrivateBytes=renderer['privateBytes'],treePrivateBytes=p['treePrivateBytes']))
max_lag=max(p['lagSeconds'] for p in matched)
timely=max_lag<=protocol['maximumMatchingLagSeconds']
expected=dict(entries=80,httpEntries=80,fingerprintedHttpEntries=80,uniqueTransactionArrays=80,
    logicalRows=10240,uniqueRetainedRows=10240,immutableArrays=80,activeRows=128)
counts_match=all(all(p['counters'].get(k)==v for k,v in expected.items()) for p in matched)
def metric(values):
    return dict(firstMedian=statistics.median(values[:3]),lastMedian=statistics.median(values[-3:]),
        delta=statistics.median(values[-3:])-statistics.median(values[:3]),
        slopePerCycle=statistics.median((values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8)))
metrics={k:metric([p[k] for p in matched]) for k in ['rendererPrivateBytes','treePrivateBytes']}
heap_available=all(p['counters'].get('jsHeap') is not None for p in matched)
if heap_available:
    for k in ['used','total']:metrics['jsHeap'+k.title()]=metric([p['counters']['jsHeap'][k] for p in matched])
for key in ['delta','slopePerCycle']:
    assert abs(metrics['treePrivateBytes'][key]-report['memoryAssessment']['metrics']['treePrivateBytes'][key])<1e-6
out=dict(scope='Read-only instrumented native Windows correlation; separate from ordinary screens, no forced GC/trim, no causal allocation or universal leak-free proof',
    instrumented=True,nativeAcceptance=False,countsQualified=timely and counts_match,maximumLagSeconds=max_lag,
    jsHeapAvailable=heap_available,expectedCounters=expected,metrics=metrics,matched=matched,
    ordinaryPolicyAssessmentInDiagnosticRun=report['memoryAssessment'],binarySha256=report['binarySha256'],
    reportSha256=hashlib.sha256(raw).hexdigest(),layoutSha256=hashlib.sha256(layout_raw).hexdigest(),memoryStreamSha256=hashlib.sha256(memory_raw).hexdigest(),memoryRecords=len(records))
(args.public/'native-retention-diagnostic.json').write_text(json.dumps(out,indent=2)+'\n',encoding='utf-8')
(args.public/'native-retention-diagnostic-raw.json').write_bytes(raw)
(args.public/'native-retention-layout.json').write_bytes(layout_raw)
(args.public/'native-retention-observations.jsonl').write_bytes(memory_raw)
print(json.dumps(dict(countsQualified=out['countsQualified'],maximumLagSeconds=max_lag,metrics=metrics)))
if not timely or not counts_match or not heap_available:raise SystemExit(1)
