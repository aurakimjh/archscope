#!/usr/bin/env python3
"""Create a body-free synthetic Sentry transaction using in-memory transport only."""
import argparse,json,pathlib,time
p=argparse.ArgumentParser();p.add_argument('--out',required=True);a=p.parse_args()
if pathlib.Path(a.out).exists():raise SystemExit('output exists')
import sentry_sdk
from sentry_sdk.transport import Transport
from importlib.metadata import version
if version('sentry-sdk')!='2.69.2':raise SystemExit('qualified sentry-sdk2.69.2 required')
captured=[]
class Capture(Transport):
 def capture_envelope(self,envelope):
  for item in envelope.items:
   if item.headers.get('type')=='transaction':captured.append(item.payload.json)

sentry_sdk.init(dsn='http://public@127.0.0.1/1',transport=Capture,traces_sample_rate=1.0,default_integrations=False,auto_enabling_integrations=False,send_default_pii=False,send_client_reports=False,auto_session_tracking=False,server_name='synthetic-host',environment='synthetic',release='synthetic-workload',enable_backpressure_handling=False)
with sentry_sdk.start_transaction(name='synthetic-request',op='http.server') as transaction:
 with sentry_sdk.start_span(op='db',name='synthetic-query'):sum(i*i for i in range(10000))
 with sentry_sdk.start_span(op='http.client',name='synthetic-dependency'):time.sleep(.002)
sentry_sdk.flush()
assert len(captured)==1
def structure(row):
 return {k:row[k] for k in ('trace_id','span_id','parent_span_id','op','start_timestamp','timestamp','status') if k in row}
f=captured[0];out={k:f[k] for k in ('type','start_timestamp','timestamp')};out['sdk']={k:f['sdk'][k] for k in ('name','version')};out['contexts']={'trace':structure(f['contexts']['trace'])};out['spans']=[structure(v) for v in f['spans']]
with pathlib.Path(a.out).open('x',encoding='utf-8') as target:json.dump(out,target,indent=2);target.write('\n')
