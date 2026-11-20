import argparse,hashlib,io,json,plistlib,struct,subprocess,tarfile,zipfile
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('root',type=Path);p.add_argument('--metadata',type=Path);p.add_argument('--output',type=Path,required=True);p.add_argument('--phase',required=True);p.add_argument('--source',default='fe92b5bcfd1fdcd3595b30a9d25d2204a8c31937');args=p.parse_args()
root=args.root;sha=args.source
expected={'RELEASE_NOTES.en.md','RELEASE_NOTES.ko.md','SHA256SUMS','archscope-arm64.dmg','archscope-conformance-v1.zip','archscope-darwin-arm64.zip','archscope-docs.zip','archscope-linux-amd64.tar.gz','archscope-windows-amd64.zip','archscope.deb','archscope.rpm'}
files={x.name:x for x in root.iterdir() if x.is_file()};assert set(files)==expected,set(files)^expected
assets=[]
meta=json.loads(args.metadata.read_text()) if args.metadata else None
for name,file in sorted(files.items()):
 digest=hashlib.sha256(file.read_bytes()).hexdigest()
 if meta:
  item=next(x for x in meta['assets'] if x['name']==name);assert item['digest']=='sha256:'+digest,name;assert item['size']==file.stat().st_size,name
 assets.append({'name':name,'bytes':file.stat().st_size,'sha256':digest})
checks={}
for line in files['SHA256SUMS'].read_text().splitlines():
 digest,name=line.split(None,1);name=name.lstrip('* ');assert name in files and name not in checks;assert hashlib.sha256(files[name].read_bytes()).hexdigest()==digest,name;checks[name]=digest
assert set(checks)==expected-{'SHA256SUMS'}
for name,file in files.items():
 if name.endswith('.zip'):
  with zipfile.ZipFile(file) as z:assert z.testzip() is None,name
for lang in ['en','ko']:
 expected_note=subprocess.check_output(['git','show',f'{sha}:docs/{lang}/releases/v0.4.0-rc.2.md'])
 assert files[f'RELEASE_NOTES.{lang}.md'].read_bytes()==expected_note,lang
with zipfile.ZipFile(files['archscope-docs.zip']) as z:
 names={n for n in z.namelist() if not n.endswith('/')}
 tracked=set(subprocess.check_output(['git','ls-tree','-r','--name-only',sha,'docs/en','docs/ko','examples'],text=True).splitlines())
 assert names==tracked,(len(names),len(tracked))
 for name in sorted(names):assert z.read(name)==subprocess.check_output(['git','show',f'{sha}:{name}']),name
 documentation_count=len(names)
extract=root.parent/(root.name+'-binaries');extract.mkdir(exist_ok=True)
binaries={}
def identity(platform,data):
 path=extract/platform;path.write_bytes(data)
 info=subprocess.check_output(['go','version','-m',str(path)],text=True)
 revision=next((line.split('vcs.revision=',1)[1].strip() for line in info.splitlines() if 'vcs.revision=' in line),None)
 if revision is not None:assert revision==sha,(platform,revision)
 assert sha.encode() in data,platform
 binaries[platform]={'sha256':hashlib.sha256(data).hexdigest(),'vcsRevision':revision,'embeddedFrontendBuildCommit':sha,'sourceBinding':'Embedded frontend build identity plus exact hosted checkout SHA; native build may omit Go VCS metadata','buildInfo':info}
with zipfile.ZipFile(files['archscope-darwin-arm64.zip']) as z:
 member=z.getinfo('archscope.app/Contents/MacOS/archscope');assert member.external_attr>>16&0o111
 mac_info=plistlib.loads(z.read('archscope.app/Contents/Info.plist'));assert mac_info['CFBundleShortVersionString']=='0.4.0' and mac_info['CFBundleVersion']=='4002'
 identity('darwin-arm64',z.read(member))
with zipfile.ZipFile(files['archscope-windows-amd64.zip']) as z:
 data=z.read('archscope.exe');off=struct.unpack_from('<I',data,0x3c)[0];assert data[off:off+4]==b'PE\0\0' and struct.unpack_from('<H',data,off+4)[0]==0x8664
 # The native Windows packaging step checks both application and installer
 # version resources; independently retain the downloaded PE identity here.
 identity('windows-amd64',data)
with tarfile.open(files['archscope-linux-amd64.tar.gz']) as t:
 member=t.getmember('archscope');assert member.isfile() and member.mode&0o111
 identity('linux-amd64',t.extractfile(member).read())
# Read Debian control data without installing/executing the package.
data=files['archscope.deb'].read_bytes();assert data[:8]==b'!<arch>\n';off=8;control=None
while off<len(data):
 header=data[off:off+60];size=int(header[48:58]);name=header[:16].decode().strip().rstrip('/');body=data[off+60:off+60+size];off+=60+size+(size%2)
 if name.startswith('control.tar'):
  with tarfile.open(fileobj=io.BytesIO(body),mode='r:*') as t:
   control=t.extractfile(next(x for x in t.getmembers() if x.name.lstrip('./')=='control')).read().decode()
assert control and 'Version: 0.4.0~rc.2-1\n' in control,control
# RPM headers: skip signature header then read version/release string tags.
data=files['archscope.rpm'].read_bytes();assert data[:4]==bytes.fromhex('edabeedb')
def rpm_header(offset):
 assert data[offset:offset+4]==bytes.fromhex('8eade801')
 count,size=struct.unpack_from('>II',data,offset+8);store=offset+16+16*count;values={}
 for i in range(count):
  tag,kind,index,length=struct.unpack_from('>IIII',data,offset+16+16*i)
  if kind==6:values[tag]=data[store+index:].split(b'\0',1)[0].decode()
 return values,store+size
_,end=rpm_header(96);version,end=rpm_header((end+7)//8*8);assert version[1001]+'-'+version[1002]=='0.4.0~rc.2-1',version
report={'date':'2026-11-20','tag':'v0.4.0-rc.2','candidateCommit':sha,'phase':args.phase,'assetCount':len(assets),'assets':assets,'githubDigestsMatched':bool(meta),'checksumEntriesVerified':len(checks),'zipCRCVerified':True,'documentationFilesMatchedToSource':documentation_count,'macos':{'shortVersion':mac_info['CFBundleShortVersionString'],'build':mac_info['CFBundleVersion'],'executableModePreserved':True},'linux':{'debVersion':'0.4.0~rc.2-1','rpmVersion':version[1001]+'-'+version[1002],'executableModePreserved':True},'binaries':binaries,'scope':'Artifact identity, metadata and archive integrity; not clean-host installation, upgrade or new native feature acceptance.'}
args.output.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'phase':args.phase,'assets':len(assets),'checksums':len(checks),'docs':documentation_count,'source':sha,'passed':True}))
