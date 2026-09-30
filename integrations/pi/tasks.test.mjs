// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,readFile,writeFile} from 'node:fs/promises';
import {digest,createOperationLedger} from './operations.js';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createTasksService,registerTasksTools,tasksResult} from './tasks.js';
const ticketId='ticket:fixture:main:APP-0001';
function receipt(args,items){return {exitCode:0,stdout:JSON.stringify({profile:'taskman-command-result/0',command:args,items,outcome:'OK',codes:[],warnings:[],unknown:'NOT_OBSERVED'})}}
async function setup(t){const dir=await mkdtemp(join(tmpdir(),'pi-tasks-unit-'));t.after(()=>rm(dir,{recursive:true,force:true}));let branch='main',session='session',filesystemSha256=digest('repo'),head='a'.repeat(40),calls=[],revision='1',lost=false;
 const identity=async()=>({root:dir,gitDir:dir,branch,head,sessionSha256:digest(session),filesystemSha256});
 const runner={async run({args}){calls.push(args);if(args[0]==='help')return receipt(args,[{implemented:['ticket prioritize','submit','gate run','complete']}]);if(args.join(' ')==='ticket show '+ticketId)return receipt(args,[{ticketId,revision}]);if(args[1]==='prioritize'){revision='2';if(lost)return {fault:'timeout'};return receipt(args,[{receipt:'native',replayed:false}])}return receipt(args,[])} };
 const service=createTasksService({runner,identity});const ctx={cwd:dir,sessionManager:{getSessionId:()=> session},isProjectTrusted:()=>true,isIdle:()=>true};
 return {service,ctx,calls,runner,identity,setSession:x=>session=x,setFilesystem:x=>filesystemSha256=x,setHead:x=>head=x,setBranch:x=>branch=x,setLost:x=>lost=x};
}
const request={requestId:'priority-1',operation:'ticket prioritize',input:{ticketId,expectedRevision:'1',priority:'P1',order:'2'}};
test('PWV reads retain raw unknowns, reject injection and never call mutations',async t=>{
 const {service,ctx,calls}=await setup(t);const r=await service.read('queue',{},ctx);assert.equal(r.raw.unknown,'NOT_OBSERVED');assert.equal(tasksResult(r).isError,false);
 assert.equal((await service.read('init',{},ctx)).fault,'unsupported-operation');assert.equal((await service.read('ticket',{id:'--help'},ctx)).fault,'invalid-input');assert.equal(calls.length,1);
 assert.equal((await service.read('queue',{}, {...ctx,isProjectTrusted:()=>false})).fault,'untrusted-project');
});
test('PWV slash authorizes exact input; model tool has no write route or permission boolean',async t=>{
 const {service,ctx,calls}=await setup(t),tools=[],commands=[];registerTasksTools({registerTool:x=>tools.push(x),registerCommand:(n,x)=>commands.push(x),sendMessage(){}},{service});
 assert.equal(tools.length,1);assert.equal((await tools[0].execute('x',{operation:'complete',input:{authorized:true}},undefined,undefined,ctx)).isError,true);
 assert.equal((await service.command({...request,authorized:true},ctx)).fault,'invalid-input');assert.equal(calls.length,0);
 const r=await service.command(request,ctx);assert.equal(r.ok,true);assert.equal(calls.filter(x=>x[1]==='prioritize').length,1);
 assert.equal((await service.command(request,ctx)).fault,'stale-ticket');
});
test('PWV lost response blocks new IDs; explicit same-ID reconciliation replays frozen timestamp',async t=>{
 const {service,ctx,calls,setLost}=await setup(t);setLost(true);const r=await service.command(request,ctx);assert.equal(r.mutation,'unknown');setLost(false);
 const result=await service.command({...request,resume:true},ctx);assert.equal(result.ok,true);
 const writes=calls.filter(x=>x[1]==='prioritize');assert.deepEqual(writes[0],writes[1]);
});
test('PWV invalidated in-flight reads are discarded and native malformed results become structured errors',async t=>{
 const {ctx}=await setup(t);let finish;const service=createTasksService({identity:async()=>({root:ctx.cwd,gitDir:ctx.cwd,branch:'main',head:'a'.repeat(40),sessionSha256:'b'.repeat(64)}),runner:{run:()=>new Promise(r=>finish=r)}});
 const pending=service.read('queue',{},ctx);while(!finish)await new Promise(r=>setImmediate(r));service.clear();finish(receipt([],[]));assert.equal((await pending).fault,'stale-context');
 const broken=createTasksService({identity:async()=>({}),runner:{run:async()=>({exitCode:0,stdout:'bad'})}});assert.equal(tasksResult(await broken.read('queue',{},ctx)).isError,true);
});
test('PWV concurrent dispatch and canceled commands do not start a second mutation',async t=>{
 const {service,ctx,calls}=await setup(t);const results=await Promise.all([service.command(request,ctx),service.command({...request,requestId:'second'},ctx)]);assert.equal(results.filter(r=>r.ok).length,1);assert.ok(results.some(r=>r.fault==='operation-in-flight'));assert.equal(calls.filter(x=>x[1]==='prioritize').length,1);
 const c=new AbortController();c.abort();const canceled=await service.command({...request,requestId:'cancel'}, {...ctx,signal:c.signal});assert.equal(canceled.ok,false);assert.equal(calls.filter(x=>x[1]==='prioritize').length,1);
});
test('PWV changed-payload recovery cannot reuse a pending native request',async t=>{const {service,ctx,calls,setLost}=await setup(t);setLost(true);await service.command(request,ctx);setLost(false);const result=await service.command({...request,resume:true,input:{...request.input,order:'9'}},ctx);assert.equal(result.fault,'request-id-conflict');assert.equal(calls.filter(x=>x[1]==='prioritize').length,1)});
test('PWV same-ID terminal refusal and error replay never become successful',async t=>{
 for(const outcome of ['REFUSED','ERROR','NOT_RUN'])await t.test(outcome,async t=>{
  const {ctx}=await setup(t);let writes=0;
  const identity=async()=>({root:ctx.cwd,gitDir:ctx.cwd,branch:'main',head:'a'.repeat(40),sessionSha256:'b'.repeat(64)});
  const runner={async run({args}){
   if(args[0]==='help')return receipt(args,[{implemented:['ticket prioritize']}]);
   if(args[0]==='ticket'&&args[1]==='show')return receipt(args,[{ticketId,revision:'1'}]);
   if(args[1]==='prioritize'){writes++;const r=receipt(args,[{receipt:'',outcome:'REFUSED'}]);const raw=JSON.parse(r.stdout);raw.outcome=outcome;return {...r,exitCode:1,stdout:JSON.stringify(raw)}}
   return receipt(args,[]);
  }};
  const first=await createTasksService({runner,identity}).command(request,ctx);assert.equal(first.ok,false);assert.equal(first.fault,'native-refusal');
  const replay=await createTasksService({runner,identity}).command({...request,resume:true},ctx);
  assert.equal(replay.ok,false);assert.equal(replay.fault,'native-refusal');assert.equal(replay.nativeOutcome,outcome);assert.equal(replay.exitCode,1);assert.equal(tasksResult(replay).isError,true);assert.equal(writes,1,'terminal refusal does not redispatch');
  const file=join(ctx.cwd,'corvint-pi-operations',digest(request.requestId)+'.json'),legacy=JSON.parse(await readFile(file,'utf8'));delete legacy.terminal;await writeFile(file,JSON.stringify(legacy));
  const unknown=await createTasksService({runner,identity}).command({...request,resume:true},ctx);assert.equal(unknown.ok,false);assert.equal(unknown.fault,'native-outcome-unknown');assert.equal(writes,1);
 });
});
test('PWV uncertain gate replay is blocked while known terminal gate replay preserves truth',async t=>{
 for(const uncertain of [true,false])await t.test(uncertain?'uncertain':'known-terminal',async t=>{
  const {ctx}=await setup(t);let dispatches=0;const attemptId='attempt:fixture:main:fixture';
  const identity=async()=>({root:ctx.cwd,gitDir:ctx.cwd,branch:'main',head:'a'.repeat(40),sessionSha256:'b'.repeat(64)});
  const runner={async run({args}){
   if(args[0]==='help')return receipt(args,[{implemented:['gate run']}]);
   if(args[0]==='ticket')return receipt(args,[{ticketId,revision:'1',acceptanceRevision:'1'}]);
   if(args[0]==='attempt')return receipt(args,[{attemptId,ticketId,ticketRevision:'1',generation:'1',branch:'main',phase:'BUILT',lease:{holder:'fixture',expiresAt:new Date(Date.now()+60000).toISOString()}}]);
   if(args[0]==='gate'){dispatches++;return uncertain?{fault:'aborted'}:receipt(args,[{receipt:'native-gate'}])}
   return receipt(args,[]);
  }};
  const request={operation:'gate run',requestId:'gate',input:{ticketId,expectedRevision:'1',holder:'fixture',attemptId,generation:'1',gate:'verify'}};
  const first=await createTasksService({runner,identity}).command(request,ctx);assert.equal(first.ok,!uncertain);
  const resumed=await createTasksService({runner,identity}).command({...request,resume:true},ctx);
  assert.equal(dispatches,1);
  if(uncertain){assert.equal(resumed.fault,'gate-replay-unavailable');assert.equal(resumed.mutation,'unknown');assert.equal(resumed.requestId,'gate');assert.ok(resumed.reconciliation.attempt);assert.equal(tasksResult(resumed).isError,true)}else{assert.equal(resumed.ok,true);assert.equal(resumed.mutation,'previously-observed')}
 });
});


