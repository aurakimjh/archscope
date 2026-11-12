import os,platform,json,pathlib,torch,torch.distributed as dist,torch.multiprocessing as mp
from torch.profiler import profile,ProfilerActivity,record_function
root=pathlib.Path(os.environ['FIXTURE_OUTPUT']).resolve();root.mkdir(parents=True,exist_ok=True)
def worker(rank):
 os.environ['GLOO_SOCKET_IFNAME']='lo0' if platform.system()=='Darwin' else 'lo'
 torch.set_num_threads(1)
 dist.init_process_group('gloo',init_method='file://'+str(root/'rendezvous'),rank=rank,world_size=2)
 torch.manual_seed(7+rank);x=torch.randn(32,32)
 with profile(activities=[ProfilerActivity.CPU],record_shapes=False,with_stack=False) as prof:
  for step in range(2):
   with record_function('synthetic.prefill'):y=torch.mm(x,x)
   with record_function('synthetic.all_reduce'):dist.all_reduce(y)
   with record_function('synthetic.decode'):y.relu_()
 prof.export_chrome_trace(str(root/f'rank-{rank}.raw.json'))
 dist.destroy_process_group()
if __name__=='__main__':
 mp.spawn(worker,nprocs=2,join=True)
 for rank in range(2):
  d=json.loads((root/f'rank-{rank}.raw.json').read_text());events=d['traceEvents'];print('rank',rank,'root keys',list(d),'events',len(events),'categories',sorted(set(e.get('cat','') for e in events)))
  print('sample',next(e for e in events if e.get('cat')=='cpu_op'))
  print('distributedInfo',d.get('distributedInfo'))
