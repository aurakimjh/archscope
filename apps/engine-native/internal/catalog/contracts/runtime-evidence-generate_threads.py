import sys,pathlib,threading,faulthandler,subprocess,time,re,json,sys
out=pathlib.Path(sys.argv[1]).resolve();tools=pathlib.Path(sys.argv[2]).resolve()
event=threading.Event();threads=[threading.Thread(target=event.wait,name=n) for n in ['synthetic-w1','synthetic-w2']]
for t in threads:t.start()
p=tools/'faulthandler-raw.txt'
with p.open('w') as f:faulthandler.dump_traceback(file=f,all_threads=True)
event.set()
for t in threads:t.join()
raw=p.read_text();raw=re.sub(r'File "[^"]*[/\\]([^/\\"]+)"',r'File "/sanitized/\1"',raw)
(out/'faulthandler-3.14.4.txt').write_text(raw);p.unlink()
jdk=pathlib.Path(sys.argv[3]).resolve()/'bin';source=tools/'SyntheticBlocked.java';source.write_text('''public class SyntheticBlocked {static final Object lock=new Object();public static void main(String[]args)throws Exception{Thread owner=new Thread(()->{synchronized(lock){try{Thread.sleep(30000);}catch(Exception e){}}},"synthetic-owner");owner.setDaemon(true);owner.start();Thread.sleep(50);Thread blocked=new Thread(()->{synchronized(lock){}},"synthetic-blocked");blocked.setDaemon(true);blocked.start();System.out.println(ProcessHandle.current().pid());Thread.sleep(30000);}}''');subprocess.run([str(jdk/'javac'),str(source)],check=True)
child=subprocess.Popen([str(jdk/'java'),'-cp',str(tools),'SyntheticBlocked'],stdout=subprocess.PIPE,text=True)
try:
 pid=child.stdout.readline().strip();dumps=[]
 for i in range(2):
  text=subprocess.check_output([str(jdk/'jstack'),'-l',pid],text=True);text=re.sub(r'"([^"\n]+)"',lambda m:'"'+m[1]+'"',text);(out/('jdk21-blocked-'+str(i+1)+'.txt')).write_text(text);dumps.append(text)
finally:child.terminate();child.wait(timeout=5)
# Explicit synthetic lifetime declarations; the producer does not observe starts.
ids=sorted(set(re.findall(r'^"synthetic-(?:blocked|owner)"[^\n]*\btid=(0x[0-9a-fA-F]+)',dumps[0],re.M)))
context={'schema_version':'1.0.0','source_format':'java_jstack','host_id':'synthetic-host','boot_id':'synthetic-boot','pid_namespace':'synthetic-ns','pid':'500','process_start_ns':'0','threads':[{'runtime_id':i,'thread_start_ns':'0'} for i in ids]}
(out/'jdk21-correlation-context.json').write_text(json.dumps(context,indent=2)+'\n')
print('actual faulthandler',sys.version.split()[0],len(threads)+1,'threads; actual JDK21 dumps, runtime IDs',len(ids))
