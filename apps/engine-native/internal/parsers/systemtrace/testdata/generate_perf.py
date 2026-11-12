import struct,pathlib
q=lambda *x:struct.pack('<'+'Q'*len(x),*x)
def record(kind,body,misc=0):return struct.pack('<IHH',kind,misc,len(body)+8)+body
attr=bytearray(128);struct.pack_into('<IIQQQQQ',attr,0,1,128,0,1000000,1|2|4|32|128|256,0,0)
# software cpu-clock: explicit sample period is nanoseconds, never guessed by importer.
records=record(3,struct.pack('<II',123,124)+b'synthetic\0'+b'\0'*6)
for i,period in enumerate((1000000,2000000,3000000)):
 body=q(0x400000+i*16)+struct.pack('<II',123,124)+q(1000000001+i*1000000000)+struct.pack('<II',i%2,0)+q(period)+q(2,0x400000+i*16,0x400100)
 records+=record(9,body,2)
header=q(0x32454c4946524550,104,144,104,144,248,len(records),0,0,0,0,0,0)
pathlib.Path('/fixtures').mkdir(exist_ok=True);pathlib.Path('/fixtures/perf.data').write_bytes(header+attr+q(0,0)+records)
