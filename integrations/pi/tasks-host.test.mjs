// SPDX-License-Identifier: AGPL-3.0-or-later
// Native Tasks fixture qualification; this does not claim Pi host/OS support.
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {mkdtemp,mkdir,readFile,writeFile,rm,readdir,stat} from 'node:fs/promises';
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
