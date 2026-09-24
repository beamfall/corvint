// SPDX-License-Identifier: AGPL-3.0-or-later
// Explicit native-host check: Pi 0.85.1 and the pinned Go toolchain must be installed.
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, cpSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here=dirname(fileURLToPath(import.meta.url)), root=resolve(here,'../..');
const active=new Set(), nativeGroups=new Set();
const kill=child=>{try{process.kill(-child.pid,'SIGKILL')}catch{}};
const cleanup=()=>{
 for(const child of active)kill(child);
 for(const path of nativeGroups) {
  try{const pid=Number(readFileSync(path,'utf8'));if(Number.isSafeInteger(pid)&&pid>1)process.kill(-pid,'SIGKILL')}catch{}
 }
};
process.on('exit',cleanup);
process.once('SIGINT',()=>{cleanup();process.exit(130)});
process.once('SIGTERM',()=>{cleanup();process.exit(143)});

function run(command,args,options={}) {
 return new Promise((resolve,reject)=>{
  const {signal,onSpawn,onStdout,stdin=false,...spawnOptions}=options;
  const child=spawn(command,args,{...spawnOptions,detached:true,stdio:[stdin?'pipe':'ignore','pipe','pipe']});
  active.add(child);
  let stdout='',stderr='',error;
  const abort=()=>{error=Error('interrupted');kill(child)};
  const timer=setTimeout(()=>{error=Error('child timeout');kill(child)},45000);
  signal?.addEventListener('abort',abort,{once:true});
  if(signal?.aborted)abort();
  child.stdout.on('data',data=>{stdout+=data;onStdout?.(data,child);if(stdout.length>2**20)abort()});
  child.stderr.on('data',data=>{stderr+=data;if(stderr.length>2**20)abort()});
  child.on('error',value=>{error=value});
  onSpawn?.(child);
  child.on('close',(code,termination)=>{
   clearTimeout(timer);signal?.removeEventListener('abort',abort);kill(child);active.delete(child);
   if(error)return reject(error);
   resolve({code,termination,stdout,stderr});
  });
 });
}
const provider=`import { appendFileSync, readFileSync, writeFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createAssistantMessageEventStream } from '@earendil-works/pi-ai';
export default function(pi) {
 pi.on('tool_result',async e=>{if(e.toolName==='corvint_context')appendFileSync(process.env.CORVINT_PI_CAPTURE+'.observed',JSON.stringify(e.details?.corvint)+'\\n')});
 pi.registerTool({name:'corvint_fixture_edit',label:'Fixture edit',description:'Fixture edit and verification',parameters:{type:'object',properties:{},additionalProperties:false},async execute(){
  writeFileSync('main.go','package example\\nfunc One() int { return 2 }\\n');
  execFileSync('git',['diff','--check'],{timeout:2000});
  execFileSync('git',['add','main.go'],{timeout:2000});
  execFileSync('git',['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture edit'],{timeout:2000});
  return {content:[{type:'text',text:'private fixture tool body'}],details:{corvint:{changedPaths:['main.go'],verification:[{commandSha256:createHash('sha256').update('git diff --check').digest('hex'),status:'passed'}]}}};
 }});
 pi.registerProvider('corvint-fixture',{
  baseUrl:'http://127.0.0.1:1',apiKey:'fixture-only',api:'corvint-fixture',
  models:[{id:'fixture',name:'Offline fixture',reasoning:false,input:['text'],cost:{input:0,output:0,cacheRead:0,cacheWrite:0},contextWindow:200000,maxTokens:1000}],
  streamSimple(model,context){
   appendFileSync(process.env.CORVINT_PI_CAPTURE,JSON.stringify(context)+'\\n');
   const stream=createAssistantMessageEventStream();
   const output={role:'assistant',content:[{type:'text',text:'fixture response'}],api:model.api,provider:model.provider,model:model.id,usage:{input:1,output:1,cacheRead:0,cacheWrite:0,totalTokens:2,cost:{input:0,output:0,cacheRead:0,cacheWrite:0,total:0}},stopReason:'stop',timestamp:Date.now()};
   if(process.env.CORVINT_PI_TOOLS==='1') {
    const results=context.messages.filter(m=>m.role==='toolResult');
    let call;
    if(results.length===0)call={name:'corvint_context',arguments:{task:'main.go'}};
    if(results.length===1){
     const text=results[0].content.map(c=>c.text??'').join('');
     const handle=text.match(/Expansion handle: (packet-[0-9]+)/)?.[1];
     if(!handle)throw Error('context handle absent: '+text);
     call={name:'corvint_expand',arguments:{handle,result:0,evidence:0,lines:'1:1'}};
    }
    if(results.length===2)call={name:'corvint_fixture_edit',arguments:{}};
    if(results.length===3)call={name:'corvint_record_outcome',arguments:{task:'explicit native Pi fixture outcome',changedPaths:['main.go'],verification:['git diff --check'],outcome:'passed'}};
    if(call){output.content=[{type:'toolCall',id:'fixture-'+results.length,...call}];output.stopReason='toolUse'}
   }
   queueMicrotask(()=>{stream.push({type:'done',reason:output.stopReason,message:output});stream.end()});
   return stream;
  }
 });
}`;

