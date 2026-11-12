#!/usr/bin/env python3
"""Manually subset a qualified Nsight SQLite export; never called by import."""
import argparse,hashlib,json,os,pathlib,sqlite3,tempfile,time
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--in',dest='source',required=True);p.add_argument('--out',required=True)
p.add_argument('--from-ns',required=True);p.add_argument('--to-ns',required=True);p.add_argument('--receipt')
a=p.parse_args();source=pathlib.Path(a.source).resolve();output=pathlib.Path(a.out).resolve();receipt=pathlib.Path(a.receipt or str(output)+'.receipt.json').resolve()
def uint(v):
 if len(v)>20 or not v.isascii() or not v.isdecimal() or str(int(v))!=v or int(v)>2**63-1:raise ValueError('canonical signed-range nanoseconds required')
 return int(v)
lo,hi=uint(a.from_ns),uint(a.to_ns)
if lo>=hi:raise ValueError('from must precede to')
if source in (output,receipt) or output==receipt or output.exists() or receipt.exists():raise ValueError('OUTPUT_EXISTS_OR_ALIAS: choose unused output and receipt files')
if source.stat().st_size>2<<30:raise ValueError('SUBSET_INPUT_BUDGET: maximum2GiB; reduce capture with producer tools')
for suffix in ('-wal','-shm','-journal'):
 if pathlib.Path(str(source)+suffix).exists():raise ValueError('standalone quiescent SQLite export required; sidecars refused')
started=time.monotonic()
def deadline():
 if time.monotonic()-started>300:raise TimeoutError('SUBSET_DEADLINE:300 seconds')
def digest(path):
 h=hashlib.sha256()
 with path.open('rb') as f:
  while data:=f.read(1024*1024):deadline();h.update(data)
 return h.hexdigest()
con=sqlite3.connect(source.as_uri()+'?mode=ro',uri=True,timeout=1)
con.execute('PRAGMA query_only=ON');con.execute('PRAGMA trusted_schema=OFF');con.execute('PRAGMA cache_size=-4096');con.set_progress_handler(lambda:int(time.monotonic()-started>300),10000);con.execute('BEGIN')
objects={n:(kind,ddl or '') for n,kind,ddl in con.execute('SELECT name,type,sql FROM sqlite_schema LIMIT 257')}
if len(objects)>256:raise ValueError('SUBSET_SCHEMA_BUDGET:256 objects')
spec={
 'CUPTI_ACTIVITY_KIND_RUNTIME':['start','end','globalTid','correlationId','nameId'],
 'CUPTI_ACTIVITY_KIND_KERNEL':['start','end','globalPid','deviceId','contextId','streamId','correlationId','shortName'],
 'CUPTI_ACTIVITY_KIND_MEMCPY':['start','end','globalPid','deviceId','contextId','streamId','correlationId'],
 'NVTX_EVENTS':['start','end','globalTid','eventType']}
for table in ['META_DATA_EXPORT','StringIds',*spec]:
 if table not in objects:
  if table in ('META_DATA_EXPORT','StringIds'):raise ValueError('qualified metadata and names tables required')
  continue
 kind,ddl=objects[table]
 if kind!='table' or 'CREATE VIRTUAL TABLE' in ddl.upper():raise ValueError('physical supported tables required; views/virtual tables refused')
 if any(r[6] for r in con.execute('PRAGMA table_xinfo("'+table+'")')):raise ValueError('generated/hidden columns refused')
