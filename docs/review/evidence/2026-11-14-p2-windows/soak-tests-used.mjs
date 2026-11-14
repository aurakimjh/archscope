import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createHash } from 'node:crypto';
import { waitForCapturePersistence, finalizeDesktopCapture } from './lib/desktop-soak-checks.mjs';

const healthy = (state = 'running') => ({ state, captured: 2, persisted: 2, dropped: 0, sessionId: 'private-fixture' });

test('a running quiescent capture passes without exposing or changing counters', async () => {
  const stats = healthy();
  assert.equal(await waitForCapturePersistence(async () => stats), stats);
});

test('in-flight persistence may drain within the bounded polling window', async () => {
  let elapsed = 0, reads = 0;
  const stats = await waitForCapturePersistence(async () => (++reads < 3 ? { ...healthy(), persisted: 1 } : healthy()), {
    timeoutMs: 100, pollMs: 25, now: () => elapsed, sleep: async ms => { elapsed += ms; },
  });
  assert.equal(stats.persisted, 2);
  assert.equal(reads, 3);
  assert.equal(elapsed, 50);
});

test('a persistent mismatch fails at the deadline without an unbounded loop', async () => {
  let elapsed = 0, reads = 0;
  await assert.rejects(waitForCapturePersistence(async () => { reads++; return { ...healthy(), persisted: 1 }; }, {
    timeoutMs: 60, pollMs: 25, now: () => elapsed, sleep: async ms => { elapsed += ms; },
  }), /mismatch did not drain/);
  assert.equal(elapsed, 60);
  assert.equal(reads, 4);
});

for (const state of ['recoverable', 'failed', 'paused', 'finalized', undefined]) {
  test(`checkpoint refuses ${state} even with equal counters and zero drops`, async () => {
    await assert.rejects(waitForCapturePersistence(async () => ({ ...healthy(), state })), /capture state/);
  });
}

for (const change of [{ dropped: 1 }, { persisted: 3 }, { captured: NaN }, { persisted: -1 }, { captured: '2' }]) {
  test(`checkpoint refuses invalid/lost counters ${JSON.stringify(change)}`, async () => {
    await assert.rejects(waitForCapturePersistence(async () => ({ ...healthy(), ...change })));
  });
}

function fixture({ stopState = 'finalized', stats = healthy('finalized'), failCommand, savedRows, selectedId = 'retained' } = {}) {
  const calls = [];
  const bytes = Buffer.from(JSON.stringify({ transactions: savedRows ?? [{ id: 'retained' }] }));
  const control = async (command, params) => {
    calls.push({ command, params });
    if (command === failCommand) throw Error(`injected ${command} failure`);
    if (command === 'stop') return { state: stopState };
    if (command === 'stats') return stats;
    if (command === 'list') return { total: 1, items: [{ transaction: { id: 'retained' } }] };
    if (command === 'open') return { selectedId };
    return {};
  };
  return { calls, bytes, run: () => finalizeDesktopCapture(control, 'private-fixture', '/fixture/session', { readSaved: async () => bytes }) };
}

test('finalization uses explicit session, post-stop counters and exact saved bytes', async () => {
  const f = fixture();
  const result = await f.run();
  assert.deepEqual(f.calls.map(v => v.command), ['stop', 'stats', 'list', 'save', 'open']);
  assert.equal(f.calls[0].params.sessionId, 'private-fixture');
  assert.equal(f.calls[1].params.sessionId, 'private-fixture');
  assert.equal(result.savedRows, 1);
  assert.equal(result.savedSessionSha256, createHash('sha256').update(f.bytes).digest('hex'));
  assert.equal('sessionId' in result.finalStats, false);
});

for (const stopState of ['recoverable', 'failed', undefined]) {
  test(`finalization refuses ${stopState} stop before any export`, async () => {
    const f = fixture({ stopState: stopState ?? null });
    await assert.rejects(f.run(), /stop must finalize/);
    assert.deepEqual(f.calls.map(v => v.command), ['stop']);
  });
}

for (const change of [{ captured: 2, persisted: 1 }, { state: 'recoverable' }, { state: 'failed' }, { dropped: 1 }]) {
  test(`finalization refuses bad authoritative counters ${JSON.stringify(change)}`, async () => {
    const f = fixture({ stats: { ...healthy('finalized'), ...change } });
    await assert.rejects(f.run());
    assert.deepEqual(f.calls.map(v => v.command), ['stop', 'stats']);
  });
}

for (const failCommand of ['stop', 'stats', 'save', 'open']) {
  test(`${failCommand} failure cannot yield a successful final result`, async () => {
    const f = fixture({ failCommand });
    await assert.rejects(f.run(), new RegExp(`injected ${failCommand} failure`));
  });
}

for (const invalid of [{ savedRows: [] }, { savedRows: [{ id: 'wrong' }] }, { selectedId: 'wrong' }]) {
  test(`incomplete saved snapshot/selection is refused ${JSON.stringify(invalid)}`, async () => {
    await assert.rejects(fixture(invalid).run(), /readback mismatch/);
  });
}
