// Owned native WebView2 smoke. Renderer geometry is separate from OS input/font QA.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {mkdir, readFile, writeFile, appendFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {performance} from 'node:perf_hooks';

assert.equal(process.platform, 'win32');
const [binary, destination] = process.argv.slice(2);
assert(binary && destination, 'Usage: node scripts/verify-windows-ribbon.mjs EXE NEW_OUTPUT_DIRECTORY');
const exe = path.resolve(binary), out = path.resolve(destination);
await mkdir(out, {recursive: false});
const data = await readFile(exe), pe = data.readUInt32LE(0x3c), machine = data.readUInt16LE(pe+4);
assert([0x8664, 0x14c].includes(machine), 'Expected Windows x64/x86 PE');
const script = path.resolve('scripts/verify-ribbon-native.js');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const summary = {schemaVersion: 1, scope: 'Windows native WebView2 renderer smoke; OS keyboard/font/IME/DPI require separate receipts', binarySha256: hash(data), scriptSha256: hash(await readFile(script)), PEMachine: '0x'+machine.toString(16), runs: [], passed: false};
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
for (const [name, width, height, zoom] of [['wide',1280,800,1], ['compact',1024,768,1.25], ['narrow',900,768,1.25]]) {
  const profile = path.join(out, name); await mkdir(profile);
  const resultPath = path.join(profile, 'native.json'), started = performance.now();
  const child = spawn(exe, ['--automation-descriptor', path.join(profile,'.private-controller.json')], {windowsHide: false, stdio: ['ignore','pipe','pipe'], env: {...process.env, ARCHSCOPE_LAYOUT_SCRIPT: script, ARCHSCOPE_LAYOUT_OUT: resultPath, ARCHSCOPE_LAYOUT_MAX_SECONDS: '300', ARCHSCOPE_LAYOUT_VIEWPORT: JSON.stringify({width,height,zoom})}});
  let launchError, logBytes = 0;
  child.on('error', error => { launchError = error; });
  const log = chunk => { const kept = chunk.subarray(0, Math.max(0,262144-logBytes)); logBytes += kept.length; if(kept.length) appendFile(path.join(profile,'.private-desktop.log'),kept).catch(()=>{}); };
  child.stdout.on('data',log); child.stderr.on('data',log);
  let native;
  try {
    while(performance.now()-started < 310000) {
      if(launchError) throw launchError;
      try { native=JSON.parse(await readFile(resultPath,'utf8')); break; } catch(error) { if(error.code!=='ENOENT') throw error; }
      if(child.exitCode!==null || child.signalCode!==null) throw Error('Native app exited without a receipt');
      await pause(250);
    }
    assert(native, 'No complete receipt before timeout; partial renderer/progress output retained privately');
    const report = native.report;
    assert.equal(native.webviewZoom, zoom, 'Actual WebView zoom differs');
    assert.equal(report.passed, true, report.failure);
    assert.deepEqual(report.locales.map(item=>item.locale), ['en','ko','ja','zh-CN','zh-TW']);
    assert.equal(report.themes.length,3);
    assert(report.locales.every(item=>item.selected==='true' && item.menuRows===1 && item.ribbonHeight<=126));
    assert.equal(report.viewport.pixelRatio, native.osScreenScaleFactor*zoom, 'Device ratio must reflect observed OS scale and WebView zoom');
    summary.runs.push({name,requested:{width,height,zoom},elapsedSeconds:(performance.now()-started)/1000,receipt:native});
    console.log(JSON.stringify({name,passed:true,actualZoom:native.webviewZoom,osScale:native.osScreenScaleFactor,viewport:report.viewport}));
  } catch(error) {
    summary.runs.push({name,passed:false,failure:String(error)});
    throw error;
  } finally {
    for(let i=0;i<40 && child.exitCode===null && child.signalCode===null;i++) await pause(100);
    if(child.exitCode===null && child.signalCode===null) child.kill();
    summary.passed=summary.runs.length===3 && summary.runs.every(item=>item.receipt?.report.passed);
    await writeFile(path.join(out,'result.json'),JSON.stringify(summary,null,2)+'\n');
  }
}
