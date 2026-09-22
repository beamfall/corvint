// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import register from './extension.js';

function fixture(t, {hasUI=true, trusted=true}={}) {
 const handlers=new Map(), commands=new Map(), calls=[], notices=[], messages=[];
 const runner={async close(){}, async run(request){calls.push(request);return {context:`CTX:${request.event}:${request.input.startSource??request.input.task??''}`,receiptId:String(calls.length),degradations:request.input.outcome?['outcome-persistence-unavailable']:[]};}};
 const pi={on(name,handler){handlers.set(name,handler)},registerCommand(name,command){commands.set(name,command)},registerTool(){},sendMessage(message,options){messages.push({message,options})}};
 const ctx={cwd:'/fixture',hasUI,ui:{notify:(...args)=>notices.push(args)},isProjectTrusted:()=>trusted,sessionManager:{getSessionId:()=> 'private-session'}};
 const listeners=new Set([...process.listeners('SIGINT'),...process.listeners('SIGTERM')]);
 register(pi,{runner,version:'0.85.1'});
 t.after(()=>{for(const signal of ['SIGINT','SIGTERM'])for(const listener of process.listeners(signal))if(!listeners.has(listener))process.removeListener(signal,listener);});
 return {ctx,runner,calls,notices,messages,emit:(name,event={})=>handlers.get(name)(event,ctx),command:(name,text)=>commands.get(name).handler(text,ctx)};
}

test('AHI-003 AHI-024 startup and compaction recovery reach exactly the next turn without session storage',async t=>{
 const f=fixture(t);
 await f.emit('session_start',{reason:'startup'});
 let turn=await f.emit('before_agent_start',{prompt:'one',systemPrompt:'BASE'});
 assert.match(JSON.stringify(await f.emit('context',{messages:[]})),/CTX:session-start:startup/);
 assert.match(turn.systemPrompt,/CTX:user-prompt:one/);
 turn=await f.emit('before_agent_start',{prompt:'two',systemPrompt:'BASE'});
 assert.equal(await f.emit('context',{messages:[]}),undefined);
 await f.emit('session_compact');
 turn=await f.emit('before_agent_start',{prompt:'three',systemPrompt:'BASE'});
 assert.match(JSON.stringify(await f.emit('context',{messages:[]})),/CTX:session-start:compact/);
 assert.equal(f.messages.length,0);
});

test('AHI-024 transitions clear prior recovery and refresh session identity',async t=>{
 const f=fixture(t);
 for(const [reason,expected] of [['startup','startup'],['reload','resume'],['new','clear'],['resume','resume'],['fork','resume']]) {
  await f.emit('session_start',{reason});
  const turn=await f.emit('before_agent_start',{prompt:reason,systemPrompt:'BASE'});
  const recovery=await f.emit('context',{messages:[]});
  assert.equal(recovery.messages.length,1);
  assert.match(recovery.messages[0].content,new RegExp(`CTX:session-start:${expected}`));
 }
 assert.ok(f.calls.every(r=>r.input.sessionIdSha256===createHash('sha256').update('private-session').digest('hex')));
});

test('AHI-024 outcome rejects nonobjects, duplicate keys and caller identity before child effects',async t=>{
 const f=fixture(t);
 for(const raw of ['null','[]','4','"text"','{"sessionIdSha256":"forged"}','{"outcome":"passed","outcome":"failed"}','{"verification":[{"status":"passed","status":"failed"}]}',' '.repeat(131073)+'{}']) {
  await f.command('corvint-outcome',raw);
  assert.equal(f.calls.length,0,raw.slice(0,80));
 }
 assert.equal(f.notices.length,8);
});

test('AHI-009 AHI-024 explicit outcome persistence degradation is visible',async t=>{
 const f=fixture(t);
 await f.command('corvint-outcome',JSON.stringify({outcome:'passed',taskSha256:'a'.repeat(64),changedPaths:['main.go'],verification:[{commandSha256:'b'.repeat(64),status:'passed'}]}));
 assert.equal(f.calls[0].event,'session-end');
 assert.match(JSON.stringify(f.notices),/outcome-persistence-unavailable/);
 assert.equal(f.messages.length,0);
});

test('AHI-008 AHI-024 untrusted contexts never invoke native reads or recover cached context',async t=>{
 const f=fixture(t,{trusted:false});
 await f.emit('session_start',{reason:'startup'});
 assert.equal(await f.emit('before_agent_start',{prompt:'secret',systemPrompt:'BASE'}),undefined);
 assert.equal(f.calls.length,0);
 assert.match(JSON.stringify(f.notices),/untrusted-project/);
});

test('AHI-005 AHI-024 tool content and terminal messages never leave Pi',async t=>{
 const f=fixture(t);
 await f.emit('tool_result',{toolName:'bash',input:{secret:'never-send'},content:[{text:'raw-output'}],details:{verification:[{status:'passed'}]}});
 await f.emit('agent_end',{messages:[{role:'assistant',stopReason:'stop',content:[{text:'private-message'}]}]});
 assert.deepEqual(f.calls.map(c=>c.event),['post-tool','stop']);
 assert.doesNotMatch(JSON.stringify(f.calls),/never-send|raw-output|private-message|verification/);
});

test('AHI-024 failed prompt discards pending recovery and cancellation signal is forwarded',async t=>{
 const f=fixture(t);
 await f.emit('session_start',{reason:'startup'});
 const run=f.runner.run;
 f.runner.run=async()=>({fault:'aborted'});
 f.ctx.signal=new AbortController().signal;
 assert.equal(await f.emit('before_agent_start',{prompt:'failed',systemPrompt:'BASE'}),undefined);
 f.runner.run=run;
 const turn=await f.emit('before_agent_start',{prompt:'next',systemPrompt:'BASE'});
 assert.equal(await f.emit('context',{messages:[]}),undefined);
 assert.equal(f.calls.at(-1).signal,f.ctx.signal);
});

test('AHI-003 recovery also reaches a compaction retry with no new agent-start event',async t=>{
 const f=fixture(t);
 await f.emit('session_compact');
 const original=[{role:'user',content:'continue'}];
 const result=await f.emit('context',{messages:original});
 assert.equal(original.length,1);
 assert.match(result.messages[1].content,/CTX:session-start:compact/);
 assert.equal(await f.emit('context',{messages:original}),undefined);
});

test('AHI-024 late prior-session results and changed roots cannot supply recovery',async t=>{
 const f=fixture(t);
 let resolve;
 f.runner.run=()=>new Promise(r=>{resolve=r});
 const old=f.emit('session_compact');
 f.runner.run=async()=>({fault:'aborted'});
 await f.emit('session_tree');
 resolve({context:'STALE',receiptId:'old',degradations:[]});await old;
 assert.equal(await f.emit('context',{messages:[]}),undefined);
 f.runner.run=async()=>({context:'OTHER ROOT',receiptId:'new',degradations:[]});
 await f.emit('session_compact');f.ctx.cwd='/other';
 assert.equal(await f.emit('context',{messages:[]}),undefined);
});

test('AHI-024 explicit context is visible without triggering a model turn; shutdown removes signal handlers',async t=>{
 const before={SIGINT:process.listenerCount('SIGINT'),SIGTERM:process.listenerCount('SIGTERM')};
 const f=fixture(t);
 await f.command('corvint-context','inspect parser');
 assert.equal(f.messages[0].message.content,'CTX:user-prompt:inspect parser');
 assert.deepEqual(f.messages[0].options,{triggerTurn:false});
 await f.emit('session_shutdown');
 for(const signal of Object.keys(before))assert.equal(process.listenerCount(signal),before[signal]);
});
