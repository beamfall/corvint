import test from 'node:test';
import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import { ownedLifecycle } from '../lifecycle.mjs';

test('AFU-V0-010: repeated signals join delayed browser cleanup', async () => {
  const runtime = new EventEmitter(), exits = [], e = {};
  runtime.exit = code => exits.push(code);
  let release, closes = 0;
  const delayed = new Promise(resolve => { release = resolve; });
  const owner = ownedLifecycle(e, () => {}, runtime);
  await owner.launch(async () => ({ close: async () => { closes++; await delayed; } }));
  runtime.emit('SIGINT');
  runtime.emit('SIGTERM');
  runtime.emit('SIGTERM');
  const first = owner.close();
  assert.equal(owner.close(), first);
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(exits, []);
  assert.equal(closes, 1);
  release();
  await first;
  assert.equal(e.browserClosed, true);
  assert.deepEqual(exits, [130, 130, 130]);
  owner.dispose();
  assert.equal(runtime.listenerCount('SIGTERM'), 0);
});

test('AFU-V0-010: interruption waits for pending browser launch', async () => {
  const runtime = new EventEmitter(), e = {};
  runtime.exit = () => { assert.equal(e.browserClosed, true); };
  let launch, closed = 0;
  const owner = ownedLifecycle(e, () => {}, runtime);
  const pending = owner.launch(() => new Promise(resolve => { launch = resolve; }));
  runtime.emit('SIGINT');
  assert.throws(() => owner.launch(() => Promise.resolve()), /stopping/);
  launch({close:async () => { closed++; }});
  await pending;
  await owner.close();
  assert.equal(closed, 1);
  owner.dispose();
});
