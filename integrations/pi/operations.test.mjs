// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,mkdir,writeFile,readFile,readdir,realpath,rename} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createOperationLedger,digest,createWorktreeIdentity} from './operations.js';
import {spawnSync} from 'node:child_process';
import {createCommandRunner} from './process.js';
import {workflowIdentity} from './workflow.js';
const intent={requestId:'one',intentSha256:digest('input'),scopeSha256:digest('scope'),issuedAt:'2026-09-30T00:00:00Z'};
async function setup(t){const dir=await mkdtemp(join(tmpdir(),'pi-ledger-'));t.after(()=>rm(dir,{recursive:true,force:true}));return {dir,ledger:createOperationLedger(dir)}}
test('PWV private ledger persists before dispatch, deduplicates and retains only digests',async t=>{
 const {dir,ledger}=await setup(t);const entry=await ledger.begin(intent);assert.equal((await ledger.inspect()).requestId,'one');
 await assert.rejects(ledger.begin(intent),/reconciliation-required/);
 await assert.rejects(ledger.begin({...intent,requestId:'two'}),/pending-operation/);
 await assert.rejects(ledger.begin({...intent,intentSha256:digest('changed')},true),/request-id-conflict/);
 const resume=await createOperationLedger(dir).begin(intent,true);assert.equal(resume.resume,true);
 await ledger.finish(entry,{private:'not persisted'});
 assert.equal(await ledger.inspect(),null);assert.equal((await ledger.begin(intent)).completed,true);
 const bytes=await readFile(join(dir,'corvint-pi-operations',digest('one')+'.json'),'utf8');assert.doesNotMatch(bytes,/not persisted/);
});
test('PWV interrupted persistence refuses reuse and simultaneous requests dispatch at most once',async t=>{
 const {dir,ledger}=await setup(t);await mkdir(join(dir,'corvint-pi-operations'));await writeFile(join(dir,'corvint-pi-operations','pending.json'),'{');
 await assert.rejects(ledger.begin(intent,true));await assert.rejects(ledger.inspect());
 await rm(join(dir,'corvint-pi-operations'),{recursive:true});
 const results=await Promise.allSettled([ledger.begin(intent),ledger.begin({...intent,requestId:'two'})]);
 assert.equal(results.filter(r=>r.status==='fulfilled').length,1);assert.equal((await readdir(join(dir,'corvint-pi-operations'))).length,1);
});
test('PWV inspect is a read and does not initialize a ledger',async t=>{const {dir,ledger}=await setup(t);assert.equal(await ledger.inspect(),null);assert.deepEqual(await readdir(dir),[])});
test('PWV recovery retires a pending marker only with its durable matching terminal record',async t=>{
 const {dir,ledger}=await setup(t);const entry=await ledger.begin(intent);await ledger.finish(entry,{receipt:'native'});
 await writeFile(join(dir,'corvint-pi-operations','pending.json'),JSON.stringify(entry));
 assert.equal((await ledger.begin(intent,true)).completed,true);assert.equal(await ledger.inspect(),null);
});


async function identityFixture(t) {
 const dir=await mkdtemp(join(tmpdir(),'pi-identity-'));t.after(()=>rm(dir,{recursive:true,force:true}));
 const git=args=>{const result=spawnSync('git',args,{cwd:dir,encoding:'utf8'});assert.equal(result.status,0,result.stderr);return result.stdout.trim()};
 git(['init','-q','-b','main']);await writeFile(join(dir,'source'),'one\n');git(['add','.']);git(['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture']);
 const ctx={cwd:dir,sessionManager:{getSessionId:()=> 'session'},isProjectTrusted:()=>true};
 return {dir,git,ctx};
}
const fixedArgs=['rev-parse','--show-toplevel','--absolute-git-dir','HEAD','--symbolic-full-name','HEAD'];

test('PWV owned identity uses one fixed Git invocation and resolves packed refs and detached HEAD',async t=>{
 const f=await identityFixture(t),calls=[],native=createCommandRunner({binary:'git',timeoutMs:1700,maxBytes:16384});
 const owner=createWorktreeIdentity({runner:{run:call=>{calls.push(call);return native.run(call)},cancel:()=>native.cancel(),close:()=>native.close()}});t.after(()=>owner.close());
 f.git(['pack-refs','--all']);const attached=await owner.read(f.ctx);
 assert.equal(calls.length,1);assert.deepEqual(calls[0].args,fixedArgs);assert.equal(attached.branch,'main');assert.equal(attached.head,f.git(['rev-parse','HEAD']));assert.equal(attached.detached,false);assert.equal(attached.sessionSha256,digest('session'));assert.match(attached.filesystemSha256,/^[a-f0-9]{64}$/);
 f.git(['checkout','--detach','-q']);const detached=await owner.read(f.ctx);assert.equal(detached.branch,'HEAD');assert.equal(detached.detached,true);assert.equal(detached.head,attached.head);assert.equal(detached.filesystemSha256,attached.filesystemSha256);
});

test('PWV trust and pre-abort stop identity before Git dispatch',async t=>{
 const f=await identityFixture(t);let calls=0;const owner=createWorktreeIdentity({runner:{run(){calls++;throw Error('unexpected')}}});t.after(()=>owner.close());
 await assert.rejects(owner.read({...f.ctx,isProjectTrusted:()=>false}),/untrusted-project/);
 const controller=new AbortController();controller.abort();await assert.rejects(owner.read({...f.ctx,signal:controller.signal}),/aborted/);assert.equal(calls,0);
});