metadata=con.execute("SELECT name,value FROM META_DATA_EXPORT WHERE name IN ('EXPORT_PRODUCT_NAME','EXPORT_PRODUCT_VERSION','EXPORT_SCHEMA_VERSION') LIMIT 4").fetchall()
if len(metadata)!=3 or len({r[0] for r in metadata})!=3:raise ValueError('three unique qualified metadata entries required')
meta=dict(metadata)
if meta!={'EXPORT_PRODUCT_NAME':'NVIDIA Nsight Systems','EXPORT_PRODUCT_VERSION':'2024.2.3.38','EXPORT_SCHEMA_VERSION':'3.11.1'}:raise ValueError('qualified Nsight2024.2.3.38/schema3.11.1 required')
original=digest(source);counts={};names=set();retained=0
with tempfile.TemporaryDirectory(prefix='nsight-subset-',dir=output.parent) as directory:
 temp=pathlib.Path(directory)/'subset.sqlite';dest=sqlite3.connect(temp);dest.execute('CREATE TABLE META_DATA_EXPORT(name TEXT,value TEXT)');dest.executemany('INSERT INTO META_DATA_EXPORT VALUES(?,?)',meta.items());dest.execute('CREATE TABLE StringIds(id INTEGER PRIMARY KEY,value TEXT)')
 for table,columns in spec.items():
  if table not in objects:counts[table]={'status':'absent','retained_rows':0};continue
  actual={r[1] for r in con.execute('PRAGMA table_info("'+table+'")')}
  if not set(columns)<=actual:raise ValueError('unsupported column layout in '+table)
  fields=','.join('"'+v+'"' for v in columns);dest.execute('CREATE TABLE "'+table+'"('+','.join('"'+v+'" INTEGER' for v in columns)+')')
  rows=con.execute('SELECT rowid,'+fields+' FROM "'+table+'" WHERE start IS NOT NULL AND end IS NOT NULL AND start < ? AND (end > ? OR (start=end AND start>=?)) ORDER BY start,rowid LIMIT 50001',(hi,lo,lo))
  count=0
  for row in rows:
   deadline();retained+=1
   if retained>50000:raise ValueError('SUBSET_EVENT_BUDGET:at_least50001; choose a smaller window')
   if any(type(v) is not int for v in row):raise ValueError('integer serialized fields required')
   if row[1]<0 or row[2]<row[1]:raise ValueError('invalid interval')
   dest.execute('INSERT INTO "'+table+'"(rowid,'+fields+') VALUES('+','.join('?' for _ in row)+')',row);count+=1
   for field in ('nameId','shortName'):
    if field in columns:names.add(row[1+columns.index(field)])
  counts[table]={'status':'present','retained_rows':count}
 for identity in sorted(names):
  rows=con.execute('SELECT value FROM StringIds WHERE id=? LIMIT 2',(identity,)).fetchall()
  if len(rows)!=1 or not isinstance(rows[0][0],str) or len(rows[0][0].encode())>1024:raise ValueError('one bounded technical name per referenced StringId required')
  dest.execute('INSERT INTO StringIds VALUES(?,?)',(identity,rows[0][0]))
 dest.commit();dest.close()
 if temp.stat().st_size>64<<20:raise ValueError('SUBSET_OUTPUT_BUDGET:64MiB; choose smaller window')
 if original!=digest(source):raise ValueError('SOURCE_CHANGED: retry with a quiescent standalone export')
 proof={'tool':'manual-nsight-subset','version':'1.0.0','original_sha256':original,'original_bytes':source.stat().st_size,'subset_sha256':digest(temp),'subset_bytes':temp.stat().st_size,'from_ns':a.from_ns,'to_ns':a.to_ns,'selection':'whole intersecting intervals, including zero-duration points; no clipping','retained_rows':retained,'tables':counts,'excluded':'environment/session/command-line/NVTX text/unreferenced strings; source DDL never executed','scope':'manual preprocessing; import source digest identifies subset bytes, not original report'}
 note=pathlib.Path(directory)/'receipt.json';note.write_text(json.dumps(proof,indent=2)+'\n');os.chmod(temp,0o600);os.chmod(note,0o600)
 published=False
 try:
  os.link(temp,output);published=True;os.link(note,receipt)
 except BaseException:
  if published:output.unlink()
  raise
con.close();print(json.dumps(proof))