async function tuiFixture(scratch) {
 const binary=join(scratch,'pi-tui-fixture');
 const build=await run('go',['build','-o',binary,'./tools/pi-tui-fixture'],{cwd:root,env:{...process.env,GOTOOLCHAIN:'local'}});
 assert.equal(build.code,0,build.stderr);
 return binary;
}

test('AHI-002 AHI-024 native Pi package lifecycle and ephemeral context',async t=>{
 assert.notEqual(process.platform,'win32','Pi adapter requires POSIX group cleanup');
 const scratch=mkdtempSync(join(tmpdir(),'corvint-pi-host-'));
 t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const home=join(scratch,'home'), repo=join(scratch,'repo'), agent=join(home,'.pi/agent'), pkg=join(scratch,'package'), capture=join(scratch,'capture');
 for(const path of [home,repo,agent])mkdirSync(path,{recursive:true});
 cpSync(here,pkg,{recursive:true,filter:path=>!path.endsWith('.test.mjs')});
 writeFileSync(join(scratch,'provider.ts'),provider);
 writeFileSync(join(repo,'AGENTS.md'),'# Fixture\nUse evidence from main.go.\n');
 writeFileSync(join(repo,'.gitignore'),'.context-corvint/\n.corvint/\n');
 writeFileSync(join(repo,'go.mod'),'module fixture.local/example\n\ngo 1.27.1\n');
 writeFileSync(join(repo,'main.go'),'package example\nfunc One() int { return 1 }\n');
 for(const args of [['init','-q'],['add','.'],['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture']]) {
  const result=spawnSync('git',args,{cwd:repo,encoding:'utf8'});assert.equal(result.status,0,result.stderr);
 }
 const binary=join(scratch,'corvint');
 const build=await run('go',['build','-o',binary,'./cmd/corvint'],{cwd:root,env:{...process.env,GOTOOLCHAIN:'local'}});
 assert.equal(build.code,0,build.stderr);
 const env={PATH:process.env.PATH,HOME:home,LANG:'en_US.UTF-8',PI_CODING_AGENT_DIR:agent,PI_OFFLINE:'1',CORVINT_BIN:binary,CORVINT_PI_CAPTURE:capture};
 const pi=(args,extra={})=>run(process.env.PI_BIN??'pi',args,{cwd:repo,env:{...env,...extra}});
 const version=await pi(['--version']);assert.equal(version.stdout.trim(),'0.85.1');
 const installed=await pi(['install',pkg]);assert.equal(installed.code,0,installed.stderr);
 assert.match((await pi(['list'])).stdout,/package/);
 const flags=['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--mode','json','--provider','corvint-fixture','--model','fixture','--thinking','off','-e',join(scratch,'provider.ts'),'-p'];
 const result=await pi([...flags,'inspect main.go','inspect One']);
 assert.equal(result.code,0,result.stderr);
 assert.doesNotMatch(result.stderr,/Corvint unavailable|Failed to load extension/);
 const contexts=readFileSync(capture,'utf8').trim().split('\n').map(JSON.parse);
 assert.equal(contexts.length,2);
 for(const ctx of contexts)assert.equal(ctx.systemPrompt.split('BEGIN CORVINT REPOSITORY DATA').length,2);
 assert.equal(JSON.stringify(contexts[0].messages).split('BEGIN CORVINT REPOSITORY DATA').length,2,'startup reaches first provider request');
 assert.doesNotMatch(JSON.stringify(contexts[1].messages),/BEGIN CORVINT REPOSITORY DATA/,'recovery is ephemeral');
 const sessions=join(agent,'sessions');
 const { readdirSync }=await import('node:fs');
 const sessionFiles=readdirSync(sessions,{recursive:true}).filter(p=>p.endsWith('.jsonl'));
 assert.ok(sessionFiles.length);
 for(const path of sessionFiles)assert.doesNotMatch(readFileSync(join(sessions,path),'utf8'),/BEGIN CORVINT REPOSITORY DATA/,'automatic context must not persist');
 const invalid=await pi([...flags,'/corvint-outcome null']);assert.match(invalid.stderr,/invalid-input/);
 const outcome=await pi([...flags,'/corvint-outcome '+JSON.stringify({outcome:'passed',taskSha256:'a'.repeat(64),changedPaths:['main.go'],verification:[{commandSha256:'b'.repeat(64),status:'passed'}]})]);
 assert.match(outcome.stderr,/outcome-persistence-unavailable/);
 const toolFlow=await pi([...flags.filter(flag=>flag!=='--no-tools'),'--tools','corvint_context,corvint_expand,corvint_record_outcome,corvint_fixture_edit','exercise the explicitly authorized Pi outcome learning fixture'],{CORVINT_PI_TOOLS:'1'});
 assert.equal(toolFlow.code,0,toolFlow.stderr);
 assert.doesNotMatch(toolFlow.stderr,/Corvint unavailable|Failed to load extension/);
 assert.match(toolFlow.stdout,/selector_complete/);
 assert.match(toolFlow.stdout,/corvint-explicit-record/);
 const toolContexts=readFileSync(capture,'utf8').trim().split('\n').map(JSON.parse);
 const lastTools=toolContexts.at(-1).messages.filter(m=>m.role==='toolResult');
 assert.deepEqual(lastTools.map(m=>m.toolName),['corvint_context','corvint_expand','corvint_fixture_edit','corvint_record_outcome']);
 assert.ok(lastTools.every(m=>!m.isError),JSON.stringify(lastTools));
 const observed=JSON.parse(readFileSync(capture+'.observed','utf8').trim());
 assert.match(observed.observedEvidenceHandles[0],/^context-packet:sha256:[0-9a-f]{64}$/);
 assert.ok(lastTools[0].content[0].text.includes(observed.observedEvidenceHandles[0]));
 assert.match(readFileSync(join(repo,'main.go'),'utf8'),/return 2/);
 const rpcEvents=[];let buffered='',settled=0;
 const rpcFlags=flags.filter(flag=>flag!=='-p').map(flag=>flag==='json'?'rpc':flag);
 const rpc=await run(process.env.PI_BIN??'pi',rpcFlags,{cwd:repo,env,stdin:true,onSpawn(child){child.stdin.write(JSON.stringify({id:'ready',type:'get_state'})+'\n')},onStdout(data,child){
  buffered+=data;
  for(;;){const index=buffered.indexOf('\n');if(index<0)break;const line=buffered.slice(0,index);buffered=buffered.slice(index+1);if(!line)continue;
   const event=JSON.parse(line);rpcEvents.push(event);
   if(event.id==='ready'&&event.type==='response')child.stdin.write(JSON.stringify({id:'first',type:'prompt',message:'inspect main.go'})+'\n');
   if(event.type==='agent_settled'&&++settled===1)child.stdin.write(JSON.stringify({id:'new',type:'new_session'})+'\n');
   if(event.id==='new'&&event.type==='response')child.stdin.write(JSON.stringify({id:'second',type:'prompt',message:'inspect One'})+'\n');
   if(event.type==='agent_settled'&&settled===2)child.stdin.end();
  }
 }});
 assert.equal(rpc.code,0,rpc.stderr);
 assert.equal(settled,2);
 assert.ok(rpcEvents.some(e=>e.id==='new'&&e.success===true));
 assert.doesNotMatch(rpc.stderr,/Corvint unavailable|Failed to load extension/);
 const rpcContexts=readFileSync(capture,'utf8').trim().split('\n').slice(-2).map(JSON.parse);
 assert.ok(rpcContexts.every(c=>c.systemPrompt.includes('BEGIN CORVINT REPOSITORY DATA')));
 assert.ok(rpcContexts.every(c=>JSON.stringify(c.messages).includes('BEGIN CORVINT REPOSITORY DATA')));
 const tuiCapture=join(scratch,'tui-capture'),tuiWitness=join(scratch,'tui-group');
 const tuiFlags=flags.filter(flag=>flag!=='-p'&&flag!=='--mode'&&flag!=='json');
 nativeGroups.add(tuiWitness);
 t.after(()=>nativeGroups.delete(tuiWitness));
 const tui=await run(await tuiFixture(scratch),[process.env.PI_BIN??'pi',...tuiFlags],{cwd:repo,env:{...env,TERM:'xterm-256color',CORVINT_PI_CAPTURE:tuiCapture,CORVINT_PI_PTY_WITNESS:tuiWitness}});
 assert.equal(tui.code,0,tui.stderr);
 assert.match(tui.stdout,/Native Pi TUI prompt and clean shutdown passed/);
 const tuiContext=JSON.parse(readFileSync(tuiCapture,'utf8').trim().split('\n')[0]);
 assert.match(tuiContext.systemPrompt,/BEGIN CORVINT REPOSITORY DATA/);
 const settingsPath=join(agent,'settings.json'), settings=JSON.parse(readFileSync(settingsPath,'utf8'));
 settings.packages=[{source:pkg,extensions:[]}];writeFileSync(settingsPath,JSON.stringify(settings));
 const disabled=await pi([...flags,'disabled']);assert.equal(disabled.code,0,disabled.stderr);
 assert.doesNotMatch(JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1)).systemPrompt,/BEGIN CORVINT/);
 settings.packages=[pkg];writeFileSync(settingsPath,JSON.stringify(settings));
 const upgraded=await pi(['update',pkg]);assert.equal(upgraded.code,0,upgraded.stderr);
 const enabled=await pi([...flags,'enabled']);assert.equal(enabled.code,0,enabled.stderr);
 assert.match(JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1)).systemPrompt,/BEGIN CORVINT/);
 for(const signal of ['SIGINT','SIGTERM']) {
  const witness=join(scratch,signal+'.pid'), group=join(scratch,signal+'.group'), slow=join(scratch,'slow-'+signal);
  writeFileSync(slow,`#!/bin/sh\necho $$ > "${group}"\ntrap 'exit 0' TERM\n/bin/sh -c 'trap "" TERM; echo $$ > "${witness}"; while :; do sleep 1; done' &\nwait\n`,{mode:0o755});
  nativeGroups.add(group);
  t.after(()=>{try{if(existsSync(group))process.kill(-Number(readFileSync(group,'utf8')),'SIGKILL')}catch{}nativeGroups.delete(group)});
  let started;const ready=new Promise(r=>{started=r});
  const prior=readFileSync(capture,'utf8');
  const running=run(process.env.PI_BIN??'pi',[...flags,'must not reach provider'],{cwd:repo,env:{...env,CORVINT_BIN:slow},onSpawn:started});
  const child=await ready;
  for(let i=0;i<150&&!existsSync(witness);i++)await new Promise(r=>setTimeout(r,10));
  assert.ok(existsSync(witness),'startup native child witness');
  if(process.env.CORVINT_PI_INTERRUPT_WITNESS){
   writeFileSync(process.env.CORVINT_PI_INTERRUPT_WITNESS,JSON.stringify({group:Number(readFileSync(group,'utf8')),descendant:Number(readFileSync(witness,'utf8'))}));
   await running;assert.fail('outer harness did not interrupt');
  }
  child.kill(signal);
  const stopped=await running;
  assert.equal(stopped.code,signal==='SIGINT'?130:143,stopped.stderr);
  assert.equal(readFileSync(capture,'utf8'),prior,'cancelled startup must not run provider');
  const pid=Number(readFileSync(witness,'utf8'));
  assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});
 }
 const removed=await pi(['remove',pkg]);assert.equal(removed.code,0,removed.stderr);
 assert.deepEqual(JSON.parse(readFileSync(settingsPath,'utf8')).packages,[]);
 const uninstalled=await pi([...flags,'removed']);assert.equal(uninstalled.code,0,uninstalled.stderr);
 assert.doesNotMatch(JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1)).systemPrompt,/BEGIN CORVINT/);
});

