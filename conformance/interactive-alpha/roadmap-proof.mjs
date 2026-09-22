import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { join } from "node:path";

const [consolePath, atmPath, snapshotPath, repository, specs, dependencyRoot] = process.argv.slice(2);
for (const value of [consolePath, atmPath, snapshotPath, repository, specs, dependencyRoot]) {
  if (!value || !value.startsWith("/")) throw new Error("roadmap proof requires six absolute paths");
}
const require = createRequire(join(dependencyRoot, "package.json"));
const { chromium } = require("playwright");
let child;
let browser;
let stderr = "";
let result;
let cleaning;
if (process.argv[8] === "--core") {
  await coreProof();
} else {
for (const [signal, code] of [["SIGINT", 130], ["SIGTERM", 143]]) {
  process.on(signal, () => { void cleanup().then(() => process.exit(code), error => { console.error(error); process.exit(1); }); });
}
try {
  child = spawn(consolePath, ["--addr", "127.0.0.1:0", "--repo", repository, "--specs", specs, "--atm", atmPath, "--snapshot", snapshotPath], {
    stdio: ["ignore", "ignore", "pipe"], env: { PATH: "/usr/bin:/bin:/usr/sbin:/sbin" },
  });
  child.stderr.on("data", chunk => { stderr += chunk; if (stderr.length > (1 << 20)) child.kill("SIGTERM"); });
  const address = await waitForAddress(() => stderr, 30_000);
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  const response = await page.goto(`http://${address}/roadmap`, { waitUntil: "networkidle", timeout: 30_000 });
  if (!response?.ok()) throw new Error(`roadmap HTTP ${response?.status()}`);
  const body = await page.locator("body").innerText();
  const links = await page.locator('.roadmap-ticket a[href^="/ticket?id="]').count();
  if (links !== 11) throw new Error(`roadmap rendered ${links} planning tickets, expected 11: ${body.slice(0, 2000)}`);
  for (const text of ["Finish the smallest installed core / ticket / console path", "Qualify and prepare the public release", "M0", "M4", "read by", "ticket:corvint:planning:IPR-10"]) {
    if (!body.includes(text)) throw new Error(`roadmap lacks ${JSON.stringify(text)}`);
  }
  const forms = await page.locator("form").count();
  if (forms !== 0) throw new Error(`roadmap exposed ${forms} mutation forms`);
  result = {profile:"corvint-installed-roadmap-proof/0",status:"PASS",tickets:links,forms,bodySha256:createHash("sha256").update(body).digest("hex"),sourceDigest:createHash("sha256").update(await readFile(join(specs,"script/seed-planning-store-data.json"))).digest("hex")};
} finally {
  await cleanup();
}
process.stdout.write(`${JSON.stringify(result)}\n`);

}

async function cleanup() {
  if (cleaning) return cleaning;
  cleaning = (async () => {
    const errors = [];
    if (browser) try { await browser.close(); } catch (error) { errors.push(error); }
    if (child && child.exitCode === null) {
      try { child.kill("SIGTERM"); await waitChildExit(child, 5_000); } catch (error) { errors.push(error); }
      if (child.exitCode === null) {
        try { child.kill("SIGKILL"); await waitChildExit(child, 5_000); } catch (error) { errors.push(error); }
      }
      if (child.exitCode === null) errors.push(new Error("console survived forced shutdown"));
    }
    if (errors.length) throw new AggregateError(errors, "roadmap proof cleanup failed");
  })();
  return cleaning;
}

async function waitChildExit(processChild, timeout) {
  if (processChild.exitCode !== null) return;
  await Promise.race([new Promise(resolve => processChild.once("exit", resolve)), new Promise(resolve => setTimeout(resolve, timeout))]);
}

async function waitForAddress(output, timeout) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const match = output().match(/corvint console on http:\/\/(127\.0\.0\.1:\d+)/);
    if (match) return match[1];
    if (child.exitCode !== null) throw new Error(`console exited ${child.exitCode}: ${output()}`);
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  throw new Error(`console did not report address: ${output()}`);
}


