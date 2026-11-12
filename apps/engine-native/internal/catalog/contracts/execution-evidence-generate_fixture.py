# 개발 전용 합성 예제 생성: 별도 JVM 프로세스를 의도적으로 종료합니다.
# 인자: 출력 디렉토리, 고정 원본/도구 디렉토리, JDK home. 코어 덤프는 끕니다.
import sys,pathlib,subprocess,json,threading,http.server,os,ast,types,time,contextlib,io,hashlib,re
out=pathlib.Path(sys.argv[1]).resolve();out.mkdir(parents=True,exist_ok=True)
tools=pathlib.Path(sys.argv[2]).resolve();k6=tools/'k6-v1.3.0-macos-arm64/k6';k6.chmod(0o755)
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):
  self.send_response(200);self.end_headers();self.wfile.write(b'synthetic response')
 def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
script=tools/'fixture-k6.js';script.write_text('''import http from 'k6/http'; import {sleep} from 'k6'; export const options={vus:2,duration:'1s'};export default function(){http.get('http://127.0.0.1:'+__ENV.FIXTURE_PORT+'/synthetic');sleep(0.05);}export function handleSummary(data){return {'summary.json':JSON.stringify(data,null,2)};}''')
subprocess.run([str(k6),'run',str(script)],cwd=tools,env=os.environ|{'K6_NO_USAGE_REPORT':'true','FIXTURE_PORT':str(server.server_port)},check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);server.shutdown()
summary=json.loads((tools/'summary.json').read_text());summary['root_group']['name']='SYNTHETIC_BODY_NOT_RETAINED';(out/'k6-1.3.0.json').write_text(json.dumps(summary,indent=2)+'\n')
jdk=pathlib.Path(sys.argv[3]).resolve()/'bin';java=tools/'SyntheticCrash.java';java.write_text('''import sun.misc.Unsafe;import java.lang.reflect.Field;public class SyntheticCrash{public static void main(String[] args)throws Exception{Field f=Unsafe.class.getDeclaredField("theUnsafe");f.setAccessible(true);((Unsafe)f.get(null)).putAddress(0L,1L);}}''')
subprocess.run([str(jdk/'javac'),str(java)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
crash=tools/'hs_err_pid.log';p=subprocess.run([str(jdk/'java'),'-XX:-CreateCoredumpOnCrash','-XX:ErrorFile='+str(crash),'-cp',str(tools),'SyntheticCrash'],cwd=tools,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);assert p.returncode!=0 and crash.exists()
lines=crash.read_text().splitlines();kept=[];capture=False
for line in lines:
 if line.startswith('#') and any(x in line for x in ['fatal error has been detected','SIGSEGV','JRE version:','Java VM:','Problematic frame:']):
  line=re.sub(r'pid=\d+,\s*tid=\d+','pid=500, tid=501',line);line=re.sub(r'at pc=0x[0-9a-fA-F]+','at pc=0x0000000000000000',line);kept.append(line);capture='Problematic frame:' in line;continue
 if capture:
  if line.startswith('# C ')or line.startswith('# V '):kept.append(line)
  capture=False
 if line.startswith('elapsed time:'):kept.append(line)
kept.extend(['# Sanitized fragment from actual HotSpot 21.0.10 synthetic crash; other sections intentionally omitted.','# Command Line: SYNTHETIC_COMMAND_NOT_RETAINED','# Environment Variables: SYNTHETIC_ENV_NOT_RETAINED'])
(out/'hs-err-21.0.10.txt').write_text('\n'.join(kept)+'\n')
# The unredacted report contains host state and stays outside both repositories.
crash.unlink()
(out/'jmeter-5.6.3.csv').write_text('timeStamp,elapsed,label,responseCode,success,Latency,Connect,allThreads,URL,failureMessage\n1700000000000,100,synthetic-a,200,true,40,10,2,http://127.0.0.1/?SYNTHETIC_URL_NOT_RETAINED,\n1700000000100,200,synthetic-a,503,false,80,20,2,http://127.0.0.1/,SYNTHETIC_FAILURE_NOT_RETAINED\n1700000000200,300,synthetic-b,200,true,100,20,2,http://127.0.0.1/,\n')
# Execute only the inspected upstream statistics function and JSON formatter,
# without importing the benchmark runner, clients, tokenizers or network code.
import pandas as pd
from collections.abc import Iterable
source=(tools/'llmperf-token_benchmark_ray.py').read_text();tree=ast.parse(source);function=next(x for x in tree.body if isinstance(x,ast.FunctionDef) and x.name=='metrics_summary');function.returns=None
for arg in function.args.args:arg.annotation=None
metrics=types.SimpleNamespace();exec((tools/'llmperf-common_metrics.py').read_text(),metrics.__dict__)
ns={'pd':pd,'Iterable':Iterable,'common_metrics':metrics};exec(compile(ast.Module(body=[function],type_ignores=[]),'upstream-metrics-summary','exec'),ns)
rows=[{metrics.INTER_TOKEN_LAT:0.02,metrics.TTFT:0.1,metrics.E2E_LAT:1,metrics.REQ_OUTPUT_THROUGHPUT:10,metrics.NUM_INPUT_TOKENS:100,metrics.NUM_OUTPUT_TOKENS:10,metrics.ERROR_CODE:None},{metrics.INTER_TOKEN_LAT:0.03,metrics.TTFT:0.2,metrics.E2E_LAT:2,metrics.REQ_OUTPUT_THROUGHPUT:5,metrics.NUM_INPUT_TOKENS:120,metrics.NUM_OUTPUT_TOKENS:10,metrics.ERROR_CODE:None},{metrics.INTER_TOKEN_LAT:0.01,metrics.TTFT:0.05,metrics.E2E_LAT:0.5,metrics.REQ_OUTPUT_THROUGHPUT:20,metrics.NUM_INPUT_TOKENS:80,metrics.NUM_OUTPUT_TOKENS:10,metrics.ERROR_CODE:None},{metrics.INTER_TOKEN_LAT:0,metrics.TTFT:0,metrics.E2E_LAT:0,metrics.REQ_OUTPUT_THROUGHPUT:0,metrics.NUM_INPUT_TOKENS:100,metrics.NUM_OUTPUT_TOKENS:0,metrics.ERROR_CODE:'SYNTHETIC_ERROR_NOT_RETAINED'}]
rows=[{k:float(v) if isinstance(v,int) else v for k,v in row.items()} for row in rows]
with contextlib.redirect_stdout(io.StringIO()):stats=ns['metrics_summary'](rows,0,5)
utils=ast.parse((tools/'llmperf-utils.py').read_text());selected=[x for x in utils.body if isinstance(x,(ast.ClassDef,ast.FunctionDef)) and x.name in ['LLMPerfResults','flatten_dict']]
ns={'json':json,'time':types.SimpleNamespace(time=lambda:1700000000),'RESULTS_VERSION':'2023-08-31','Dict':dict,'Any':object};exec(compile(ast.Module(body=selected,type_ignores=[]),'upstream-json-formatter','exec'),ns)
metadata={'model':'synthetic-model','mean_input_tokens':100,'stddev_input_tokens':20,'mean_output_tokens':10,'stddev_output_tokens':0,'num_concurrent_requests':2,'additional_sampling_params':{},'results':stats}
obj=json.loads(ns['LLMPerfResults']('synthetic',metadata).json());(out/'llmperf-summary.json').write_text(json.dumps(obj,indent=2)+'\n')
sha=(tools/'llmperf-revision.txt').read_text().strip();(out/'producer-provenance.json').write_text(json.dumps({'k6_version':subprocess.check_output([str(k6),'version'],text=True).strip(),'jvm_version':'Temurin 21.0.10+7-LTS; actual putAddress(0) synthetic crash with -XX:-CreateCoredumpOnCrash; sanitized fragment','llmperf_revision':sha,'llmperf_summary_source_sha256':hashlib.sha256(source.encode()).hexdigest(),'llmperf_utils_source_sha256':hashlib.sha256((tools/'llmperf-utils.py').read_bytes()).hexdigest(),'pandas':pd.__version__,'qualification':'k6 실제 loopback 실행; hs_err 실제 합성 JVM crash의 정제 발췌; LLMPerf 실제 통계/JSON 함수에 합성 수치 입력. 모델·GPU·벤치마크 클라이언트 실행 아님; JMeter는 합성 CSV 구조 예제.'},ensure_ascii=False,indent=2)+'\n')
print(out)
