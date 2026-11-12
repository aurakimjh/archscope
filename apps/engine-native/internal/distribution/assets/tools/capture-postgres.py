#!/usr/bin/env python3
"""Explicit read-only PostgreSQL16.14 wrapper collection; never invoked by import.
Use standard libpq environment/pgpass configuration; no credentials in arguments/output.
"""
import argparse
import json
import pathlib
import re
import subprocess
import time

SNAPSHOT_SQL = "WITH observed AS MATERIALIZED (\n SELECT * FROM pg_stat_database WHERE datname=current_database()\n)\nSELECT json_build_object(\n 'capture_start_ns', (extract(epoch FROM statement_timestamp())*1000000000)::numeric(30,0)::text,\n 'capture_end_ns', (extract(epoch FROM clock_timestamp())*1000000000)::numeric(30,0)::text,\n 'postmaster_start_ns',(extract(epoch FROM pg_postmaster_start_time())*1000000000)::numeric(30,0)::text,\n 'stats_reset_ns',(extract(epoch FROM stats_reset)*1000000000)::numeric(30,0)::text,\n 'numbackends',numbackends::text,\n 'xact_commit',xact_commit::text,'xact_rollback',xact_rollback::text,\n 'blks_read',blks_read::text,'blks_hit',blks_hit::text,\n 'tup_returned',tup_returned::text,'tup_fetched',tup_fetched::text,\n 'tup_inserted',tup_inserted::text,'tup_updated',tup_updated::text,'tup_deleted',tup_deleted::text,\n 'conflicts',conflicts::text,'temp_files',temp_files::text,'temp_bytes',temp_bytes::text,\n 'deadlocks',deadlocks::text,'sessions',sessions::text\n) FROM observed"
QUERY = "BEGIN READ ONLY; SET LOCAL stats_fetch_consistency=snapshot; SELECT json_build_object('server_version_num',current_setting('server_version_num'),'track_counts',current_setting('track_counts')::boolean,'database_oid',(SELECT oid::text FROM pg_database WHERE datname=current_database()),'snapshot',("+SNAPSHOT_SQL+")); COMMIT;"

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out',required=True)
    for key in ('host-id','boot-id','instance-id','database-tag'):p.add_argument('--'+key,required=True)
    p.add_argument('--samples',type=int,default=3)
    p.add_argument('--interval',type=float,default=1)
    p.add_argument('--psql',default='psql',help='local psql executable; connection uses libpq environment')
    a=p.parse_args()
    if not 1<=a.samples<=256 or not .2<=a.interval<=60:p.error('samples1..256 / interval0.2..60')
    for value in (a.host_id,a.boot_id,a.instance_id,a.database_tag):
        if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}',value):p.error('sanitized local tags required')
    if pathlib.Path(a.out).exists():p.error('output already exists')
    capture={'contract':'postgres-stat-database.v1','collector_version':'1.0.0','postgres_version':'16.14','server_version_num':'160014','stats_fetch_consistency':'snapshot','track_counts':True,'host_id':a.host_id,'boot_id':a.boot_id,'instance_id':a.instance_id,'database_tag':a.database_tag,'snapshots':[]}
    for i in range(a.samples):
        if i:time.sleep(a.interval)
        result=subprocess.run([a.psql,'-X','-w','-A','-t','-q','-v','ON_ERROR_STOP=1','-c',QUERY],capture_output=True,timeout=30)
        if result.returncode or len(result.stdout)>65536:raise SystemExit('PSQL_CAPTURE_FAILED: check local connection/permissions; connection details omitted')
        value=json.loads(result.stdout)
        if value['server_version_num']!='160014' or value['track_counts'] is not True:raise SystemExit('POSTGRES_CAPTURE_VERSION_OR_STATS: PostgreSQL16.14 / track_counts required')
        if i and value['database_oid']!=capture['database_oid']:raise SystemExit('POSTGRES_CAPTURE_DATABASE_CHANGED')
        capture['database_oid']=value['database_oid'];capture['snapshots'].append(value['snapshot'])
    with open(a.out,'x',encoding='utf-8') as f:json.dump(capture,f,indent=2);f.write('\n')
    print('Saved read-only capture; NULL reset remains unknown. No reset performed.')

if __name__=='__main__':main()