test('AHI-024 native-host harness cancellation kills descendants',{skip:!!process.env.CORVINT_PI_INTERRUPT_WITNESS},async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-host-cancel-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const pidFile=join(scratch,'pid');const abort=new AbortController();
 const script=join(scratch,'child.js');
 writeFileSync(script,`const {spawn}=require('node:child_process');const {writeFileSync}=require('node:fs');const child=spawn(process.execPath,['-e','process.on("SIGTERM",()=>{});setInterval(()=>{},1000)'],{stdio:'ignore'});writeFileSync(${JSON.stringify(pidFile)},String(child.pid));setInterval(()=>{},1000);`);
 const work=run(process.execPath,[script],{signal:abort.signal});
 const rejected=assert.rejects(work,/interrupted/);
 for(let i=0;i<200&&!existsSync(pidFile);i++)await new Promise(r=>setTimeout(r,10));
 assert.ok(existsSync(pidFile));const pid=Number(readFileSync(pidFile,'utf8'));
 abort.abort();await rejected;
 for(let i=0;i<100;i++){try{process.kill(pid,0)}catch{break}await new Promise(r=>setTimeout(r,10))}
 assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});
});


test('AHI-024 interrupting the host check reaps its separately detached native group',{skip:!!process.env.CORVINT_PI_INTERRUPT_WITNESS},async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-host-interrupt-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const witness=join(scratch,'witness');
 let spawned;const ready=new Promise(r=>{spawned=r});
 // Run the test file directly so the signal targets its registered cleanup boundary.
 const work=run(process.execPath,[fileURLToPath(import.meta.url)],{cwd:root,env:{...process.env,CORVINT_PI_INTERRUPT_WITNESS:witness},onSpawn:spawned});
 const child=await ready;
 for(let i=0;i<2500&&!existsSync(witness);i++)await new Promise(r=>setTimeout(r,10));
 assert.ok(existsSync(witness),'native detached child reached');
 const {group,descendant}=JSON.parse(readFileSync(witness,'utf8'));
 const groupFile=join(scratch,'group');writeFileSync(groupFile,String(group));nativeGroups.add(groupFile);
 t.after(()=>{try{process.kill(-group,'SIGKILL')}catch{}nativeGroups.delete(groupFile)});
 child.kill('SIGTERM');
 const result=await work;assert.equal(result.code,143,result.stderr);
 for(let i=0;i<100;i++){try{process.kill(descendant,0)}catch{break}await new Promise(r=>setTimeout(r,10))}
 assert.throws(()=>process.kill(descendant,0),{code:'ESRCH'});
 assert.throws(()=>process.kill(-group,0),{code:'ESRCH'});
});

