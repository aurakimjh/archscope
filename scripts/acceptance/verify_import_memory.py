"""Independently recompute the predeclared ordinary Windows import screen."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import statistics
import struct

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--private', type=Path, required=True)
parser.add_argument('--public', type=Path, required=True)
parser.add_argument('--arch', choices=['amd64','386'], required=True)
parser.add_argument('--scenario', choices=['unique','body-rich'], required=True)
args = parser.parse_args()
key = args.scenario + '-' + args.arch
run = args.private / key
raw = (run / 'result.json').read_bytes()
report = json.loads(raw)
protocol = json.loads((args.public / 'memory-protocol.json').read_text(encoding='utf-8'))
assert protocol['declaredBeforeRuns'] and key in protocol['order']
assert report['buildCommit'] == protocol['sourceCommit']
assert report['scenario'] == args.scenario and 'failure' not in report
assert report['sourceFixtureSha256'] == '5adb3f87ada2644caadd1c23b5b753376e1c31575088888eeec29a9ddd28bcc9'
assert report['protocol'] == dict(cycles=36,warmupCycles=28,workspaceCapacity=80,
    openSizes=[512 if args.scenario == 'unique' else 128]*3,settleAfterOpenSeconds=3,postCycleIdleSeconds=15)
assert len(report['cycles']) == len(report['samples']) == 36
assert report['desktopAliveAtEnd'] and report['sourceFilesUnchanged'] and report['functionalPassed']
assert len(report['files']) == 108
binary = (args.private / ('archscope-production-' + args.arch + '.exe')).read_bytes()
binary_sha = hashlib.sha256(binary).hexdigest()
assert binary_sha == report['binarySha256'] == protocol['binaries'][args.arch]
assert struct.unpack_from('<H',binary,struct.unpack_from('<I',binary,0x3c)[0]+4)[0] == (0x8664 if args.arch == 'amd64' else 0x14c)
assert b'__archscopeWorkspaceRetention' not in binary
rows = 512 if args.scenario == 'unique' else 128
file_hashes, row_hashes = set(), set()
for index, fixture in enumerate(report['files']):
    name = f'synthetic-{args.scenario}-{index:03}.httpsession'
    assert fixture['fileName'] == name
    data = (run / name).read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    assert digest == fixture['sha256']; file_hashes.add(digest)
    doc = json.loads(data)
    assert len(doc['transactions']) == fixture['rows'] == rows
    assert doc['selectedId'] == fixture['selectedId'] == f'import-memory-{args.scenario}-{index}-0'
    row_hashes.add(hashlib.sha256(json.dumps(doc['transactions'],sort_keys=True,ensure_ascii=True,separators=(',',':')).encode()).hexdigest())
    for i,tx in enumerate(doc['transactions']):
        assert tx['id'] == f'import-memory-{args.scenario}-{index}-{i}'
        assert tx['starred'] == (i == 0)
        assert tx['comment'] == (f'메모리 재열기 synthetic note {args.scenario} {index}' if i == 0 else '')
        for side in ['request','response']:
            m = tx[side]
            if args.scenario == 'unique':
                assert m['bodyStorage'] == 'omitted'
            else:
                body = f'synthetic-{side}-{index}-{i}-' + 'x'*1024
                size = len(body.encode('utf-8'))
                assert m['bodyStorage'] == 'inline' and m['bodyPreview'] == body
                assert m['contentType'] == 'text/plain'
                assert m['bodySize'] == m['bodyDecoded'] == m['transferSize'] == size
                headers = {h['name']:h['value'] for h in m['headers']}
                assert headers == {'Content-Type':'text/plain','Content-Length':str(size)}
                assert 'bodyRef' not in m and 'bodyEncoding' not in m and not m.get('redacted')
assert len(file_hashes) == len(row_hashes) == 108
for number, cycle in enumerate(report['cycles'],1):
    assert cycle['cycle'] == number and cycle['warmup'] == (number <= 28)
    assert [o['rows'] for o in cycle['opens']] == [rows]*3
    assert all(o['selectedIdMatches'] for o in cycle['opens'])
    assert cycle['settled'] == report['samples'][number-1]
    if number > 1:
        assert cycle['settled']['elapsedSeconds'] - report['samples'][number-2]['elapsedSeconds'] >= 24
points = report['samples'][28:]
identities = []
for point in report['samples']:
    members = point['members']
    assert point['treePrivateBytes'] == sum(m['privateBytes'] for m in members)
    app, = [m for m in members if m['role'] == 'app']
    assert [point['rootPrivateBytes'],point['rootHandles'],point['rootThreads']] == [app['privateBytes'],app['handles'],app['threads']]
    ids = sorted(m['ordinal'] for m in members)
    assert len(ids) == len(set(ids))
    if point in points: identities.append(ids)
stable = all(ids == identities[0] for ids in identities)
assert stable == report['stableMeasuredProcessIdentities']
budgets = dict(treePrivateBytes=(64*2**20,4*2**20),rootPrivateBytes=(16*2**20,2**20),rootHandles=(24,2),rootThreads=(4,0.5))
metrics = {}
for metric,(delta_budget,slope_budget) in budgets.items():
    assert protocol['budgets'][metric] == dict(delta=delta_budget,perCycle=slope_budget)
    assert report['screeningPolicy'][metric] == protocol['budgets'][metric]
    values = [p[metric] for p in points]
    assert all(type(v) == int and v >= 0 for v in values)
    first,last = statistics.median(values[:3]),statistics.median(values[-3:])
    delta = last-first
    slope = statistics.median((values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8))
    original = report['memoryAssessment']['metrics'][metric]
    assert original['firstMedian'] == first and original['lastMedian'] == last
    assert original['budget'] == dict(delta=delta_budget,perCycle=slope_budget)
    assert math.isclose(delta,original['delta'],abs_tol=1e-6) and math.isclose(slope,original['slopePerCycle'],abs_tol=1e-6)
    passed = delta <= delta_budget and slope <= slope_budget
    assert passed == original['withinBudget']
    metrics[metric] = dict(firstMedian=first,lastMedian=last,delta=delta,slopePerCycle=slope,withinBudget=passed)
within = all(m['withinBudget'] for m in metrics.values())
assert within == report['memoryAssessment']['withinBudget']
passed = within and stable
assert passed == report['passed']
(args.public / ('memory-' + key + '.json')).write_bytes(raw)
validation = dict(passed=passed,scope='One finite ordinary production offline import screen; shared Windows host, no forced GC or trim; not universal leak proof',
    arch=args.arch,scenario=args.scenario,metrics=metrics,stableMeasuredIdentities=stable,distinctFiles=108,rowsPerFile=rows,
    sourceReportSha256=hashlib.sha256(raw).hexdigest(),binarySha256=binary_sha,elapsedSeconds=report['elapsedSeconds'])
(args.public / ('memory-readback-' + key + '.json')).write_text(json.dumps(validation,indent=2)+'\n',encoding='utf-8')
print(json.dumps(validation))
if not passed:
    raise SystemExit(1)
