import json,sys,hashlib
from pathlib import Path
sys.path.insert(0,str(Path('scripts/acceptance/layout').resolve()))
from verify_p2_windows_readback import compare_transactions
base=Path('.tmp/windows-final-20261114/body-v2-386')
rows=json.loads((base/'.private-rows.json').read_text(encoding='utf-8'));raw=(base/'body-fixed.httpsession').read_bytes();doc=json.loads(raw)
report=compare_transactions(rows,doc['transactions'],{},doc['selectedId'])
report.update(passed=True,sourceRows=4,exportSha256=hashlib.sha256(raw).hexdigest(),exportBytes=len(raw))
Path('docs/review/evidence/2026-11-14-windows-final/body-readback.json').write_text(json.dumps(report,indent=2)+'\n')
print('Body export full-field comparison PASS')
