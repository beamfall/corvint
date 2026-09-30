// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {createCoreService,registerCoreTools} from './core.js';
const sha='a'.repeat(40),ctx={cwd:'/tmp',isProjectTrusted:()=>true};
function fixture(raw={exitCode:0,stdout:'{"unknowns":["non-Go"],"authority":"UNAVAILABLE"}',stderr:''}){const calls=[];const service=createCoreService({runner:{run:async input=>{calls.push(input);return raw},close:async()=>{}}});return {service,calls};}
test('native read mappings retain raw receipt and limitations',async()=>{
 const {service,calls}=fixture();const result=await service.read('impact',{paths:['x.go'],limit:1},ctx);assert.deepEqual(calls[0].args,['impact','--limit','1','--','x.go']);assert.deepEqual(result.receipt.unknowns,['non-Go']);assert.equal(result.receipt.authority,'UNAVAILABLE');
 await service.read('cem-status',{map:'.corvint/change.cem.json',base:sha,target:sha},ctx);assert.deepEqual(calls[1].args,['cem','status','--map','.corvint/change.cem.json','--expected-base',sha,'--target',sha]);
 await service.read('dogfood-status',{sessionKey:'b'.repeat(64)},ctx);assert.equal(calls[2].args[2],'--session-key');
});
test('closed operations reject writes, extra flags, conflicting selectors and untrusted projects',async()=>{
 const {service,calls}=fixture();for(const [op,input] of [['index',{}],['prove',{task:'x',mutate:true}],['impact',{paths:['--provider-command']}],['impact',{paths:['x.go'],base:sha}],['cem-status',{map:'x'}],['query',{task:'x',limit:21}]])assert.equal((await service.read(op,input,ctx)).fault,'invalid-input');
 assert.equal((await service.read('affected',{}, {...ctx,isProjectTrusted:()=>false})).fault,'untrusted-project');assert.equal(calls.length,0);
});
test('frontier exit 1 retains open advisory queue; transport and native failures stay distinct',async()=>{
 const raw={exitCode:1,stdout:'{"items":[{"reason":"open"}]}',stderr:''};let f=fixture(raw);assert.equal((await f.service.read('frontier',{cem:'c',ocm:'o',base:sha,target:sha},ctx)).fault,undefined);
 f=fixture({...raw,exitCode:2,stderr:'{"code":"unsupported-profile"}'});const failed=await f.service.read('affected',{},ctx);assert.equal(failed.fault,'native-command-failed');assert.equal(failed.raw.stderr,'{"code":"unsupported-profile"}');
 f=fixture({exitCode:0,stdout:'not json',stderr:''});assert.equal((await f.service.read('affected',{},ctx)).fault,'malformed-output');
 f=fixture({...raw,fault:'timeout'});assert.equal((await f.service.read('affected',{},ctx)).fault,'timeout');
});
test('registration exposes one read tool and structured failures',async()=>{
 let tool;const {service}=fixture();registerCoreTools({registerTool:t=>tool=t},{service});assert.equal(tool.name,'corvint_core_read');const result=await tool.execute('id',{operation:'index',input:{}},undefined,undefined,ctx);assert.equal(result.isError,true);assert.equal(result.details.corvint.fault,'invalid-input');
});
test('explicit generation invalidation rejects an inflight view',async()=>{
 let resolve;const service=createCoreService({runner:{run:()=>new Promise(r=>resolve=r),close:async()=>{}}});const pending=service.read('affected',{},ctx);service.clear();resolve({exitCode:0,stdout:'{}',stderr:''});assert.equal((await pending).fault,'stale-context');
});
