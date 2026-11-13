#!/usr/bin/env python3
"""Explicit local Langfuse4.79 observations API export; credentials from environment, body-free fields."""
import argparse,base64,json,os,pathlib,re,urllib.request,urllib.parse
p=argparse.ArgumentParser();p.add_argument('--trace-id',required=True);p.add_argument('--out',required=True);a=p.parse_args()
if not re.fullmatch('[0-9a-f]{32}',a.trace_id) or pathlib.Path(a.out).exists():raise SystemExit('new output and explicit trace id required')
base=os.environ.get('LANGFUSE_HOST','http://127.0.0.1:3000').rstrip('/');url=urllib.parse.urlsplit(base)
if url.scheme!='http' or url.hostname not in ('127.0.0.1','localhost','::1') or url.username or url.password or url.path not in ('','/'):raise SystemExit('local loopback Langfuse host required')
auth=base64.b64encode((os.environ['LANGFUSE_PUBLIC_KEY']+':'+os.environ['LANGFUSE_SECRET_KEY']).encode()).decode()
query=urllib.parse.urlencode({'traceId':a.trace_id,'fields':'core,basic,usage,model','limit':1000})
request=urllib.request.Request(base+'/api/public/v2/observations?'+query,headers={'Authorization':'Basic '+auth})
opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
try:
 with opener.open(request,timeout=30) as response:raw=response.read((64<<20)+1)
except Exception:raise SystemExit('local export failed; credentials and response body omitted')
if len(raw)>64<<20:raise SystemExit('export exceeds64MiB')
f=json.loads(raw)
if not isinstance(f.get('data'),list) or not f['data'] or f.get('meta') not in ({},None):raise SystemExit('expected selected complete observation page; narrow trace/export selection')
# Allowlist normalization before disk: API names/user/session/project/costs and
# arbitrary body attributes are excluded even when the backend adds them.
fields=('id','traceId','parentObservationId','type','startTime','endTime','level','model','usageDetails')
clean={'data':[{k:row[k] for k in fields if k in row} for row in f['data']],'meta':{}}
with pathlib.Path(a.out).open('x',encoding='utf-8') as target:json.dump(clean,target,indent=2);target.write('\n')
