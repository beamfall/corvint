// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {createCommandRunner} from './process.js';
const cwd=process.cwd();
const testEnv={...process.env};delete testEnv.NODE_TEST_CONTEXT;
const runner=options=>createCommandRunner({binary:process.execPath,env:testEnv,...options});
test('argv and stdin remain literal; successful native output retained',async()=>{
 const r=runner();const result=await r.run({cwd,args:['-e','process.stdin.pipe(process.stdout)'],input:'$(echo unsafe)'});assert.equal(result.stdout,'$(echo unsafe)');assert.equal(result.exitCode,0);assert.equal(result.fault,undefined);await r.close();
});
test('aggregate output is bounded and process retired',async()=>{
 const r=runner({maxBytes:64});const result=await r.run({cwd,args:['-e','process.stdout.write("x".repeat(10000));setInterval(()=>{},100)']});assert.equal(result.fault,'output-too-large');assert.ok(Buffer.byteLength(result.stdout)+Buffer.byteLength(result.stderr)<=64);await r.close();
});
test('deadline, abort and close join active requests',async()=>{
 for(const mode of ['timeout','aborted','closed']){
  const r=runner({timeoutMs:mode==='timeout'?50:2000});const controller=new AbortController();const result=r.run({cwd,args:['-e','setInterval(()=>{},100)'],signal:controller.signal});
  if(mode==='aborted')controller.abort();if(mode==='closed')await r.close();assert.equal((await result).fault,mode);await r.close();assert.equal((await r.run({cwd,args:[]})).fault,'closed');
 }
});
test('leader exit retires a child that ignores TERM and owns inherited pipes',async()=>{
 const r=runner();const childCode='process.on("SIGTERM",()=>{});process.send("ready");setInterval(()=>{},100)';
 const script=`const {spawn}=require("node:child_process");const p=spawn(process.execPath,["-e",${JSON.stringify(childCode)}],{stdio:["ignore",process.stdout,process.stderr,"ipc"]});p.on("message",()=>{console.log(p.pid);p.disconnect();p.unref()});`;
 const result=await r.run({cwd,args:['-e',script]});assert.equal(result.fault,undefined);const pid=Number(result.stdout.trim());assert.ok(pid>0,JSON.stringify(result));
 assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});await r.close();
});
test('invalid arguments and absent executable produce typed faults',async()=>{
 const r=runner();assert.equal((await r.run({cwd,args:['bad\0arg']})).fault,'invalid-request');await r.close();
 const missing=createCommandRunner({binary:'/definitely/missing/corvint'});assert.equal((await missing.run({cwd,args:[]})).fault,'spawn-failed');await missing.close();
});
test('invalid UTF-8 never expands past the output budget',async()=>{
 const r=runner({maxBytes:32});const result=await r.run({cwd,args:['-e','process.stdout.write(Buffer.from([255,255,255]))']});assert.equal(result.fault,'invalid-encoding');assert.equal(result.stdout,'');await r.close();
});
