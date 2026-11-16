"""Read-only pinned-commit audit. FAIL screens are verified, never promoted."""
import collections
import hashlib
import json
import math
from pathlib import Path
import statistics
import struct
import subprocess
import sys
import xml.etree.ElementTree as ET
import zlib

ROOT=Path(__file__).resolve().parents[4]
REF='acdd1d49b2507b20f1b400883dcc99809d670c9e'
PREFIX='docs/review/evidence/2026-11-16-windows-additional/'
def check(ok,message):
    if not ok: raise ValueError(message)
def blob(path):return subprocess.check_output(['git','show',REF+':'+path],cwd=ROOT)
def raw(name):return blob(PREFIX+name)
def load(name):return json.loads(raw(name))
def sha(b):return hashlib.sha256(b).hexdigest()
def metric(values):
    return {'firstMedian':statistics.median(values[:3]),'lastMedian':statistics.median(values[-3:]),'delta':statistics.median(values[-3:])-statistics.median(values[:3]),'slopePerCycle':statistics.median((values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8))}
def equal_metrics(a,b,label):
    for k in ['firstMedian','lastMedian','delta','slopePerCycle']:check(math.isclose(a[k],b[k],abs_tol=1e-6),label+' '+k)
def png(body):
    check(body[:8]==b'\x89PNG\r\n\x1a\n','PNG signature');offset=8;compressed=b'';size=None;ended=False
    while offset<len(body):
        n=struct.unpack_from('>I',body,offset)[0];kind=body[offset+4:offset+8];data=body[offset+8:offset+8+n]
        check(zlib.crc32(kind+data)&0xffffffff==struct.unpack_from('>I',body,offset+8+n)[0],'PNG CRC')
        if kind==b'IHDR':
            w,h,depth,color,compression,filtering,interlace=struct.unpack('>IIBBBBB',data);check(depth==8 and color in [2,6] and interlace==compression==filtering==0,'PNG format');size=[w,h];channels=3 if color==2 else 4
        if kind==b'IDAT':compressed+=data
        offset+=n+12
        if kind==b'IEND':ended=True;break
    check(ended and offset==len(body) and size is not None,'PNG complete')
    decoded=zlib.decompress(compressed);check(len(decoded)==h*(1+w*channels),'PNG decompressed size')
    return size

