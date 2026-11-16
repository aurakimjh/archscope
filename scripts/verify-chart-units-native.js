// Explicit layoutaudit build only. Synthetic file-drop events load an owned
// collapsed fixture through the real page/analyzer; this is NOT physical drag QA.
// CSV actions open real native Save dialogs, which the operator must complete.
(async()=>{
  const fixture='__ARCHSCOPE_UNIT_FIXTURE__';
  const out={passed:false,scope:'Actual Windows profiler→Chart Studio→native CSV Save; synthetic input dispatch, not physical drag or universal export qualification',cases:[]};
  const wait=async (predicate,limit=180000)=>{const end=Date.now()+limit;while(!predicate()){if(Date.now()>end)throw Error('Chart unit readiness timeout');await new Promise(r=>setTimeout(r,75));}await new Promise(r=>setTimeout(r,100));};
  const require=(value,message)=>{if(!value)throw Error(message);};
  const emit=async(name,data)=>fetch(new URL('/wails/runtime',location.href),{method:'POST',headers:{'Content-Type':'application/json','x-wails-client-id':window._wails.clientId},body:JSON.stringify({object:3,method:0,args:{name,data}})});
  const route=async name=>{window.dispatchEvent(new CustomEvent('desktop:navigate',{detail:name}));await wait(()=>document.querySelector('.tool-pane[data-route="'+name+'"] main'));};
  const select=(node,value)=>{require(node,'Missing select');node.value=value;node.dispatchEvent(new Event('change',{bubbles:true}));};
  const button=(container,label)=>[...container.querySelectorAll('button')].find(node=>node.textContent.trim()===label);
  const units={samples:'samples',nanoseconds:'ns',microseconds:'µs',milliseconds:'ms',seconds:'s',bytes:'bytes',unknown:'Unknown'};
  let phase='readiness';
  try{
    await wait(()=>document.querySelector('.desktop-ribbon'));
    // Locale is controlled by the ordinary Settings page, not catalog injection.
    await route('settings');
    await wait(()=>document.querySelector('.locale-switch button'));
    button(document.querySelector('.locale-switch'),'English').click();
    await wait(()=>document.documentElement.lang==='en');
    for(const [unit,axis] of Object.entries(units)){
      phase='analyze '+unit;await emit('layout:progress',{phase,completed:out.cases.length});
      await route('profiler');
      await wait(()=>document.querySelector('[data-file-drop]'));
      const file=new File(['Root;work 1000000\n'],'units.collapsed');Object.defineProperty(file,'path',{value:fixture});
      const transfer=new DataTransfer();transfer.items.add(file);
      document.querySelector('[data-file-drop]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:transfer}));
      await wait(()=>button(document.querySelector('main'),'Analyze')?.disabled===false);
      button(document.querySelector('main'),'Analyzer options').click();
      await wait(()=>document.querySelector('select option[value="lock"]'));
      select(document.querySelector('select option[value="lock"]').parentElement,'lock');
      select(document.querySelector('select option[value="nanoseconds"]').parentElement,unit);
      await wait(()=>document.querySelector('select option[value="nanoseconds"]').parentElement.value===unit);
      document.querySelector('[role="dialog"] [data-panel-close]').click();
      await wait(()=>!document.querySelector('[role="dialog"][aria-hidden="false"]'));
      button(document.querySelector('main'),'Analyze').click();
      await wait(()=>{const summary=document.querySelector('[role="tab"][id$="trigger-summary"]');return summary && !summary.disabled;});
      await route('chart_studio');
      await wait(()=>document.querySelector('.chartstudio select option[value="profiler.components"]') && !button(document.querySelector('.chartstudio'),'CSV')?.disabled);
      // Select the most recently analyzed result, not an older compatible card.
      const resultSelect=document.querySelector('.chartstudio select');
      select(resultSelect,resultSelect.options[0].value);
      await wait(()=>document.querySelector('.chartstudio canvas'));
      const renderer=document.querySelector('.chartstudio select option[value="svg"]').parentElement;
      select(renderer,'svg');await wait(()=>document.querySelector('.chartstudio svg text'));
      const labels=[...document.querySelectorAll('.chartstudio svg text')].map(node=>node.textContent);
      require(labels.includes(axis),'Wrong native SVG axis for '+unit+': '+JSON.stringify(labels));
      const point={unit,axis,svgAxisObserved:true,canvasObserved:true};
      phase='native save '+unit;await emit('layout:progress',{phase,completed:out.cases.length});
      button(document.querySelector('.chartstudio'),'CSV').click();
      await wait(()=>document.querySelector('.chartstudio [role="status"]')?.textContent.includes('Export completed'),900000);
      point.nativeSaveCompleted=true;out.cases.push(point);
    }
    out.passed=true;
  }catch(error){out.failure=String(error);out.phase=phase;}
  finally{await emit('layout:audit',out);}
})();
