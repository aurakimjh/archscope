import hashlib,json,math,shutil,statistics
from pathlib import Path
root=Path('.tmp/windows-final-20261114');out=Path('docs/review/evidence/2026-11-14-windows-final')
cases=[('import-amd64/result.json','import-initial-amd64.json',2,False),('import-386/result.json','import-initial-386.json',2,False),('import-saturated-amd64/result.json','import-history36-amd64.json',28,False),('import-saturated-386/result.json','import-history36-386.json',28,True),('import-saturated-amd64/continuation.json','import-history52-amd64.json',8,True)]
checks=[]
for source,dest,warmup,passed in cases:
    report=json.loads((root/source).read_text(encoding='utf-8'))
    assert report['passed']==passed
    points=[c['settled'] for c in report['cycles'][warmup:]];assert len(points)==8
    computed={}
    for field in ['treePrivateBytes','rootPrivateBytes','rootHandles','rootThreads']:
        values=[p[field] for p in points]
        delta=statistics.median(values[-3:])-statistics.median(values[:3])
        slope=statistics.median([(values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8)])
        actual=report['memoryAssessment']['metrics'][field]
        assert math.isclose(delta,actual['delta'],abs_tol=0.0001) and math.isclose(slope,actual['slopePerCycle'],abs_tol=0.0001)
        assert actual['withinBudget']==(delta<=actual['budget']['delta'] and slope<=actual['budget']['perCycle'])
        computed[field]={'delta':delta,'slopePerCycle':slope}
    for p in points:
        assert len(p['members'])==7
        assert p['treePrivateBytes']==sum(m['privateBytes'] for m in p['members'])
    identities=[sorted(m['ordinal'] for m in p['members']) for p in points]
    assert all(i==identities[0] for i in identities)
    checks.append({'file':dest,'reportedPassed':passed,'independentArithmeticMatches':True,'sameSevenProcessIdentitiesWithinMeasuredWindow':True,'metrics':computed})
    shutil.copyfile(root/source,out/dest)
for source,dest in [('scripts/verify-windows-import-memory.mjs','import-memory-runner-used.mjs'),(str(root/'import-runner-initial.mjs'),'import-memory-initial-runner-used.mjs'),(str(root/'continue-import-memory.mjs'),'import-memory-continuation-used.mjs'),('scripts/lib/desktop-memory-checks.mjs','import-memory-checks-used.mjs'),('scripts/acceptance/layout/sample-owned-process-tree.ps1','import-memory-sampler-used.ps1')]:shutil.copyfile(source,out/dest)
shutil.copyfile(root/'import-history-state.log',out/'import-history-state.txt')
(out/'import-memory-validation.json').write_text(json.dumps({'passed':True,'scope':'Arithmetic/identity checks; original failed budgets are preserved','runs':checks},indent=2)+'\n',encoding='utf-8')
print('Five memory receipts independently verified; original FAILs preserved')