test('PWV identity parser refuses malformed, oversized and noncanonical native output',async t=>{
 const f=await identityFixture(t),i=workflowIdentity(f.ctx),good=[i.root,i.gitDir,f.git(['rev-parse','HEAD']),'refs/heads/main'].join('\n')+'\n';
 for(const output of [good+'extra\n',good.replace('refs/heads/main','--option'),good.replace(f.git(['rev-parse','HEAD']),'not-an-oid'),good.slice(0,-1),'x'.repeat(16385),good.replace(i.root,'relative')]){
  const owner=createWorktreeIdentity({runner:{run:async()=>({exitCode:0,stdout:output})}});
  await assert.rejects(owner.read(f.ctx),/identity-unavailable/);await owner.close();
 }
});

test('PWV identity freezes cwd/session and rejects transition during Git or filesystem awaits',async t=>{
 for(const mode of ['cwd','session','trust','cancel','abort'])await t.test(mode,async t=>{
  const f=await identityFixture(t);let release,session='session';f.ctx.sessionManager.getSessionId=()=>session;const c=new AbortController();f.ctx.signal=c.signal;
  const i=workflowIdentity(f.ctx),output=[i.root,i.gitDir,f.git(['rev-parse','HEAD']),'refs/heads/main'].join('\n')+'\n';
  const owner=createWorktreeIdentity({runner:{run:()=>new Promise(resolve=>release=()=>resolve({exitCode:0,stdout:output})),cancel:async()=>release()}});t.after(()=>owner.close());
  const pending=owner.read(f.ctx);let cancel;
  if(mode==='cwd')f.ctx.cwd='/other';if(mode==='session')session='new';if(mode==='trust')f.ctx.isProjectTrusted=()=>false;if(mode==='abort')c.abort();if(mode==='cancel')cancel=owner.cancel();else release();
  await assert.rejects(pending,/stale-context|aborted/);await cancel;
 });
 const f=await identityFixture(t),native=createCommandRunner({binary:'git'});let release,started;const entered=new Promise(resolve=>started=resolve);
 const barrier=new Promise(resolve=>release=resolve);let returned=false;
 const owner=createWorktreeIdentity({runner:native,realpathImpl:async path=>{started();await barrier;return realpath(path)}});t.after(()=>owner.close());
 const work=owner.read(f.ctx);await entered;const cancel=owner.cancel().then(()=>returned=true);await new Promise(resolve=>setImmediate(resolve));assert.equal(returned,false);
 release();await assert.rejects(work,/stale-context/);await cancel;assert.equal(returned,true);
});

test('PWV same-path Gitdir replacement during identity resolution changes filesystem pins',async t=>{
 const f=await identityFixture(t),native=createCommandRunner({binary:'git'});let release,started;const entered=new Promise(resolve=>started=resolve),barrier=new Promise(resolve=>release=resolve);
 const owner=createWorktreeIdentity({runner:native,realpathImpl:async path=>{started();await barrier;return realpath(path)}});t.after(()=>owner.close());
 const pending=owner.read(f.ctx);await entered;await rename(join(f.dir,'.git'),join(f.dir,'old-git'));await mkdir(join(f.dir,'.git'));await writeFile(join(f.dir,'.git','HEAD'),'ref: refs/heads/main\n');release();
 await assert.rejects(pending,/stale-context/);
});

test('PWV identity cancel and close join TERM-ignoring owned descendants',async t=>{
 for(const action of ['cancel','close'])await t.test(action,async t=>{
  const f=await identityFixture(t),marker=join(f.dir,'pids'),script=join(f.dir,'identity-runner');let pids=[];
  const child='process.on("SIGTERM",()=>{});process.send("ready");setInterval(()=>{},100)';
  await writeFile(script,'#!'+process.execPath+'\n'+`const {spawn}=require('node:child_process');process.on('SIGTERM',()=>{});const p=spawn(process.execPath,['-e',${JSON.stringify(child)}],{stdio:['ignore',process.stdout,process.stderr,'ipc']});p.on('message',()=>{require('node:fs').writeFileSync(${JSON.stringify(marker)},JSON.stringify([process.pid,p.pid]));p.disconnect()});setInterval(()=>{},100);\n`,{mode:0o755});
  const env={...process.env};delete env.NODE_TEST_CONTEXT;
  const runner=createCommandRunner({binary:script,env,timeoutMs:1700,maxBytes:16384}),owner=createWorktreeIdentity({runner});
  t.after(async()=>{await owner.close();for(const pid of pids)try{process.kill(pid,'SIGKILL')}catch{}});
  const pending=owner.read(f.ctx);for(let n=0;n<100;n++){try{pids=JSON.parse(await readFile(marker,'utf8'));break}catch{await new Promise(resolve=>setTimeout(resolve,10))}}
  assert.equal(pids.length,2);const rejected=assert.rejects(pending,/stale-context|identity-unavailable/);await owner[action]();await rejected;
  for(const pid of pids)assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});
  if(action==='close')await assert.rejects(owner.read(f.ctx),/identity-closed/);
 });
});


test('PWV attached branch transition during filesystem resolution cannot mix native HEAD identity',async t=>{
 const f=await identityFixture(t),native=createCommandRunner({binary:'git'});let release,started;const entered=new Promise(resolve=>started=resolve),barrier=new Promise(resolve=>release=resolve);
 const owner=createWorktreeIdentity({runner:native,realpathImpl:async path=>{started();await barrier;return realpath(path)}});t.after(()=>owner.close());
 const pending=owner.read(f.ctx);await entered;f.git(['checkout','-qb','other']);release();await assert.rejects(pending,/stale-context/);
});
