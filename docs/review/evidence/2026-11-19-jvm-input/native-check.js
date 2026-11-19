(async()=>{
 const out={passed:false,scope:'Actual macOS WebView; synthetic file drop through real single-file page and real engine; no physical drag or Windows claim',cases:[]};
 const wait=async pred=>{const end=Date.now()+20000;while(!pred()){if(Date.now()>end)throw Error('readiness timeout: '+pred.toString());await new Promise(r=>setTimeout(r,75));}await new Promise(r=>setTimeout(r,100));};
 const emit=async(name,data)=>fetch(new URL('/wails/runtime',location.href),{method:'POST',headers:{'Content-Type':'application/json','x-wails-client-id':window._wails.clientId},body:JSON.stringify({object:3,method:0,args:{name,data}})});
 const route=async name=>{window.dispatchEvent(new CustomEvent('desktop:navigate',{detail:name}));await wait(()=>document.querySelector('.tool-pane[data-route="'+name+'"] main'));};
 const button=(root,label)=>[...root.querySelectorAll('button')].find(n=>n.textContent.trim()===label);
 const drop=async name=>{const file=new File(['owned input'],name);Object.defineProperty(file,'path',{value:'/private/tmp/archscope-jvm-repro/'+name});const transfer=new DataTransfer();transfer.items.add(file);document.querySelector('[data-file-drop]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:transfer}));await wait(()=>(button(document.querySelector('main'),'Analyze dump')??button(document.querySelector('main'),'Analyze'))?.disabled===false);(button(document.querySelector('main'),'Analyze dump')??button(document.querySelector('main'),'Analyze')).click();};
 const original=localStorage.getItem('archscope.profiler.locale');
 try {
  await wait(()=>document.querySelector('.desktop-ribbon'));
  await route('settings');await wait(()=>document.querySelector('.locale-switch button'));button(document.querySelector('.locale-switch'),'English').click();await wait(()=>document.documentElement.lang==='en');
  for(const name of ['jcmd.log','jstack.log']) {
   await route('thread_dump');button(document.querySelector('main'),'Single dump')?.click();await wait(()=>document.querySelector('[data-file-drop]'));
   await drop(name);await wait(()=>[...document.querySelectorAll('[role="tab"]')].find(n=>/^Dumps\s*\(20\)$/.test(n.textContent.trim()))?.textContent.includes('(20)'));
   const tab=[...document.querySelectorAll('[role="tab"]')].find(n=>/^Dumps\s*\(20\)$/.test(n.textContent.trim()));tab.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,button:0}));tab.click();await wait(()=>document.querySelectorAll('[role="tabpanel"][data-state="active"] tbody tr').length===20);
   const rows=[...document.querySelectorAll('[role="tabpanel"][data-state="active"] tbody tr')].map(n=>n.textContent);
   if(!rows[0].includes(name+'#1') || !rows[19].includes(name+'#20'))throw Error('wrong capture labels');
   if(document.querySelector('[data-file-drop]'))throw Error('did not switch to multi mode');
   out.cases.push({name,dumps:20,rows:rows.length,firstLabel:name+'#1',lastLabel:name+'#20'});
  }
  for(const name of ['async-profiler-v2.html','async-profiler-v3.html']) {
   await route('profiler');await wait(()=>document.querySelector('[data-file-drop]'));await drop(name);
   await wait(()=>[...document.querySelectorAll('[role="tab"]')].some(n=>n.textContent.trim()==='Summary'&&!n.disabled));
   const summary=[...document.querySelectorAll('[role="tab"]')].find(n=>n.textContent.trim()==='Summary');summary.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,button:0}));summary.click();
   await wait(()=>[...document.querySelectorAll('.metric-card')].some(n=>n.textContent.includes('Total samples')&&n.querySelector('strong')?.textContent==='10'));
   out.cases.push({name,samples:10});
  }
  out.passed=true;
 }catch(e){out.failure=String(e);out.page=document.querySelector('main')?.innerText.slice(0,3000);}
 finally{if(original===null)localStorage.removeItem('archscope.profiler.locale');else localStorage.setItem('archscope.profiler.locale',original);await emit('layout:audit',out);}
})();
