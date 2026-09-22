import { test } from 'node:test';
import { strict as assert } from 'node:assert';
import { mkdtemp, rm, writeFile, mkdir, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { coreBrowserExecutable, Child, cleanup, registerCleanup, until, fileSHA, treeSHA } from './core-process.mjs';

if (process.argv.includes('--browser-hook-fixture')) {
  const { chromium } = createRequire(process.env.CORVINT_TEST_PLAYWRIGHT + '/package.json')('playwright');
  const browser = await chromium.launch({ headless: true, executablePath: coreBrowserExecutable(process.env.CORVINT_TEST_PLAYWRIGHT), handleSIGINT: false, handleSIGTERM: false }); await browser.newPage();
  registerCleanup(() => new Promise(() => {}));
  process.stdout.write('READY\n'); await new Promise(() => {});
} else if (process.argv.includes('--escape-fixture')) {
  const code = 'import os,signal,time\npid=os.fork()\nif pid==0:\n os.setsid();signal.signal(signal.SIGTERM,signal.SIG_IGN);signal.signal(signal.SIGINT,signal.SIG_IGN)\n while True:time.sleep(1)\nelse:\n print(pid,flush=True)\n while True:time.sleep(1)\n';
  try {
    const child = new Child(['/usr/bin/python3', '-c', code], process.cwd(), 60000);
    await until(() => child.observed.size > 0, child, 5000);
    process.stdout.write('READY\n');
    if (process.argv.includes('--blocked-hook')) { registerCleanup(() => new Promise(() => {})); throw new Error('intentional blocked hook'); }
    if (process.argv.includes('--setup-failure')) throw new Error('intentional setup failure');
    await new Promise(() => {});
  } finally { await cleanup(); }
} else {
  for (const signal of ['SIGINT', 'SIGTERM', 'setup-failure', 'blocked-hook']) test(`bounded ${signal} cleanup retires escaped pipe-holder`, { timeout: 25000 }, async () => {
    const child = new Child([process.execPath, fileURLToPath(import.meta.url), '--escape-fixture', ...(['setup-failure', 'blocked-hook'].includes(signal) ? ['--' + signal] : [])], process.cwd(), 20000);
    try {
      await until(() => child.stdout.includes('READY'), child, 10000);
      if (signal.startsWith('SIG')) child.process.kill(signal);
      await child.join(1);
      assert.equal(child.code, 1, 'escaped child requires visible fallback failure');
      assert.ok(child.observed.size > 0);
      assert.deepEqual(child.remaining, []);
    } finally { await child.stop(); console.log(JSON.stringify({ observedIdentities: [...child.observed.values()], remainingIdentities: child.remaining })); }
  });
  for (const signal of ['SIGINT', 'SIGTERM']) test(`blocked browser close ${signal} retires real browser`, { timeout: 25000, skip: !process.env.CORVINT_TEST_PLAYWRIGHT }, async () => {
    const child = new Child([process.execPath, fileURLToPath(import.meta.url), '--browser-hook-fixture'], process.cwd(), 20000);
    try {
      await until(() => child.stdout.includes('READY'), child, 10000); child.observe();
      assert.ok([...child.observed.values()].some(row => /chromium/i.test(row.command)));
      child.process.kill(signal); await child.join(1); assert.deepEqual(child.remaining, []);
    } finally { await child.stop(); console.log(JSON.stringify({ observedIdentities: [...child.observed.values()], remainingIdentities: child.remaining })); }
  });
  test('inventory refuses nonregular and escaping inputs', async () => {
    const root = await mkdtemp(join(tmpdir(), 'core-inventory-'));
    try {
      await mkdir(join(root, 'd')); await writeFile(join(root, 'file'), 'content');
      assert.match(await treeSHA(root), /^[a-f0-9]{64}$/);
      await symlink('/etc/hosts', join(root, 'escape'));
      await assert.rejects(treeSHA(root), /escapes/);
      await assert.rejects(fileSHA(join(root, 'escape')), /regular-file/);
    } finally { await rm(root, { recursive: true, force: true }); }
  });
}
