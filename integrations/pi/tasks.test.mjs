// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,readFile,writeFile} from 'node:fs/promises';
import {digest} from './operations.js';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createTasksService,registerTasksTools,tasksResult} from './tasks.js';
const ticketId='ticket:fixture:main:APP-0001';
function receipt(args,items){return {exitCode:0,stdout:JSON.stringify({profile:'taskman-command-result/0',command:args,items,outcome:'OK',codes:[],warnings:[],unknown:'NOT_OBSERVED'})}}
async function setup(t){const dir=await mkdtemp(join(tmpdir(),'pi-tasks-unit-'));t.after(()=>rm(dir,{recursive:true,force:true}));let branch='main',calls=[],revision='1',lost=false;
 const identity=async()=>({root:dir,gitDir:dir,branch,head:'a'.repeat(40),sessionSha256:'b'.repeat(64)});
 const runner={async run({args}){calls.push(args);if(args[0]==='help')return receipt(args,[{implemented:['ticket prioritize','submit','gate run','complete']}]);if(args.join(' ')==='ticket show '+ticketId)return receipt(args,[{ticketId,revision}]);if(args[1]==='prioritize'){revision='2';if(lost)return {fault:'timeout'};return receipt(args,[{receipt:'native',replayed:false}])}return receipt(args,[])} };
 const service=createTasksService({runner,identity});const ctx={cwd:dir,sessionManager:{getSessionId:()=> 'session'},isProjectTrusted:()=>true,isIdle:()=>true};
 return {service,ctx,calls,setBranch:x=>branch=x,setLost:x=>lost=x};
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
