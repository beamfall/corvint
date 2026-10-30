import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { mkdtemp, readFile, writeFile, copyFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'node:http';

const assets = fileURLToPath(new URL('..', import.meta.url));
const fixture = fileURLToPath(new URL('./fixture/', import.meta.url));
const core = process.env.CORVINT_CORE_BIN;
const observer = process.env.CORVINT_FLOW_BIN;
if (!core || !observer) throw new Error('run through script/web-flows-gate with built binaries');
const children = new Set(), servers = new Set();
const directories = [];
let interrupted = false;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const stop = () => {
  for (const child of children) child.kill('SIGTERM');
  for (const server of servers) { server.closeAllConnections(); server.close(); }
};
process.once('SIGINT', () => { interrupted = true; stop(); process.exitCode = 130; });
process.once('SIGTERM', () => { interrupted = true; stop(); process.exitCode = 143; });
process.on('exit', stop);

function run(command, args, cwd) {
  return new Promise((resolve, reject) => {
    if (interrupted) { reject(new Error('gate interrupted')); return; }
    const child = spawn(command, args, { cwd, stdio:['ignore','pipe','pipe'] });
    children.add(child);
    let out = '', err = '';
    const timer = setTimeout(() => child.kill('SIGTERM'), 60000);
    child.stdout.on('data', b => { out += b; if (out.length > 6*1024*1024) child.kill('SIGTERM'); });
    child.stderr.on('data', b => { err += b; });
    child.on('error', reject);
    child.on('close', code => {
      clearTimeout(timer); children.delete(child);
      if (code !== 0) { reject(new Error(`${command} exit ${code}: ${err.slice(0,1000)}`)); return; }
      resolve(out);
    });
  });
}

function descendants(pid) {
  const lines = execFileSync('ps',['-axo','pid=,ppid=,comm='],{encoding:'utf8'}).trim().split('\n');
  const processes = lines.map(line => /^\s*(\d+)\s+(\d+)\s+(.+)$/.exec(line)).filter(Boolean)
    .map(m => ({pid:Number(m[1]),parent:Number(m[2]),name:m[3]}));
  const ids = new Set([pid]);
  for (let pass=0;pass<10;pass++) for (const p of processes) if (ids.has(p.parent)) ids.add(p.pid);
  return processes.filter(p => p.pid!==pid && ids.has(p.pid));
}

async function interruption(root, signal) {
  const child=spawn(observer,['--experimental','--trusted-local','--observe','--root',root,'--manifest','flows.json','--assets',assets],{cwd:root,stdio:'ignore'});
  children.add(child);
  const done=new Promise(resolve=>child.once('close',code=>{children.delete(child);resolve(code);}));
  try {
    let owned=[];
    const deadline=Date.now()+15000;
    while(Date.now()<deadline) {
      owned=descendants(child.pid);
      if(owned.some(p=>/chrome|chromium/i.test(p.name))) break;
      await sleep(40);
    }
    assert.ok(owned.some(p=>/chrome|chromium/i.test(p.name)),'real browser never started');
    child.kill(signal);
    const code=await Promise.race([done,sleep(5000).then(()=>{throw new Error('interrupted observer did not exit');})]);
    assert.notEqual(code,0);
    for(const p of owned) {
      let present=true;
      for(let i=0;i<50;i++) {
        try {process.kill(p.pid,0);} catch {present=false;break;}
        await sleep(20);
      }
      assert.equal(present,false,`descendant ${p.pid} survived ${signal}`);
    }
    return owned.length;
  } finally {child.kill('SIGTERM');await done;}
}

async function port() {
  const s = createServer();
  servers.add(s);
  await new Promise(resolve => s.listen(0, '127.0.0.1', resolve));
  const p = s.address().port;
  await new Promise(resolve => s.close(resolve));
  servers.delete(s);
  return p;
}

function commit(root) {
  execFileSync('git', ['add','.'], { cwd:root, stdio:'ignore' });
  execFileSync('git', ['-c','user.name=Flow Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture'], { cwd:root });
}

async function prepare(mode = 'good', variant = false, addition = '') {
  const root = await mkdtemp(join(tmpdir(), 'corvint-web-flow-'));
  directories.push(root);
  for (const f of ['server.cjs','index.html','flows.spec.ts']) await copyFile(join(fixture,f),join(root,f));
  let html = await readFile(join(root,'index.html'),'utf8');
  let tests = await readFile(join(root,'flows.spec.ts'),'utf8');
  if (variant) {
    html = html.replaceAll('name"','title"').replaceAll('#name','#title').replaceAll('save"','submit"');
    tests = tests.replaceAll('#name','#title').replaceAll('#save','#submit').replaceAll('/editor','/author');
  }
  await writeFile(join(root,'index.html'),html+addition);
  await writeFile(join(root,'flows.spec.ts'),tests);
  const p = await port();
  const name = variant ? '#title' : '#name', save = variant ? '#submit' : '#save';
  const edit = variant ? '/author' : '/editor';
  const actions = value => [{ kind:'fill',selector:name,value },{ kind:'click',selector:save }];
  const manifest = {
    profile:'application-flow-intent/0', application:'fixture', origin:`http://127.0.0.1:${p}`,
    sources:['index.html','server.cjs'], tests:['flows.spec.ts'], backendSource:'server.cjs',
    fixture:`fixture-${mode}`, identityPath:'/__identity', resetPath:'/__reset',
    server:[process.execPath,'server.cjs',String(p),mode,`fixture-${mode}`], scenarios:[
      { id:'create',role:'editor',path:edit,basis:'declared',actions:[...actions('sample'),{ kind:'reload' }], checks:[
        { id:'visible-item',kind:'text',selector:'#items',want:'sample' },
        { id:'persisted',kind:'json',path:'/__probe',pointer:'/count',want:1 },
      ] },
      { id:'click-only',role:'editor',path:edit,basis:'declared',actions:actions('draft'),checks:[
        { id:'saved-status',kind:'text',selector:'#status',want:'saved' },
      ] },
      { id:'viewer-denied',role:'viewer',path:'/viewer',basis:'declared',actions:actions('forbidden'),checks:[
        { id:'denied-status',kind:'text',selector:'#status',want:'denied' },
        { id:'unchanged',kind:'json',path:'/__probe',pointer:'/count',want:0 },
      ] },
    ],
  };
  await writeFile(join(root,'flows.json'),JSON.stringify(manifest));
  execFileSync('git',['init','-q'],{cwd:root});
  commit(root);
  return { root, manifest };
}

async function observe(root, live = true) {
  const args = ['--experimental','--trusted-local','--root',root,'--manifest','flows.json','--assets',assets];
  if (live) args.push('--observe');
  const raw = await run(observer,args,root);
  const evidence = JSON.parse(raw);
  const file = join(root,'evidence.json');
  await writeFile(file,raw);
  const report = JSON.parse(await run(core,['--root',root,'flows','--manifest','flows.json','--evidence',file],root));
  return { evidence,report,file };
}

const rows = [];
try {
  for (const variant of [false,true]) {
    const { root } = await prepare('good',variant);
    const { evidence,report,file } = await observe(root);
    assert.deepEqual(report.flows.map(f => f.runtime),['matched','matched','matched']);
    assert.equal(report.flows[0].coverage[0].state,'assertion-candidate');
    assert.equal(report.flows[1].coverage[0].state,'action-candidate-without-assertion');
    assert.equal(report.flows[2].coverage[0].state,'no-test-in-declared-inventory');
    assert.equal(report.complete,false);
    assert.ok(report.discovered.some(c => c.selector==='#help' && !c.exercised));
    assert.ok(!JSON.stringify(evidence).includes('forbidden'));
    assert.ok(evidence.cleanup);
    assert.ok(evidence.browserClosed && evidence.serverExited);
    assert.equal(report.candidates.length,2);
    assert.ok(report.candidates.every(c=>c.basis==='test-inferred'));
    const record = join(root,'retained.json');
    await run(core,['--root',root,'flows','record','--manifest','flows.json','--evidence',file,'--output',record],root);
    await assert.rejects(run(core,['--root',root,'flows','record','--manifest','flows.json','--evidence',file,'--output',record],root));
    await writeFile(join(root,'index.html'),(await readFile(join(root,'index.html'),'utf8'))+'\n<!-- changed -->');
    commit(root);
    const stale = JSON.parse(await run(core,['--root',root,'flows','--manifest','flows.json','--evidence',file],root));
    assert.equal(stale.freshness,'stale');
    assert.ok(stale.flows.every(f => f.runtime==='stale'));
    rows.push({case:variant?'held-back-selectors':'development-fixture',result:'PASS',discoveredControls:evidence.controls.length,knownFlowCount:3});
  }
  for (const mode of ['persist-broken','auth-broken']) {
    const { root } = await prepare(mode);
    const { report } = await observe(root);
    const index = mode==='persist-broken'?0:2;
    assert.equal(report.flows[index].runtime,'contradicted');
    assert.ok(report.flows[index].checks.some(c => c.layer==='backend' && c.outcome==='contradicted'));
    if (index===0) assert.equal(report.flows[0].checks[0].outcome,'matched');
    rows.push({case:mode,result:'PASS',independentBackendDetected:true});
  }
  const wrong = await prepare('wrong-identity');
  const w = await observe(wrong.root);
  assert.ok(w.evidence.gaps.includes('served-identity-unavailable'));
  assert.ok(w.report.flows.every(f => f.runtime!=='matched'));
  rows.push({case:'wrong-served-identity',result:'PASS'});
  let external = 0;
  const sentinel = createServer((req,res) => { external++; res.end('unexpected'); });
  servers.add(sentinel);
  await new Promise(resolve => sentinel.listen(0,'127.0.0.1',resolve));
  const url = `http://127.0.0.1:${sentinel.address().port}/out-of-scope`;
  const hostile = await prepare('good',false,`<script>fetch(${JSON.stringify(url)}).catch(()=>{}); new WebSocket(${JSON.stringify(url.replace('http:','ws:'))});</script>`);
  const h = await observe(hostile.root);
  assert.equal(external,0);
  assert.ok(h.evidence.gaps.includes('request-blocked-or-unavailable'));
  assert.ok(h.evidence.gaps.includes('websocket-blocked'));
  rows.push({case:'cross-origin-http-and-websocket',result:'PASS',externalRequests:external});
  sentinel.closeAllConnections();
  await new Promise(resolve => sentinel.close(resolve));
  servers.delete(sentinel);
  const scanOnly = await prepare();
  const scanned = await observe(scanOnly.root,false);
  assert.equal(scanned.evidence.browser,'');
  assert.equal(scanned.evidence.runs.length,0);
  rows.push({case:'source-only',result:'PASS'});
  for(const signal of ['SIGINT','SIGTERM']) {
    const {root}=await prepare();
    rows.push({case:signal,result:'PASS',retiredDescendants:await interruption(root,signal)});
  }
  process.stdout.write(JSON.stringify({profile:'application-flow-evaluation/0',rows,falseConfirmations:0,seededDefectsDetected:2,externalApplicationAccuracy:'NOT_OBSERVED'})+'\n');
} finally {
  stop();
  await Promise.all([...children].map(c => new Promise(resolve => c.once('close',resolve))));
  for (const root of directories) await rm(root,{recursive:true,force:true});
}
