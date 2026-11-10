(async () => {
  const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
  const report = {native: true, viewport: {width: innerWidth, height: innerHeight, dpr: devicePixelRatio}, views: []};
  try {
    await wait(1200);
    report.rootText=document.querySelector("#root")?.textContent.slice(0,200);
    report.bridge={webkit:typeof window.webkit?.messageHandlers?.external?.postMessage,webview:typeof window.chrome?.webview?.postMessage};
    for (const button of document.querySelectorAll('.nav-group-toggle')) {
      if (button.getAttribute('aria-expanded') === 'false') button.click();
    }
    await wait(100);
    const names = [...document.querySelectorAll('nav button.nav-item')].map(b => b.textContent.trim());
    for (const name of names) {
      [...document.querySelectorAll('nav button.nav-item')].find(b => b.textContent.trim() === name).click();
      await wait(400);
      const main = document.querySelector('main') || document.querySelector('.page-scroll');
      const rect = main?.getBoundingClientRect();
      report.views.push({name, state: 'initial', top: rect?.top, width: rect?.width, height: rect?.height,
        rows: [...document.querySelectorAll('tbody tr')].filter(e => { const r=e.getBoundingClientRect();return r.top>=0 && r.bottom<=innerHeight; }).length,
        outerOverflow: document.documentElement.scrollWidth > innerWidth});
    }
    if (!report.views.length) throw new Error('No native routes found');
  } catch (error) { report.error = String(error); }
  window.wails.Events.Emit('layout:audit', report);
})();