test('PWV stable same-session pending replay survives transitions and service restart',async t=>{
 for(const restart of [false,true])await t.test(restart?'restart':'transitions',async t=>{
  const f=await setup(t);f.service.clear();f.setLost(true);assert.equal((await f.service.command(request,f.ctx)).mutation,'unknown');
  f.service.clear();f.service.clear();f.setLost(false);
  const resumed=await (restart?createTasksService({runner:f.runner,identity:f.identity}):f.service).command({...request,resume:true},f.ctx);
  assert.equal(resumed.ok,true,JSON.stringify(resumed));
  const calls=f.calls.filter(args=>args[1]==='prioritize');assert.equal(calls.length,2);assert.deepEqual(calls[0],calls[1]);
 });
});

test('PWV fork session and filesystem rebinding cannot reuse pending metadata',async t=>{
 for(const change of [f=>f.setSession('fork'),f=>f.setFilesystem(digest('replacement'))])await t.test('binding mismatch',async t=>{
  const f=await setup(t);f.setLost(true);await f.service.command(request,f.ctx);f.setLost(false);
  const path=join(f.ctx.cwd,'corvint-pi-operations','pending.json'),before=await readFile(path,'utf8');change(f);
  assert.equal((await f.service.command({...request,resume:true},f.ctx)).fault,'request-id-conflict');
  assert.equal(await readFile(path,'utf8'),before);assert.equal(f.calls.filter(args=>args[1]==='prioritize').length,1);
 });
});

