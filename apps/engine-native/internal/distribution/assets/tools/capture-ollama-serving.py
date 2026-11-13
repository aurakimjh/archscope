#!/usr/bin/env python3
"""Explicit local two-request synthetic Ollama capture; no prompt/response/model bodies in output."""
import argparse,json,pathlib,time,urllib.request
p=argparse.ArgumentParser();p.add_argument('--model',required=True,help='already cached local completion model; never pulled');p.add_argument('--host-tag',required=True);p.add_argument('--boot-tag',required=True);p.add_argument('--out',required=True);a=p.parse_args()
if pathlib.Path(a.out).exists():raise SystemExit('output exists')
opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
def call(path,body=None):
 req=urllib.request.Request('http://127.0.0.1:11434/api/'+path,data=json.dumps(body).encode() if body is not None else None,headers={'Content-Type':'application/json'})
 with opener.open(req,timeout=120) as response:return json.load(response)
if call('version').get('version')!='0.40.16':raise SystemExit('qualified Ollama0.40.16 required')
if a.model not in {r['name'] for r in call('tags').get('models',[])}:raise SystemExit('select already cached model')
if 'completion' not in call('show',{'model':a.model}).get('capabilities',[]):raise SystemExit('cached completion model required')
previously_loaded=a.model in {r['name'] for r in call('ps').get('models',[])}
observations=[]
try:
 for i in range(2):
  start=time.time_ns();tick=time.monotonic_ns();reply=call('generate',{'model':a.model,'prompt':'Reply OK.','stream':False,'options':{'num_predict':8,'temperature':0},'keep_alive':'2m'});elapsed=time.monotonic_ns()-tick;end=time.time_ns()
  if reply.get('done') is not True:raise SystemExit('final successful response required')
  metrics={}
  for key in ('total_duration','load_duration','prompt_eval_count','prompt_eval_duration','eval_count','eval_duration'):
   if key in reply:
    value=reply[key]
    if type(value) is not int or not 0<=value<2**64:raise SystemExit('invalid final-response numeric field')
    metrics[key]=str(value)
  observations.append({'capture_start_ns':str(start),'capture_end_ns':str(end),'client_elapsed_ns':str(elapsed),'metrics':metrics})
finally:
 if not previously_loaded:call('generate',{'model':a.model,'keep_alive':0})
with pathlib.Path(a.out).open('x',encoding='utf-8') as f:json.dump({'contract':'ollama-generate-final.v1','producer':'ollama','producer_version':'0.40.16','host_id':a.host_tag,'boot_id':a.boot_tag,'config':{'num_predict':'8','temperature':'0','stream':False},'sdk':{'name':'manual','version':'1.0.0'},'scope':'owner generated synthetic completion; final response only','observations':observations},f,indent=2);f.write('\n')
