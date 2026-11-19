from pathlib import Path
import json,subprocess
p=Path('/private/tmp/archscope-jvm-repro')
for version in ['v2','v3']:
    text=(p/f'async-profiler-{version}-template.html').read_text()
    for a,b in {'/*height:*/300':'96','/*depth:*/0':'3','/*title:*/':'JVM importer regression','/*highlight:*/':''}.items(): text=text.replace(a,b)
    if version=='v2': frames="f(0,0,10,0,'all')\nf(1,0,6,1,'main')\nf(2,0,3,1,'compute')\nf(2,3,3,1,'render')\nf(1,6,4,1,'worker')\nf(2,6,4,1,'poll')\n"
    else:
        text=text.replace('/*cpool:*/',"'all',\n' main',\n' compute',\n' render',\n' worker',\n' poll'")
        frames='f(0,0,0,10)\nu(8,6)\nu(16,3)\nn(24)\nf(32,1,3,4)\nu(40)\n'
    path=p/f'async-profiler-{version}.html'
    path.write_text(text.replace('/*frames:*/',frames))
    subprocess.run([str(p/'archscope-engine'),'profiler','analyze-flamegraph-html','--in',str(path),'--out',str(p/f'{version}-result.json')],check=True)
for name in ['jcmd','jstack','v2','v3']:
    r=json.loads((p/f'{name}-result.json').read_text())
    print(json.dumps({'case':name,'type':r['type'],'summary':r['summary'],'errors':r['metadata']['diagnostics']['error_count']},ensure_ascii=False))