test('PWV legacy ephemeral-epoch pending binding refuses without migration or metadata deletion',async t=>{
 const f=await setup(t),i=await f.identity(),ledger=createOperationLedger(i.gitDir);
 await ledger.begin({requestId:request.requestId,intentSha256:digest({operation:request.operation,input:request.input,head:i.head}),scopeSha256:digest({root:i.root,gitDir:i.gitDir,branch:i.branch,sessionSha256:i.sessionSha256,branchGeneration:1}),issuedAt:'2026-09-30T00:00:00Z'});
 const path=join(i.gitDir,'corvint-pi-operations','pending.json'),before=await readFile(path,'utf8');
 const result=await f.service.command({...request,resume:true},f.ctx);assert.equal(result.fault,'request-id-conflict');
 assert.equal(await readFile(path,'utf8'),before);assert.equal(f.calls.filter(args=>args[1]==='prioritize').length,0);
});

test('PWV stale and failed post-native identity retain raw Tasks read receipt truth',async t=>{
 for(const mode of ['transition','identity-failure'])await t.test(mode,async t=>{
  const f=await setup(t);let reads=0,service;
  service=createTasksService({identity:async()=>{if(++reads===2&&mode==='identity-failure')throw Error('identity unavailable');return f.identity()},runner:{async run({args}){const result={...receipt(args,[{receipt:'original'}]),stderr:'native warning'};if(mode==='transition')service.clear();return result}}});
  const result=await service.read('queue',{},f.ctx);assert.equal(result.ok,false);assert.equal(result.raw.outcome,'OK');assert.equal(result.raw.unknown,'NOT_OBSERVED');assert.equal(result.exitCode,0);assert.deepEqual(JSON.parse(result.nativeResult.stdout),result.raw);assert.equal(result.nativeResult.stderr,'native warning');assert.deepEqual(result.raw.items,[{receipt:'original'}]);assert.equal(result.wrapperFault,mode==='transition'?'stale-context':'read-unavailable');
 });
});

