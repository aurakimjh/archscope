#!/usr/bin/env python3
"""Vendor dependency notices at development time; never downloads at runtime."""
import hashlib,json,pathlib,re,subprocess
root=pathlib.Path(__file__).resolve().parents[1]; app=root/'apps/engine-native'; frontend=app/'cmd/archscope-app/frontend';out=app/'internal/distribution/assets/licenses';out.mkdir(parents=True,exist_ok=True)
raw=subprocess.check_output(['go','list','-deps','-json','./cmd/archscope-engine','./cmd/archscope-app'],cwd=app,text=True);decoder=json.JSONDecoder();mods={}
while raw.strip():
 raw=raw.lstrip();pkg,pos=decoder.raw_decode(raw);raw=raw[pos:];m=pkg.get('Module')
 if m:mods[m['Path']]=m
for old in out.iterdir():
 if old.is_file():old.unlink()
items=[]
def notice(kind,name,version,folder,declared=''):
 texts=[]
 if folder.exists():
  for p in sorted(folder.iterdir()):
   if p.is_file() and re.match(r'^(?:third[-_]party[-_])?(licen[sc]e|copying|notice|copyright)([._-]|$)',p.name,re.I):
    b=p.read_bytes()
    if len(b)>300000:raise SystemExit('license too large: '+name)
    n=hashlib.sha256((kind+':'+name+'/'+p.name).encode()).hexdigest()[:20]+'.txt';(out/n).write_bytes(b);texts.append({'file':n,'upstream_name':p.name,'sha256':hashlib.sha256(b).hexdigest()})
 items.append({'ecosystem':kind,'name':name,'version':version,'declared_license':declared,'notices':texts,'notice_status':'included' if texts else 'no_root_notice_found'})
for m in mods.values():
 if m.get('Main'):continue
 source=m.get('Replace',m);notice('go',m['Path'],m.get('Version',''),pathlib.Path(source.get('Dir','/missing-dependency')))
lock=json.loads((frontend/'package-lock.json').read_text())
for n,v in sorted(lock['packages'].items()):
 if not n or v.get('dev') or not (frontend/n).is_dir():continue
 folder=frontend/n;meta=json.loads((folder/'package.json').read_text()) if (folder/'package.json').exists() else {}
 notice('npm',meta.get('name',n.removeprefix('node_modules/')),v.get('version',''),folder,meta.get('license',''))
notice('project','archscope','0.3.5',root,'MIT')
b=(frontend/'Inter Font License.txt').read_bytes();(out/'inter-OFL.txt').write_bytes(b)
items.append({'ecosystem':'font','name':'Inter','version':'bundled','declared_license':'OFL-1.1','notices':[{'file':'inter-OFL.txt','upstream_name':'Inter Font License.txt','sha256':hashlib.sha256(b).hexdigest()}],'notice_status':'included'})
for item in items:
 if item['name']=='@wailsio/runtime':
  source=pathlib.Path(mods['github.com/wailsapp/wails/v3']['Dir'])/'LICENSE';n='wails-runtime-MIT.txt'
 elif item['name'].startswith('@rolldown/binding-'):
  source=frontend/'node_modules/rolldown/LICENSE';n='rolldown-MIT.txt'
 else:continue
 b=source.read_bytes();(out/n).write_bytes(b);item['notices']=[{'file':n,'upstream_name':source.name,'sha256':hashlib.sha256(b).hexdigest(),'source':'same upstream project root license'}];item['notice_status']='included_upstream_project_root'
manifest={'version':'1.0.0','go_sum_sha256':hashlib.sha256((app/'go.sum').read_bytes()).hexdigest(),'npm_lock_sha256':hashlib.sha256((frontend/'package-lock.json').read_bytes()).hexdigest(),'dependencies':items,'scope':'host compiled Go modules and installed production npm lock entries (including build tooling); missing root notices explicit; platform native/system/runtime licenses require platform distribution review; Noto notices separate'}
(out/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'dependencies':len(items),'without_root_notice':[x['name'] for x in items if not x['notices']]}))
