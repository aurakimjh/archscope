// Execute only in an explicit layoutaudit native build via ARCHSCOPE_LAYOUT_SCRIPT.
// Uses the real renderer controls; never starts a capture or imports private input.
(async () => {
  const output = { schemaVersion: 2, platform: navigator.platform, scope: "Native ribbon and five-language UI smoke", passed: false, locales: [], themes: [], checks: [], viewport: { width: innerWidth, height: innerHeight, pixelRatio: devicePixelRatio } };
  let phase = "readiness";
  const progress = () => fetch(new URL('/wails/runtime', location.href), {
    method: 'POST', headers: { 'Content-Type': 'application/json', 'x-wails-client-id': window._wails.clientId },
    body: JSON.stringify({ object: 3, method: 0, args: { name: 'layout:progress', data: { phase, language: document.documentElement.lang, completedLocales: output.locales.length } } }),
  }).catch(() => {});
  const keys = ["archscope.profiler.locale", "archscope.desktop.ribbon.mode", "archscope.profiler.theme"];
  const original = Object.fromEntries(keys.map(key => [key, localStorage.getItem(key)]));
  const wait = async predicate => {
    const end = Date.now() + 15000;
    while (!predicate()) {
      if (Date.now() >= end) throw Error("Native renderer readiness timeout");
      await new Promise(resolve => setTimeout(resolve, 25));
    }
    await new Promise(resolve => setTimeout(resolve, 40));
  };
  const require = (condition, message) => { if (!condition) throw Error(message); };
  const click = selector => {
    const element = document.querySelector(selector);
    require(element && !element.disabled, "Unavailable control: " + selector);
    element.click();
  };
  const tab = async value => {
    const button = document.querySelector('[data-ribbon-tab="' + value + '"]');
    if (button?.getClientRects().length) button.click();
    else {
      const more = document.querySelector('.ribbon-more-tabs');
      require(more?.getClientRects().length, 'No visible tab selector');
      more.value = value; more.dispatchEvent(new Event('change', { bubbles: true }));
    }
    await wait(() => document.querySelector('[data-ribbon-tab="' + value + '"]').getAttribute('aria-pressed') === 'true');
  };
  try {
    await wait(() => document.querySelector('.desktop-ribbon') && document.querySelector('.tool-pane')?.firstElementChild);
    if (document.querySelector('.desktop-ribbon').dataset.mode === "hidden") click('.ribbon-restore');
    await wait(() => document.querySelector('.desktop-ribbon').dataset.mode !== 'hidden');
    const pin = document.querySelector('.ribbon-fold');
    if (pin.getAttribute('aria-expanded') === "false") pin.click();
    await wait(() => !document.querySelector('.ribbon-panel').hidden);
    await tab('view');
    const compact = document.querySelector('.desktop-ribbon').dataset.compact;
    const reachedRoutes = new Set();
    const checkCommands = () => {
      const panel = document.querySelector('.ribbon-panel');
      require(panel.scrollWidth <= panel.clientWidth + 1, 'Ribbon overflows horizontally');
      const targets = [...panel.querySelectorAll(compact ? 'details > summary' : 'button,select')].filter(node => node.getClientRects().length);
      for (const node of targets) {
        const rect = node.getBoundingClientRect();
        require(rect.width > 0 && rect.left >= -1 && rect.right <= innerWidth + 1, 'Command outside viewport: ' + node.textContent.trim());
      }
      if (compact) {
        for (const group of panel.querySelectorAll('.ribbon-group-overflow')) {
          group.open = true;
          for (const node of group.querySelectorAll('button,select')) {
            const rect = node.getBoundingClientRect();
            require(rect.width > 0 && rect.left >= -1 && rect.right <= innerWidth + 1, 'Popup command outside viewport: ' + node.textContent.trim());
          }
          group.open = false;
        }
      }
      panel.querySelectorAll('[data-ribbon-command^="navigate:"]').forEach(node => reachedRoutes.add(node.dataset.ribbonCommand));
    };
    if (compact) document.querySelectorAll('.ribbon-group-overflow').forEach(element => { element.open = true; });
    click('[data-ribbon-command="navigate:settings"]');
    await wait(() => document.querySelector('.tool-pane[data-route="settings"] .locale-switch'));
    const pane = document.querySelector('.tool-pane');
    const settingsNode = pane.firstElementChild;
    const expected = { en: "Settings", ko: "설정", ja: "設定", "zh-CN": "设置", "zh-TW": "設定" };
    const names = { en: "English", ko: "한국어", ja: "日本語", "zh-CN": "简体中文", "zh-TW": "繁體中文（台灣）" };
    for (const locale of Object.keys(expected)) {
      phase = "locale " + locale; progress();
      const pill = [...pane.querySelectorAll('.locale-switch button')].find(button => button.textContent.trim() === names[locale]);
      require(pill, "Missing language choice " + locale); pill.click();
      await wait(() => document.documentElement.lang === locale && pane.textContent.includes(expected[locale]));
      require(localStorage.getItem(keys[0]) === locale, "Language preference not persisted");
      const bounds = document.querySelector('.ribbon-tabs').getBoundingClientRect();
      const buttons = [...document.querySelectorAll('.ribbon-tabs > .ribbon-tab')].filter(button => button.getClientRects().length);
      const rows = new Set(buttons.map(button => Math.round(button.getBoundingClientRect().top)));
      require(rows.size === 1, "Duplicated/wrapped menu rows");
      require(document.querySelectorAll('.as-menubar,.as-native').length === 0, "Legacy mock menu remains");
      require(document.querySelector(".desktop-ribbon").getBoundingClientRect().height <= 126, "Ribbon exceeds compact height budget");
      for (const value of ['home', 'analyze', 'http', 'workspace', 'view', 'help']) {
        await tab(value); checkCommands();
        require(pane.firstElementChild === settingsNode, 'Language/tab switch remounted the page');
      }
      output.locales.push({ locale, selected: pill.getAttribute('aria-pressed'), menuRows: rows.size, tabHeight: bounds.height, ribbonHeight: document.querySelector('.desktop-ribbon').getBoundingClientRect().height });
    }
    require(reachedRoutes.size === 21, 'Missing route commands: ' + reachedRoutes.size);
    output.checks.push('all 21 route commands reachable and all six tabs fit in all five locales without remounting');
    phase = 'themes'; progress();
    await tab('view');
    if (compact) document.querySelectorAll('.ribbon-group-overflow').forEach(element => { element.open = true; });
    const themeSelect = document.querySelector('.ribbon-panel select option[value="light"]').parentElement;
    for (const theme of ['light', 'dark', 'system']) {
      if (compact) document.querySelectorAll('.ribbon-group-overflow').forEach(element => { element.open = true; });
      themeSelect.value = theme; themeSelect.dispatchEvent(new Event('change', { bubbles: true }));
      await wait(() => localStorage.getItem(keys[2]) === theme && document.documentElement.dataset.theme === (theme === 'system' ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light') : theme));
      const computed = getComputedStyle(document.querySelector('.desktop-ribbon'));
      require(computed.color !== computed.backgroundColor, 'Ribbon text has no theme contrast');
      output.themes.push({ theme, resolved: document.documentElement.dataset.theme, foreground: computed.color, background: computed.backgroundColor });
    }
    await tab('http');
    require([...document.querySelectorAll('[data-ribbon-command^="capture:"]')].every(button => button.disabled), 'Capture command enabled outside HTTP page');
    output.checks.push('capture commands disabled outside the HTTP page');
    phase = 'tab keyboard'; progress();
    const first = document.querySelector('.ribbon-file');
    first.focus(); first.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true }));
    require(document.activeElement?.dataset.ribbonTab === 'home', 'Arrow navigation failed');
    document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true, cancelable: true }));
    require(document.activeElement === first, 'Home navigation failed');
    output.checks.push('tab ArrowRight/Home keyboard focus navigation');
    phase = 'File screen'; progress();
    click('.ribbon-file');
    await wait(() => pane.hidden && document.querySelector('.ribbon-backstage'));
    require(pane.firstElementChild === settingsNode, "File screen unmounted page");
    document.querySelector('.ribbon-backstage button').click();
    await wait(() => !pane.hidden && !document.querySelector('.ribbon-backstage'));
    require(pane.firstElementChild === settingsNode, "Returning from File remounted page");
    output.checks.push("File screen preserves the mounted settings page");
    phase = 'visibility'; progress();
    phase = 'collapse'; progress();
    click('.ribbon-fold'); await wait(() => document.querySelector('.desktop-ribbon').dataset.mode === 'tabs');
    require(document.querySelector('.ribbon-panel').hidden, "Collapse did not hide commands");
    phase = 'temporary expansion'; progress();
    await tab('analyze'); await wait(() => !document.querySelector('.ribbon-panel').hidden);
    phase = 'Escape'; progress();
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await wait(() => document.querySelector('.ribbon-panel').hidden);
    phase = 'outside pointer'; progress();
    await tab('analyze'); await wait(() => !document.querySelector('.ribbon-panel').hidden);
    pane.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    await wait(() => document.querySelector('.ribbon-panel').hidden);
    require(localStorage.getItem(keys[1]) === 'tabs', "Ribbon mode not persisted");
    phase = 'pin'; progress();
    click('.ribbon-fold'); await wait(() => document.querySelector('.desktop-ribbon').dataset.mode === 'expanded');
    await tab('view');
    if (compact) document.querySelectorAll('.ribbon-group-overflow').forEach(element => { element.open = true; });
    const hide = [...document.querySelectorAll('.ribbon-panel button')].find(button => button.textContent.trim() === '隱藏功能區');
    phase = 'hide'; progress();
    require(hide, "Missing Traditional Chinese ribbon option"); hide.click();
    await wait(() => document.querySelector('.desktop-ribbon').dataset.mode === 'hidden');
    require(document.activeElement === document.querySelector('.ribbon-restore'), "Hidden ribbon must expose focused restore control");
    phase = 'restore'; progress();
    click('.ribbon-restore'); await wait(() => document.querySelector('.desktop-ribbon').dataset.mode === 'expanded');
    require(pane.firstElementChild === settingsNode, "Ribbon visibility remounted page");
    output.checks.push("collapse, temporary expansion, Escape, outside pointer, hide and restore preserve page and saved mode");
    output.passed = true;
    phase = 'completed'; progress();
  } catch (error) { output.failure = String(error); output.phase = phase; output.renderer = { language: document.documentElement.lang, paneHidden: document.querySelector(".tool-pane")?.hidden, heading: document.querySelector(".tool-pane h2")?.textContent }; }
  finally {
    for (const [key, value] of Object.entries(original)) {
      if (value === null) localStorage.removeItem(key); else localStorage.setItem(key, value);
    }
    await fetch(new URL('/wails/runtime', location.href), {
      method: 'POST', headers: { 'Content-Type': 'application/json', 'x-wails-client-id': window._wails.clientId },
      body: JSON.stringify({ object: 3, method: 0, args: { name: 'layout:audit', data: output } }),
    });
  }
})();