test('PWV idle, pending messages and transition changes after durable begin prevent dispatch',async t=>{
 for(const mode of ['busy','pending','transition'])await t.test(mode,async t=>{
  const f=await setup(t);let service;
  const ledger=gitDir=>{const native=createOperationLedger(gitDir);return {...native,async begin(...args){const entry=await native.begin(...args);if(mode==='busy')f.ctx.isIdle=()=>false;if(mode==='pending')f.ctx.hasPendingMessages=()=>true;if(mode==='transition')service.clear();return entry}}};
  service=createTasksService({runner:f.runner,identity:f.identity,ledger});
  const result=await service.command(request,f.ctx);assert.equal(result.fault,mode==='transition'?'stale-context':'host-not-idle');assert.equal(result.mutation,'pending-reconciliation');assert.equal(f.calls.filter(args=>args[1]==='prioritize').length,0);
  const pending=await createOperationLedger(f.ctx.cwd).inspect();assert.equal(pending.requestId,request.requestId);
 });
});

test('PWV transition during resume preserves the same pending record and blocks redispatch',async t=>{
 const f=await setup(t);f.setLost(true);await f.service.command(request,f.ctx);f.setLost(false);
 const path=join(f.ctx.cwd,'corvint-pi-operations','pending.json'),before=await readFile(path,'utf8');let service;
 const ledger=gitDir=>{const native=createOperationLedger(gitDir);return {...native,async begin(...args){const entry=await native.begin(...args);service.clear();return entry}}};
 service=createTasksService({runner:f.runner,identity:f.identity,ledger});
 const result=await service.command({...request,resume:true},f.ctx);assert.equal(result.fault,'stale-context');assert.equal(result.mutation,'pending-reconciliation');assert.equal(await readFile(path,'utf8'),before);assert.equal(f.calls.filter(args=>args[1]==='prioritize').length,1);
});

test('PWV identity await transition cannot dispatch reads, and detached HEAD cannot mutate',async t=>{
 const f=await setup(t);let service,release;
 service=createTasksService({runner:f.runner,identity:()=>new Promise(resolve=>release=resolve)});
 const pending=service.read('queue',{},f.ctx);service.clear();release(await f.identity());assert.equal((await pending).fault,'stale-context');assert.equal(f.calls.length,0);
 const detached=createTasksService({runner:f.runner,identity:async()=>({...await f.identity(),branch:'HEAD',detached:true})});
 assert.equal((await detached.command(request,f.ctx)).fault,'detached-head');assert.equal(f.calls.length,0);
});