def audit():
    m=load('artifact-manifest.json')
    for x in m['files']:
        b=raw(x['path']);check(len(b)==x['bytes'] and sha(b)==x['sha256'],'artifact '+x['path'])
    tracked=subprocess.check_output(['git','ls-tree','-r','--name-only',REF,PREFIX],cwd=ROOT,text=True).splitlines()
    check(set(tracked)=={PREFIX+x['path'] for x in m['files']}|{PREFIX+'artifact-manifest.json'},'manifest completeness')
    images=[];axes={'samples':'samples','nanoseconds':'ns','microseconds':'µs','milliseconds':'ms','seconds':'s','bytes':'bytes','unknown':'Unknown'}
    for arch in ['amd64','386']:
        record=load('chart-readback-'+arch+'.json');native=load('chart-native-'+arch+'.json')
        check(record['sourceReportSha256']==sha(raw('chart-native-'+arch+'.json')) and record['passed'],'chart receipt')
        check(native['report']['passed'] and native['osScreenScaleFactor']==native['webviewZoom']==1,'chart native scope')
        check(len(record['cases'])==7 and {x['unit'] for x in record['cases']}==set(axes),'chart coverage')
        for c in record['cases']:
            unit=c['unit'];svg=raw('images-'+arch+'/'+unit+'-canvas.svg');image=raw('images-'+arch+'/'+unit+'-svg.png')
            check(sha(svg)==c['svgSha256'] and sha(image)==c['pngSha256'],'image digests')
            root=ET.fromstring(svg);size=[int(root.attrib['width']),int(root.attrib['height'])];texts=[e.text for e in root.iter() if e.tag.endswith('}text')]
            check(root.tag=='{http://www.w3.org/2000/svg}svg' and all(x in texts for x in [axes[unit],'Other','1,000,000']),'SVG values/axis')
            check(len([e for e in root.iter() if e.tag.endswith('}path') and e.attrib.get('fill')=='#0f766e' and e.attrib.get('d')])==1,'SVG actual bar')
            check(size==c['svgSize'] and png(image)==c['pngSize']==[2*v for v in size],'saved image dimensions')
            if arch=='386':check(image==raw('images-amd64/'+unit+'-svg.png'),'cross-architecture PNG')
            images.append({'arch':arch,'unit':unit,'svgSize':size,'pngSize':c['pngSize']})
    protocol=load('memory-protocol.json');check(protocol['declaredBeforeRuns'] and not protocol['forcedGC'] and not protocol['trim'],'ordinary protocol')
    budgets={'treePrivateBytes':(64*2**20,4*2**20),'rootPrivateBytes':(16*2**20,2**20),'rootHandles':(24,2),'rootThreads':(4,0.5)}
    screens={}
    for arch in ['amd64','386']:
        for scenario in ['unique','body-rich']:
            key=scenario+'-'+arch;r=load('memory-'+key+'.json');rows=512 if scenario=='unique' else 128
            check(r['scenario']==scenario and r['buildCommit']=='635512cf' and r['binarySha256']==protocol['binaries'][arch],'ordinary binding')
            check(r['sourceFixtureSha256']==sha(raw('memory-seed.httpsession')),'seed binding')
            check(len(r['cycles'])==len(r['samples'])==36 and len(r['files'])==108,'ordinary counts')
            check(len({f['sha256'] for f in r['files']})==108 and all(f['rows']==rows for f in r['files']),'distinct source files')
            check(r['protocol']==dict(cycles=36,warmupCycles=28,workspaceCapacity=80,openSizes=[rows]*3,settleAfterOpenSeconds=3,postCycleIdleSeconds=15),'timing protocol')
            for i,cycle in enumerate(r['cycles']):
                check(cycle['cycle']==i+1 and cycle['warmup']==(i<28) and cycle['settled']==r['samples'][i],'cycles')
                check([x['rows'] for x in cycle['opens']]==[rows]*3 and all(x['selectedIdMatches'] for x in cycle['opens']),'opens')
                if i:check(r['samples'][i]['elapsedSeconds']-r['samples'][i-1]['elapsedSeconds']>=24,'ordinary pacing')
            points=r['samples'][28:];identities=[]
            for p in points:
                check(p['treePrivateBytes']==sum(x['privateBytes'] for x in p['members']),'tree sum');app=[x for x in p['members'] if x['role']=='app'];check(len(app)==1,'app identity')
                check([p['rootPrivateBytes'],p['rootHandles'],p['rootThreads']]==[app[0]['privateBytes'],app[0]['handles'],app[0]['threads']],'root sum')
                identities.append(sorted(x['ordinal'] for x in p['members']))
            check(all(ids==identities[0] and len(ids)==len(set(ids)) for ids in identities),'stable identities')
            computed={}
            for name,(delta_limit,slope_limit) in budgets.items():
                check(protocol['budgets'][name]==r['screeningPolicy'][name]=={'delta':delta_limit,'perCycle':slope_limit},'fixed budgets')
                v=metric([p[name] for p in points]);equal_metrics(v,r['memoryAssessment']['metrics'][name],key+' '+name)
                v['withinBudget']=v['delta']<=delta_limit and v['slopePerCycle']<=slope_limit;computed[name]=v
                check(v['withinBudget']==r['memoryAssessment']['metrics'][name]['withinBudget'],'budget classification')
            verdict=all(v['withinBudget'] for v in computed.values());check(verdict==r['passed']==r['memoryAssessment']['withinBudget'],'original verdict')
            check(verdict==(scenario=='unique'),'expected PASS/FAIL preservation')
            byrole={}
            for role in sorted({x['role'] for p in points for x in p['members']}):byrole[role]=metric([sum(x['privateBytes'] for x in p['members'] if x['role']==role) for p in points])
            screens[key]={'passed':verdict,'elapsedSeconds':r['elapsedSeconds'],'metrics':computed,'roles':byrole,'processes':len(identities[0])}
    projections=[load('projection-fixed-'+arch+'.json') for arch in ['amd64','386','linux']]
    check(all(r['passed'] and len(r['files'])==216 for r in projections),'projection count')
    check(projections[0]['files']==projections[1]['files']==projections[2]['files'],'full projection identities')
    diagnostic=load('native-retention-diagnostic.json');data=load('native-retention-diagnostic-raw.json');dp=load('native-retention-diagnostic-protocol.json')
    records=[json.loads(line) for line in raw('native-retention-observations.jsonl').splitlines()]
    check(not dp['nativeAcceptance'] and not dp['forcedGC'] and not dp['trim'] and not diagnostic['nativeAcceptance'],'diagnostic distinction')
    check(len(records)<=5000 and all(len(line)<=4096 for line in raw('native-retention-observations.jsonl').splitlines()),'bounded records')
    matched=[]
    for p in data['samples'][28:]:
        previous=[x for x in records if x['epochMs']<=p['epochMs']];check(bool(previous),'preceding observation');record=max(previous,key=lambda x:x['epochMs']);lag=(p['epochMs']-record['epochMs'])/1000
        check(0<=lag<=dp['maximumMatchingLagSeconds'],'timely record')
        check(all(record['counters'].get(k)==v for k,v in diagnostic['expectedCounters'].items()),'bounded native counters')
        renderer=[x for x in p['members'] if x['role']=='renderer'];check(len(renderer)==1,'renderer identity')
        matched.append({'lagSeconds':lag,'rendererPrivateBytes':renderer[0]['privateBytes'],'treePrivateBytes':p['treePrivateBytes'],'jsHeapUsed':record['counters']['jsHeap']['used'],'jsHeapTotal':record['counters']['jsHeap']['total']})
    check(len(matched)==8 and math.isclose(max(x['lagSeconds'] for x in matched),diagnostic['maximumLagSeconds']),'diagnostic matching')
    dm={}
    for name in ['rendererPrivateBytes','treePrivateBytes','jsHeapUsed','jsHeapTotal']:
        dm[name]=metric([x[name] for x in matched]);equal_metrics(dm[name],diagnostic['metrics'][name],'native '+name)
    final=load('final-validation.json')
    for key,r in screens.items():equal_metrics(r['metrics']['treePrivateBytes'],final['ordinaryMemory'][key]['treeMetrics'],key+' final')
    return {'reviewedRef':REF,'artifacts':len(m['files']),'manifestComplete':True,'savedImageFiles':len(images)*2,'crossArchitecturePngPairs':7,'images':images,'ordinaryScreens':screens,'projectionFilesAcrossThreeHosts':216,'nativeDiagnostic':{'ordinaryAcceptance':False,'records':len(records),'maxLagSeconds':max(x['lagSeconds'] for x in matched),'metrics':dm,'boundedCounterMatches':8},'privateWindowsBinariesNotReopened':True,'originalFailuresPreserved':True,'newConfirmedProductFindings':[]}

if __name__=='__main__':
    result=json.dumps(audit(),indent=2)+'\n'
    if len(sys.argv)>1:Path(sys.argv[1]).write_text(result,encoding='utf-8')
    else:print(result,end='')
