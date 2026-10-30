// SPDX-License-Identifier: Apache-2.0
// Native Tasks fixture qualification; this does not claim Pi host/OS support.
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {mkdtemp,mkdir,readFile,writeFile,rm,readdir,stat,rename} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {createTasksService} from './tasks.js';
import {canonical,digest} from './operations.js';
const exec=promisify(execFile), templates=resolve('internal/tasks/cli/testdata/external-agents');
test('PWV native fixture CAS, lost receipt replay, actors, lease, gates and audit',async t=>{
 const cwd=await mkdtemp(join(tmpdir(),'pi-tasks-native-'));t.after(()=>rm(cwd,{recursive:true,force:true}));
 const env={PATH:process.env.PATH,HOME:process.env.HOME,CORVINT_TASKS_ACTOR:'pi-fixture'};
 const git=async args=>(await exec('git',args,{cwd,env,timeout:10000,maxBuffer:65536})).stdout.trim();
 await git(['init','-q','-b','main']);await git(['config','user.name','Fixture']);await git(['config','user.email','fixture@example.invalid']);
 await mkdir(join(cwd,'.taskman'));for(const name of ['queue.json','policy.json']){const v=JSON.parse(await readFile(join(templates,name),'utf8'));if(name==='queue.json')v.fixture=true;await writeFile(join(cwd,'.taskman',name),canonical(v)+'\n')}
 await writeFile(join(cwd,'Makefile'),'verify:\n\t@true\n');await git(['add','.']);await git(['commit','-qm','fixture']);
 const binary=process.env.CORVINT_TASKS_BIN??'corvint-tasks';
 const {createCommandRunner}=await import(process.env.CORVINT_PI_PROCESS_MODULE??'./process.js');
 const nativeRunner=createCommandRunner({binary,env,timeoutMs:15000});t.after(()=>nativeRunner.close());
 const run=call=>nativeRunner.run({cwd,...call});
 const native=async args=>{const r=await run({args});assert.equal(r.exitCode,0,r.stdout);return JSON.parse(r.stdout)};
 await native(['init','--request-id','fixture-init']);
 const payload=await readFile(join(templates,'ticket-create.json'),'utf8');const created=await run({args:['ticket','create','--request-id','fixture-create','--payload-stdin'],input:payload});assert.equal(created.exitCode,0,created.stdout);
 const ticketId=JSON.parse(created.stdout).items[0].ticketId;await git(['add','.taskman']);await git(['commit','-qm','fixture ticket']);
 let lose=false,race=false;const runner={run:async call=>{if(race&&call.args[1]==='prioritize'){race=false;await native(['ticket','prioritize','--target',ticketId,'--expected-revision','2','--request-id','other-actor','--payload','{"order":"3","priority":"P2"}']);}const r=await run(call);if(lose&&call.args[1]==='prioritize')return {fault:'timeout'};return r}};
 const ctx={cwd,isProjectTrusted:()=>true,isIdle:()=>true,sessionManager:{getSessionId:()=> 'fixture-session'}};
 let service=createTasksService({runner});
 async function snapshot(dir){const out={};for(const name of (await readdir(dir)).sort()){const path=join(dir,name),st=await stat(path);out[name]=st.isDirectory()?await snapshot(path):[st.mtimeMs,digest(await readFile(path,'utf8'))]}return out}
 const before=await snapshot(cwd);for(const op of ['help','audit','queue','gates','releases','plan'])assert.equal((await service.read(op,{},ctx)).ok,true);assert.deepEqual(await snapshot(cwd),before,'read routes preserve all fixture files');
 const request={operation:'ticket prioritize',requestId:'priority',input:{ticketId,expectedRevision:'1',priority:'P1',order:'2'}};
 lose=true;assert.equal((await service.command(request,ctx)).mutation,'unknown');lose=false;
 service=createTasksService({runner});const recovered=await service.command({...request,resume:true},ctx);assert.equal(recovered.ok,true,JSON.stringify(recovered));assert.equal(recovered.raw.items[0].replayed,true);
 assert.equal((await service.command({...request,requestId:'stale'},ctx)).fault,'stale-ticket');
 const read=await service.read('ticket',{id:ticketId},ctx);assert.equal(read.raw.items[0].revision,'2');
 race=true;const conflict=await service.command({...request,requestId:'race',input:{...request.input,expectedRevision:'2'}},ctx);assert.equal(conflict.ok,false);assert.equal(conflict.raw.items[0].outcome,'REVISION_CONFLICT',JSON.stringify(conflict));
 await git(['add','.taskman']);await git(['commit','-qm','fixture priority']);
 const claim=await service.command({operation:'claim',requestId:'claim',input:{ticketId,expectedRevision:'3',holder:'fixture-owner'}},ctx);assert.equal(claim.ok,true,JSON.stringify(claim));
 const attemptId=claim.raw.items[0].attemptId,generation=claim.raw.items[0].generation;
 const common={ticketId,expectedRevision:'3',holder:'fixture-owner',attemptId,generation};
 const wrong=await service.command({operation:'renew',requestId:'wrong-owner',input:{...common,holder:'another-owner'}},ctx);assert.equal(wrong.fault,'attempt-owner-mismatch');
 const tree=await git(['rev-parse','HEAD^{tree}']);const submit=await service.command({operation:'submit',requestId:'submit',input:{...common,tree}},ctx);assert.equal(submit.ok,true,JSON.stringify(submit));
 const a=await service.read('attempt',{id:attemptId},ctx);common.generation=a.raw.items[0].generation;
 const gate=await service.command({operation:'gate run',requestId:'gate',input:{...common,gate:'verify'}},ctx);assert.equal(gate.ok,true,JSON.stringify(gate));
 const latest=await service.read('attempt',{id:attemptId},ctx);common.generation=latest.raw.items[0].generation;
 const complete=await service.command({operation:'complete',requestId:'complete',input:{...common,commit:await git(['rev-parse','HEAD'])}},ctx);assert.equal(complete.ok,true,JSON.stringify(complete));
 assert.equal((await service.read('audit',{},ctx)).ok,true);
});

