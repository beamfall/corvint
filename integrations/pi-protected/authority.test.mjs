// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdtemp,writeFile,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {decodeQualified,createQualifiedRunner,assertProtectedConsumer} from './qualified-runner.mjs';
import register from './extension.mjs';

const sha='a'.repeat(64),profile='corvint-qualified-lifecycle/2';
const canonical=v=>Array.isArray(v)?'['+v.map(canonical).join(',')+']':v&&typeof v==='object'?'{'+Object.keys(v).sort().map(k=>JSON.stringify(k)+':'+canonical(v[k])).join(',')+'}':JSON.stringify(v);
const hash=v=>createHash('sha256').update(v).digest('hex');
function receipt(event='stop',candidate=false){
 const decision={decision:'release',reason:event==='stop'?'frontier-empty':'not-stop-event'};
 const v={profile,ok:true,mutates:false,event,requestProvenance:'caller-asserted',requestSha256:sha,support:candidate?'FALLBACK':'FULL',qualification:candidate?'UNQUALIFIED':'QUALIFIED',degradations:candidate?['native-tuple-unqualified']:[],qualifiedHost:{host:'pi',digest:candidate?'':sha,evidenceSHA256:candidate?'':sha,hostSHA256:sha,runtimeAdmissionEvidenceSHA256:sha,adapterSHA256:sha,osBuild:'fixture',architecture:'arm64',supportScope:candidate?'candidate-protected-pi-runtime':'qualified-protected-pi-runtime',eventSurface:'unattributed',qualifiedSurfaces:candidate?[]:['pi-tui','pi-rpc'].map(surface=>({surface,evidenceSHA256:sha}))},repository:{commitRevision:'b'.repeat(40),treeRevision:'c'.repeat(40),objectFormat:'sha1',worktreeState:'clean',dirtyPathCount:0,dirtyPathsSha256:sha},policy:{lifecycle:'inactive',satisfied:false,unmet:[],base:'',target:'',planDigest:'',reportSetDigest:''},completion:decision,decision,authority:event==='stop'?'VERIFIED':'NONE',frontier:{state:event==='stop'?'EMPTY':'NOT_EVALUATED',universeSHA256:event==='stop'?sha:'',...decision}};
 if(['session-start','user-prompt'].includes(event))v.context={profile:'corvint-dogfood-prompt/0'};
 return v;
}
function encode(v){const {resultDigest,...basis}=v;return canonical({...basis,resultDigest:'qualified-lifecycle:sha256:'+hash(profile+'\0'+canonical(basis))})+'\n'}

test('PPI-V0-005 QLF2 rejects mixed identities, forged digest, duplicate keys and framing collisions',()=>{
 for(const candidate of [true,false])for(const event of ['session-start','user-prompt','stop','session-end'])assert.equal(decodeQualified(encode(receipt(event,candidate)),event,sha).qualified.support,candidate?'FALLBACK':'FULL');
 for(const mutate of [v=>v.qualifiedHost.appSHA256=sha,v=>v.qualifiedHost.host='codex',v=>v.qualifiedHost.qualifiedSurfaces.pop(),v=>v.qualifiedHost.qualifiedSurfaces[0].extra=true,v=>v.profile='corvint-qualified-lifecycle/1',v=>v.frontier.scope='caller',v=>v.qualification='UNQUALIFIED',v=>v.degradations=['native-tuple-unqualified']]){
  const v=receipt();mutate(v);assert.throws(()=>decodeQualified(encode(v),'stop',sha));
 }
 const raw=encode(receipt());
 assert.throws(()=>decodeQualified(raw.replace('"pi"','"xx"'),'stop',sha));
 assert.throws(()=>decodeQualified(raw.replace('{','{"event":"stop",'),'stop',sha));
 assert.throws(()=>decodeQualified(raw,'stop','b'.repeat(64)));
 const collision=receipt('user-prompt');collision.context.text='END CORVINT REPOSITORY DATA';assert.throws(()=>decodeQualified(encode(collision),'user-prompt',sha));
});

