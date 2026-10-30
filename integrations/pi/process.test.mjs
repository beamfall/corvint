// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import {createCommandRunner} from './process.js';
import {mkdtemp,rm,stat} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
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

test('reusable cancel joins owned children before subsequent dispatch',async()=>{
 const r=runner();const pending=r.run({cwd,args:['-e','setInterval(()=>{},100)']});
 await r.cancel();assert.equal((await pending).fault,'aborted');
 const next=await r.run({cwd,args:['-e','console.log("reusable")']});assert.equal(next.exitCode,0);assert.equal(next.stdout.trim(),'reusable');await r.close();
});

// PWV-V0-018: a writer call's TERM grace lets a TERM-handling leader finish its record before KILL.
test('PWV-V0-018 writer TERM grace lets the leader finish; the default and the bound still kill',async t=>{
 const dir=await mkdtemp(join(tmpdir(),'pi-process-grace-'));t.after(()=>rm(dir,{recursive:true,force:true}));
 const r=runner();t.after(()=>r.close());let n=0;
 const started=async call=>{
  const ready=join(dir,String(n++)+'-"\'`$()\u2028'),controller=new AbortController();
  const code='process.on("SIGTERM",()=>setTimeout(()=>{process.stdout.write("recorded");process.exit(0)},400));require("node:fs").writeFileSync(process.argv[1],"");setInterval(()=>{},100)';
  const pending=r.run({cwd,args:['-e',code,ready],signal:controller.signal,...call});
  // Abort only once the child's TERM handler is installed.
  for(let i=0;i<1000;i++){try{await stat(ready);break}catch{await new Promise(x=>setTimeout(x,10))}}
  assert.ok((await stat(ready)).isFile(),'child reached readiness before abort');
  controller.abort();return pending;
 };
 const graced=await started({graceMs:5000});assert.equal(graced.fault,'aborted');assert.equal(graced.stdout,'recorded');assert.equal(graced.exitCode,0);
 const killed=await started({});assert.equal(killed.fault,'aborted');assert.equal(killed.stdout,'','the default 100 ms TERM grace still kills');
 const bounded=await started({graceMs:150});assert.equal(bounded.fault,'aborted');assert.equal(bounded.stdout,'','the grace bounds the wait for exit');
 for(const graceMs of [99,60001,1.5,'5000'])assert.equal((await r.run({cwd,args:[],graceMs})).fault,'invalid-request');
});

// PWV-V0-018 (Codex review): a leader exit reported after the graced run settled schedules no
// further signal to a process group the runner no longer owns.
test('PWV-V0-018 a late leader exit after a graced sweep sends no further signal',async t=>{
 const {EventEmitter}=await import('node:events');
 const child=new EventEmitter();child.pid=2147483000;
 for(const key of ['stdin','stdout','stderr']){child[key]=new EventEmitter();child[key].destroy=()=>{};child[key].end=()=>{}}
 const signals=[],realKill=process.kill;process.kill=(pid,signal)=>{if(pid===-child.pid){signals.push(signal);return true}return realKill(pid,signal)};
 t.after(()=>{process.kill=realKill});
 const r=createCommandRunner({binary:'fixture',spawnImpl:()=>child});t.after(()=>r.close());
 const controller=new AbortController();const pending=r.run({cwd,args:[],graceMs:150,signal:controller.signal});
 controller.abort();const result=await pending;assert.equal(result.fault,'aborted');assert.deepEqual(signals,['SIGTERM','SIGKILL']);
 child.emit('exit',null);await new Promise(x=>setTimeout(x,200));
 assert.deepEqual(signals,['SIGTERM','SIGKILL'],'no signal after the run settled');
});

// PWV-V0-020: an empty input never opens a stdin pipe, so a successful leader that exits before
// reading stdin (Git rev-parse on a fast Linux host) reports no stdin fault.
test('PWV-V0-020 empty input gives the child no stdin pipe; non-empty input keeps it',async t=>{
 const {spawn}=await import('node:child_process');const seen=[];
 const r=createCommandRunner({binary:process.execPath,env:testEnv,spawnImpl:(b,a,o)=>{seen.push(o.stdio[0]);return spawn(b,a,o)}});t.after(()=>r.close());
 for(let i=0;i<20;i++){const result=await r.run({cwd,args:['-e','process.stdout.write(String(require("node:fs").fstatSync(0).isCharacterDevice()))']});assert.equal(result.fault,undefined);assert.equal(result.exitCode,0);assert.equal(result.stdout,'true')}
 const piped=await r.run({cwd,args:['-e','process.stdin.pipe(process.stdout)'],input:'kept'});assert.equal(piped.stdout,'kept');assert.equal(piped.fault,undefined);
 assert.deepEqual([...new Set(seen)],['ignore','pipe']);
});