async function interruptionFixture(t) {
 const scratch=await mkdtemp(join(tmpdir(),'pi-tasks-interrupt-')),cwd=join(scratch,'repo');await mkdir(cwd);
 const env={PATH:process.env.PATH,HOME:process.env.HOME,CORVINT_TASKS_ACTOR:'pi-interrupt-fixture'};
 const {createCommandRunner}=await import(process.env.CORVINT_PI_PROCESS_MODULE??'./process.js');
 const runner=createCommandRunner({binary:process.env.CORVINT_TASKS_BIN??'corvint-tasks',env,timeoutMs:120000});
 const marker=join(scratch,'gate-pids'),hold=join(scratch,'hold'),runs=join(scratch,'runs');
 // Only this disposable, explicit fixture gate has external effects, all under scratch.
 const gate=`echo $$ > '${marker}'; echo run >> '${runs}'; if test -f '${hold}'; then sleep 180 & echo $! >> '${marker}'; wait; fi`;
 t.after(async()=>{await runner.close();try{for(const pid of (await readFile(marker,'utf8')).trim().split('\n').map(Number)){if(pid>1)try{process.kill(pid,'SIGKILL')}catch{}}}catch{}await rm(scratch,{recursive:true,force:true})});
 const git=async args=>(await exec('git',args,{cwd,env,timeout:10000,maxBuffer:65536})).stdout.trim();
 const run=call=>runner.run({cwd,...call});
 const native=async args=>{const r=await run({args});assert.equal(r.exitCode,0,r.stdout);return JSON.parse(r.stdout)};
 await git(['init','-q','-b','main']);await git(['config','user.name','Fixture']);await git(['config','user.email','fixture@example.invalid']);
 await mkdir(join(cwd,'.taskman'));
 for(const name of ['queue.json','policy.json']){const v=JSON.parse(await readFile(join(templates,name),'utf8'));if(name==='queue.json')v.fixture=true;else v.gates[0].argv=['/bin/sh','gate.sh'];await writeFile(join(cwd,'.taskman',name),canonical(v)+'\n')}
 await writeFile(join(cwd,'gate.sh'),gate+'\n');
 await git(['add','.']);await git(['commit','-qm','fixture']);await native(['init','--request-id','init']);
 const created=await run({args:['ticket','create','--request-id','create','--payload-stdin'],input:await readFile(join(templates,'ticket-create.json'),'utf8')});assert.equal(created.exitCode,0,created.stdout);
 const ticketId=JSON.parse(created.stdout).items[0].ticketId;await git(['add','.taskman']);await git(['commit','-qm','ticket']);
 const ctx={cwd,isProjectTrusted:()=>true,isIdle:()=>true,sessionManager:{getSessionId:()=> 'interrupt-session'}};
 return {cwd,runner,run,native,git,ticketId,ctx,marker,hold,runs};
}
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));

