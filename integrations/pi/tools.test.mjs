// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { registerTools,toolObservations } from './tools.js';
import { validToolEnvelope,createRunner } from './runtime.js';
import { workflowIdentity } from './workflow.js';
import { mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

function fixture() {
 const definitions=new Map(),commands=new Map(),calls=[],notices=[];
 const pi={registerTool(t){definitions.set(t.name,t)},registerCommand(name,command){commands.set(name,command)}};
 const runner={async tool(request){calls.push(request);return {context:'native framed result',packet:{json:'opaque packet',sha256:'a'.repeat(64),commit:'b'.repeat(40),evidenceHandle:'context-packet:sha256:'+'a'.repeat(64)},mutation:'not-attempted'}}};
 const ctx={cwd:'/one',sessionManager:{getSessionId:()=> 'session'},isProjectTrusted:()=>true};
 const identity=async ctx=>({root:ctx.cwd,gitDir:ctx.cwd+'/.git',session:ctx.sessionManager.getSessionId()});
 const identityCalls=[];const identify={read:identity};
 const tools=registerTools(pi,{identity:ctx=>{identityCalls.push(ctx);return identify.read(ctx)},runner,version:'0.99.1',notice(...args){notices.push(args)}});
 return {ctx,runner,calls,tools,notices,identify,identityCalls,command:(name,text)=>commands.get(name).handler(text,ctx),call:(name,args,signal)=>definitions.get(name).execute('test',args,signal,undefined,ctx)};
}

test('AHI-025 opaque packets are bounded to four and expire on root or session transitions',async()=>{
 const f=fixture();const handles=[];
 for(let i=0;i<5;i++)handles.push((await f.call('corvint_context',{task:'source'})).details.handle);
 await assert.rejects(f.call('corvint_expand',{handle:handles[0],result:0,evidence:0,lines:'1:1'}),/stale-context/);
 const selection={handle:handles[4],result:0,evidence:0,lines:'1:1'};
 await f.call('corvint_expand',selection);
 assert.equal(f.calls.at(-1).input.packet,'opaque packet');
 assert.equal(f.calls.at(-1).input.packetSha256,'a'.repeat(64));
 f.ctx.cwd='/two';await assert.rejects(f.call('corvint_expand',selection),/stale-context/);
 f.ctx.cwd='/one';f.tools.clear();await assert.rejects(f.call('corvint_expand',selection),/stale-context/);
});

test('AHI-025 tool cancellation and trust prevent child effects, and late packets cannot cross sessions',async()=>{
 const f=fixture();const controller=new AbortController();controller.abort();
 await assert.rejects(f.call('corvint_context',{task:'secret'},controller.signal),/aborted/);
 assert.equal(f.calls.length,0);
 f.ctx.isProjectTrusted=()=>false;
 await assert.rejects(f.call('corvint_record_outcome',{},undefined),/untrusted-project/);
 assert.equal(f.calls.length,0);
 f.ctx.isProjectTrusted=()=>true;
 let complete;f.runner.tool=()=>new Promise(resolve=>{complete=resolve});
 const pending=f.call('corvint_context',{task:'old'});
 await new Promise(resolve=>setImmediate(resolve));
 f.tools.clear();complete({context:'old',packet:{}});
 await assert.rejects(pending,/stale-context/);
});

test('AHI-025 tool errors are actual Pi execution errors; record uncertainty stays explicit',async()=>{
 const f=fixture();const controller=new AbortController();
 f.runner.tool=async(request)=>{assert.equal(request.signal,controller.signal);return {fault:'record-unavailable',mutation:'unknown'}};
 await assert.rejects(f.call('corvint_record_outcome',{},controller.signal),/may have written/);
});

test('AHI-005 AHI-025 typed supplied observations exclude raw output and malformed verification',()=>{
 const metadata={changedPaths:['main.go'],observedEvidenceHandles:['supplied'],verification:[{commandSha256:'a'.repeat(64),status:'passed'}],secret:'private-body'};
 assert.deepEqual(Object.keys(toolObservations(metadata)),['observedEvidenceHandles','changedPaths','verification']);
 assert.doesNotMatch(JSON.stringify(toolObservations(metadata)),/private-body|secret/);
 assert.throws(()=>toolObservations({verification:[{command:'raw shell',status:'passed'}]}));
 assert.throws(()=>toolObservations({changedPaths:['x'.repeat(4097)]}));
 assert.throws(()=>toolObservations({observedEvidenceHandles:Array(257).fill('x')}));
 assert.deepEqual(toolObservations(undefined),{});
});

test('AHI-025 tool envelopes cannot forge profile, persistence or runtime version',()=>{
 const good={profile:'corvint-pi-tool/0',operation:'record',hostVersion:'0.99.1',adapterVersion:'0.3.2',support:'FALLBACK',ok:true,mutation:'recorded',context:'BEGIN CORVINT REPOSITORY DATA\n{}\nEND CORVINT REPOSITORY DATA',packet:null,fault:null};
 assert.equal(validToolEnvelope(good,'record'),true);
 for(const delta of [{support:'FULL'},{hostVersion:'unknown'},{extra:true},{mutation:'not-attempted'},{packet:{}},{context:'unframed'}])assert.equal(validToolEnvelope({...good,...delta},'record'),false);
});

test('AHI-025 explicit record command retains uncertain-write notice and supplied context evidence',async()=>{
 const f=fixture();
 const result=await f.call('corvint_context',{task:'source'});
 assert.deepEqual(result.details.corvint.observedEvidenceHandles,['context-packet:sha256:'+'a'.repeat(64)]);
 assert.match(result.content[0].text,/Evidence handle: context-packet:sha256:/);
 f.runner.tool=async()=>({fault:'deadline',mutation:'unknown'});
 await f.command('corvint-record','{}');
 assert.match(f.notices[0][2],/may have been written.*inspect/);
});

test('PWV-V0-004 same path and session cannot reuse a packet after canonical Gitdir rebinding',async()=>{
 const f=fixture();let gitDir='/git/one';
 f.identify.read=async ctx=>({root:ctx.cwd,gitDir,session:'session'});
 const packet=await f.call('corvint_context',{task:'source'});
 gitDir='/git/two';
 await assert.rejects(f.call('corvint_expand',{handle:packet.details.handle,result:0,evidence:0,lines:'1:1'}),/stale-context/);
 assert.equal(f.calls.length,1);
});

test('PWV-V0-004 trust and cancellation precede identity reads and native dispatch',async()=>{
 const f=fixture();f.ctx.isProjectTrusted=()=>false;
 await assert.rejects(f.call('corvint_context',{task:'source'}),/untrusted-project/);
 f.ctx.isProjectTrusted=()=>true;const c=new AbortController();c.abort();
 await assert.rejects(f.call('corvint_context',{task:'source'},c.signal),/aborted/);
 assert.equal(f.identityCalls.length,0);assert.equal(f.calls.length,0);
});

test('PWV-V0-004 deferred identity cannot follow a changed cwd/session or dispatch after abort',async()=>{
 for(const change of [f=>{f.ctx.cwd='/two'},f=>{f.ctx.sessionManager.getSessionId=()=> 'new'},f=>f.tools.clear(),f=>{f.ctx.isProjectTrusted=()=>false},(_f,c)=>c.abort()]){
  const f=fixture(),c=new AbortController();let release;
  f.identify.read=ctx=>new Promise(resolve=>{release=()=>resolve({root:ctx.cwd,gitDir:ctx.cwd+'/.git',session:ctx.sessionManager.getSessionId()})});
  const pending=f.call('corvint_context',{task:'source'},c.signal);
  change(f,c);release();await assert.rejects(pending,/stale-context/);
  assert.equal(f.calls.length,0);
 }
});

test('PWV-V0-004 identity failure refuses before dispatch and retains uncertainty after native write',async()=>{
 const f=fixture();f.identify.read=async()=>{throw Error('unavailable')};
 await assert.rejects(f.call('corvint_record_outcome',{}),/identity-unavailable/);
 assert.equal(f.calls.length,0);
 let reads=0;f.identify.read=async ctx=>{if(++reads===2)throw Error('unavailable');return {root:ctx.cwd,gitDir:'/git/one',session:'session'}};
 f.runner.tool=async()=>({mutation:'recorded',context:'native record'});
 await assert.rejects(f.call('corvint_record_outcome',{}),/identity-unavailable.*may have written/);
});

test('PWV-V0-004 post-dispatch canonical identity change discards the context packet',async()=>{
 const f=fixture();let reads=0;
 f.identify.read=async ctx=>({root:ctx.cwd,gitDir:++reads===1?'/git/one':'/git/two',session:'session'});
 await assert.rejects(f.call('corvint_context',{task:'source'}),/stale-context/);
 assert.equal(f.calls.length,1);
 await assert.rejects(f.call('corvint_expand',{handle:'packet-1',result:0,evidence:0,lines:'1:1'}),/stale-context/);
});


test('PWV-V0-004 bounded filesystem identity detects same-root Gitdir pointer replacement',async t=>{
 const root=mkdtempSync(join(tmpdir(),'pi-cache-binding-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 for(const dir of ['git-one','git-two']){mkdirSync(join(root,dir));writeFileSync(join(root,dir,'HEAD'),'ref: refs/heads/main\n')}
 writeFileSync(join(root,'.git'),'gitdir: git-one\n');
 const f=fixture();f.ctx.cwd=root;f.identify.read=workflowIdentity;
 const packet=await f.call('corvint_context',{task:'source'});
 writeFileSync(join(root,'.git'),'gitdir: git-two\n');
 await assert.rejects(f.call('corvint_expand',{handle:packet.details.handle,result:0,evidence:0,lines:'1:1'}),/stale-context/);
 assert.equal(f.calls.length,1);
});

test('PWV-V0-004 native recorded trace survives post-dispatch identity failure with explicit uncertainty',{skip:!process.env.CORVINT_PI_HOST_BIN},async t=>{
 const root=mkdtempSync(join(tmpdir(),'pi-cache-record-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 writeFileSync(join(root,'.gitignore'),'.context-corvint/\n.corvint/\n');writeFileSync(join(root,'main.go'),'package fixture\nfunc One() int { return 1 }\n');
 const git=args=>{const r=spawnSync('git',args,{cwd:root,encoding:'utf8'});assert.equal(r.status,0,r.stderr);return r.stdout.trim()};
 git(['init','-q']);git(['add','.']);git(['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','base']);
 writeFileSync(join(root,'main.go'),'package fixture\nfunc One() int { return 2 }\n');git(['add','.']);git(['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','change']);
 const revision=git(['rev-parse','HEAD']),f=fixture(),native=createRunner({binary:process.env.CORVINT_PI_HOST_BIN});t.after(()=>native.close());
 f.ctx.cwd=root;let reads=0,nativeResult;
 f.identify.read=ctx=>{if(++reads===2)throw Error('identity unavailable after write');return workflowIdentity(ctx)};
 f.runner.tool=async request=>{nativeResult=await native.tool(request);return nativeResult};
 await assert.rejects(f.call('corvint_record_outcome',{task:'explicit cache identity record fixture',changedPaths:['main.go'],verification:['git diff --check'],outcome:'passed'}),/identity-unavailable.*may have written/);
 assert.equal(nativeResult.mutation,'recorded');
 const rows=readFileSync(join(root,'.context-corvint','traces',revision+'.jsonl'),'utf8').trim().split('\n').map(JSON.parse);
 assert.equal(rows.length,1);assert.equal(rows[0].task,'explicit cache identity record fixture');assert.equal(rows[0].revision,revision);assert.equal(rows[0].outcome,'passed');
});

test('PWV-V0-004 transition queued after invocation cannot publish a context packet',async()=>{
 const f=fixture();let reads=0;
 f.identify.read=ctx=>{
  if(++reads===2)queueMicrotask(()=>queueMicrotask(()=>queueMicrotask(()=>f.tools.clear())));
  return {root:ctx.cwd,gitDir:'/git/one',session:'session'};
 };
 await assert.rejects(f.call('corvint_context',{task:'source'}),/stale-context/);
 assert.equal(reads,2);assert.equal(f.calls.length,1);
 await assert.rejects(f.call('corvint_expand',{handle:'packet-1',result:0,evidence:0,lines:'1:1'}),/stale-context/);
});
