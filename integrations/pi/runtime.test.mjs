import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp,writeFile,readFile,rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { createRunner,validEnvelope,decodeEnvelope,PI_HOST_VERSION,PI_ADAPTER_VERSION } from './runtime.js';
const good={profile:'corvint-pi-adapter/0',event:'stop',host:'pi',surface:'extension',hostVersion:'0.99.1',adapterVersion:PI_ADAPTER_VERSION,support:'FALLBACK',receiptId:'harness-receipt:sha256:'+'a'.repeat(64),context:'',degradations:['frontier-authority-unavailable'],fault:null,shouldContinue:false};
test('AHI-024 Pi closed envelope rejects forged identity and continuation',()=>{assert.equal(validEnvelope(good,'stop'),true);for(const delta of [{hostVersion:'0.85.1'},{adapterVersion:'0.2.0'},{host:'codex'},{hostVersion:null},{shouldContinue:true},{extra:1},{receiptId:'fake'},{degradations:['unknown']},{fault:'unknown'}])assert.equal(validEnvelope({...good,...delta},'stop'),false);});
test('AHI-024 Pi config conflicts refuse before child effects; explicit option wins',async()=>{let calls=0;const spawnImpl=()=>{calls++;throw Error('marker')};const a=createRunner({env:{CORVINT_BIN:''},spawnImpl});assert.equal((await a.run({event:'stop',input:{},hostVersion:'0.99.1'})).fault,'invalid-binary-config');assert.equal(calls,0);const b=createRunner({binary:'/explicit',env:{CORVINT_BIN:''},spawnImpl});assert.equal((await b.run({event:'stop',input:{},hostVersion:'0.99.1'})).fault,'missing-binary');assert.equal(calls,1);});
test('AHI-024 Pi kills TERM-ignoring grandchild after leader closes',async()=>{const root=await mkdtemp(path.join(tmpdir(),'pi-cleanup-'));let pid;try{const script=path.join(root,'child');await writeFile(script,`#!/bin/sh\ntrap 'exit 0' TERM\n/bin/sh -c 'trap "" TERM; echo $$ > "${root}/pid"; while :; do sleep 1; done' &\nwait\n`,{mode:0o755});const abort=new AbortController();const runner=createRunner({binary:script});const work=runner.run({cwd:root,event:'stop',input:{},hostVersion:'0.99.1',signal:abort.signal});for(let n=0;n<100;n++){try{pid=Number(await readFile(path.join(root,'pid'),'utf8'));break}catch{await new Promise(r=>setTimeout(r,10))}}assert.ok(pid);abort.abort();assert.equal((await work).fault,'aborted');await runner.close();assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});}finally{if(pid)try{process.kill(pid,'SIGKILL')}catch{}await rm(root,{recursive:true,force:true});}});

test('AHI-024 Pi duplicate output identity is refused before shape decoding',()=>{
 const raw=JSON.stringify(good);assert.deepEqual(decodeEnvelope(raw),good);
 const duplicateHost='{"host":"forged",'+raw.slice(1);
 assert.equal((duplicateHost.match(/"host":/g)||[]).length,2);
 assert.throws(()=>decodeEnvelope(duplicateHost));
});

test('AHI-024 Pi success cannot invent an event',()=>{assert.equal(validEnvelope({...good,event:'invented'},'invented'),false);});

test('PWV-V0-016 a host version Core cannot admit is refused before any process starts',async()=>{
 let calls=0;const spawnImpl=()=>{calls++;throw Error('must not spawn')};
 const runner=createRunner({binary:'/fixture/corvint',spawnImpl});
 for(const hostVersion of ['0.85.1','0.99.2','',undefined,null])for(const request of [{event:'stop',input:{}},{operation:'context',input:{}}]){
  const result=await (request.operation?runner.tool({...request,hostVersion}):runner.run({...request,hostVersion}));
  assert.deepEqual(result,{fault:'unsupported-host-version'});
 }
 assert.equal(calls,0);
 assert.equal(PI_HOST_VERSION,'0.99.1');
 assert.equal((await runner.run({event:'stop',input:{},hostVersion:PI_HOST_VERSION})).fault,'missing-binary');assert.equal(calls,1);
 await runner.close();
});
