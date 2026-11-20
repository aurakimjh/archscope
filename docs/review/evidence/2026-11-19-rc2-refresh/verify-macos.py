import argparse,hashlib,json,plistlib,subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('root',type=Path);p.add_argument('--output',type=Path,required=True);args=p.parse_args();root=args.root
out=root.parent/(root.name+'-macos');out.mkdir(exist_ok=True);mount=out/'dmg-mount';mount.mkdir(exist_ok=True)
subprocess.run(['ditto','-x','-k',str(root/'archscope-darwin-arm64.zip'),str(out/'zip')],check=True)
zapp=out/'zip/archscope.app'
subprocess.run(['hdiutil','verify',str(root/'archscope-arm64.dmg')],check=True,capture_output=True)
attached=False
try:
 subprocess.run(['hdiutil','attach','-readonly','-nobrowse','-mountpoint',str(mount),str(root/'archscope-arm64.dmg')],check=True,capture_output=True);attached=True
 apps=list(mount.glob('*.app'));assert len(apps)==1
 for app in [zapp,apps[0]]:
  subprocess.run(['codesign','--verify','--deep','--strict',str(app)],check=True,capture_output=True)
  info=plistlib.loads((app/'Contents/Info.plist').read_bytes());assert info['CFBundleShortVersionString']=='0.4.0' and info['CFBundleVersion']=='4002'
 a=(zapp/'Contents/MacOS/archscope').read_bytes();b=(apps[0]/'Contents/MacOS/archscope').read_bytes();assert a==b
 signature=subprocess.run(['codesign','-dv',str(zapp)],check=True,capture_output=True,text=True).stderr
 report={'date':'2026-11-20','dmgChecksumVerified':True,'dmgAndZipBinaryIdentical':True,'binarySha256':hashlib.sha256(a).hexdigest(),'shortVersion':info['CFBundleShortVersionString'],'build':info['CFBundleVersion'],'deepStrictSignatureVerifiedForZipAndDmg':True,'signing':'ad-hoc' if 'Signature=adhoc' in signature else 'other; see hosted signing step','notDeveloperIdOrNotarizationAcceptance':True}
 args.output.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
finally:
 if attached:subprocess.run(['hdiutil','detach',str(mount)],check=True,capture_output=True)
