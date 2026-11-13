#!/usr/bin/env python3
"""Explicit local two-request manual OTel GenAI metric capture; pinned SDK1.38.0, schema1.37.0."""
import argparse,json,pathlib,time,urllib.request
from importlib.metadata import version
from google.protobuf.json_format import MessageToDict
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import InMemoryMetricReader
from opentelemetry.sdk.metrics.view import View,ExplicitBucketHistogramAggregation
from opentelemetry.exporter.otlp.proto.common.metrics_encoder import encode_metrics
if version('opentelemetry-sdk')!='1.38.0' or version('opentelemetry-exporter-otlp-proto-http')!='1.38.0':raise SystemExit('qualified OTel Python1.38.0 SDK/encoder required')
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
reader=InMemoryMetricReader();provider=MeterProvider(metric_readers=[reader],resource=Resource({'service.name':'synthetic-genai-app','service.instance.id':'synthetic-instance'}),views=[View(instrument_name='gen_ai.client.operation.duration',aggregation=ExplicitBucketHistogramAggregation(boundaries=[.01,.1,1,10,100]))])
meter=provider.get_meter('manual.ollama.genai','1.0.0',schema_url='https://opentelemetry.io/schemas/1.37.0')
duration=meter.create_histogram('gen_ai.client.operation.duration',unit='s');tokens=meter.create_histogram('gen_ai.client.token.usage',unit='{token}')
attributes={'gen_ai.operation.name':'text_completion','gen_ai.provider.name':'ollama','gen_ai.request.model':'synthetic-local-model'}
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
  duration.record(elapsed/1e9,attributes)
  for field,kind in [('prompt_eval_count','input'),('eval_count','output')]:
   if field in reply:tokens.record(reply[field],attributes|{'gen_ai.token.type':kind})
  observations.append({'capture_start_ns':str(start),'capture_end_ns':str(end),'client_elapsed_ns':str(elapsed),'metrics':metrics})
 data=MessageToDict(encode_metrics(reader.get_metrics_data()),preserving_proto_field_name=False,use_integers_for_enums=True)
finally:
 provider.shutdown()
 if not previously_loaded:call('generate',{'model':a.model,'keep_alive':0})
with pathlib.Path(a.out).open('x',encoding='utf-8') as f:json.dump(data,f,indent=2);f.write('\n')
