"""Read-only recheck of immutable received Windows evidence; safe under Python -O."""
import collections
import csv
import hashlib
import io
import json
import math
from pathlib import Path
import statistics
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[4]
REF = '635512cfdfe7673f8a255cd80fae1dcbbc0e16ba'
PREFIX = 'docs/review/evidence/2026-11-16-windows-validation/'


def check(condition, message):
    if not condition:
        raise ValueError(message)


def blob(path):
    return subprocess.check_output(['git', 'show', REF + ':' + path], cwd=ROOT)


def raw(name):
    return blob(PREFIX + name)


def load(name):
    return json.loads(raw(name))


def sha(body):
    return hashlib.sha256(body).hexdigest()


def audit():
    manifest = load('artifact-manifest.json')
    for item in manifest['files']:
        body = raw(item['path'])
        check(len(body) == item['bytes'] and sha(body) == item['sha256'], 'artifact ' + item['path'])
    tracked = subprocess.check_output(['git','ls-tree','-r','--name-only',REF,PREFIX],cwd=ROOT,text=True).splitlines()
    check(set(tracked) == {PREFIX + x['path'] for x in manifest['files']} | {PREFIX + 'artifact-manifest.json'}, 'manifest completeness')
    received = load('final-validation.json')
    sources = []
    for item in received['sourceHashes']:
        canonical = blob(item['path'])
        variants = [('git',canonical),('windows-crlf',canonical.replace(b'\r\n',b'\n').replace(b'\n',b'\r\n'))]
        hits = [kind for kind, body in variants if len(body)==item['bytes'] and sha(body)==item['sha256']]
        check(bool(hits),'unresolved source ' + item['path'])
        sources.append({'path':item['path'],'receiptEncoding':hits[0],'originalReceiptSha256':item['sha256'],'gitSha256':sha(canonical)})
    units = {'samples':'samples','nanoseconds':'ns','microseconds':'µs','milliseconds':'ms','seconds':'s','bytes':'bytes','unknown':'Unknown'}
    csv_hashes = {}
    for suffix in ['', '-386']:
        native = load('chart-native' + suffix + '.json')
        readback = load('chart-readback' + suffix + '.json')
        check(readback['nativeReportSha256']==sha(raw('chart-native'+suffix+'.json')), 'native chart binding')
        check(len(native['report']['cases'])==7 and {c['unit'] for c in native['report']['cases']}==set(units) and native['report']['passed'], 'native chart count')
        check(len(readback['cases'])==7 and {c['unit'] for c in readback['cases']}==set(units), 'CSV unit coverage')
        check(readback['fixtureSha256']==sha(b'Root;work 1000000\n'), 'synthetic fixture identity')
        check(native['osScreenScaleFactor']==1 and native['webviewZoom']==1, 'chart environment')
        for case in native['report']['cases']:
            check(case['axis']==units[case['unit']] and all(case[k] for k in ['canvasObserved','svgAxisObserved','nativeSaveCompleted']), 'native chart case')
        for case in readback['cases']:
            unit=case['unit'];body=raw('chart-exports'+suffix+'/'+unit+'.csv')
            records=list(csv.DictReader(io.StringIO(body.decode('utf-8-sig'))))
            check(len(records)==1,'CSV row count')
            row=records[0];sources_field=json.loads(row['source_files'])
            check(row['label']=='Other' and row['value']=='1000000' and row['value_unit']==unit,'CSV values/units')
            check(row['template_id']=='profiler.components' and row['result_type']=='profiler_collapsed','CSV provenance')
            check(len(sources_field)==1 and sources_field[0].endswith('/units.collapsed'),'CSV source')
            check(sha(body)==case['csvSha256'] and case['passed'],'CSV digest')
            if suffix: check(body==raw('chart-exports/'+unit+'.csv'),'cross-architecture CSV')
            csv_hashes[(suffix or '-amd64')+'/'+unit]=sha(body)
    contrasts=[];environments=[];locale_checks=0;theme_checks=0
    for arch in ['amd64','386']:
        ribbon=load('ribbon-'+arch+'.json')
        check(ribbon['passed'] and len(ribbon['runs'])==3,'ribbon runs')
        source=next(s for s in sources if s['path']=='scripts/verify-ribbon-native.js')
        check(ribbon['scriptSha256']==source['originalReceiptSha256'],'ribbon source binding')
        for run in ribbon['runs']:
            receipt=run['receipt'];report=receipt['report']
            check(report['passed'] and {v['locale'] for v in report['locales']}=={'en','ko','ja','zh-CN','zh-TW'},'locales')
            check(all(v['menuRows']==1 and v['selected']=='true' and v['ribbonHeight']<=126 for v in report['locales']),'ribbon geometry')
            locale_checks+=len(report['locales'])*6
            check({v['theme'] for v in report['themes']}=={'light','dark','system'},'themes')
            for theme in report['themes']:
                theme_checks+=1
                for contrast in theme['selectedTextContrast']:
                    def luminance(color):
                        linear=[v/255/12.92 if v/255<=0.04045 else ((v/255+0.055)/1.055)**2.4 for v in color]
                        return sum(a*b for a,b in zip(linear,[0.2126,0.7152,0.0722]))
                    a,b=sorted([luminance(contrast['foreground']),luminance(contrast['background'])])
                    ratio=(b+0.05)/(a+0.05)
                    check(math.isclose(ratio,contrast['ratio'],abs_tol=1e-9) and ratio>=4.5,'contrast')
                    contrasts.append(ratio)
            check(receipt['osScreenScaleFactor']==1,'unexpected new OS DPI')
            environments.append({'arch':arch,'name':run['name'],'osScale':receipt['osScreenScaleFactor'],'webviewZoom':receipt['webviewZoom'],'viewport':report['viewport']})
    memory=load('memory-386.json');validation=load('memory-validation.json')
    expected={'cycles':36,'warmupCycles':28,'workspaceCapacity':80,'openSizes':[4096,512,32],'settleAfterOpenSeconds':3,'matchedSmallFileIdleSeconds':15}
    check(memory['protocol']==expected and len(memory['cycles'])==36,'memory protocol')
    check(memory['functionalPassed'] and memory['sourceFilesUnchanged'] and memory['desktopAliveAtEnd'],'memory functional flags')
    check(memory['buildCommit']=='10228156','memory build')
    binary=next(b for b in received['ordinaryBuilds'] if b['PEMachine']=='0x14c')
    check(memory['binarySha256']==binary['sha256'] and validation['sourceReportSha256']==sha(raw('memory-386.json')),'memory receipt/binary binding')
    for i,cycle in enumerate(memory['cycles'],1):
        check(cycle['cycle']==i and cycle['warmup']==(i<=28),'memory cycles')
        check([x['rows'] for x in cycle['opens']]==[4096,512,32] and all(x['selectedIdMatches'] for x in cycle['opens']),'memory opens')
    points=[c['settled'] for c in memory['cycles'][28:]];identities=[]
    for point in points:
        members=point['members'];check(point['treePrivateBytes']==sum(m['privateBytes'] for m in members),'tree sum')
        apps=[m for m in members if m['role']=='app'];check(len(apps)==1,'owned app identity');app=apps[0]
        check([point['rootPrivateBytes'],point['rootHandles'],point['rootThreads']]==[app['privateBytes'],app['handles'],app['threads']],'root metrics')
        ids=sorted(m['ordinal'] for m in members);check(len(ids)==len(set(ids)),'unique identity ordinals');identities.append(ids)
    check(all(x==identities[0] for x in identities) and memory['stableMeasuredProcessIdentities'],'stable measured identities')
    budgets={'treePrivateBytes':(64*2**20,4*2**20),'rootPrivateBytes':(16*2**20,2**20),'rootHandles':(24,2),'rootThreads':(4,0.5)}
    metrics={}
    for key,(delta_limit,slope_limit) in budgets.items():
        values=[p[key] for p in points];check(all(isinstance(v,int) and v>=0 for v in values),'metric values')
        first,last=statistics.median(values[:3]),statistics.median(values[-3:]);delta=last-first
        slope=statistics.median((values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8))
        old=memory['memoryAssessment']['metrics'][key]
        check(old['budget']=={'delta':delta_limit,'perCycle':slope_limit},'original budget')
        check(delta<=delta_limit and slope<=slope_limit and old['withinBudget'],'memory budget')
        for target in [old,validation['metrics'][key]]:
            check(math.isclose(target['delta'],delta,abs_tol=1e-6) and math.isclose(target['slopePerCycle'],slope,abs_tol=1e-6),'memory recalculation')
        metrics[key]={'delta':delta,'slopePerCycle':slope,'withinBudget':True}
    check(memory['passed'] and validation['passed'] and memory['memoryAssessment']['classification']=='BOUNDED_IN_THIS_RUN','finite verdict')
    for arch in ['amd64','386','docker']:
        workflow=load('workspace-'+arch+'.json');check(workflow['passed'] and workflow['cases']==3 and workflow['exports']==9,'workspace receipt')
    for arch in ['amd64','386']:
        packs=load('packs-'+arch+'.json');matrix=packs['matrix']
        check(packs['passed'] and packs['productionTrustUnchanged'] and [m['appVersion'] for m in matrix]==['0.4.0-rc.1','0.4.0','0.4.1'],'packs matrix')
        check([m['candidateAccepted'] for m in matrix]==[True,True,False] and all(m['officialSignerQualified'] is False for m in matrix),'fixture signing scope')
    parity=load('model-parity.json');check(len(parity['cases'])==219 and parity['passed'],'model parity receipt')
    check(len({x['model'] for x in parity['cases']})==219 and all(x['rows']>0 and len(x['fingerprint'])==67 and len(x['projectionSha256'])==64 for x in parity['cases']),'model parity identities')
    native=collections.Counter(c['result'] for c in load('native-cases.json'))
    remaining=load('remaining-acceptance.json')
    task_states={k:v['status'] for k,v in remaining.items() if k.startswith('T-')}
    check(task_states=={'T-601':'REVIEW','T-627':'REVIEW','T-611':'REVIEW','T-613':'PENDING','T-654':'TODO','T-662':'TODO','T-663':'REVIEW','T-664':'REVIEW'},'eight open task inventory')
    return {'reviewedRef':REF,'artifacts':len(manifest['files']),'artifactManifestComplete':True,'sourceCounts':dict(collections.Counter(x['receiptEncoding'] for x in sources)),'sources':sources,'csvFiles':len(csv_hashes),'crossArchitectureCsvBytesEqual':True,'ribbonLocaleTabChecks':locale_checks,'themeSelections':theme_checks,'computedContrastChecks':len(contrasts),'minimumSelectedContrast':min(contrasts),'environments':environments,'nativeCaseResultsIncludingHistoricalFailures':dict(native),'memory':{'cycles':36,'opens':108,'measuredPoints':8,'processIdentities':len(identities[0]),'metrics':metrics,'classification':'BOUNDED_IN_THIS_RUN','privateBinaryAndSourceFixturesNotReopened':True},'headlessWorkflowReceipts':3,'syntheticPackMatrices':2,'nativeWindowsRerun':False,'openTaskStates':task_states,'newConfirmedProductFindings':[],'scope':'Independent local receipt/code recheck; original author/review CONDITIONAL dispositions are not closed or renamed'}


if __name__=='__main__':
    result=audit();body=json.dumps(result,indent=2)+'\n'
    if len(sys.argv)>1:Path(sys.argv[1]).write_text(body,encoding='utf-8')
    else:print(body,end='')
