"""CPython 3.14.4 stdlib의 실제 API를 이용하는 합성 fixture 생성기. 외부 수집 없음."""
import asyncio,gc,json,pathlib,sys,threading,time,tracemalloc
BASE=pathlib.Path(__file__).parent
assert sys.version.split()[0]=='3.14.4'
def stack(frames):
 return [{'symbol':f.f_code.co_name,'file':'/synthetic/'+pathlib.Path(f.f_code.co_filename).name,'line':f.f_lineno,'language':'Python'} for f in frames]
async def parked(gate): await gate.wait()
async def main():
 tracemalloc.start(5);allocated=[bytearray(512+i) for i in range(8)];gate=asyncio.Event();task=asyncio.create_task(parked(gate),name='synthetic-waiter');await asyncio.sleep(0)
 tasks=[{'id':'task-'+str(i),'name':t.get_name() if t is task else 'synthetic-main','state':'done' if t.done() else 'pending','stack':stack(t.get_stack())} for i,t in enumerate(sorted(asyncio.all_tasks(),key=lambda t:t.get_name()))]
 frame=sys._current_frames()[threading.get_ident()]
 allocations=[]
 for stat in tracemalloc.take_snapshot().statistics('traceback'):
  if any(pathlib.Path(f.filename).name==pathlib.Path(__file__).name for f in stat.traceback):
   allocations.append({'size_bytes':stat.size,'blocks':stat.count,'traceback':[{'symbol':'<allocation traceback>','file':'/synthetic/'+pathlib.Path(f.filename).name,'line':f.lineno,'language':'Python'} for f in stat.traceback]})
  if len(allocations)==3:break
 events=[];started={};origin=time.time_ns()
 def callback(phase,info):
  now=str(time.time_ns()-origin+1_000_000_000)
  if phase=='start':started[info['generation']]=now
  elif info['generation'] in started:events.append({'from_ns':started.pop(info['generation']),'to_ns':now,'phase':'pause'})
 gc.callbacks.append(callback);gc.collect();gc.callbacks.remove(callback)
 value={'schema':'python-runtime-export.v1','python_version':sys.version.split()[0],'pid':'500','time_ns':'1000000000','tasks':tasks,'workers':[{'id':'main','name':'synthetic-main-thread','state':'unknown','native_tid':'500','stack':stack([frame])}],'allocations':allocations,'gc_events':events}
 (BASE/'python-3.14.4.json').write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
 gate.set();await task;tracemalloc.stop();assert len(allocated)==8
asyncio.run(main())
