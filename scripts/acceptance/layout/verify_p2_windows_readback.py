"""Read-only P2 Windows store/export verification; no native UI verdict.

Store annotations.json is authoritative for live-session exports made without
request-level annotation overrides. Missing annotation fields mean false/empty,
matching the Go zero values and omitempty encoding. No fields are excluded.
"""
import argparse
from copy import deepcopy
import hashlib
import json
from pathlib import Path
from urllib.parse import urlsplit


class ReadbackMismatch(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise ReadbackMismatch(message)


def read(path):
    return json.loads(path.read_text(encoding='utf-8-sig'))


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def annotation(value):
    require(isinstance(value, dict), 'invalid annotation object')
    starred, comment = value.get('starred', False), value.get('comment', '')
    require(type(starred) is bool and isinstance(comment, str), 'invalid annotation types')
    return {'starred': starred, 'comment': comment}


def index_rows(rows):
    require(isinstance(rows, list) and bool(rows), 'empty or invalid transaction list')
    result = {}
    for row in rows:
        require(isinstance(row, dict) and isinstance(row.get('id'), str) and bool(row['id']),
                'missing transaction identity')
        require(row['id'] not in result, 'duplicate transaction identity')
        result[row['id']] = row
    return result


def compare_transactions(source, exported, annotations, selected_id):
    originals, actuals = index_rows(source), index_rows(exported)
    require(originals.keys() == actuals.keys(), 'transaction identity sets differ')
    require(selected_id in actuals, 'selected transaction is absent')
    require(isinstance(annotations, dict), 'invalid annotation map')
    # Validate orphan entries too, but never attach them to a retained row.
    for value in annotations.values():
        annotation(value)
    for ident, original in originals.items():
        expected = deepcopy(original)
        parsed = urlsplit(original['url'])
        expected.update(host=parsed.netloc, query=parsed.query)
        # ExportCaptureFile replaces inline annotation fields from this map,
        # even when the map has no entry for the transaction.
        expected.update(annotation(annotations.get(ident, {})))
        actual = deepcopy(actuals[ident])
        actual.update(annotation(actual))
        # Go omitempty can omit the two URL fields when their value is empty.
        actual.setdefault('host', '')
        actual.setdefault('query', '')
        require(expected == actual, 'transaction fields differ under declared comparison policy')
    return {
        'comparisonPolicyVersion': '1.0.0',
        'allTransactionFieldsMatchDeclaredPolicy': True,
        'annotationsMatch': True,
        'rows': len(actuals),
        'comparisonPolicy': {
            'excludedFields': [],
            'expectedURLFields': 'host/query derived from source retained URL; exported fields are checked',
            'annotationSource': 'store annotations.json replaces inline live-row starred/comment by transaction ID',
            'missingAnnotationDefaults': {'starred': False, 'comment': ''},
            'annotationTypes': {'starred': 'boolean', 'comment': 'string'},
            'requestAnnotationOverrides': 'not supported by this verification scope',
        },
    }


def verify(run, kind, arch):
    run = Path(run).resolve()
    session = read(run / '.private-session.json')
    store = Path(session['storePath']).resolve()
    require(store.is_relative_to(run / '.archscope-captures'), 'store is outside isolated run')
    source = [read(p)['transaction'] for p in sorted((store / 'rolling').glob('*.json'))]
    try:
        annotations = read(store / 'annotations.json')
    except FileNotFoundError:
        annotations = {}
    raw = (run / 'final.httpsession').read_bytes()
    doc = json.loads(raw)
    rows = doc['transactions']
    result = compare_transactions(source, rows, annotations, doc['selectedId'])
    require(all(tx['statusCode'] == 200 and tx['state'] == 'complete'
                and tx['process']['attribution'] == 'confirmed' for tx in rows),
            'unexpected response/state/process attribution')
    selected = next(tx for tx in rows if tx['id'] == doc['selectedId'])
    result.update(schemaVersion=2, arch=arch, scope=kind, selectedSequence=selected['sequence'],
                  fileSha256=digest(raw), fileBytes=len(raw), retention=doc['retention'])
    if kind == 'query':
        seed = read(run / 'seed.json')
        require(seed['requests'] == len(rows) == 4, 'query request count differs')
        require([tx['url'] for tx in rows] == seed['retainedURLs'] and selected['sequence'] == 2,
                'query URL ordering/selection differs')
        require(selected['url'].endswith('/lookup?next=/detail?id=7&keep=2'), 'nested query differs')
        require(all(not tx.get('query') for tx in read(run / '.private-rows.json')),
                'expected empty live query fallback')
        stopped = read(run / 'command-02-stop.json.done')
        stats = stopped['stats']
        require(stopped['state'] == 'finalized' and stats['persisted'] == stats['captured'] == 4
                and stats['dropped'] == 0, 'query did not stop cleanly')
        result.update(separateLiveQueryEmpty=True, retainedURLSpellingExact=True)
    else:
        soak = read(run / 'result.json')
        require(soak['schema'] == 4 and soak['passed'] and soak['requestedSeconds'] == 130
                and soak['elapsedSeconds'] >= 130, 'unexpected short-soak result')
        require(soak['pauses'] >= 2 and soak['clears'] == 0 and bool(soak['checkpoints']),
                'unexpected short-soak coverage')
        for point in soak['checkpoints']:
            stats = point['stats']
            require(stats['state'] == 'running' and stats['captured'] == stats['persisted']
                    and stats['dropped'] == 0, 'unhealthy soak checkpoint')
            require(point['checkedRows'] == point['retainedRows'] <= 128
                    and point['hasMainWindow'] and point['Responding'], 'invalid retained/liveness checkpoint')
        stats = soak['finalStats']
        require(stats['state'] == 'finalized' and stats['captured'] == stats['persisted']
                and stats['dropped'] == 0, 'unhealthy final soak counters')
        require(soak['savedRows'] == len(rows) == 128 and soak['savedSessionSha256'] == digest(raw),
                'saved soak bytes/count differ')
        retention = doc['retention']
        require(retention['persisted'] == stats['persisted'] and retention['retained'] == 128,
                'soak retention differs')
        require(selected['id'] == rows[0]['id'] and len({(tx['process']['key']['pid'],
                tx['process']['key']['startTime']) for tx in rows}) == 1, 'soak identity/selection differs')
        require(all(urlsplit(tx['url']).netloc == '127.0.0.1:11510'
                    and urlsplit(tx['url']).path == '/soak' for tx in rows), 'unexpected soak origin')
        result.update(elapsedSeconds=soak['elapsedSeconds'], requests=soak['requests'],
                      pauses=soak['pauses'], clears=0, checkpoints=len(soak['checkpoints']),
                      finalState=stats['state'], finalCountersEqual=True)
    result['passed'] = True
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run-dir', type=Path, required=True)
    parser.add_argument('--kind', choices=['query', 'soak'], required=True)
    parser.add_argument('--arch', choices=['amd64', '386'], required=True)
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    try:
        result = verify(args.run_dir, args.kind, args.arch)
        # Never overwrite an older evidence report, even on successful recheck.
        with args.report.open('x', encoding='utf-8', newline='\n') as output:
            json.dump(result, output, ensure_ascii=False, indent=2)
            output.write('\n')
    except (ValueError, KeyError, TypeError, OSError, StopIteration):
        parser.exit(1, 'FAIL: readback validation or exclusive report publication failed; no private values printed\n')
    print(f"PASS: {result['rows']} rows; all fields including annotations match declared policy")


if __name__ == '__main__':
    main()
