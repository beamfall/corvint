// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, renameSync, symlinkSync, realpathSync } from 'node:fs';
import { join } from 'node:path';
import { canonicalWorkflowJSON as canonical, decodeWorkflowEnvelope, registerWorkflow, workflowSessionKey, workflowIdentity } from './workflow.js';
const hash=v=>createHash('sha256').update(v).digest('hex');
const sign=v=>({...v,resultDigest:'dogfood-event:sha256:'+hash('corvint-dogfood-event/0\0'+canonical(v))});
function envelope(event,input,{lifecycle='active',satisfied=false}={}) {
 const p={lifecycle,satisfied,unmet:satisfied?[]:['selected-check-unverified'],base:'a'.repeat(40),target:'a'.repeat(40),planDigest:'b'.repeat(64),reportSetDigest:''};
 const incomplete=lifecycle==='active'||lifecycle==='satisfied'&&!satisfied;
 return sign({profile:'corvint-dogfood-event/0',ok:true,mutates:false,support:'FALLBACK',event,
 adapter:{host:'pi',hostVersion:'0.99.1',surface:'extension',adapterVersion:'0.3.2'},
 repository:{commitRevision:'a'.repeat(40),treeRevision:'b'.repeat(40),objectFormat:'sha1',worktreeState:'clean',dirtyPathCount:0,dirtyPathsSha256:hash('[]')},
 requestSha256:hash(canonical(input)),degradations:['frontier-authority-unavailable'],frontier:{state:'UNAVAILABLE',shouldContinue:false,reason:'frontier-authority-unavailable'},policy:p,
 completion:{decision:event==='stop'&&incomplete&&!input.stopHookActive?'block':'release',reason:event!=='stop'?'not-stop-event':incomplete?(input.stopHookActive?'local-policy-continuation-limit':'local-policy-incomplete'):'local-policy-'+lifecycle},
 ...(event==='session-start'?{context:{private:'source packet <x>\u2028',numericKeys:{'10':10,'2':2}}}:{})});
}
function fixture(t,options={}) {
 const handlers=new Map(),commands=new Map(),calls=[],notices=[];
 let session='one';const root=mkdtempSync(join(tmpdir(),'pi-workflow-unit-'));mkdirSync(join(root,'.git'));writeFileSync(join(root,'.git/HEAD'),'ref: refs/heads/main\n');t.after(()=>rmSync(root,{recursive:true,force:true}));
 const ctx={cwd:root,hasUI:true,mode:'json',isProjectTrusted:()=>true,hasPendingMessages:()=>false,sessionManager:{getSessionId:()=>session},ui:{notify:(...v)=>notices.push(v)}};
 const runner={async run(req){calls.push(req);if(req.args[1]!=='event')return {exitCode:0,stdout:JSON.stringify({ok:true,profile:'corvint-local-completion/0',tool:'dogfood-'+req.args[1],mutates:req.args[1]==='begin',claim:'caller-owned-selected-workflow-only',policy:{lifecycle:'active',satisfied:false}})};const event=req.args[req.args.indexOf('--event')+1];return {exitCode:0,stdout:JSON.stringify(envelope(event,JSON.parse(req.input),options))}},async close(){}};
 const bridge=registerWorkflow({on:(name,fn)=>handlers.set(name,fn),registerCommand:(name,fn)=>commands.set(name,fn)},{runner,version:'0.99.1',notice:(ctx,code)=>notices.push(code)});
 t.after(()=>bridge.close());
 const boundary={entries:[],outcome:'completed',continue:false,context:{canContinue:false,pendingMessages:[]}};
 return {root,runner,calls,ctx,notices,bridge,boundary,setSession:v=>session=v,emit:(event,value={})=>handlers.get(event)(value,ctx),command:text=>commands.get('corvint-workflow').handler(text,ctx)};
}
test('LCP-V0-008 closed Pi workflow envelopes verify complete digest and request binding',()=>{
 const input={sessionIdSha256:workflowSessionKey('one'),stopHookActive:false,changedPaths:[]},value=envelope('stop',input);
 assert.deepEqual(decodeWorkflowEnvelope(JSON.stringify(value),'stop',input),value);
 for(const mutate of [v=>v.policy.satisfied=true,v=>v.completion.decision='release',v=>v.repository.commitRevision='c'.repeat(40),v=>v.adapter.hostVersion='0.85.1',v=>v.frontier.shouldContinue=true,v=>v.extra=1]){const bad=structuredClone(value);mutate(bad);assert.throws(()=>decodeWorkflowEnvelope(JSON.stringify(bad),'stop',input))}
 const {resultDigest,...base}=value;
 for(const delta of [{mutates:true},{completion:{decision:'block',reason:'not-stop-event'}},{policy:{...base.policy,satisfied:true}},{adapter:{...base.adapter,adapterVersion:'0.2.0'}}])assert.throws(()=>decodeWorkflowEnvelope(JSON.stringify(sign({...base,...delta})),'stop',input));
 assert.throws(()=>decodeWorkflowEnvelope(JSON.stringify(value).replace('{','{"ok":true,'),'stop',input));
 assert.throws(()=>decodeWorkflowEnvelope(JSON.stringify(value),'stop',{...input,stopHookActive:true}));
});
test('LCP-V0-008 canonical envelope sorts numeric and Unicode keys and rejects ambiguous numbers',()=>{
 assert.equal(canonical({'2':2,'10':10,'\u{10000}':1,'\ue000':2}),'{"10":10,"2":2,"":2,"𐀀":1}');
 assert.throws(()=>canonical({v:9007199254740992}));assert.throws(()=>canonical({v:'\ud800'}));
});
test('LCP-V0-008 one continuation per external input; generated activity never resets allowance',async t=>{
 const f=fixture(t);await f.emit('input',{source:'interactive'});
 const first=await f.emit('agent_before_settle',f.boundary);assert.equal(first.continue,true);assert.equal(first.entries.length,1);
 assert.doesNotMatch(JSON.stringify(first),/source packet/);assert.match(first.entries[0].content,/Unmet policy categories: selected-check-unverified/);assert.ok(first.entries[0].content.includes(canonical(['dogfood','status','--session-key',workflowSessionKey('one')])));assert.match(first.entries[0].content,/Receipt: dogfood-event:sha256:/);
 await f.emit('input',{source:'extension'});
 assert.equal(await f.emit('agent_before_settle',f.boundary),undefined);assert.equal(JSON.parse(f.calls.at(-1).input).stopHookActive,true);
 await f.emit('agent_settled');assert.ok(f.notices.includes('local-policy-incomplete'));
 await f.emit('input',{source:'rpc'});assert.equal((await f.emit('agent_before_settle',f.boundary)).continue,true);
});
test('LCP-V0-008 error abort queued continuation and trust negatives never dispatch',async t=>{
 const f=fixture(t);
 for(const delta of [{entries:null},{outcome:'error'},{outcome:'aborted'},{continue:true},{context:{pendingMessages:[{}]}}])assert.equal(await f.emit('agent_before_settle',{...f.boundary,...delta}),undefined);
 f.ctx.hasPendingMessages=()=>true;await f.emit('agent_before_settle',f.boundary);f.ctx.hasPendingMessages=()=>false;
 const c=new AbortController();c.abort();f.ctx.signal=c.signal;await f.emit('agent_before_settle',f.boundary);f.ctx.signal=undefined;
 f.ctx.isProjectTrusted=()=>false;await f.emit('agent_before_settle',f.boundary);assert.equal(f.calls.length,0);
});
test('LCP-V0-008 inactive cancelled and satisfied policies never continue',async t=>{
 for(const options of [{lifecycle:'inactive'},{lifecycle:'cancelled'},{lifecycle:'satisfied',satisfied:true}]){const f=fixture(t,options);assert.equal(await f.emit('agent_before_settle',f.boundary),undefined)}
});
test('LCP-V0-008 in-flight response discarded on transition and recovery stays ephemeral',async t=>{
 const f=fixture(t);let release;const normal=f.runner.run;
 f.runner.run=async req=>{await new Promise(r=>release=r);return normal(req)};
 const pending=f.emit('agent_before_settle',f.boundary);f.bridge.clear();release();assert.equal(await pending,undefined);
 f.runner.run=normal;await f.emit('session_start',{reason:'startup'});await f.emit('input',{source:'interactive'});
 const recovered=await f.emit('context',{messages:[]});assert.match(recovered.messages[0].content,/source packet/);assert.equal(await f.emit('context',{messages:[]}),undefined);
 await f.emit('session_compact');f.setSession('fork');assert.equal(await f.emit('context',{messages:[]}),undefined);
});
test('LCP-V0-008 explicit begin and status use namespaced key and closed operator arguments',async t=>{
 const f=fixture(t);await f.command('begin /tmp/plan with spaces.json');
 assert.deepEqual(f.calls[0].args,['dogfood','begin','--plan','/tmp/plan with spaces.json','--session-key',workflowSessionKey('one')]);
 await f.command('status');assert.deepEqual(f.calls[1].args,['dogfood','status','--session-key',workflowSessionKey('one')]);
 for(const text of ['finish','verify test','status --session-key forged','begin'])await f.command(text);
 assert.equal(f.calls.length,2);
 f.setSession('fork');await f.command('status');assert.notEqual(f.calls[2].args.at(-1),f.calls[1].args.at(-1));
});

