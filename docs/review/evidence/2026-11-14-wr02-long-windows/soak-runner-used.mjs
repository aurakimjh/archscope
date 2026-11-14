// Real production Wails process and authenticated controller; no DOM injection.
// The origin must be an isolated loopback fixture. No system policy is changed.
import { spawn, execFileSync } from 'node:child_process';
import { appendFileSync, writeFileSync } from 'node:fs';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import { performance } from 'node:perf_hooks';
import { waitForCapturePersistence, clearDesktopCapture, finalizeDesktopCapture } from './lib/desktop-soak-checks.mjs';

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
const report = { schema: 4, platform: 'windows', scope: 'production Wails desktop/controller, synthetic Docker-loopback HTTP, bodies disabled', memoryScope: 'root process plus currently alive descendants from one Windows process snapshot; not a leak proof', requestedSeconds: seconds, binarySha256, passed: false, requests: 0, pauses: 0, clears: 0, checkpoints: [], nativeUIObservations: 'record separately; controller/liveness is not a renderer correctness claim' };
report.clearChecks = [];
let child, descriptor, session, sequence = 0, started, phase = 'launch', desktopLogBytes = 0, clearedThroughRequests = 0;
const desktopLogPath = path.join(out, '.private-desktop.log');
writeFileSync(desktopLogPath, '', { mode: 0o600 });
function desktopLog(chunk) {
  const remaining = (256 << 10) - desktopLogBytes;
  if (chunk.length > remaining) report.desktopLogTruncated = true;
  if (remaining <= 0) return;
  const kept = chunk.subarray(0, remaining);
  appendFileSync(desktopLogPath, kept);
  desktopLogBytes += kept.length;
}
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
    req.on('error', error => reject(Object.assign(new Error(`request ${index}: ${error.message}`), { code: error.code })));
    req.on('timeout', () => req.destroy(Error('origin timeout')));
  });
}
async function checkpoint(elapsed) {
  if (child.exitCode !== null || child.signalCode !== null) throw Error('desktop process exited');
  const page = await control('list', { sessionId: session.sessionId, limit: 100 });
  let checkedRows = 0, current = page;
  do {
    if (page.total > 128 || current.items.some(x => new URL(x.transaction.url).origin !== origin.origin || x.transaction.process?.key.pid !== process.pid || x.transaction.process?.attribution !== 'confirmed')) throw Error('retention/privacy/process mismatch');
    checkedRows += current.items.length;
    if (!current.nextCursor) break;
    current = await control('list', { sessionId: session.sessionId, limit: 100, cursor: current.nextCursor });
  } while (checkedRows <= 128);
  if (checkedRows !== page.total) throw Error('retained snapshot pagination mismatch');
  const stats = await waitForCapturePersistence(() => control('stats', { sessionId: session.sessionId }));
  if (stats.captured !== report.requests - clearedThroughRequests || stats.generation !== report.clears) throw Error('generation/request count mismatch');
  const script = `
$rootProcess=Get-Process -Id ${child.pid} -ErrorAction Stop
$snapshot=@(Get-CimInstance Win32_Process)
$rootEntry=$snapshot | Where-Object ProcessId -EQ ${child.pid}
if (-not $rootEntry) { throw 'Owned root missing from process snapshot' }
$queue=[System.Collections.Generic.Queue[object]]::new()
$queue.Enqueue($rootEntry)
$members=[System.Collections.Generic.List[object]]::new()
$seen=@{}
while ($queue.Count -gt 0) {
  $parent=$queue.Dequeue()
  if ($seen.ContainsKey([string]$parent.ProcessId)) { continue }
  $seen[[string]$parent.ProcessId]=$true
  $process=Get-Process -Id $parent.ProcessId -ErrorAction SilentlyContinue
  if ($process) { $members.Add($process) }
  foreach ($entry in $snapshot) {
    if ($entry.ParentProcessId -eq $parent.ProcessId -and $entry.CreationDate -ge $parent.CreationDate) { $queue.Enqueue($entry) }
  }
}
[ordered]@{
  WorkingSet64=$rootProcess.WorkingSet64; PrivateMemorySize64=$rootProcess.PrivateMemorySize64
  HandleCount=$rootProcess.HandleCount; Responding=$rootProcess.Responding; MainWindowHandle=[long]$rootProcess.MainWindowHandle
  TreeProcessCount=$members.Count
  TreePrivateMemorySize64=($members | Measure-Object PrivateMemorySize64 -Sum).Sum
  TreeWorkingSet64=($members | Measure-Object WorkingSet64 -Sum).Sum
} | ConvertTo-Json -Compress`;
  const memory = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-Command', script], { encoding: 'utf8', windowsHide: true }));
  if (!memory.Responding || !memory.MainWindowHandle || memory.PrivateMemorySize64 > 1073741824) throw Error('desktop liveness/private-memory guard failed');
  const { sessionId: _privateID, ...counters } = stats;
  const { MainWindowHandle: _handle, ...memoryCounters } = memory;
  report.checkpoints.push({ elapsedSeconds: elapsed, requests: report.requests, retainedRows: page.total, checkedRows, stats: counters, hasMainWindow: true, ...memoryCounters });
  report.elapsedSeconds = elapsed;
  await writeFile(path.join(out, 'progress.json'), JSON.stringify(report, null, 2));
}
try {
  child = spawn(exe, ['--automation-descriptor', descriptorPath], { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: false });
  child.stdout.on('data', desktopLog);
  child.stderr.on('data', desktopLog);
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
  started = performance.now(); let nextCheck = 0, nextPause = 60, nextClear = 600;
  phase = 'traffic';
  while (performance.now() - started < seconds * 1000) {
    for (let n = 0; n < 4; n++) { await request(session.listenAddress, report.requests); report.requests++; }
    const elapsed = (performance.now() - started) / 1000;
    if (elapsed >= nextPause) { await control('pause'); await delay(100); await control('resume'); report.pauses++; nextPause += 60; }
    if (elapsed >= nextClear) {
      const boundary = report.requests;
      const checked = await clearDesktopCapture(control, session.sessionId, async () => {
        const urls = [];
        for (let n = 0; n < 4; n++) {
          urls.push(`${origin}soak?sequence=${report.requests}`);
          await request(session.listenAddress, report.requests); report.requests++;
        }
        return urls;
      });
      clearedThroughRequests = boundary;
      report.clears++;
      report.clearChecks.push({ elapsedSeconds: elapsed, clearedThroughRequests: boundary, ...checked });
      nextClear += 600;
    }
    if (elapsed >= nextCheck) { await checkpoint(elapsed); nextCheck += 30; }
    await delay(1000);
  }
  // A clear exactly at the duration boundary must not turn round-trip coverage
  // into a vacuous empty-file assertion. Seed a final bounded snapshot.
  for (let n = 0; n < 4; n++) { await request(session.listenAddress, report.requests); report.requests++; }
  await checkpoint((performance.now() - started) / 1000);
  phase = 'finalize';
  const destination = path.join(out, 'final.httpsession');
  Object.assign(report, await finalizeDesktopCapture(control, session.sessionId, destination));
  report.passed = true;
} catch (error) {
  report.failure = String(error);
  report.failurePhase = phase;
  report.failureCode = error.code;
  if (started !== undefined) report.elapsedSeconds = (performance.now() - started) / 1000;
  // Diagnostics do not retry traffic or turn a transport failure into a pass.
  if (session && descriptor) {
    try {
      const { sessionId: _privateID, ...counters } = await control('stats', { sessionId: session.sessionId });
      report.failureStats = counters;
    } catch { report.failureStatsUnavailable = true; }
    try {
      const status = await control('status');
      const first = await control('events', { after: 0, limit: 1 });
      const tail = await control('events', { after: Math.max(0, first.latest - 100), limit: 100 });
      report.failureSessionState = status.state;
      report.failureCaptureErrorEvents = tail.items.filter(item => item.kind === 'error').length;
      await writeFile(path.join(out, '.private-failure-context.json'), JSON.stringify({ status, tail }, null, 2), { mode: 0o600 });
    } catch { report.failureContextUnavailable = true; }
  }
  try {
    const response = await fetch(new URL('soak?diagnostic=direct', origin), { signal: AbortSignal.timeout(10000) });
    report.directOriginHealthyAfterFailure = response.status === 200 && (await response.json()).marker === 'desktop-soak-origin';
  } catch { report.directOriginHealthyAfterFailure = false; }
  process.exitCode = 1;
} finally {
  if (session && descriptor && child?.exitCode === null) { try { await control('stop'); } catch {} }
  report.desktopAliveAtEnd = child?.exitCode === null && child?.signalCode === null;
  // Leave the owned app alive for final native observation. The caller must
  // stop only this PID after inspection; existing user processes are untouched.
  report.cleanupRequired = 'Stop the owned desktop PID recorded only in the private descriptor after final native observation.';
  await writeFile(path.join(out, 'result.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ passed: report.passed, requests: report.requests, elapsedSeconds: report.elapsedSeconds, failure: report.failure }));
  // Keep the referenced app alive so terminal-job cleanup cannot erase the
  // native final observation. Stop only the descriptor PID after inspection.
}
