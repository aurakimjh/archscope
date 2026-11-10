(async () => {
  const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
  const report = {native: true, nativeMenuInteractionVerified:false, viewport: {width: innerWidth, height: innerHeight, dpr: devicePixelRatio}, views: []};
  try {
    await wait(1200);
    report.rootText=document.querySelector("#root")?.textContent.slice(0,200);
    report.bridge={webkit:typeof window.webkit?.messageHandlers?.external?.postMessage,webview:typeof window.chrome?.webview?.postMessage};
    for (const button of document.querySelectorAll('.nav-group-toggle')) {
      if (button.getAttribute('aria-expanded') === 'false') button.click();
    }
    await wait(100);
    const nativeMenuShell = !!document.querySelector(".desktop-shell");
    const routes = nativeMenuShell ? ["profiler", "diff", "access_log", "gc_log", "jfr", "exception", "thread_dump", "msa_profile", "trace_import", "http_capture", "browser_cpu", "browser_audit", "analysis_workspace", "export_center", "report_diff", "chart_studio", "incident_timeline", "slo_golden_signals", "service_flow", "evidence_board", "settings"] : [...document.querySelectorAll('nav button.nav-item')].map(b => b.textContent.trim());
    for (const name of routes) {
      if (nativeMenuShell) window.dispatchEvent(new CustomEvent("desktop:navigate",{detail:name}));
      else [...document.querySelectorAll('nav button.nav-item')].find(b => b.textContent.trim() === name).click();
      await wait(400);
      const main = document.querySelector('main') || document.querySelector('.page-scroll');
      const rect = main?.getBoundingClientRect();
      report.views.push({name, state: 'initial', navigation: nativeMenuShell ? 'route-event' : 'sidebar', top: rect?.top, width: rect?.width, height: rect?.height,
        rows: [...document.querySelectorAll('tbody tr')].filter(e => { const r=e.getBoundingClientRect();return r.top>=0 && r.bottom<=innerHeight; }).length,
        outerOverflow: document.documentElement.scrollWidth > innerWidth});
    }
    if (!report.views.length) throw new Error('No native routes found');
  } catch (error) { report.error = String(error); }
  await fetch(new URL('/wails/runtime',location.href), {
    method:'POST',headers:{'Content-Type':'application/json','x-wails-client-id':window._wails.clientId},
    body:JSON.stringify({object:3,method:0,args:{name:'layout:audit',data:report}})
  });
})();
