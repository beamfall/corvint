import * as assert from "node:assert/strict";
import { test } from "node:test";
import { AsyncTaskTracker } from "../src/lifecycle.js";

test("shutdown waits for every admitted asynchronous command", async () => {
  const tracker = new AsyncTaskTracker();
  let release: (() => void) | undefined;
  const task = new Promise<void>((resolve) => {
    release = resolve;
  });
  void tracker.track(task);

  let closed = false;
  const closing = tracker.close().then(() => {
    closed = true;
  });
  const concurrentClose = tracker.close();
  await Promise.resolve();
  assert.equal(closed, false);
  release?.();
  await Promise.all([closing, concurrentClose]);
  assert.equal(closed, true);
  await assert.rejects(tracker.track(Promise.resolve()), /LIFECYCLE_CLOSED/);
});