test('AHI-025 native TUI fixture interruption reaps its PTY process group',{skip:!!process.env.CORVINT_PI_INTERRUPT_WITNESS},async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-pty-cancel-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const witness=join(scratch,'group'),descendant=join(scratch,'descendant'),fake=join(scratch,'fake.sh');
 writeFileSync(fake,`trap '' TERM\nsleep 86400 &\necho $! > ${JSON.stringify(descendant+'.tmp')}\nmv ${JSON.stringify(descendant+'.tmp')} ${JSON.stringify(descendant)}\nwait\n`);
 nativeGroups.add(witness);t.after(()=>nativeGroups.delete(witness));
 let spawned;const ready=new Promise(r=>{spawned=r});
 const work=run(await tuiFixture(scratch),['/bin/sh',fake],{env:{...process.env,CORVINT_PI_PTY_WITNESS:witness},onSpawn:spawned});
 const child=await ready;
 for(let i=0;i<300&&!existsSync(descendant);i++)await new Promise(r=>setTimeout(r,10));
 assert.ok(existsSync(descendant));const pid=Number(readFileSync(descendant,'utf8'));
 child.kill('SIGTERM');const ended=await work;assert.equal(ended.code,143,ended.stderr);
 for(let i=0;i<100;i++){try{process.kill(pid,0)}catch{break}await new Promise(r=>setTimeout(r,10))}
 assert.throws(()=>process.kill(pid,0),{code:'ESRCH'});
 assert.throws(()=>process.kill(-Number(readFileSync(witness,'utf8')),0),{code:'ESRCH'});
});