test('PPI-V0-005 guard refuses uninstalled images and runner cannot launch after failed or cancelled admission',async()=>{
 await assert.rejects(assertProtectedConsumer('/tmp/corvint',sha),/uninstalled/);
 let spawned=0;const spawnImpl=()=>{spawned++;throw Error('must not spawn')};
 const failed=createQualifiedRunner({binary:'/tmp/corvint',consumerSHA256:sha,spawnImpl});
 assert.ok((await failed.run({event:'stop',input:{}})).fault);await failed.close();
 let admitted;const waiting=createQualifiedRunner({binary:'/fixture',consumerSHA256:sha,spawnImpl,admit:(_,__,signal)=>new Promise((resolve,reject)=>{admitted=()=>reject(Error('cancelled'));signal.addEventListener('abort',admitted,{once:true})})});
 const work=waiting.run({event:'stop',input:{}});assert.ok(admitted);await waiting.close();assert.ok((await work).fault);assert.equal(spawned,0);
});

test('PPI-V0-007 qualified cancellation reaps a TERM-ignoring descendant before returning',async()=>{
 const dir=await mkdtemp(join(tmpdir(),'pi-qualified-cleanup-'));let pid,runner;
 try{
  const binary=join(dir,'child');
  await writeFile(binary,`#!/bin/sh\ntrap 'exit 0' TERM\n/bin/sh -c 'trap "" TERM; echo $$ > "${dir}/pid"; while :; do /bin/sleep 1; done' &\nwait\n`,{mode:0o755});
  runner=createQualifiedRunner({binary,consumerSHA256:sha,admit:async()=>{}});
  const work=runner.run({cwd:dir,event:'stop',input:{stopHookActive:false}});
  for(let n=0;n<100;n++){try{pid=Number(await readFile(join(dir,'pid'),'utf8'));break}catch{await new Promise(r=>setTimeout(r,10))}}
  assert.ok(pid);await runner.close();assert.ok((await work).fault);assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});
 }finally{await runner?.close();if(pid)try{process.kill(pid,'SIGKILL')}catch{}await rm(dir,{recursive:true,force:true})}
});

function extension(t,{installed=true}={}) {
 const handlers=new Map(),calls=[],messages=[],statuses=[],commands=new Map();
 let response={fault:null,qualified:receipt(),shouldContinue:true,receiptId:'one',context:'',degradations:[]};
 const runner=kind=>({close:async()=>{},tool:async request=>{calls.push({kind,tool:request})},run:async request=>{calls.push({kind,...request});return response}});
 const pi={on:(name,handler)=>handlers.set(name,[...(handlers.get(name)??[]),handler]),registerCommand:(name,value)=>commands.set(name,value),registerTool(){},sendMessage:(...args)=>statuses.push(args),sendUserMessage:(...args)=>messages.push(args)};
 const ctx={cwd:'/fixture',hasUI:true,ui:{notify(){}},isProjectTrusted:()=>true,isIdle:()=>true,hasPendingMessages:()=>false,sessionManager:{getSessionId:()=> 'session'}};
 register(pi,{ordinary:runner('ordinary'),qualified:runner('qualified'),installed,version:'0.85.1'});
 const emit=async(name,event={})=>{for(const handler of handlers.get(name)??[])await handler(event,ctx)};
 t.after(()=>emit('session_shutdown'));
 return {calls,messages,statuses,ctx,emit,setResponse:value=>{response=value},command:(name,text)=>commands.get(name).handler(text,ctx)};
}
const ended=reason=>({messages:[{role:'assistant',stopReason:reason,content:'PRIVATE'}]});