test('PWV native claim binds current acceptance revision, not adapter requested-revision CAS',async t=>{
 const f=await interruptionFixture(t);let raced=false;
 const runner={run:async call=>{if(call.args[0]==='claim'&&!raced){raced=true;await f.native(['ticket','refine','--target',f.ticketId,'--expected-revision','1','--request-id','competing-refine','--payload','{"acceptanceCriteria":["New acceptance criterion from competing operator."]}'])}return f.run(call)}};
 const service=createTasksService({runner});
 const claim=await service.command({operation:'claim',requestId:'claim-race',input:{ticketId:f.ticketId,expectedRevision:'1',holder:'fixture-owner'}},f.ctx);assert.equal(claim.ok,true,JSON.stringify(claim));
 const attempt=(await f.native(['attempt','show',claim.raw.items[0].attemptId])).items[0];
 const current=(await f.native(['ticket','show',f.ticketId])).items[0];
 assert.equal(current.revision,'2');assert.equal(current.acceptanceRevision,'2');assert.equal(attempt.ticketRevision,current.acceptanceRevision);
 const unsupported=await f.run({args:['claim',f.ticketId,'--holder','fixture-owner','--request-id','unsupported-cas','--expected-revision','1']});
 assert.notEqual(unsupported.exitCode,0);assert.match(unsupported.stdout,/unknown flag --expected-revision/);
 t.diagnostic('ATOMIC_REQUESTED_REVISION_CAS=UNSUPPORTED: native admission binds current acceptanceRevision; adapter expectedRevision is preflight only.');
});

test('PWV-V0-013 native search and list on an initialized store return tickets, page and warnings without mutation',async t=>{
 const f=await interruptionFixture(t);
 const payload=JSON.parse(await readFile(join(templates,'ticket-create.json'),'utf8'));
 Object.assign(payload,{title:'Pi search fixture',kind:'BUG',priority:'P1',labels:['needs review'],milestone:'v1 0'});
 const created=await f.run({args:['ticket','create','--request-id','create-search','--payload-stdin'],input:canonical(payload)+'\n'});assert.equal(created.exitCode,0,created.stdout);
 const labelled=JSON.parse(created.stdout).items[0].ticketId;await f.git(['add','.taskman']);await f.git(['commit','-qm','search ticket']);
 async function tree(dir){const out={};for(const name of (await readdir(dir)).sort()){const path=join(dir,name),st=await stat(path);out[name]=st.isDirectory()?await tree(path):[st.mtimeMs,digest(await readFile(path,'utf8'))]}return out}
 const service=createTasksService({runner:f.runner});t.after(()=>service.close());
 const before=await tree(f.cwd);
 const ok=result=>{assert.equal(result.ok,true,JSON.stringify(result).slice(0,2000));assert.equal(result.raw.outcome,'OK');assert.ok(Array.isArray(result.raw.warnings),'native warnings are retained');assert.ok(result.raw.page&&typeof result.raw.page==='object','native page is retained');return result.raw};
 const found=ok(await service.read('search',{label:'needs review',kind:'BUG',milestone:'v1 0',limit:5},f.ctx));
 assert.deepEqual(found.items.map(i=>i.ticketId),[labelled],'the label with a space matches natively');
 assert.equal(found.page.total,'1');
 const page=ok(await service.read('list',{limit:1,offset:0},f.ctx));
 assert.equal(page.items.length,1);assert.equal(page.page.total,'2');assert.equal(page.page.truncated,true,'the native page bound is reported');
 const all=ok(await service.read('list',{limit:100},f.ctx));
 assert.deepEqual(all.items.map(i=>i.ticketId).sort(),[f.ticketId,labelled].sort());
 const none=ok(await service.read('search',{label:'absent label',summary:true},f.ctx));
 assert.deepEqual(none.items,[]);
 assert.deepEqual(await tree(f.cwd),before,'search and list leave the store, worktree and Git directory unchanged');
 t.diagnostic(`search page=${JSON.stringify(found.page)} list page=${JSON.stringify(page.page)} warnings=${JSON.stringify(found.warnings)}`);
});

