"""Diagnostic role/isolate attribution, without changing ordinary verdicts."""
import json
import math
from pathlib import Path
import statistics
import subprocess
import sys
ROOT=Path(__file__).resolve().parents[4]
REF='acdd1d49b2507b20f1b400883dcc99809d670c9e'
PREFIX='docs/review/evidence/2026-11-16-windows-additional/'
def load(name):return json.loads(subprocess.check_output(['git','show',REF+':'+PREFIX+name],cwd=ROOT))
def check(condition,message):
 if not condition:raise ValueError(message)
def metric(values):return {'delta':statistics.median(values[-3:])-statistics.median(values[:3]),'slopePerCycle':statistics.median((values[j]-values[i])/(j-i) for i in range(len(values)) for j in range(i+1,len(values)))}
def analyze():
 result={'sourceRef':REF,'scope':'Role and isolate evidence only; not an allocator diagnosis or new Windows execution','ordinary':{},'causalAllocationProven':False,'ordinaryFailuresPreserved':True,'windowsTraceRequired':True}
 for arch in ['amd64','386']:
  for scenario in ['unique','body-rich']:
   r=load('memory-'+scenario+'-'+arch+'.json');p=r['samples'][28:];roles={x['role'] for s in p for x in s['members']}
   check(len(r['samples'])==36 and len(p)==8,'unexpected ordinary sampling window')
   check(r['screeningPolicy']['treePrivateBytes']=={'delta':67108864,'perCycle':4194304},'ordinary budget changed')
   check(all(sum(x['privateBytes'] for x in s['members'])==s['treePrivateBytes'] for s in p),'role sum differs from tree')
   role_metrics={role:metric([sum(x['privateBytes'] for x in s['members'] if x['role']==role) for s in p]) for role in sorted(roles)}
   tree=metric([s['treePrivateBytes'] for s in p]);leave_one_out=[]
   check(all(math.isclose(value,r['memoryAssessment']['metrics']['treePrivateBytes'][key],abs_tol=1e-6) for key,value in tree.items()),'received tree metric mismatch')
   check(r['passed']==(scenario=='unique'),'ordinary historical verdict changed')
   # Sensitivity is diagnostic only: never replace the predeclared eight-point verdict.
   for omitted in range(8):
    points=[(i,p[i]['treePrivateBytes']) for i in range(8) if i!=omitted]
    leave_one_out.append(statistics.median((y2-y1)/(x2-x1) for i,(x1,y1) in enumerate(points) for x2,y2 in points[i+1:]))
   result['ordinary'][scenario+'-'+arch]={'passed':r['passed'],'metrics':tree,'roleMetrics':role_metrics,'originalSlopeBudget':4194304,'diagnosticLeaveOneOutSlopeRange':[min(leave_one_out),max(leave_one_out)],'originalVerdictUnchanged':True}
 d=load('native-retention-diagnostic.json')
 result['instrumented']={'nativeAcceptance':False,'matchedPoints':len(d['matched']),'maximumLagSeconds':d['maximumLagSeconds'],'expectedCounters':d['expectedCounters'],'metrics':d['metrics'],'missingDomains':['DOM document/node/listener counts','V8 backing storage and embedder heap','allocation stacks/native allocator/graphics commitment','repeat ordinary run after cause-specific change']}
 result['conclusion']='Growth is concentrated in renderer-private bytes, with bounded workspace ownership and a small measured isolate used-heap trend. These accounting domains do not partition process memory; do not label their difference a proven native leak.'
 return result
if __name__=='__main__':
 body=json.dumps(analyze(),indent=2)+'\n'
 if len(sys.argv)>1:Path(sys.argv[1]).write_text(body,encoding='utf-8')
 else:print(body,end='')
