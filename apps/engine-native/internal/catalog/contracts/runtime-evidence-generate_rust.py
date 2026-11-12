"""rustc 1.94.1의 실제 v0 symbol과 합성 panic/backtrace를 고정한다."""
import json,pathlib,re,subprocess,tempfile,os
base=pathlib.Path(__file__).parent
version=subprocess.check_output(['rustc','--version'],text=True).strip()
assert version.startswith('rustc 1.94.1 ')
with tempfile.TemporaryDirectory(prefix='synthetic-rust-fixture-') as d:
 source=pathlib.Path(d)/'synthetic.rs';source.write_text('''#[inline(never)] pub fn synthetic_worker(x:u64)->u64 {std::hint::black_box(x*3)}
fn main(){let _=synthetic_worker(7);panic!("SYNTHETIC_PANIC_BODY_NOT_RETAINED");}
''')
 obj=pathlib.Path(d)/'symbols.o';exe=pathlib.Path(d)/'synthetic'
 subprocess.run(['rustc','--crate-name','synthetic_symbols','--crate-type=lib','--emit=obj','-C','symbol-mangling-version=v0',str(source),'-o',str(obj)],check=True)
 symbols=subprocess.check_output(['nm','-gj',str(obj)],text=True).splitlines()
 selected=[s.strip() for s in symbols if s.strip().startswith(('_R','__R')) and 'synthetic_worker' in s]
 assert selected
 subprocess.run(['rustc','--crate-name','synthetic_symbols','-g',str(source),'-o',str(exe)],check=True)
 env=os.environ.copy();env['RUST_BACKTRACE']='1';out=subprocess.run([str(exe)],capture_output=True,text=True,env=env);assert out.returncode!=0
 backtrace=out.stderr.replace(d,'/synthetic').replace(str(pathlib.Path.home()),'/synthetic-home')
 (base/'rust-1.94.1-backtrace.txt').write_text(backtrace)
 (base/'rust-symbols.json').write_text(json.dumps({'producer':version,'raw':selected+['_ZN9synthetic4work17h0123456789abcdefE','_ZN9synthetic4work17h1123456789abcdefE','Math::work','trust::work','_Rinvalid']},indent=2)+'\n')
