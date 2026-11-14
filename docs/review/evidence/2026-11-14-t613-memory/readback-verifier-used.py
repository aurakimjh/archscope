import hashlib,json,statistics,sys
from pathlib import Path
root=Path('C:/workspace/archscope');base=root/'.tmp/t613-memory-20261114';public=root/'docs/review/evidence/2026-11-14-t613-memory'
sys.path.insert(0,str(root/'scripts/acceptance/layout'))
from verify_p2_windows_readback import compare_transactions,require,read
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def median(v):return statistics.median(v)
for arch in ['amd64','386']:
    run=(base/arch).resolve();r=read(run/'result.json')
    require(r['passed'] and r['functionalPassed'],'memory run did not pass')
    require(r['buildCommit']=='324a8d97432a8f9a144cb48b545cc7cb8f21da08','unexpected app source')
    require(r['binarySha256']==digest(root/f'.tmp/p2-windows-20261114/archscope-p2-{arch}.exe'),'binary hash mismatch')
    require(r['requests']==5120 and len(r['cycles'])==10,'workload count mismatch')
    for number,c in enumerate(r['cycles'],1):
        require(c['cycle']==number and c['warmup']==(number<=2),'cycle/warmup mismatch')
        stats=c['stats'];clear=c['clear']
        require(stats['state']=='running' and stats['generation']==number and stats['captured']==stats['persisted']==512 and stats['dropped']==0,'unhealthy cycle')
        require(clear['empty']['captured']==clear['empty']['persisted']==clear['empty'].get('retained',0)==0,'clear not empty')
        require(clear['empty']['generation']==clear['refilled']['generation']==number,'clear generation mismatch')
        require(clear['refilled']['captured']==clear['refilled']['persisted']==clear['refilled']['retained']==4,'clear refill mismatch')
        require(c['settled']['elapsedSeconds']-c['loaded']['elapsedSeconds']>=180,'idle phase shortened')
    points=[c['settled'] for c in r['cycles'] if not c['warmup']]
    identities=[{(m['ordinal'],m['role']) for m in p['members']} for p in points]
    require(all(ids==identities[0] for ids in identities) and len(identities[0])==7,'matched process identity set changed')
    metrics={}
    for key in ['treePrivateBytes','rootPrivateBytes','rootHandles','rootThreads']:
        values=[p[key] for p in points]
        first,last=median(values[:3]),median(values[-3:])
        slope=median([(values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8)])
        recorded=r['memoryAssessment']['metrics'][key]
        require(abs(recorded['slopePerCycle']-slope)<1e-6 and recorded['delta']==last-first,'assessment arithmetic mismatch')
        require(last-first<=recorded['budget']['delta'] and slope<=recorded['budget']['perCycle'],'growth exceeds predeclared policy')
        metrics[key]={'firstMedian':first,'lastMedian':last,'delta':last-first,'slopePerCycle':slope}
    roles={m['role'] for p in points for m in p['members']}
    role_delta={role:median([sum(m['privateBytes'] for m in p['members'] if m['role']==role) for p in points[-3:]])-median([sum(m['privateBytes'] for m in p['members'] if m['role']==role) for p in points[:3]]) for role in sorted(roles)}
    idle=[p for p in r['samples'] if p['label'].startswith('finalized-open-file-idle')]
    require(idle[-1]['elapsedSeconds']-idle[0]['elapsedSeconds']>=295,'final idle phase shortened')
    final=r['finalStats'];require(final['state']=='finalized' and final['captured']==final['persisted']==512 and final['dropped']==0,'bad final counters')
    store=Path(read(run/'.private-session.json')['storePath']).resolve();require(store.is_relative_to(run/'.archscope-captures'),'store escaped run')
    saved=run/'final.httpsession';doc=read(saved)
    source=[read(p)['transaction'] for p in sorted((store/'rolling').glob('*.json'))]
    annotations=read(store/'annotations.json') if (store/'annotations.json').exists() else {}
    comparison=compare_transactions(source,doc['transactions'],annotations,doc['selectedId'])
    require(comparison['rows']==r['savedRows']==128 and digest(saved)==r['savedSessionSha256'],'final save mismatch')
    require(doc['selectedId']==doc['transactions'][0]['id'],'selection mismatch')
    require([tx['url'] for tx in doc['transactions']]==[f'http://127.0.0.1:11512/soak?sequence={n}' for n in range(4992,5120)],'final URL range mismatch')
    out={'schemaVersion':1,'arch':arch,'passed':True,'functionalPassed':True,'classification':'BOUNDED_IN_THIS_RUN','seconds':r['elapsedSeconds'],
        'cycles':10,'warmupCycles':2,'measuredCycles':8,'totalRequests':5120,'sameSevenProcessIdentitiesAtMatchedIdle':True,
        'assessmentArithmeticIndependentlyRecalculated':True,'metrics':metrics,'rolePrivateMedianDeltas':role_delta,
        'finalizedIdleSecondsObserved':idle[-1]['elapsedSeconds']-idle[0]['elapsedSeconds'],
        'finalIdleRootHandles':idle[-1]['rootHandles'],'finalIdleRootThreads':idle[-1]['rootThreads'],
        'finalIdleTreePrivateBytes':idle[-1]['treePrivateBytes'],'finalIdleRootPrivateBytes':idle[-1]['rootPrivateBytes'],
        'selectedSequence':doc['transactions'][0]['sequence'],'selectedURL':doc['transactions'][0]['url'],
        'fileSha256':digest(saved),'fileBytes':saved.stat().st_size,**comparison,
        'limitations':'Fixed synthetic body-disabled128retained workload on source324a8d97, shared host; finite budgets, no universal leak-free/pressure/body/renderer-selection claim. Bootstrap fix8985736f is validated separately.'}
    with (public/f'readback-{arch}.json').open('x',encoding='utf-8',newline='\n') as file:file.write(json.dumps(out,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'arch':arch,'passed':True,'seconds':r['elapsedSeconds'],'metrics':metrics,'roleDeltas':role_delta,'finalIdleRootHandles':idle[-1]['rootHandles']}))