test('LCP-V0-008 cancelled native reads are joined and never become remediation',async t=>{
 const f=fixture(t);let retired=false;
 f.runner.run=req=>new Promise(resolve=>req.signal.addEventListener('abort',()=>{retired=true;resolve({exitCode:null,stdout:'',fault:'aborted'})},{once:true}));
 const pending=f.emit('agent_before_settle',f.boundary);await f.bridge.clear();assert.equal(retired,true);assert.equal(await pending,undefined);
});
test('LCP-V0-008 incomplete enrollment survives input and abort without native Stop',async t=>{
 const f=fixture(t);await f.command('begin plan.json');await f.emit('input',{source:'interactive'});
 await f.emit('agent_before_settle',{...f.boundary,outcome:'aborted'});await f.emit('agent_settled');
 assert.equal(f.calls.length,1);assert.ok(f.notices.includes('local-policy-incomplete'));
});
test('LCP-V0-008 same-session reload does not reset recursive-remediation protection',async t=>{
 const f=fixture(t);await f.emit('session_start',{reason:'startup'});assert.equal((await f.emit('agent_before_settle',f.boundary)).continue,true);
 await f.emit('session_start',{reason:'reload'});assert.equal(await f.emit('agent_before_settle',f.boundary),undefined);
});

test('LCP-V0-008 native mixed worktree receipts remain valid and invented dirty state refuses',()=>{
 const input={sessionIdSha256:workflowSessionKey('one'),stopHookActive:false,changedPaths:[]};
 const {resultDigest,...basis}=envelope('stop',input);
 const mixed=sign({...basis,repository:{...basis.repository,worktreeState:'mixed',dirtyPathCount:1,dirtyPathsSha256:hash('["main.go"]')}});
 assert.equal(decodeWorkflowEnvelope(JSON.stringify(mixed),'stop',input).repository.worktreeState,'mixed');
 const invalid=sign({...basis,repository:{...basis.repository,worktreeState:'dirty',dirtyPathCount:1}});
 assert.throws(()=>decodeWorkflowEnvelope(JSON.stringify(invalid),'stop',input));
});
test('LCP-V0-008 remediation preserves earlier Pi boundary entries without mutating the event',async t=>{
 const f=fixture(t),entry={type:'custom',customType:'unrelated-extension',data:{preserve:true}},entries=[entry];
 const result=await f.emit('agent_before_settle',{...f.boundary,entries});
 assert.equal(result.continue,true);assert.equal(result.entries.length,2);assert.equal(result.entries[0],entry);assert.deepEqual(entries,[entry]);
 assert.equal(result.entries[1].customType,'corvint-workflow-remediation');
});

