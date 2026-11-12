#!/usr/bin/env python3
"""Build/check local manual assets; shared recipe refresh is explicit."""
import argparse,hashlib,json,pathlib
p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--shared-root',type=pathlib.Path);a=p.parse_args()
root=pathlib.Path(__file__).resolve().parents[1];dest=root/'apps/engine-native/internal/inputassist/assets'
if a.shared_root:
 source=a.shared_root/'collection/manual-evidence-v1/recipes.json'
 if a.check:assert source.read_bytes()==(dest/'recipes.json').read_bytes()
 else:(dest/'recipes.json').write_bytes(source.read_bytes())
for lang in ('en','ko'):
 source=root/f'docs/{lang}/INPUT_ASSISTANCE.md';target=dest/f'INPUT_ASSISTANCE.{lang}.md'
 if a.check:assert source.read_bytes()==target.read_bytes(),target
 else:target.write_bytes(source.read_bytes())
recipes=json.loads((dest/'recipes.json').read_text());assert recipes['execution']=='manual_only';assert recipes['schema_version']=='1.0.0'
for recipe in recipes['recipes']:assert set(recipe)>={'id','commands','context','limitations_en','limitations_ko'}
manifest={'asset_id':recipes['asset_id'],'version':'1.0.0','source':'collection/manual-evidence-v1/recipes.json','files':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(dest.iterdir()) if p.name!='manifest.json'}}
body=(json.dumps(manifest,indent=2)+'\n').encode();target=dest/'manifest.json'
if a.check:assert target.read_bytes()==body
else:target.write_bytes(body)
print('input assistance assets verified' if a.check else 'input assistance assets built')
