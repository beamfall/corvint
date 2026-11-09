import test from 'node:test';
import assert from 'node:assert/strict';
import { scan, digest } from '../scan.mjs';

const prefix = "import { test, expect } from '@playwright/test';\n";
const body = `test('case', async ({ page }) => {
  await page.goto('/editor');
  await page.locator('#name').fill('sample');
  await page.locator('#save').click();
  await expect(page.locator('#status')).toHaveText('saved');
});`;
const run = text => scan([{ path:'flow.spec.ts', text }], { 'flow.spec.ts':digest(text) });

test('AFU-V0-004: source/test/assertion anchors and input digests', () => {
  const r = run(prefix + body);
  assert.equal(r.inventoryComplete, true);
  assert.equal(r.tests[0].complete, true);
  assert.equal(r.tests[0].actions[0].value, digest('sample'));
  assert.equal(r.tests[0].assertions[0].anchor.line, 6);
  assert.equal(r.tests[0].assertions[0].wantDigest, digest(JSON.stringify('saved')));
  assert.ok(!JSON.stringify(r).includes('sample'));
});

test('AFU-V0-004/005: unsupported syntax cannot certify missing assertions', () => {
  const changes = [
    body.replace('async ({ page })', 'async ({ page = evil })'),
    body.replace("await page.locator('#save').click();", "if (true) await page.locator('#save').click();"),
    body.replace("await page.goto('/editor');", "await helper(page);"),
    body.replace("await expect", "expect"),
    body.replace('test(', 'test.skip('),
    body.replace("await page.locator('#save').click();", "const expect = fake;"),
    body.replace("await page.locator('#name').fill('sample');", "await page.locator(selector).fill(value);"),
    body.replace('});', "await page.reload();\n});"),
  ];
  for (const b of changes) assert.equal(run(prefix+b).inventoryComplete, false, b);
  assert.equal(run(prefix+"import helper from './helper';\n"+body).inventoryComplete, false);
});

test('AFU-V0-002: comments and string contents are not runnable tests', () => {
  const r = run(prefix + `/* ${body} */`);
  assert.equal(r.tests.length, 0);
  const q = run(prefix + `const text = ${JSON.stringify(body)};`);
  assert.equal(q.inventoryComplete, false);
  assert.ok(q.tests.every(t => !t.complete));
});

test('AFU-V0-005: no assertion remains an action candidate', () => {
  const r = run(prefix + body.replace("  await expect(page.locator('#status')).toHaveText('saved');\n", ''));
  assert.equal(r.inventoryComplete, true);
  assert.equal(r.tests[0].assertions.length, 0);
});
