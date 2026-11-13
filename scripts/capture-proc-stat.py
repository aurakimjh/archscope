#!/usr/bin/env python3
"""Explicit Docker Linux aggregate /proc/stat export; no process/container attribution."""
import argparse,json,platform,pathlib,os,time,re
p=argparse.ArgumentParser();p.add_argument('--out',required=True);p.add_argument('--host-tag',required=True);p.add_argument('--samples',type=int,default=3);p.add_argument('--interval',type=float,default=.2);a=p.parse_args()
if platform.release()!='7.0.14-linuxkit' or os.sysconf('SC_CLK_TCK')!=100:raise SystemExit('qualified Docker Linux 7.0.14-linuxkit / USER_HZ100 required')
if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}',a.host_tag) or not 1<=a.samples<=10000 or not .01<=a.interval<=60:raise SystemExit('invalid bounded capture options')
fields=['user','nice','system','idle','iowait','irq','softirq','steal','guest','guest_nice']
snapshots=[]
for n in range(a.samples):
 if n:time.sleep(a.interval)
 values=pathlib.Path('/proc/stat').read_text().splitlines()[0].split()
 assert values[0]=='cpu' and len(values)>=11
 metrics=[]
 for field,value in zip(fields,values[1:11]):
  assert value.isdecimal()
  metrics.append({'name':'linux.proc.stat.cpu.'+field,'unit':'{tick}','kind':'gauge','temporality':'unspecified','monotonic':False,'value':value,'attributes':{'user_hz':str(os.sysconf('SC_CLK_TCK')),'scope':'Docker shared kernel aggregate; not container or process CPU; guest overlaps user and iowait may decrease'},'resource':{'host_id':a.host_tag,'kernel_release':platform.release()},'scope':{'name':'manual.linux.proc-stat','version':'1.0.0','attributes':{},'dropped_attributes':'0'}})
 snapshots.append({'time_unix_nano':str(time.time_ns()),'metrics':metrics})
with pathlib.Path(a.out).open('x',encoding='utf-8') as f:json.dump({'contract':'linux-proc-stat.v1','collector_version':'1.0.0','producer_version':platform.release(),'history':{'schema_version':'1.0.0','snapshots':snapshots}},f,indent=2);f.write('\n')