test('PWV actual native gate interruption retains uncertainty; native same-ID gate replay reruns execution',async t=>{
 const f=await interruptionFixture(t);
 const claim=await f.native(['claim',f.ticketId,'--holder','fixture-owner','--request-id','claim','--branch','main']);
 const attemptId=claim.items[0].attemptId,generation=claim.items[0].generation;
 await f.native(['submit','--attempt',attemptId,'--generation',generation,'--request-id','submit','--tree',await f.git(['rev-parse','HEAD^{tree}'])]);
 const input={ticketId:f.ticketId,expectedRevision:'1',holder:'fixture-owner',attemptId,generation,gate:'verify'},request={operation:'gate run',requestId:'interrupted-gate',input};
 let dispatches=0;const service=createTasksService({runner:{run:call=>{if(call.args[0]==='gate')dispatches++;return f.run(call)}}});
 await writeFile(f.hold,'hold');const abort=new AbortController(),pending=service.command(request,{...f.ctx,signal:abort.signal});
 let pids;const deadline=Date.now()+15000;
 while(Date.now()<deadline){try{pids=(await readFile(f.marker,'utf8')).trim().split('\n').map(Number);if(pids.length===2)break}catch{}await pause(25)}
 assert.equal(pids?.length,2,'actual native gate and descendant must start before interruption');abort.abort();
 const result=await pending;assert.equal(result.ok,false);assert.equal(result.mutation,'unknown');assert.equal(result.requestId,request.requestId);
 for(const pid of pids){let alive=true;for(let n=0;n<80;n++){try{process.kill(pid,0)}catch{alive=false;break}await pause(25)}assert.equal(alive,false,`owned gate process ${pid} survived abort`)}
 const {createOperationLedger}=await import('./operations.js');const ledger=createOperationLedger(join(f.cwd,'.git'));assert.equal((await ledger.inspect()).requestId,request.requestId);
 const blocked=await service.command({...request,requestId:'replacement-gate'},f.ctx);assert.equal(blocked.fault,'pending-operation');assert.equal(dispatches,1,'new ID cannot retry an uncertain native operation');
 const resumed=await service.command({...request,resume:true},f.ctx);assert.equal(resumed.fault,'gate-replay-unavailable');assert.equal(resumed.mutation,'unknown');assert.equal(resumed.requestId,request.requestId);assert.ok(resumed.reconciliation.attempt);assert.equal(dispatches,1,'same-ID uncertain gate resume never reexecutes');
 assert.equal((await service.read('audit',{},f.ctx)).ok,true);
 // Do not resume the wrapper operation: current native GateRun executes before
 // its replay lookup. Prove that limitation using a harmless separate request.
 await rm(f.hold);const args=['gate','run','--attempt',attemptId,'--generation',generation,'--request-id','native-replay-probe','--gate','verify','--worktree',f.cwd];
 await f.native(args);const repeated=await f.native(args);assert.equal(repeated.items[0].replayed,true);
 assert.equal((await readFile(f.runs,'utf8')).trim().split('\n').length,3,'interrupted execution plus both same-ID native gate executions');
 assert.equal((await ledger.inspect()).requestId,request.requestId,'uncertain wrapper intent remains visibly blocked');
 t.diagnostic('GATE_EXECUTION_REPLAY=UNSUPPORTED: native same-ID receipt replay reexecutes gate; uncertain wrapper gate stays pending, not completed.');
});

