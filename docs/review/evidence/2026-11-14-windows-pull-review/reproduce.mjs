// Read-only reproductions against the pulled sources. No Windows execution.
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../../', import.meta.url));
const compiled = new URL('../../../../apps/engine-native/cmd/archscope-app/frontend/.tmp/state-tests/state/', import.meta.url);
const { httpColumnValue, httpColumnMatches } = await import(new URL('httpColumns.js', compiled));
const { buildRawRequest } = await import(new URL('httpAnalyzer.js', compiled));
const row = {
  method: 'GET', path: '/lookup', query: '', host: 'example.test',
  url: 'http://example.test/lookup?next=/detail?id=7&keep=2', request: { headers: [] },
};
const query = {
  url: row.url, expectedPath: '/lookup?next=/detail?id=7&keep=2',
  actualPath: httpColumnValue(row, 'path'),
  containsNestedID: httpColumnMatches(row, 'path', 'contains', 'id=7'),
  rawStartLine: buildRawRequest(row).startLine,
};

// Execute the actual checkpoint and finalization bodies with controlled ports.
// These are deterministic JS predicate probes, not native controller tests.
const source = readFileSync(path.join(root, 'scripts/verify-windows-desktop-soak.mjs'), 'utf8');
const checkpointBody = source.slice(source.indexOf('async function checkpoint(elapsed) {'), source.indexOf('\ntry {\n  child = spawn'));
const report = { checkpoints: [] };
const item = { transaction: { url: 'http://127.0.0.1:8080/soak', process: { key: { pid: 123 }, attribution: 'confirmed' } } };
const stats = { state: 'recoverable', captured: 2, persisted: 1, dropped: 0 };
const control = async command => command === 'list' ? { total: 1, items: [item] } : stats;
const memory = { Responding: true, MainWindowHandle: 1, PrivateMemorySize64: 1234, TreeProcessCount: 1 };
const runCheckpoint = new Function('child', 'control', 'session', 'origin', 'process', 'execFileSync', 'report', 'writeFile', 'path', 'out', checkpointBody + '; return checkpoint(1);');
let rejected = false;
try {
  await runCheckpoint({ pid: 456, exitCode: null, signalCode: null }, control, { sessionId: 'fixture' }, new URL('http://127.0.0.1:8080/'), { pid: 123 }, () => JSON.stringify(memory), report, async () => {}, { join: () => '/unused' }, '/unused');
} catch { rejected = true; }

const finalBody = source.slice(source.indexOf("  phase = 'finalize';"), source.indexOf('} catch (error) {\n  report.failure'));
const finalReport = {};
const finalControl = async command => {
  if (command === 'stop') return { state: 'recoverable' };
  if (command === 'list') return { total: 1, items: [{ transaction: { id: 'retained' } }] };
  if (command === 'open') return { selectedId: 'retained' };
  return {};
};
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
await new AsyncFunction('control', 'session', 'path', 'out', 'readFile', 'report', 'createHash', 'let phase;' + finalBody)(
  finalControl, { sessionId: 'fixture' }, { join: () => '/unused' }, '/unused',
  async () => Buffer.from(JSON.stringify({ transactions: [{ id: 'retained' }] })), finalReport, createHash,
);
console.log(JSON.stringify({
  schemaVersion: 1, reviewedHead: 'ab42eeb8', platformScope: 'macOS source review and controlled JavaScript probes; no new Windows run',
  WR01: query,
  WR02: { inputStats: stats, checkpointRejected: rejected, stopState: 'recoverable', finalRunnerPassed: finalReport.passed, savedSubsetRows: finalReport.savedRows },
}, null, 2));
