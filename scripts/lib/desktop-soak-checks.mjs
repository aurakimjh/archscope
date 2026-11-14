// Shared by the native Windows runner and host-side predicate regressions.
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { performance } from 'node:perf_hooks';

export async function waitForCapturePersistence(readStats, {
  expectedState = 'running', timeoutMs = 5000, pollMs = 50,
  now = () => performance.now(), sleep = ms => new Promise(resolve => setTimeout(resolve, ms)),
} = {}) {
  const deadline = now() + timeoutMs;
  for (;;) {
    const stats = await readStats();
    if (stats?.state !== expectedState) throw Error(`capture state must be ${expectedState}; got ${stats?.state}`);
    for (const key of ['captured', 'persisted', 'dropped']) {
      if (!Number.isSafeInteger(stats[key]) || stats[key] < 0) throw Error(`invalid capture counter: ${key}`);
    }
    if (stats.dropped !== 0) throw Error('unexpected captured-request drops');
    if (stats.persisted > stats.captured) throw Error('persisted count exceeds captured count');
    if (stats.captured === stats.persisted) return stats;
    const remaining = deadline - now();
    if (remaining <= 0) throw Error('captured/persisted mismatch did not drain within deadline');
    await sleep(Math.min(pollMs, remaining));
  }
}

export async function finalizeDesktopCapture(control, sessionId, destination, {
  readSaved = readFile,
} = {}) {
  const stopped = await control('stop', { sessionId });
  if (stopped?.state !== 'finalized') throw Error(`capture stop must finalize; got ${stopped?.state}`);
  // Re-read authoritative counters after Stop drains and finalizes the pipeline.
  const stats = await waitForCapturePersistence(() => control('stats', { sessionId }), {
    expectedState: 'finalized', timeoutMs: 0,
  });
  const page = await control('list', { sessionId, limit: 100 });
  if (!Number.isSafeInteger(page.total) || page.total < 1 || page.total > 128 || !page.items?.[0]?.transaction?.id) {
    throw Error('invalid final retained snapshot');
  }
  const selectedId = page.items[0].transaction.id;
  await control('save', { source: { sessionId }, destination, format: 'session', scope: 'all', ids: [], selectedId });
  const bytes = await readSaved(destination);
  const saved = JSON.parse(bytes.toString('utf8'));
  const opened = await control('open', { path: destination });
  if (!Array.isArray(saved.transactions) || saved.transactions.length !== page.total ||
      !saved.transactions.some(row => row.id === selectedId) || opened.selectedId !== selectedId) {
    throw Error('saved-session readback mismatch');
  }
  const { sessionId: _privateID, ...finalStats } = stats;
  return { finalStats, savedRows: saved.transactions.length, savedSessionSha256: createHash('sha256').update(bytes).digest('hex') };
}