test('PWV real minimum-duration native lease expiry refuses stale commands', {skip:process.env.CORVINT_PI_REAL_LEASE_EXPIRY!=='1',timeout:360000},async t=>{
 const f=await interruptionFixture(t);
 const claim=await f.native(['claim',f.ticketId,'--holder','fixture-owner','--request-id','short-claim','--branch','main','--lease-minutes','5']);
 const attemptId=claim.items[0].attemptId,generation=claim.items[0].generation;
 const attempt=(await f.native(['attempt','show',attemptId])).items[0],expiry=Date.parse(attempt.lease.expiresAt);
 assert.ok(expiry>Date.now());t.diagnostic(`Waiting for actual native expiry ${attempt.lease.expiresAt}; no clock or store modification.`);
 while(Date.now()<=expiry)await pause(Math.min(5000,expiry-Date.now()+25));
 const service=createTasksService({runner:f.runner});
 const stale=await service.command({operation:'renew',requestId:'stale-adapter',input:{ticketId:f.ticketId,expectedRevision:'1',holder:'fixture-owner',attemptId,generation}},f.ctx);
 assert.equal(stale.fault,'stale-attempt');assert.equal(stale.mutation,'not-attempted');
 const native=await f.run({args:['renew','--attempt',attemptId,'--generation',generation,'--request-id','stale-native']});assert.notEqual(native.exitCode,0);
 const raw=JSON.parse(native.stdout);assert.ok(raw.codes.includes('FENCED'),native.stdout);assert.match(raw.warnings.join('\n'),/lease expired/);
 assert.equal((await f.native(['attempt','show',attemptId])).items[0].lease.expiresAt,attempt.lease.expiresAt,'no automatic lease renewal');
 await f.native(['receipt','audit']);
});

// PWV-V0-019 (V1-2573): a native gate run SIGKILLed after its receipt commit and before its head
// apply leaves REDO_PENDING. The real native run completes here and the test then restores the
// head.json it replaced: that is the store a SIGKILL leaves just before the head rename, the last
// §5.2 effect, with the receipt and post files written. The wrapper must report pending-operation.
test('PWV-V0-019 native gate run killed after its receipt commit reconciles as pending-operation',{timeout:240000},async t=>{
 const f=await interruptionFixture(t);
 const claim=await f.native(['claim',f.ticketId,'--holder','fixture-owner','--request-id','claim','--branch','main']);
 const attemptId=claim.items[0].attemptId,generation=claim.items[0].generation;
 await f.native(['submit','--attempt',attemptId,'--generation',generation,'--request-id','submit','--tree',await f.git(['rev-parse','HEAD^{tree}'])]);
 const head=join(f.cwd,'.git','taskman','head.json');
 let dispatches=0;const runner={run:async call=>{
  if(call.args[0]!=='gate')return f.run(call);
  dispatches++;const prior=await readFile(head);const r=await f.run(call);assert.equal(r.exitCode,0,r.stdout);
  await writeFile(head+'.crash',prior);await rename(head+'.crash',head);
  return {exitCode:null,stdout:'',stderr:'',fault:'aborted'};
 }};
 const service=createTasksService({runner});t.after(()=>service.close());
 const input={ticketId:f.ticketId,expectedRevision:'1',holder:'fixture-owner',attemptId,generation,gate:'verify'},request={operation:'gate run',requestId:'committed-gate',input};
 const first=await service.command(request,f.ctx);assert.equal(first.ok,false);assert.equal(first.mutation,'unknown',JSON.stringify(first));
 const audit=await f.run({args:['receipt','audit']});assert.notEqual(audit.exitCode,0);assert.ok(JSON.parse(audit.stdout).codes.includes('REDO_PENDING'),'the restored head leaves the designed crash state: '+audit.stdout);
 for(const next of [{...request,requestId:'replacement-gate'},{...request,resume:true}]){
  const r=await service.command(next,f.ctx);
  assert.equal(r.fault,'pending-operation',JSON.stringify(r).slice(0,2000));assert.equal(r.mutation,'not-attempted');
  assert.ok(r.reconciliation.audit.raw.codes.includes('REDO_PENDING'),'the native receipt is retained');
 }
 assert.equal(dispatches,1,'no writer is dispatched while the receipt is pending');
 // Reads never redo (TM-V0-008); the next native writer settles the receipt, then reads succeed.
 const redo=await f.run({args:['renew','--attempt',attemptId,'--generation',generation,'--request-id','redo-writer']});t.diagnostic(`redo writer exit=${redo.exitCode} outcome=${JSON.parse(redo.stdout).outcome}`);
 assert.equal((await service.read('audit',{},f.ctx)).ok,true);
 const ticket=await service.read('ticket',{id:f.ticketId},f.ctx);assert.equal(ticket.ok,true);
});
