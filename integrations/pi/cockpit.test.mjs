// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {createCockpitComponent,escapeTerminal,registerCockpit} from './cockpit.js';

const uiKit={matchesKey:(data,key)=>({left:'L',right:'R',up:'U',down:'D',pageUp:'P',pageDown:'N',escape:'E'}[key]===data),truncateToWidth:(s,w,e='…')=>s.length<=w?s:s.slice(0,Math.max(0,w-e.length))+e,visibleWidth:s=>s.replace(/\x1b\[[0-9;]*m/g,'').length};
const theme={fg:(_c,s)=>s,bold:s=>s};
function fixture(mode='print') {
 const commands=new Map(),handlers=new Map(),messages=[],notices=[];
 const pi={registerCommand:(n,c)=>commands.set(n,c),on:(n,h)=>handlers.set(n,h),sendMessage:(...a)=>messages.push(a)};
 const calls=[];
 const service=name=>({read:async(operation,input)=>{calls.push({name,operation,input});return {operation,receipt:`${name}-receipt`,raw:{stdout:`${name}\x1b[31m\n${'x'.repeat(9000)}`},exitCode:0}}});
 const ctx={mode,hasUI:mode==='rpc'||mode==='tui',cwd:'/repo',sessionManager:{getSessionId:()=> 's1'},ui:{setStatus(){},notify:(...a)=>notices.push(a)}};
 const lifecycle=registerCockpit(pi,{core:service('core'),tasks:service('tasks'),notice:(...a)=>notices.push(a),uiKit});
 return {commands,handlers,messages,notices,calls,ctx,lifecycle};
}

test('PWV cockpit registers one direct command and print mode returns a structured, bounded read result without UI',async()=>{
 const f=fixture();assert.deepEqual([...f.commands.keys()],['corvint']);
 await f.commands.get('corvint').handler('subject',f.ctx);
 assert.equal(f.calls.length,1);assert.equal(f.calls[0].operation,'context');
 const snapshot=f.messages[0][0].details;
 assert.equal(snapshot.profile,'corvint-cockpit/0');assert.equal(snapshot.view,'evidence');assert.equal(snapshot.authority.includes('do not establish'),true);
 assert.equal(f.notices.length,1);
});

test('terminal controls are escaped and rendered lines remain inside narrow and wide viewports',async()=>{
 assert.equal(escapeTerminal('ok\x1b[31m\x00\r\t'),'ok\\u001b[31m\\u0000\\u000d\\u0009');
 const state={selected:0,offset:0,stale:true,loading:false,reason:'test',loaded:{lines:['line','z'.repeat(200)]}};
 let rendered=0,closed;
 const tui={requestRender(){rendered++}},component=createCockpitComponent({state,refresh:async()=>{state.stale=false},done:v=>{closed=v},tui,theme,uiKit});
 for(const width of [24,100])assert.ok(component.render(width).every(line=>uiKit.visibleWidth(line)<=width));
 component.handleInput('R');assert.equal(state.selected,1);component.handleInput('5');assert.equal(state.selected,4);
 component.handleInput('D');assert.equal(state.offset,1);component.handleInput('r');await Promise.resolve();assert.equal(state.stale,false);
 component.handleInput('E');assert.equal(closed.view,'gaps');assert.ok(rendered>=4);
});

test('observed edits, compaction, tree and session transitions invalidate only memory state; clear and close are safe',async()=>{
 const f=fixture();await f.commands.get('corvint').handler('',f.ctx);
 for(const [event,payload] of [['tool_result',{toolName:'edit'}],['session_compact',{}],['session_tree',{}],['session_start',{}]])await f.handlers.get(event)(payload,f.ctx);
 f.lifecycle.invalidate('manual');f.lifecycle.clear();f.lifecycle.close();
 await f.commands.get('corvint').handler('',f.ctx);assert.equal(f.calls.length,1);
});

test('faults stay explicit uncertainty and RPC uses supported status/notification fallback',async()=>{
 const f=fixture('rpc');f.ctx.ui={setStatus:(...a)=>f.notices.push(a),notify:(...a)=>f.notices.push(a)};
 f.lifecycle.close();
 const commands=new Map(),messages=[];
 const pi={registerCommand:(n,c)=>commands.set(n,c),on(){},sendMessage:m=>messages.push(m)};
 registerCockpit(pi,{core:{read:async operation=>({operation,fault:'unsupported-profile',raw:{stderr:'no authority'},exitCode:2})},tasks:{}});
 await commands.get('corvint').handler('',f.ctx);
 assert.equal(messages[0].details.fault,'unsupported-profile');assert.equal(f.notices.some(n=>String(n).includes('UNCERTAIN')),true);
});

test('source expansion passes only explicit packet and selector to the injected validated service, or stays unavailable',async()=>{
 const commands=new Map(),messages=[],calls=[];
 const pi={registerCommand:(n,c)=>commands.set(n,c),on(){},sendMessage:m=>messages.push(m)};
 const ctx={mode:'print',hasUI:false,cwd:'/repo',sessionManager:{getSessionId:()=> 'source'},ui:{}};
 registerCockpit(pi,{core:{},tasks:{},context:{expand:async(input)=>{calls.push(input);return {operation:'expand',receipt:'pinned',raw:{stdout:'exact source'}}}}});
 const selector='{"result":0,"evidence":2,"lines":"10:20"}';
 await commands.get('corvint').handler(`source packet-4 ${selector}`,ctx);
 assert.deepEqual(calls,[{packetId:'packet-4',selector,maxBytes:8192}]);
 assert.equal(messages[0].details.view,'source-expansion');assert.equal(messages[0].details.result.receipt,'pinned');
 const unavailable=new Map(),fallback=[];
 const pi2={registerCommand:(n,c)=>unavailable.set(n,c),on(){},sendMessage:m=>fallback.push(m)};
 registerCockpit(pi2,{core:{},tasks:{}});await unavailable.get('corvint').handler('source packet-4 {"result":0,"evidence":0}',ctx);
 assert.equal(fallback[0].details.fault,'source-expansion-unavailable');
});

test('Tasks tab uses the native queue read operation and renders its structured envelope',async()=>{
 const f=fixture('tui');
 f.ctx.ui.custom=async factory=>{
  const component=await factory({requestRender(){}},theme,null,()=>{});
  component.handleInput('3');component.handleInput('r');await new Promise(resolve=>setImmediate(resolve));
  assert.match(component.render(80).join('\n'),/tasks-receipt/);
 };
 await f.commands.get('corvint').handler('',f.ctx);
 assert.equal(f.calls.at(-1).name,'tasks');assert.equal(f.calls.at(-1).operation,'queue');
});

test('an invalidation during refresh discards the late service result',async()=>{
 let release,readStarted;const pending=new Promise(resolve=>{release=resolve}),started=new Promise(resolve=>{readStarted=resolve});
 const commands=new Map(),handlers=new Map(),messages=[];
 const pi={registerCommand:(n,c)=>commands.set(n,c),on:(n,h)=>handlers.set(n,h),sendMessage:m=>messages.push(m)};
 const ctx={mode:'print',hasUI:false,cwd:'/repo',sessionManager:{getSessionId:()=> 'delayed'},ui:{}};
 registerCockpit(pi,{core:{read:async()=>{readStarted();return pending}},tasks:{}});
 const running=commands.get('corvint').handler('',ctx);await started;
 await handlers.get('tool_result')({toolName:'edit'},ctx);release({operation:'context',receipt:'late',raw:{stdout:'must not publish'}});await running;
 assert.equal(messages[0].details.stale,true);assert.equal(messages[0].details.reason,'discarded-stale-result');assert.equal(messages[0].details.result,null);
});

test('async canonical identity drift discards the result and rejected identity stays visible without reading',async()=>{
 let release,readStarted;const pending=new Promise(resolve=>{release=resolve}),started=new Promise(resolve=>{readStarted=resolve});
 let identityGeneration=1;const commands=new Map(),messages=[];
 const pi={registerCommand:(n,c)=>commands.set(n,c),on(){},sendMessage:m=>messages.push(m)};
 const ctx={mode:'print',hasUI:false,cwd:'/repo',sessionManager:{getSessionId:()=> 'identity'},ui:{}};
 registerCockpit(pi,{core:{read:async()=>{readStarted();return pending}},tasks:{},identity:async()=>({session:'identity',gitDir:'/git/repo',generation:identityGeneration})});
 const running=commands.get('corvint').handler('',ctx);await started;identityGeneration=2;release({operation:'context',receipt:'old-identity'});await running;
 assert.equal(messages[0].details.reason,'discarded-stale-result');assert.equal(messages[0].details.result,null);

 let reads=0;const rejected=new Map(),failures=[];
 const pi2={registerCommand:(n,c)=>rejected.set(n,c),on(){},sendMessage:m=>failures.push(m)};
 registerCockpit(pi2,{core:{read:async()=>{reads++}},tasks:{},identity:async()=>{throw Error('private path')}});
 await rejected.get('corvint').handler('',ctx);
 assert.equal(reads,0);assert.equal(failures[0].details.fault,'identity-unavailable');assert.equal(failures[0].details.stale,true);
});

test('canonical identity serialization ignores object key order',async()=>{
 let identityCall=0,reads=0;const commands=new Map(),messages=[];
 const pi={registerCommand:(n,c)=>commands.set(n,c),on(){},sendMessage:m=>messages.push(m)};
 const ctx={mode:'print',hasUI:false,cwd:'/repo',sessionManager:{getSessionId:()=> 'stable'},ui:{}};
 registerCockpit(pi,{core:{read:async operation=>{reads++;return {operation,receipt:'stable'}}},tasks:{},identity:async()=>++identityCall%2?{session:'stable',git:{dir:'/git/repo',generation:4}}:{git:{generation:4,dir:'/git/repo'},session:'stable'}});
 await commands.get('corvint').handler('',ctx);
 assert.equal(reads,1);assert.equal(messages[0].details.stale,false);assert.equal(messages[0].details.result.receipt,'stable');
});

test('navigation while a refresh is pending cannot publish the old view result under the new tab',async()=>{
 let release,readStarted;const pending=new Promise(resolve=>{release=resolve}),started=new Promise(resolve=>{readStarted=resolve});
 const commands=new Map(),pi={registerCommand:(n,c)=>commands.set(n,c),on(){},sendMessage(){}};
 const ctx={mode:'tui',hasUI:true,cwd:'/repo',sessionManager:{getSessionId:()=> 'navigation'},ui:{custom:async factory=>{
  const component=await factory({requestRender(){}},theme,null,()=>{});
  component.handleInput('r');await started;component.handleInput('3');
  release({operation:'context',receipt:'old-evidence',raw:{stdout:'must stay hidden'}});await new Promise(resolve=>setImmediate(resolve));
  const rendered=component.render(80).join('\n');assert.match(rendered,/Tasks/);assert.doesNotMatch(rendered,/old-evidence|must stay hidden/);assert.match(rendered,/STALE/);
 }}};
 registerCockpit(pi,{core:{read:async()=>{readStarted();return pending}},tasks:{read:async()=>({operation:'queue'})},uiKit});
 await commands.get('corvint').handler('',ctx);
});