test('LCP-V0-008 nested cwd resolves one canonical worktree and linked gitdir',async t=>{
 const f=fixture(t),nested=join(f.root,'nested/deep');mkdirSync(nested,{recursive:true});
 const expected=workflowIdentity(f.ctx);f.ctx.cwd=nested;assert.deepEqual(workflowIdentity(f.ctx),expected);
 await f.command('status');assert.equal(f.calls[0].cwd,realpathSync(f.root));
 const linked=join(f.root,'linked');mkdirSync(linked);writeFileSync(join(linked,'.git'),'gitdir: ../.git\n');f.ctx.cwd=linked;
 assert.equal(workflowIdentity(f.ctx).gitDir,expected.gitDir);assert.equal(workflowIdentity(f.ctx).root,realpathSync(linked));
});
test('LCP-V0-008 repository rebound and branch switches discard native in-flight results',async t=>{
 for(const rebound of ['gitdir','branch']){
  const f=fixture(t);let release;const normal=f.runner.run;
  f.runner.run=async req=>{await new Promise(r=>release=r);return normal(req)};
  const pending=f.emit('agent_before_settle',f.boundary);
  if(rebound==='gitdir'){renameSync(join(f.root,'.git'),join(f.root,'.git-old'));mkdirSync(join(f.root,'.git'))}
  writeFileSync(join(f.root,'.git/HEAD'),rebound==='branch'?'ref: refs/heads/other\n':'ref: refs/heads/main\n');
  release();assert.equal(await pending,undefined);
 }
});
test('LCP-V0-008 Git identity refuses symlink markers and oversized pointer files',t=>{
 const f=fixture(t);renameSync(join(f.root,'.git'),join(f.root,'gitdir'));symlinkSync('gitdir',join(f.root,'.git'));
 assert.throws(()=>workflowIdentity(f.ctx));rmSync(join(f.root,'.git'));writeFileSync(join(f.root,'.git'),'gitdir: '+'x'.repeat(8192));assert.throws(()=>workflowIdentity(f.ctx));
});
