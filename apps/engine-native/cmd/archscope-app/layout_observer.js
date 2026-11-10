// Passive diagnostics for a layoutaudit build. Navigation and input are driven
// through the native UI. Only geometry/counts are recorded, never row contents.
(() => {
  const observations = new Map();
  const viewport = () => ({left:0,top:0,right:innerWidth,bottom:innerHeight});
  const intersect = (a,b) => ({left:Math.max(a.left,b.left),top:Math.max(a.top,b.top),right:Math.min(a.right,b.right),bottom:Math.min(a.bottom,b.bottom)});
  function visibleRect(element) {
    if (!element.getClientRects().length) return null;
    let rect=intersect(element.getBoundingClientRect(),viewport());
    for(let parent=element.parentElement;parent;parent=parent.parentElement) {
      const style=getComputedStyle(parent);
      if (/(auto|scroll|hidden|clip)/.test(style.overflowX+style.overflowY)) rect=intersect(rect,parent.getBoundingClientRect());
    }
    return rect.right>rect.left && rect.bottom>rect.top ? rect : null;
  }
  function unionArea(rects) {
    const xs=[...new Set(rects.flatMap(rect=>[rect.left,rect.right]))].sort((a,b)=>a-b);
    let area=0;
    for(let i=1;i<xs.length;i++) {
      const intervals=rects.filter(r=>r.left<xs[i] && r.right>xs[i-1]).map(r=>[r.top,r.bottom]).sort((a,b)=>a[0]-b[0]);
      let height=0, end=-Infinity;
      for(const [top,bottom] of intervals) {height+=Math.max(0,bottom-Math.max(top,end));end=Math.max(end,bottom);}
      area+=(xs[i]-xs[i-1])*height;
    }
    return area;
  }
  function sample() {
    const shell=document.querySelector('.desktop-shell');
    const route=shell?.getAttribute('data-active-route') ?? document.querySelector('nav .nav-item.active')?.textContent.trim();
    const main=document.querySelector('main');
    if(!route || !main || (shell && main.closest('[data-route]')?.getAttribute('data-route') !== route)) return;
    const rowElements=[...main.querySelectorAll('tbody tr')].filter(row=>row.querySelectorAll('td').length>1);
    const rows=rowElements.map(row=>({request:!!row.closest("[data-http-grid]"),full:row.getBoundingClientRect(),visible:visibleRect(row)})).filter(row=>row.visible);
    const charts=[...main.querySelectorAll('canvas')].map(visibleRect).filter(Boolean);
    const detail=main.querySelector('[data-http-detail]');
    const detailRect=detail && visibleRect(detail);
    const rectangles=[...rows.map(row=>row.visible),...charts,...(detailRect?[detailRect]:[])];
    const area=unionArea(rectangles);
    const tab=main.querySelector('[role=tab][aria-selected=true]')?.textContent.trim() ?? '';
    const observation={route,tab,width:innerWidth,height:innerHeight,dpr:devicePixelRatio,
      mainWidth:Math.round(main.getBoundingClientRect().width),
      sidebarWidth:Math.round(document.querySelector('.sidebar')?.getBoundingClientRect().width ?? 0),
      chromeHeight:Math.round(document.querySelector('.topbar,.desktop-titlebar')?.getBoundingClientRect().height ?? 0),
      firstDataTop:rectangles.length?Math.round(Math.min(...rectangles.map(r=>r.top))):null,
      fullRows:rows.filter(r=>Math.abs(r.full.top-r.visible.top)<1 && Math.abs(r.full.bottom-r.visible.bottom)<1).length,
      fullRequestRows:rows.filter(r=>r.request && Math.abs(r.full.top-r.visible.top)<1 && Math.abs(r.full.bottom-r.visible.bottom)<1).length,
      renderedRows:rowElements.length,canvasCount:charts.length,
      resultAreaRatio:Math.round(1000*area/(innerWidth*innerHeight))/1000,
      detailHeight:detailRect?Math.round(detailRect.bottom-detailRect.top):0,
      outerOverflow:document.documentElement.scrollWidth>innerWidth,
      helpOpen:!!document.querySelector('[role=tooltip]')};
    const key=JSON.stringify(observation);
    if(observations.has(key)) return;
    if(observations.size>=200) observations.delete(observations.keys().next().value);
    observations.set(key,observation);
    fetch(new URL('/wails/runtime',location.href), {
      method:'POST', headers:{'Content-Type':'application/json','x-wails-client-id':window._wails.clientId},
      body:JSON.stringify({object:3,method:0,args:{name:'layout:audit',data:{mode:'passive',geometry:'clipped row/canvas/detail union; canvas includes axes',views:[...observations.values()]}}})
    }).catch(error=>console.error('layout audit record failed',error));
  }
  let pending;
  new MutationObserver(() => {clearTimeout(pending);pending=setTimeout(sample,100);}).observe(document.querySelector('#root'),{subtree:true,childList:true,characterData:true});
  window.addEventListener('resize',sample);
  setInterval(sample,1000);
  sample();
})();
