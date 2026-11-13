#!/usr/bin/env python3
"""Read-only bounded PRAGMA capture. Output contains no database path/SQL/rows."""
import argparse,json,pathlib,re,sqlite3,time
p=argparse.ArgumentParser();p.add_argument('--database',required=True);p.add_argument('--instance-tag',required=True);p.add_argument('--out',required=True);p.add_argument('--samples',type=int,default=3);p.add_argument('--interval',type=float,default=.2);a=p.parse_args()
if sqlite3.sqlite_version!='3.53.0':raise SystemExit('qualified SQLite version is 3.53.0')
if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}',a.instance_tag) or not 1<=a.samples<=10000 or not .01<=a.interval<=60:raise SystemExit('invalid bounded capture options')
source=pathlib.Path(a.database).resolve(strict=True)
if not source.is_file():raise SystemExit('regular database required')
# mode=ro never creates/changes the target; immutable is deliberately NOT used for a live file.
reader=sqlite3.connect(source.as_uri()+'?mode=ro',uri=True,timeout=5);reader.execute('PRAGMA query_only=ON');snapshots=[]
try:
 for n in range(a.samples):
  if n:time.sleep(a.interval)
  metrics=[]
  for key,unit in [('page_count','{page}'),('freelist_count','{page}'),('page_size','bytes'),('data_version','{version}')]:
   value=reader.execute('PRAGMA '+key).fetchone()[0]
   if type(value) is not int or not 0<=value<2**64:raise SystemExit('invalid gauge value')
   metrics.append({'name':'sqlite.'+key,'unit':unit,'kind':'gauge','temporality':'unspecified','monotonic':False,'value':str(value),'attributes':{'scope':'one read-only connection; file gauges; no SQL text or row content'},'resource':{'instance_id':a.instance_tag,'sqlite_version':sqlite3.sqlite_version},'scope':{'name':'manual.sqlite.snapshot','version':'1.0.0','attributes':{},'dropped_attributes':'0'}})
  snapshots.append({'time_unix_nano':str(time.time_ns()),'metrics':metrics})
finally:reader.close()
with pathlib.Path(a.out).open('x',encoding='utf-8') as f:json.dump({'contract':'sqlite-pragma-snapshots.v1','collector_version':'1.0.0','producer_version':sqlite3.sqlite_version,'history':{'schema_version':'1.0.0','snapshots':snapshots}},f,indent=2);f.write('\n')
