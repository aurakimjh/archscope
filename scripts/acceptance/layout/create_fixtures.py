#!/usr/bin/env python3
"""Create project-specific native UI acceptance inputs from sanitized examples."""
import argparse
import copy
import datetime as dt
import json
from pathlib import Path

root = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser()
parser.add_argument('--out', type=Path, required=True)
args = parser.parse_args()
out = args.out.resolve()
out.mkdir(parents=True, exist_ok=True)
(out / 'invalid.json').write_text('{ deliberately invalid JSON fixture', encoding='utf-8')
(out / 'tiny.collapsed').write_text('app.main;app.work 7\n', encoding='utf-8')
(out / 'gc-no-clock.log').write_text(
    '[info][gc] GC(0) Pause Young (G1 Evacuation Pause) 100M->50M(200M) 25.000ms\n'
    '[info][gc] GC(1) Pause Full (System.gc()) 150M->100M(200M) 190.120ms\n', encoding='utf-8')
(out / 'gc-60.log').write_text(''.join(
    f'[{i+1}.000s][info][gc] GC({i}) Pause Young (G1 Evacuation Pause) 100M->50M(200M) {10+i%12}.000ms\n'
    for i in range(60)), encoding='utf-8')
longdir = out
for i in range(5):
    longdir /= str(i) + '-긴경로-검증-' + 'x' * 65
longdir.mkdir(parents=True, exist_ok=True)
longfile = longdir / ('긴파일명-공간과-도움말-검증-' + '한글' * 22 + '.collapsed')
longfile.write_bytes((root / 'examples/profiler/sample-wall.collapsed').read_bytes())
(out / 'long-path.txt').write_text(str(longfile), encoding='utf-8')
pair = out / 'thread-pair'
pair.mkdir(exist_ok=True)
thread = (root / 'examples/thread-dumps/sample-java-thread-dump.txt').read_text(encoding='utf-8')
(pair / '01-before.txt').write_text(thread, encoding='utf-8')
(pair / '02-after.txt').write_text(thread.replace('10:00:00', '10:00:10'), encoding='utf-8')
events = []
for i, (kind, address, size) in enumerate([
    ('Allocation', 1, 1024), ('Allocation', 2, 2048),
    ('Allocation', 3, 512), ('Free', 2, 2048)
]):
    events.append({'type': 'jdk.NativeMemory' + kind, 'values': {
        'startTime': f'2026-04-30T01:00:0{i}Z', 'address': address, 'size': size,
        'stackTrace': {'frames': [{'method': {'type': {'name': 'app.NativeFixture'}, 'name': 'allocate'}}]}}})
(out / 'native-memory.json').write_text(json.dumps({'recording': {'events': events}}), encoding='utf-8')
source = root.parent / 'projects-assets/test-data/har-fixtures/dialects/chrome-h1.har'
har = json.loads(source.read_text(encoding='utf-8'))
original = har['log']['entries']
entries = []
for i in range(30):
    entry = copy.deepcopy(original[i % len(original)])
    entry['startedDateTime'] = (dt.datetime(2026, 11, 10, 12, tzinfo=dt.timezone.utc) + dt.timedelta(seconds=i)).isoformat()
    entry['request']['url'] = f'https://example.test/native-qa/request-{i:02d}'
    entry['response']['headers'] = [
        {'name': 'Content-Type', 'value': 'application/json'},
        {'name': 'X-Native-QA', 'value': str(i)},
        {'name': 'Cache-Control', 'value': 'no-store'},
        {'name': 'Server', 'value': 'fixture'}]
    entry['response']['content'].update(
        mimeType='application/json', size=50,
        text=json.dumps({'request': i, 'status': 'ok', 'lines': ['a', 'b', 'c', 'd']}, indent=2))
    entries.append(entry)
har['log']['entries'] = entries
(out / 'native-30.har').write_text(json.dumps(har), encoding='utf-8')
print(f'Created native UI acceptance fixtures in {out}')
