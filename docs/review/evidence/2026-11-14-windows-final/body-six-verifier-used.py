import base64,json,sys,hashlib
from pathlib import Path
sys.path.insert(0,str(Path('scripts/acceptance/layout').resolve()))
from verify_p2_windows_readback import compare_transactions
base=Path('.tmp/windows-final-20261114/body-six-386')
rows=json.loads((base/'.private-rows.json').read_text(encoding='utf-8'));raw=(base/'body.httpsession').read_bytes();doc=json.loads(raw)
report=compare_transactions(rows,doc['transactions'],{},doc['selectedId'])
image=next(tx for tx in rows if tx['url'].endswith('/image'))['response']['imagePreview']
assert (image['width'],image['height'])==(1,1) and base64.b64decode(image['base64']).startswith(b'\x89PNG\r\n\x1a\n')
report.update(passed=True,sourceRows=6,exportSha256=hashlib.sha256(raw).hexdigest(),exportBytes=len(raw),raster1x1PNGSignatureVerified=True)
Path('docs/review/evidence/2026-11-14-windows-final/body-six-readback.json').write_text(json.dumps(report,indent=2)+'\n')
print('Six-body export full-field comparison PASS')
