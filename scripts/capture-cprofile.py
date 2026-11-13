#!/usr/bin/env python3
"""Capture an owned synthetic workload; never opens untrusted .pstats/pickle."""
import argparse,cProfile,decimal,json,pathlib,platform
p=argparse.ArgumentParser();p.add_argument('--out',required=True);args=p.parse_args()
if platform.python_version()!='3.14.4':raise SystemExit('qualified version requires CPython 3.14.4')

def leaf(n):
    return sum(i*i for i in range(n))

def workload():
    return [leaf(2500) for _ in range(20)]

prof=cProfile.Profile();prof.runcall(workload)
rows=[]
for e in prof.getstats():
    code=e.code
    if isinstance(code,str) or code.co_name not in ('workload','leaf','<genexpr>','<listcomp>'):continue
    rows.append({'name':code.co_name,'self_seconds':str(e.inlinetime),'cumulative_seconds':str(e.totaltime),'calls':str(e.callcount),'recursive_calls':str(e.reccallcount)})
rows.sort(key=lambda r:r['name'])
out=pathlib.Path(args.out)
with out.open('x',encoding='utf-8') as f:
    json.dump({'producer':'CPython cProfile','version':platform.python_version(),'scope':'synthetic workload, function flat self time only; no reconstructed stacks or timeline','functions':rows},f,indent=2);f.write('\n')
