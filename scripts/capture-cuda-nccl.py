#!/usr/bin/env python3
"""Explicit owner-run two-GPU NCCL synthetic inference capture. No CPU fallback."""
import argparse,json,os,pathlib,platform,sys,time,hashlib

def file_sha256(path):
 with path.open("rb") as source:return hashlib.file_digest(source,"sha256").hexdigest()

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--preflight',action='store_true');p.add_argument('--out-dir');a=p.parse_args()
 try:import torch
 except ImportError:
  print(json.dumps({'ready':False,'reason':'pytorch_not_installed','platform':platform.system(),'architecture':platform.machine()}));return 2
 version=torch.__version__.split('+')[0]
 ready=version=='2.15.0' and torch.cuda.is_available() and torch.cuda.device_count()>=2 and torch.distributed.is_nccl_available()
 info={'ready':ready,'producer':'pytorch','producer_version':torch.__version__,'cuda_runtime':torch.version.cuda,'nccl_version':torch.cuda.nccl.version() if torch.cuda.is_available() and torch.distributed.is_nccl_available() else None,'visible_device_count':torch.cuda.device_count(),'platform':platform.system(),'architecture':platform.machine(),'workload':'owned synthetic two-rank sharded MLP inference; no trained weights/request body'}
 if a.preflight or not ready:
  print(json.dumps(info));return 0 if ready else 2
 if not a.out_dir:p.error('--out-dir required')
 # torchrun assigns LOCAL_RANK/RANK/WORLD_SIZE. Each rank writes its own trace;
 # absence of real two-GPU NCCL is a refusal, never CPU/Gloo structural evidence.
 if os.environ.get('WORLD_SIZE')!='2':raise SystemExit('run with torchrun --standalone --nproc-per-node=2')
 rank=int(os.environ['RANK']);local=int(os.environ['LOCAL_RANK']);root=pathlib.Path(a.out_dir)
 torch.cuda.set_device(local);torch.distributed.init_process_group('nccl',timeout=__import__('datetime').timedelta(seconds=60));torch.distributed.barrier()
 try:
  if rank==0:root.mkdir(mode=0o700)
  torch.distributed.barrier()
  device=torch.device('cuda',local);torch.manual_seed(7+rank)
  # Each rank owns a shard, all-reduced output. Shapes and mathematical input
  # are synthetic; this is inference, not trained-model quality/performance proof.
  x=torch.randn(16,128,device=device);w=torch.randn(128,128,device=device)
  with torch.inference_mode():
   for _ in range(2):
    y=torch.relu(x@w);torch.distributed.all_reduce(y)
   torch.cuda.synchronize()
   with torch.profiler.profile(activities=[torch.profiler.ProfilerActivity.CPU,torch.profiler.ProfilerActivity.CUDA],record_shapes=False,profile_memory=False,with_stack=False) as prof:
    for _ in range(3):
     with torch.profiler.record_function('synthetic-sharded-mlp-inference'):
      y=torch.relu(x@w);torch.distributed.all_reduce(y)
    torch.cuda.synchronize()
   path=root/f'rank-{rank}.trace.json';prof.export_chrome_trace(str(path))
   size=path.stat().st_size
   receipt=info|{'rank':rank,'world_size':2,'backend':'nccl','trace_bytes':size,'sha256':file_sha256(path),'clock':'source/rank-local; no shared offset/alignment asserted','qualification':'raw actual capture only; sanitize and import-validate before fixture promotion','within_64mib':size<=64<<20}
   with (root/f'rank-{rank}.receipt.json').open('x') as f:json.dump(receipt,f,indent=2);f.write('\n')
 finally:torch.distributed.destroy_process_group()
 return 0
if __name__=='__main__':sys.exit(main())
