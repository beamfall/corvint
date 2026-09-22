// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { registerTools,toolObservations } from './tools.js';
import { validToolEnvelope } from './runtime.js';

function fixture() {
 const definitions=new Map(),commands=new Map(),calls=[],notices=[];
 const pi={registerTool(t){definitions.set(t.name,t)},registerCommand(name,command){commands.set(name,command)}};
 const runner={async tool(request){calls.push(request);return {context:'native framed result',packet:{json:'opaque packet',sha256:'a'.repeat(64),commit:'b'.repeat(40),evidenceHandle:'context-packet:sha256:'+'a'.repeat(64)},mutation:'not-attempted'}}};
 const ctx={cwd:'/one',sessionManager:{getSessionId:()=> 'session'},isProjectTrusted:()=>true};
 const tools=registerTools(pi,{runner,version:'0.85.1',notice(...args){notices.push(args)}});
 return {ctx,runner,calls,tools,notices,command:(name,text)=>commands.get(name).handler(text,ctx),call:(name,args,signal)=>definitions.get(name).execute('test',args,signal,undefined,ctx)};
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
 const good={profile:'corvint-pi-tool/0',operation:'record',hostVersion:'0.85.1',adapterVersion:'0.2.0',support:'FALLBACK',ok:true,mutation:'recorded',context:'BEGIN CORVINT REPOSITORY DATA\n{}\nEND CORVINT REPOSITORY DATA',packet:null,fault:null};
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
