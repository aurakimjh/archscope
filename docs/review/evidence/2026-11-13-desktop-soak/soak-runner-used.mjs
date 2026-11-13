// Real production Wails process and authenticated controller; no DOM injection.
// The origin must be an isolated loopback fixture. No system policy is changed.
import { spawn, execFileSync } from 'node:child_process';
import { mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import { performance } from 'node:perf_hooks';

const options = {};
for (let i = 2; i < process.argv.length; i += 2) {
  const key = process.argv[i];
  if (!['--exe', '--out', '--origin', '--duration'].includes(key) || !process.argv[i + 1]) throw Error('invalid soak option');
  options[key.slice(2)] = process.argv[i + 1];
}
if (process.platform !== 'win32') throw Error('native Windows required');
const seconds = Number(options.duration ?? 7200);
if (!Number.isSafeInteger(seconds) || seconds < 10 || seconds > 21600) throw Error('duration must be 10..21600 seconds');
const origin = new URL(options.origin);
if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash) throw Error('use a loopback-only synthetic HTTP origin');
const exe = path.resolve(options.exe), out = path.resolve(options.out);
await mkdir(out, { recursive: false });
const descriptorPath = path.join(out, '.private-controller.json');
const binarySha256 = createHash('sha256').update(await readFile(exe)).digest('hex');
const report = { schema: 1, platform: 'windows', scope: 'production Wails desktop/controller, synthetic Docker-loopback HTTP, bodies disabled', requestedSeconds: seconds, binarySha256, passed: false, requests: 0, pauses: 0, clears: 0, checkpoints: [], nativeUIObservations: 'record separately; controller/liveness is not a renderer correctness claim' };
let child, descriptor, session, sequence = 0;
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function control(command, params = {}) {
  const response = await fetch(descriptor.url, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${descriptor.token}` }, body: JSON.stringify({ id: `soak-${++sequence}`, command, params }), signal: AbortSignal.timeout(30000) });
  const result = await response.json();
  if (response.status !== 200 || !result.ok) throw Error(`controller ${command} failed`);
  return result.result;
}
function request(proxy, index) {
  return new Promise((resolve, reject) => {
    const req = http.get({ host: '127.0.0.1', port: Number(proxy.split(':').at(-1)), path: `${origin}soak?sequence=${index}`, headers: { Host: origin.host }, agent: false, timeout: 10000 }, response => {
      const chunks = []; response.on('data', c => chunks.push(c)); response.on('error', reject);
      response.on('end', () => { try { if (response.statusCode !== 200 || JSON.parse(Buffer.concat(chunks).toString()).marker !== 'desktop-soak-origin') throw Error('origin response mismatch'); resolve(); } catch (e) { reject(e); } });
    });
    req.on('error', reject); req.on('timeout', () => req.destroy(Error('origin timeout')));
  });
}
async function checkpoint(elapsed) {
  if (child.exitCode !== null) throw Error('desktop process exited');
  const page = await control('list', { sessionId: session.sessionId, limit: 100 });
  if (page.total > 128 || page.items.some(x => new URL(x.transaction.url).origin !== origin.origin || x.transaction.process?.key.pid !== process.pid || x.transaction.process?.attribution !== 'confirmed')) throw Error('retention/privacy/process mismatch');
  const stats = await control('stats', { sessionId: session.sessionId });
  const script = `$p=Get-Process -Id ${child.pid} -ErrorAction Stop; $p | Select-Object WorkingSet64,PrivateMemorySize64,HandleCount,Responding,MainWindowHandle | ConvertTo-Json -Compress`;
  const memory = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-Command', script], { encoding: 'utf8', windowsHide: true }));
  if (!memory.Responding || !memory.MainWindowHandle || memory.PrivateMemorySize64 > 1073741824) throw Error('desktop liveness/private-memory guard failed');
  const { sessionId: _privateID, ...counters } = stats;
  const { MainWindowHandle: _handle, ...memoryCounters } = memory;
  report.checkpoints.push({ elapsedSeconds: elapsed, requests: report.requests, retainedRows: page.total, stats: counters, hasMainWindow: true, ...memoryCounters });
  report.elapsedSeconds = elapsed;
  await writeFile(path.join(out, 'progress.json'), JSON.stringify(report, null, 2));
}
try {
  child = spawn(exe, ['--automation-descriptor', descriptorPath], { stdio: 'ignore', windowsHide: false });
  let launchError; child.on('error', e => { launchError = e; });
  for (let n = 0; n < 120; n++) {
    if (launchError) throw launchError;
    if (child.exitCode !== null) throw Error('desktop exited before readiness');
    try { descriptor = JSON.parse(await readFile(descriptorPath, 'utf8')); break; } catch { await delay(250); }
  }
  if (!descriptor || descriptor.pid !== child.pid || descriptor.schemaVersion !== 1 || !/^[a-f0-9]{64}$/.test(descriptor.token)) throw Error('invalid controller descriptor');
  const endpoint = new URL(descriptor.url);
  if (endpoint.protocol !== 'http:' || endpoint.hostname !== '127.0.0.1' || endpoint.pathname !== '/v1/control' || endpoint.username || endpoint.password || endpoint.search || endpoint.hash) throw Error('invalid controller endpoint');
  await delay(15000);
  report.buildCommit = (await control('capabilities')).buildCommit;
  const own = (await control('processes')).find(p => p.key.pid === process.pid && p.key.startTime);
  if (!own) throw Error('traffic generator identity unavailable');
  await control('attach', { processes: [own.key] });
  session = await control('start', { listenAddress: '127.0.0.1:0', captureBodies: false, retentionRows: 128, liveWindow: 128 });
  // Keep exact private IDs only in the isolated working directory.
  await writeFile(path.join(out, '.private-session.json'), JSON.stringify(session));
  const started = performance.now(); let nextCheck = 0, nextPause = 60, nextClear = 600;
  while (performance.now() - started < seconds * 1000) {
    for (let n = 0; n < 4; n++) { await request(session.listenAddress, report.requests); report.requests++; }
    const elapsed = (performance.now() - started) / 1000;
    if (elapsed >= nextPause) { await control('pause'); await delay(100); await control('resume'); report.pauses++; nextPause += 60; }
    if (elapsed >= nextClear) { await control('clear'); report.clears++; nextClear += 600; }
    if (elapsed >= nextCheck) { await checkpoint(elapsed); nextCheck += 30; }
    await delay(1000);
  }
  await checkpoint((performance.now() - started) / 1000);
  await control('stop');
  const page = await control('list', { sessionId: session.sessionId, limit: 100 });
  const destination = path.join(out, 'final.httpsession');
  await control('save', { source: { sessionId: session.sessionId }, destination, format: 'session', scope: 'all', ids: [], selectedId: page.items[0]?.transaction.id ?? '' });
  const saved = JSON.parse(await readFile(destination, 'utf8'));
  const opened = await control('open', { path: destination });
  if (saved.transactions.length !== page.total || opened.selectedId !== (page.items[0]?.transaction.id ?? '')) throw Error('saved-session readback mismatch');
  report.savedRows = saved.transactions.length;
  report.savedSessionSha256 = createHash('sha256').update(await readFile(destination)).digest('hex');
  report.passed = true;
} catch (error) {
  report.failure = String(error);
  process.exitCode = 1;
} finally {
  if (session && descriptor && child?.exitCode === null) { try { await control('stop'); } catch {} }
  report.desktopAliveAtEnd = child?.exitCode === null;
  // Leave the owned app alive for final native observation. The caller must
  // stop only this PID after inspection; existing user processes are untouched.
  report.cleanupRequired = 'Stop the owned desktop PID recorded only in the private descriptor after final native observation.';
  await writeFile(path.join(out, 'result.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ passed: report.passed, requests: report.requests, elapsedSeconds: report.elapsedSeconds, failure: report.failure }));
  child?.unref();
}
