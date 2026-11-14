"""Probe the received readback comparator without opening private stores."""
import json
import runpy
from pathlib import Path

source = Path(__file__).resolve().parent.parent / '2026-11-14-p2-windows/readback-verifier-used.py'
normalize = runpy.run_path(str(source))['normalized']
original = {'url': 'http://127.0.0.1:11510/lookup?next=/detail?id=7',
            'sequence': 2, 'starred': True, 'comment': 'synthetic-note'}
changed = {**original, 'starred': False, 'comment': 'changed-note'}
print(json.dumps({'scope': 'Received comparison function only; no product failure injected',
                  'differentAnnotationValuesCompareEqual': normalize(original) == normalize(changed),
                  'excludedFields': sorted(set(original) - set(normalize(original)))}, indent=2))
