async (page) => {
  await page.waitForFunction(() => window.gcTestReady === true);
  const cases = [];
  for (let chartIndex=0; chartIndex<3; chartIndex++) {
    for (const [phase,time,duration] of [
      ['Pause Remark','2026-11-19T10:00:02.123+09:00','1.2345 ms'],
      ['Pause Cleanup','2026-11-19T10:00:03.123+09:00','0 ms'],
      ['Pause Full','2026-11-19T10:00:04.123+09:00','250 ms']
    ]) {
      const surface=page.locator('.chart').nth(chartIndex);
      await surface.scrollIntoViewIfNeeded();
      const box=await surface.boundingBox();
      const offset=await page.evaluate(({chartIndex,time})=>window.gcCharts[chartIndex].chart.convertToPixel({xAxisIndex:0},Date.parse(time)),{chartIndex,time});
      await page.mouse.move(box.x+offset,box.y+150);
      await page.waitForFunction(({chartIndex,phase,time,duration})=>{
        const text=window.gcCharts[chartIndex].el.innerText;
        return text.includes(phase)&&text.includes(time)&&text.includes(duration);
      },{chartIndex,phase,time,duration},{timeout:5000});
      const text=await surface.innerText();
      cases.push({chart:chartIndex,phase,timestamp:time,duration,tooltip:text});
    }
  }
  return {passed:true,scope:'Actual production chart builders + actual Go analysis output in isolated Chromium harness; real Playwright mouse hover; not native desktop acceptance',caseCount:cases.length,cases};
}
