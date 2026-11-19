import pathlib, subprocess, time, json
p=pathlib.Path('/private/tmp/archscope-jvm-repro')
(p/'Loop.java').write_text('public class Loop { public static void main(String[] a) throws Exception { Thread.sleep(180000); } }')
proc=subprocess.Popen(['java',str(p/'Loop.java')],stdout=subprocess.DEVNULL,stderr=(p/'java.stderr').open('w'))
try:
    time.sleep(2)
    for kind, args in [('jcmd',['jcmd',str(proc.pid),'Thread.print','-l']),('jstack',['jstack','-l',str(proc.pid)])]:
        with (p/(kind+'.log')).open('w') as out:
            for i in range(20):
                start=time.monotonic()
                result=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,check=True)
                out.write(result.stdout+'\n')
                time.sleep(max(0,1-(time.monotonic()-start)))
    print(json.dumps({'captures_per_file':20,'tools':['jcmd Thread.print -l','jstack -l'],'directory':str(p)}))
finally:
    proc.terminate()
    proc.wait(timeout=10)
