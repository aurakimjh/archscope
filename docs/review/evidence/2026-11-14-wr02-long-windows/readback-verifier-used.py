"""Independent long-run/clear checks; raw sessions, credentials and IDs stay private."""
import argparse, hashlib, json, sys
from pathlib import Path
from urllib.parse import urlsplit, parse_qs

ROOT = Path('C:/workspace/archscope')
BASE = ROOT / '.tmp/wr02-long-20261114'
PUBLIC = ROOT / 'docs/review/evidence/2026-11-14-wr02-long-windows'
sys.path.insert(0, str(ROOT / 'scripts/acceptance/layout'))
from verify_p2_windows_readback import compare_transactions
if not __debug__:
    raise RuntimeError('Run this receipt verifier without Python optimization')
def read(p): return json.loads(p.read_text(encoding='utf-8-sig'))
def digest(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def verify(arch):
    folder = BASE / arch
    r = read(folder / 'result.json')
    assert r['schema'] == 4 and r['passed'] and 'failure' not in r
    assert r['requestedSeconds'] == 7200 and r['elapsedSeconds'] >= 7200
    assert r['buildCommit'] == '324a8d97432a8f9a144cb48b545cc7cb8f21da08'
    assert r['binarySha256'] == digest(ROOT / f'.tmp/p2-windows-20261114/archscope-p2-{arch}.exe')
    assert r['pauses'] >= 119 and r['clears'] == len(r['clearChecks']) >= 11
    assert len(r['checkpoints']) >= 241
    boundary = 0
    for generation, clear in enumerate(r['clearChecks'], 1):
        before, empty, refilled = (clear[k] for k in ('before', 'empty', 'refilled'))
        assert generation * 600 <= clear['elapsedSeconds'] < generation * 600 + 30
        assert clear['clearedThroughRequests'] > boundary
        assert before['generation'] == generation - 1
        assert before['captured'] == before['persisted'] == clear['clearedThroughRequests'] - boundary
        assert all(v['state'] == 'running' and v['dropped'] == 0 for v in (before, empty, refilled))
        assert empty['generation'] == refilled['generation'] == generation
        assert empty['captured'] == empty['persisted'] == empty.get('retained', 0) == 0
        assert refilled['captured'] == refilled['persisted'] == refilled['retained'] == 4
        boundary = clear['clearedThroughRequests']
    previous = -1
    for p in r['checkpoints']:
        assert p['elapsedSeconds'] >= previous
        previous = p['elapsedSeconds']
        applicable = [c for c in r['clearChecks'] if c['elapsedSeconds'] <= p['elapsedSeconds']]
        generation = len(applicable)
        cleared = applicable[-1]['clearedThroughRequests'] if applicable else 0
        stats = p['stats']
        assert stats['state'] == 'running' and stats['generation'] == generation and stats['dropped'] == 0
        assert stats['captured'] == stats['persisted'] == p['requests'] - cleared
        assert p['checkedRows'] == p['retainedRows'] == stats.get('retained', 0) <= 128
        assert p['hasMainWindow'] and p['Responding'] and p['PrivateMemorySize64'] <= 1073741824
    stats = r['finalStats']
    assert stats['state'] == 'finalized' and stats['generation'] == r['clears'] and stats['dropped'] == 0
    assert stats['captured'] == stats['persisted'] == r['requests'] - boundary > 0
    file = folder / 'final.httpsession'
    saved = read(file)
    rows = saved['transactions']
    store = Path(read(folder / '.private-session.json')['storePath'])
    original = [read(p)['transaction'] for p in (store / 'rolling').glob('*.json')]
    assert len(rows) == len({tx['id'] for tx in rows}) == r['savedRows'] == min(128, stats['persisted'])
    annotation_file = store / 'annotations.json'
    annotations = read(annotation_file) if annotation_file.exists() else {}
    comparison = compare_transactions(original, rows, annotations, saved['selectedId'])
    assert digest(file) == r['savedSessionSha256']
    assert saved['selectedId'] == rows[0]['id']
    assert saved['retention']['persisted'] == stats['persisted']
    assert saved['retention']['retained'] == len(rows)
    assert saved['retention']['evicted'] == stats['persisted'] - len(rows)
    sequences = []
    for tx in rows:
        url = urlsplit(tx['url'])
        assert url.scheme == 'http' and url.netloc == '127.0.0.1:11511' and url.path == '/soak'
        sequence = int(parse_qs(url.query)['sequence'][0])
        assert boundary <= sequence < r['requests']
        assert tx['statusCode'] == 200 and tx['state'] == 'complete' and tx['process']['attribution'] == 'confirmed'
        sequences.append(sequence)
    assert sorted(sequences) == list(range(r['requests'] - len(rows), r['requests']))
    assert len({(tx['process']['key']['pid'], tx['process']['key']['startTime']) for tx in rows}) == 1
    memory = {}
    for key in ['PrivateMemorySize64', 'WorkingSet64', 'TreePrivateMemorySize64', 'TreeWorkingSet64', 'HandleCount', 'TreeProcessCount']:
        values = [p[key] for p in r['checkpoints']]
        memory[key] = {'first': values[0], 'last': values[-1], 'min': min(values), 'max': max(values)}
    return {'schemaVersion': 2, 'scope': 'long-soak', 'arch': arch, 'passed': True, 'seconds': r['elapsedSeconds'], 'requests': r['requests'],
        'pauses': r['pauses'], 'clears': r['clears'], 'checkpoints': len(r['checkpoints']),
        'allClearBoundariesAndGenerationsVerified': True, 'allCheckpointCountersAndPaginationVerified': True,
        'finalState': stats['state'], 'finalGenerationCaptured': stats['captured'], 'finalGenerationPersisted': stats['persisted'],
        'lastClearThroughRequests': boundary, 'retainedRows': len(rows), 'selectedSequence': rows[0]['sequence'],
        'selectedURL': rows[0]['url'], **comparison,
        'noPreClearURLsInFinalExport': True, 'savedSessionSha256': digest(file), 'savedSessionBytes': file.stat().st_size,
        'idSetSha256': hashlib.sha256(json.dumps(sorted(tx['id'] for tx in rows)).encode()).hexdigest(),
        'memoryObservations': memory, 'memoryScope': r['memoryScope'],
        'limitations': 'Root 1GiB guard and observed samples passed; no leak-free or universal memory acceptance claim. Concurrent isolated x64/x86 instances share the host and Docker origin.'}

if __name__ == '__main__':
    parser = argparse.ArgumentParser(); parser.add_argument('arch', choices=['amd64', '386']); args = parser.parse_args()
    result = verify(args.arch)
    PUBLIC.mkdir(exist_ok=True)
    with (PUBLIC / f'readback-{args.arch}.json').open('x', encoding='utf-8', newline='\n') as output:
        output.write(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(result, indent=2))
