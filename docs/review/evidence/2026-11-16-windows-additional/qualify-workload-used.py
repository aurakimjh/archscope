from pathlib import Path
import os,json,subprocess,hashlib,time
root=Path('C:/workspace/archscope');private=root/'.tmp/windows-acceptance-20261116b';public=root/'docs/review/evidence/2026-11-16-windows-additional';app=root/'apps/engine-native/cmd/archscope-app'
owned=subprocess.check_output(['powershell.exe','-NoProfile','-Command',"@(Get-Process | Where-Object {$_.Path -like 'C:\\workspace\\archscope\\.tmp\\windows-acceptance-20261116b\\archscope-*'}).Count"],text=True).strip()
assert owned=='0','Run only between ordinary native screens'
receipt={'scope':'Headless actual backend projections of the216ordinary-run input files; Windowsamd64/386 and DockerLinuxamd64purego. Not native memory, GTK or live-capture qualification.','testSourceSha256':hashlib.sha256((app/'import_workload_qualification_test.go').read_bytes()).hexdigest(),'runs':{}}
for arch in ['amd64','386']:
 env=os.environ.copy();env['GOARCH']=arch;env['ARCHSCOPE_IMPORT_WORKLOAD_MANIFEST']=str(private/'workload-windows.json');env['ARCHSCOPE_IMPORT_WORKLOAD_OUT']=str(private/f'projection-fixed-{arch}.json')
 code="import { runWindowsGo } from './build/windows/go.mjs'; process.exit(runWindowsGo('test',['-tags','releasequalification','-run','^TestQualifiedImportWorkloadProjection$','-count=1','-v','.']));"
 start=time.time();r=subprocess.run(['node','--input-type=module','-e',code],cwd=app,env=env,capture_output=True)
 (private/f'projection-fixed-{arch}.log').write_bytes(r.stdout+r.stderr)
 receipt['runs'][arch]={'exitCode':r.returncode,'elapsedSeconds':time.time()-start}
 print(arch,r.returncode,flush=True)
 assert r.returncode==0,(r.stdout+r.stderr).decode(errors='replace')[-3000:]
env=os.environ.copy();env.update(GOOS='linux',GOARCH='amd64',CGO_ENABLED='0')
r=subprocess.run(['go','test','-c','-tags','releasequalification,purego','-o',str(private/'projection-linux.test'),'.'],cwd=app,env=env,capture_output=True)
(private/'projection-linux-build.log').write_bytes(r.stdout+r.stderr);assert r.returncode==0,(r.stdout+r.stderr).decode(errors='replace')[-3000:]
args=['docker','run','--rm','--label','archscope.acceptance=windows-acceptance-20261116b','--memory','1g','--cpus','1','--network','none','--cap-drop','ALL','--security-opt','no-new-privileges','--read-only','--tmpfs','/tmp:rw,size=256m','--mount','type=bind,source='+str(root)+',target=/work,readonly','--mount','type=bind,source='+str(private)+',target=/out','-e','ARCHSCOPE_IMPORT_WORKLOAD_MANIFEST=/out/workload-linux.json','-e','ARCHSCOPE_IMPORT_WORKLOAD_OUT=/out/projection-fixed-linux.json','node:22-bookworm-slim','/out/projection-linux.test','-test.v','-test.run=^TestQualifiedImportWorkloadProjection$','-test.count=1']
start=time.time();r=subprocess.run(args,capture_output=True);(private/'projection-fixed-linux.log').write_bytes(r.stdout+r.stderr)
receipt['runs']['linux']={'exitCode':r.returncode,'elapsedSeconds':time.time()-start};print('linux',r.returncode,flush=True)
assert r.returncode==0,(r.stdout+r.stderr).decode(errors='replace')[-3000:]
models=[]
for arch in ['amd64','386','linux']:
 raw=(private/f'projection-fixed-{arch}.json').read_bytes();models.append(json.loads(raw));public.joinpath(f'projection-fixed-{arch}.json').write_bytes(raw)
 log=(private/f'projection-fixed-{arch}.log').read_bytes();public.joinpath(f'projection-{arch}.txt').write_bytes(log)
 receipt['runs'][arch]['reportSha256']=hashlib.sha256(raw).hexdigest();receipt['runs'][arch]['logSha256']=hashlib.sha256(log).hexdigest()
assert models[0]==models[1]==models[2]
assert len(models[0]['files'])==216
receipt.update(passed=True,matchingProjections=216,dockerImageDigest=json.loads(subprocess.check_output(['docker','image','inspect','node:22-bookworm-slim','--format','{{json .RepoDigests}}']))[0],linuxBinarySha256=hashlib.sha256((private/'projection-linux.test').read_bytes()).hexdigest())
public.joinpath('projection-parity.json').write_text(json.dumps(receipt,indent=2)+'\n',encoding='utf-8');print('PASS216exactprojections/fingerprints across3hosts',flush=True)