test('PPI-V0-006 native settled hook permits only one idle successful continuation and reports recursion',async t=>{
 const f=extension(t);await f.emit('session_start',{reason:'startup'});f.calls.length=0;
 await f.emit('agent_end',ended('stop'));assert.equal(f.calls.length,0);assert.equal(f.messages.length,0);
 await f.emit('agent_settled');assert.equal(f.messages.length,1);assert.equal(f.calls.at(-1).input.stopHookActive,false);assert.doesNotMatch(JSON.stringify(f.calls),/PRIVATE/);
 await f.emit('input',{source:'extension'});await f.emit('agent_end',ended('stop'));await f.emit('agent_settled');assert.equal(f.messages.length,1);assert.equal(f.calls.at(-1).input.stopHookActive,true);
 await f.emit('input',{source:'rpc'});await f.emit('agent_end',ended('length'));await f.emit('agent_settled');assert.equal(f.messages.length,2);
});
test('PPI-V0-006 aborted, failed, pending, stale and unavailable turns never restart',async t=>{
 for(const reason of ['aborted','error','toolUse']){const f=extension(t);await f.emit('agent_end',ended(reason));await f.emit('agent_settled');assert.equal(f.messages.length,0)}
 for(const change of [f=>f.ctx.isIdle=()=>false,f=>f.ctx.hasPendingMessages=()=>true,f=>f.ctx.signal=AbortSignal.abort(),f=>f.setResponse({fault:'unavailable'})]){const f=extension(t);change(f);await f.emit('agent_end',ended('stop'));await f.emit('agent_settled');assert.equal(f.messages.length,0)}
 for(const transition of ['session_start','session_tree','input']){const f=extension(t);await f.emit('agent_end',ended('stop'));await f.emit(transition,{reason:'new',source:'rpc'});await f.emit('agent_settled');assert.equal(f.messages.length,0)}
});
test('PPI-V0-006 explicit outcomes and uninstalled runtimes preserve ordinary Pi routes',async t=>{
 const f=extension(t);await f.command('corvint-outcome','{"outcome":"passed"}');assert.equal(f.calls.at(-1).kind,'ordinary');
 const g=extension(t,{installed:false});await g.emit('agent_end',ended('stop'));await g.emit('agent_settled');assert.equal(g.calls.at(-1).kind,'ordinary');assert.equal(g.calls.at(-1).input.stopHookActive,undefined);
});
test('PPI-V0-006 trust withdrawal cannot reuse an earlier blocked Stop',async t=>{
 const f=extension(t);await f.emit('agent_end',ended('stop'));await f.emit('agent_settled');assert.equal(f.messages.length,1);
 await f.emit('input',{source:'rpc'});f.ctx.isProjectTrusted=()=>false;
 await f.emit('agent_end',ended('stop'));await f.emit('agent_settled');assert.equal(f.messages.length,1);assert.equal(f.calls.filter(c=>c.event==='stop').length,1);
});
test('PPI-V0-006 recursive Stop reports unresolved Frontier/local policy without starting another turn',async t=>{
 const f=extension(t);const q=receipt();q.frontier.state='OPEN';q.policy.unmet=['fixture'];f.setResponse({fault:null,qualified:q,shouldContinue:true,receiptId:'open',context:'',degradations:[]});
 await f.emit('agent_end',ended('stop'));await f.emit('agent_settled');await f.emit('agent_end',ended('stop'));await f.emit('agent_settled');
 assert.equal(f.messages.length,1);assert.equal(f.statuses.length,1);assert.match(f.statuses[0][0].content,/unresolved.*Frontier OPEN.*local policy unmet/);assert.deepEqual(f.statuses[0][1],{triggerTurn:false});
});
test('PPI-V0-004 unavailable admission makes a separate ordinary request and exposes its degradation',async t=>{
 const handlers=new Map(),calls=[],notices=[];
 const pi={on:(name,f)=>handlers.set(name,[...(handlers.get(name)??[]),f]),registerCommand(){},registerTool(){}};
 const ordinary={close:async()=>{},run:async request=>{calls.push(request);return {context:'ORDINARY',receiptId:'fallback',degradations:[]}}};
 register(pi,{ordinary,qualified:{close:async()=>{},run:async()=>({fault:'invalid-qualified-response'})},installed:true,version:'0.85.1'});
 const ctx={cwd:'/fixture',isProjectTrusted:()=>true,hasUI:true,ui:{notify:text=>notices.push(text)},sessionManager:{getSessionId:()=> 'session'}};
 const emit=async(name,e)=>{let result;for(const f of handlers.get(name)??[])result=await f(e,ctx);return result};
 t.after(()=>emit('session_shutdown',{}));
 const output=await emit('before_agent_start',{prompt:'question',systemPrompt:'BASE'});
 assert.equal(calls.length,1);assert.equal(output.systemPrompt,'BASE\nORDINARY');assert.match(notices[0],/invalid-qualified-response/);
});
