(async()=>{
 const out={passed:false,scope:'Actual macOS WebView; owned GC log; synthetic file drop and mousemove, no physical pointer or Windows claim',cases:[]};
 const wait=async pred=>{const end=Date.now()+20000;while(!pred()){if(Date.now()>end)throw Error('timeout '+pred.toString());await new Promise(r=>setTimeout(r,75));}await new Promise(r=>setTimeout(r,100));};
 const emit=async(name,data)=>fetch(new URL('/wails/runtime',location.href),{method:'POST',headers:{'Content-Type':'application/json','x-wails-client-id':window._wails.clientId},body:JSON.stringify({object:3,method:0,args:{name,data}})});
 const route=async name=>{window.dispatchEvent(new CustomEvent('desktop:navigate',{detail:name}));await wait(()=>document.querySelector('.tool-pane[data-route="'+name+'"] main'));};
 const button=(root,label)=>[...root.querySelectorAll('button')].find(n=>n.textContent.trim()===label);
 const tab=async label=>{await wait(()=>[...document.querySelectorAll('[role="tab"]')].some(n=>n.textContent.trim()===label&&!n.disabled));const t=[...document.querySelectorAll('[role="tab"]')].find(n=>n.textContent.trim()===label);t.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,button:0}));t.click();await wait(()=>t.getAttribute('data-state')==='active');};
 const hover=async(type,duration)=>{
  const surface=document.querySelector('[role="tabpanel"][data-state="active"] [data-chart-surface]');surface.scrollIntoView({block:'center'});await new Promise(r=>setTimeout(r,250));
  await wait(()=>surface.querySelector('canvas'));const canvas=surface.querySelector('canvas');const box=surface.getBoundingClientRect();
  for(let x=61;x<box.width-24;x+=5){canvas.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:box.left+x,clientY:box.top+box.height/2}));await new Promise(r=>setTimeout(r,20));const text=surface.innerText;
   if(text.includes(type)&&text.includes(duration)&&text.includes('2026-11-19T10:00:'))return text;
  }
  throw Error('tooltip missing '+type+'; observed='+surface.innerText);
 };
 out.errors=[];window.addEventListener('error',event=>out.errors.push(String(event.error??event.message)));
 out.visibility=document.visibilityState;out.rafObserved=false;requestAnimationFrame(()=>out.rafObserved=true);
 const original=localStorage.getItem('archscope.profiler.locale');
 try {
  await wait(()=>document.querySelector('.desktop-ribbon'));await route('settings');await wait(()=>document.querySelector('.locale-switch button'));button(document.querySelector('.locale-switch'),'English').click();await wait(()=>document.documentElement.lang==='en');
  await route('gc_log');await wait(()=>document.querySelector('[data-file-drop]'));
  const file=new File(['owned input'],'gc.log');Object.defineProperty(file,'path',{value:'/private/tmp/archscope-gc-tooltip/gc.log'});const transfer=new DataTransfer();transfer.items.add(file);document.querySelector('[data-file-drop]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:transfer}));
  await wait(()=>button(document.querySelector('main'),'Analyze')?.disabled===false);button(document.querySelector('main'),'Analyze').click();
  await tab('Pauses');out.cases.push({chart:'pause',phase:'Pause Remark',tooltip:await hover('Pause Remark','1.2345 ms')});out.cases.push({chart:'pause',phase:'Pause Full',tooltip:await hover('Pause Full','250 ms')});
  await tab('Heap');const overlay=[...document.querySelectorAll('label')].find(n=>n.textContent.includes('Overlay pause (ms)'))?.querySelector('input');if(!overlay)throw Error('overlay checkbox missing');
  for(const enabled of [false,true]){
   if(overlay.checked!==enabled)overlay.click();await wait(()=>overlay.checked===enabled);
   out.cases.push({chart:'heap',overlay:enabled,phase:'Pause Remark',tooltip:await hover('Pause Remark','1.2345 ms')});
   out.cases.push({chart:'heap',overlay:enabled,phase:'Pause Cleanup',tooltip:await hover('Pause Cleanup','0 ms')});
  }
  out.passed=true;
 }catch(e){out.failure=String(e);out.page=document.querySelector('main')?.innerText.slice(-4000);out.surfaces=[...document.querySelectorAll('[data-chart-surface]')].map(n=>n.outerHTML.slice(0,1800));}
 finally{if(original===null)localStorage.removeItem('archscope.profiler.locale');else localStorage.setItem('archscope.profiler.locale',original);await emit('layout:audit',out);}
})();