async function coreProof() {
  const { coreBrowserExecutable, Child, cleanup: closeOwned, registerCleanup, until, processTable, descendants, treeSHA, fileSHA, sha, assert, delay } = await import('./core-process.mjs');
  const { request } = await import('node:http');
  const seen = new Map(); let coreBrowser; let consoleChild; let result;
  const track = () => { for (const row of descendants(process.pid, processTable())) seen.set(`${row.pid}:${row.startTime}`, row); };
  const timer = setInterval(track, 100);
  const closeBrowser = async () => { if (coreBrowser) { await coreBrowser.close(); coreBrowser = undefined; } };
  const unregister = registerCleanup(closeBrowser);
  const store = join(repository, '.git/taskman');
  const before = await treeSHA(store);
  try {
    consoleChild = new Child([consolePath, '--addr', '127.0.0.1:0', '--repo', repository, '--specs', specs, '--atm', atmPath, '--snapshot', snapshotPath], repository, 120000);
    const address = await until(() => consoleChild.stderr.toString().match(/Corvint Console on http:\/\/(127\.0\.0\.1:\d+)/)?.[1], consoleChild, 30000);
    const executable = coreBrowserExecutable(dependencyRoot);
    coreBrowser = await chromium.launch({ headless: true, executablePath: executable, handleSIGINT: false, handleSIGTERM: false });
    const version = coreBrowser.version(); const executableSHA = await fileSHA(executable);
    const page = await coreBrowser.newPage();
    const response = await page.goto(`http://${address}/roadmap`, { waitUntil: 'networkidle', timeout: 30000 }); assert(response?.ok(), 'real browser roadmap failed');
    const body = await page.locator('body').innerText(); const tickets = await page.locator('.roadmap-ticket a[href^="/ticket?id="]').count(); const forms = await page.locator('form').count();
    assert(tickets === 11 && forms === 0, `roadmap inventory/forms differ: tickets=${tickets}, forms=${forms}`);
    for (const text of ['Finish the smallest installed core / ticket / console path', 'Qualify and prepare the public release', 'M0', 'M4', 'read by', 'ticket:corvint:planning:IPR-10']) assert(body.includes(text), 'roadmap missing expected planning evidence');
    await page.locator('.roadmap-ticket a[href^="/ticket?id="]').first().click(); assert((await page.locator('body').innerText()).includes('IPR-'), 'real ticket detail missing');
    const evidenceResponse = await page.goto(`http://${address}/evidence`); assert(evidenceResponse?.ok(), 'real evidence page unavailable');
    async function refusal(headers, token) {
      const payload = new URLSearchParams({ verb: 'ticket-set-title', ...(token === undefined ? {} : { token }) }).toString();
      const observed = await new Promise((resolve, reject) => {
        const req = request(`http://${address}/mutate`, { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded', ...headers }, timeout: 5000 }, res => {
          let bytes = ''; res.on('data', chunk => { bytes += chunk; if (bytes.length > 65536) req.destroy(new Error('refusal response overflow')); }); res.on('end', () => resolve({ status: res.statusCode, body: bytes }));
        }); req.on('error', reject); req.on('timeout', () => req.destroy(new Error('refusal request timed out'))); req.end(payload);
      });
      console.log(JSON.stringify({ request: { method: 'POST', path: '/mutate', headers, token: token ?? null }, response: observed }));
      assert(observed.status === 403, 'unsafe console request was not refused before effects'); return observed.body;
    }
    assert((await refusal({ Origin: 'http://untrusted.invalid' }, 'invalid')).includes('unexpected Origin'), 'Origin refusal mismatch');
    assert((await refusal({ Host: 'untrusted.invalid' }, 'invalid')).includes('unexpected Host'), 'Host refusal mismatch');
    assert((await refusal({}, undefined)).includes('invalid form token'), 'missing session token accepted');
    assert((await refusal({}, 'invalid')).includes('invalid form token'), 'invalid session token accepted');
    track(); assert([...seen.values()].some(x => /chromium/i.test(x.command)), 'no actual browser descendant observed');
    result = { profile: 'corvint-core-console-proof/0', status: 'PASS', browserName: 'Chromium', browserVersion: version, browserExecutableSha256: executableSHA, tickets, forms, bodySha256: sha(body), sourceDigest: await fileSHA(join(specs, 'script/seed-planning-store-data.json')), originRefusal: true, hostRefusal: true, sessionRefusal: true, storeBeforeSha256: before, storeAfterSha256: await treeSHA(store), observedIdentities: [...seen.values()].map(({ pid, startTime, observation }) => ({ pid, startTime, observation })), remainingIdentities: [] };
    assert(result.storeAfterSha256 === before, 'console requests changed planning state');
    await consoleChild.stop('SIGINT');
  } finally {
    clearInterval(timer); try { await closeOwned(); } finally { unregister(); }
    const deadline = Date.now() + 5000;
    while (true) { const rows = processTable(); const remaining = [...seen.values()].filter(x => rows.get(x.pid)?.startTime === x.startTime); if (!remaining.length) break; assert(Date.now() < deadline, 'console/browser descendant survived interruption'); await delay(50); }
  }
  process.stdout.write(JSON.stringify(result) + '\n');
}
