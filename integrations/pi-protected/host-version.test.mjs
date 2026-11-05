// SPDX-License-Identifier: Apache-2.0
// PWV-V0-016 (V1-2149): the ordinary fallback declares the embedded Pi version, and
// the Pi runtime refuses a version Core cannot admit before any process starts.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import register from './extension.mjs';
import {PROTECTED_PI_VERSION} from './host-version.mjs';
import {createRunner,PI_HOST_VERSION,PI_ADAPTER_VERSION,validEnvelope} from '../pi/runtime.js';

test('PWV-V0-016 the declared protected Pi version is the pinned SDK version',()=>{
 const pkg=JSON.parse(readFileSync(new URL('./package.json',import.meta.url),'utf8'));
 assert.equal(pkg.dependencies['@earendil-works/pi-coding-agent'],PROTECTED_PI_VERSION);
 assert.match(readFileSync(new URL('./runtime.ts',import.meta.url),'utf8'),/installed,version:PROTECTED_PI_VERSION\}/);
});

test('PWV-V0-016 the ordinary fallback with the declared version is admitted or refused before spawn',async t=>{
 const handlers=new Map(),notices=[];let spawned=0;
 const spawnImpl=()=>{spawned++;throw Error('fixture spawn')};
 const pi={on:(name,f)=>handlers.set(name,[...(handlers.get(name)??[]),f]),registerCommand(){},registerTool(){}};
 const ordinary=createRunner({binary:'/fixture/corvint',spawnImpl});
 register(pi,{ordinary,qualified:{close:async()=>{},run:async()=>({fault:'invalid-qualified-response'})},installed:true,version:PROTECTED_PI_VERSION});
 const ctx={cwd:'/fixture',isProjectTrusted:()=>true,hasUI:true,ui:{notify:text=>notices.push(text)},sessionManager:{getSessionId:()=> 'session'}};
 const emit=async(name,e)=>{let result;for(const f of handlers.get(name)??[])result=await f(e,ctx);return result};
 t.after(()=>emit('session_shutdown',{}));
 const output=await emit('before_agent_start',{prompt:'question',systemPrompt:'BASE'});
 assert.equal(output,undefined,'a refused fallback injects no context');
 if(PROTECTED_PI_VERSION===PI_HOST_VERSION){assert.equal(spawned,1);assert.match(notices.join('\n'),/missing-binary/);return}
 // V1-2149 confirmed at source: the embedded 0.85.1 is not the admitted 0.99.1, so
 // every ordinary fallback is a typed, visible refusal and starts no process.
 assert.equal(spawned,0);
 assert.match(notices.join('\n'),/Corvint unavailable: unsupported-host-version/);
});


test('PWV-V0-8100 admitted ordinary fallback passes the real envelope validator',async t=>{
 const {spawn}=await import('node:child_process');
 const envelope={profile:'corvint-pi-adapter/0',event:'user-prompt',host:'pi',surface:'extension',hostVersion:PI_HOST_VERSION,adapterVersion:PI_ADAPTER_VERSION,support:'FALLBACK',receiptId:'harness-receipt:sha256:'+'a'.repeat(64),context:'BEGIN CORVINT REPOSITORY DATA\n{}\nEND CORVINT REPOSITORY DATA',degradations:[],fault:null,shouldContinue:false};
 assert.equal(validEnvelope(envelope,'user-prompt'),true);
 assert.equal(validEnvelope({...envelope,hostVersion:PROTECTED_PI_VERSION},'user-prompt'),PROTECTED_PI_VERSION===PI_HOST_VERSION);
 let spawned=0;const notices=[],handlers=new Map();
 const ordinary=createRunner({binary:process.execPath,spawnImpl:(_binary,_args,options)=>{spawned++;return spawn(process.execPath,['-e',`process.stdin.resume();process.stdin.on('end',()=>process.stdout.write(${JSON.stringify(JSON.stringify(envelope))}));`],options)}});
 const pi={on:(name,f)=>handlers.set(name,[...(handlers.get(name)??[]),f]),registerCommand(){},registerTool(){}};
 register(pi,{ordinary,qualified:{close:async()=>{},run:async()=>({fault:'invalid-qualified-response'})},installed:true,version:PI_HOST_VERSION});
 const ctx={cwd:process.cwd(),isProjectTrusted:()=>true,hasUI:true,ui:{notify:text=>notices.push(text)},sessionManager:{getSessionId:()=> 'admitted-fallback'}};
 const emit=async(name,e)=>{let result;for(const f of handlers.get(name)??[])result=await f(e,ctx);return result};
 t.after(()=>emit('session_shutdown',{}));
 const result=await emit('before_agent_start',{prompt:'question',systemPrompt:'BASE'});
 assert.equal(spawned,1);assert.equal(result.systemPrompt,'BASE\n'+envelope.context);assert.match(notices.join('\n'),/invalid-qualified-response/);
});
