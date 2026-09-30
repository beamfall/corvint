// SPDX-License-Identifier: AGPL-3.0-or-later
// Explicit native-host check: Pi 0.99.1 and the pinned Go toolchain must be installed.
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, cpSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { workflowSessionKey } from './workflow.js';
import { digest } from './operations.js';

const here=dirname(fileURLToPath(import.meta.url)), root=resolve(here,'../..');
// Pi 0.99.1 normalizes provider requests into a transcript with system messages.
const systemText=ctx=>ctx.messages.filter(m=>m.role==='system').map(m=>JSON.stringify(m)).join('\n');
const conversation=ctx=>ctx.messages.filter(m=>m.role!=='system');
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
 pi.on('agent_before_settle',(e,ctx)=>appendFileSync(process.env.CORVINT_PI_CAPTURE+'.settle',JSON.stringify({type:e.type,outcome:e.outcome,canContinue:e.context.canContinue,pending:e.context.pendingMessages.length,continue:e.continue,mode:ctx.mode})+'\\n'));
 pi.on('agent_settled',(e,ctx)=>appendFileSync(process.env.CORVINT_PI_CAPTURE+'.settle',JSON.stringify({type:e.type,mode:ctx.mode})+'\\n'));
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
 cpSync(resolve(process.env.CORVINT_PI_PACKAGE_SOURCE??here),pkg,{recursive:true,filter:path=>!path.endsWith('.test.mjs')});
 writeFileSync(join(scratch,'provider.ts'),provider);
 writeFileSync(join(repo,'AGENTS.md'),'# Fixture\nUse evidence from main.go.\n');
 writeFileSync(join(repo,'.gitignore'),'.context-corvint/\n.corvint/\n');
 writeFileSync(join(repo,'go.mod'),'module fixture.local/example\n\ngo 1.27.1\n');
 writeFileSync(join(repo,'main.go'),'package example\nfunc One() int { return 1 }\n');
 for(const args of [['init','-q'],['add','.'],['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture']]) {
  const result=spawnSync('git',args,{cwd:repo,encoding:'utf8'});assert.equal(result.status,0,result.stderr);
 }
 const binary=process.env.CORVINT_PI_HOST_BIN??join(scratch,'corvint');
 if(!process.env.CORVINT_PI_HOST_BIN){const build=await run('go',['build','-o',binary,'./cmd/corvint'],{cwd:root,env:{...process.env,GOTOOLCHAIN:'local'}});assert.equal(build.code,0,build.stderr);}
 const env={PATH:process.env.PATH,HOME:home,LANG:'en_US.UTF-8',PI_CODING_AGENT_DIR:agent,PI_OFFLINE:'1',CORVINT_BIN:binary,CORVINT_PI_CAPTURE:capture};
 const pi=(args,extra={})=>run(process.env.PI_BIN??'pi',args,{cwd:repo,env:{...env,...extra}});
 const version=await pi(['--version']);assert.equal(version.stdout.trim(),'0.99.1');
 const installed=await pi(['install',pkg]);assert.equal(installed.code,0,installed.stderr);
 assert.match((await pi(['list'])).stdout,/package/);
 const flags=['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--mode','json','--provider','corvint-fixture','--model','fixture','--thinking','off','-e',join(scratch,'provider.ts'),'-p'];
 const result=await pi([...flags,'inspect main.go','inspect One']);
 assert.equal(result.code,0,result.stderr);
 assert.doesNotMatch(result.stderr,/Corvint unavailable|Failed to load extension/);
 const contexts=readFileSync(capture,'utf8').trim().split('\n').map(JSON.parse);
 assert.equal(contexts.length,2);
 for(const ctx of contexts)assert.equal(systemText(ctx).split('BEGIN CORVINT REPOSITORY DATA').length,2);
 assert.equal(JSON.stringify(conversation(contexts[0])).split('BEGIN CORVINT REPOSITORY DATA').length,2,'startup reaches first provider request');
 assert.doesNotMatch(JSON.stringify(conversation(contexts[1])),/BEGIN CORVINT REPOSITORY DATA/,'recovery is ephemeral');
 const sessions=join(agent,'sessions');
 const { readdirSync }=await import('node:fs');
 const sessionFiles=readdirSync(sessions,{recursive:true}).filter(p=>p.endsWith('.jsonl'));
 assert.ok(sessionFiles.length);
 for(const path of sessionFiles)assert.doesNotMatch(readFileSync(join(sessions,path),'utf8'),/BEGIN CORVINT REPOSITORY DATA/,'automatic context must not persist');
 const settlement=readFileSync(capture+'.settle','utf8').trim().split('\n').map(JSON.parse);
 assert.deepEqual(settlement.map(e=>e.type),['agent_before_settle','agent_settled','agent_before_settle','agent_settled']);
 assert.ok(settlement.filter(e=>e.type==='agent_before_settle').every(e=>e.outcome==='completed'&&e.canContinue===false&&e.pending===0&&e.continue===false&&e.mode==='json'),JSON.stringify(settlement));
 const printed=await pi([...flags.map(flag=>flag==='json'?'text':flag),'print qualification']);
 assert.equal(printed.code,0,printed.stderr);assert.match(printed.stdout,/fixture response/);
 assert.doesNotMatch(printed.stderr,/Corvint unavailable|Failed to load extension/);
 const trustWitness=join(scratch,'untrusted-native-invocation'), trustBinary=join(scratch,'trust-binary');
 writeFileSync(trustBinary,'#!/bin/sh\necho invoked > '+JSON.stringify(trustWitness)+'\nexit 1\n',{mode:0o755});
 mkdirSync(join(repo,'.pi'));writeFileSync(join(repo,'.pi/settings.json'),'{}');
 const denied=await pi([...flags.map(flag=>flag==='--approve'?'--no-approve':flag),'untrusted qualification'],{CORVINT_BIN:trustBinary});
 assert.equal(denied.code,0,denied.stderr);assert.match(denied.stderr,/untrusted-project/);
 assert.equal(existsSync(trustWitness),false,'untrusted host must not invoke native binary');
 const deniedContext=JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1));
 assert.doesNotMatch(JSON.stringify(deniedContext),/BEGIN CORVINT REPOSITORY DATA/);
 rmSync(join(repo,'.pi'),{recursive:true});
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
 assert.ok(rpcContexts.every(c=>systemText(c).includes('BEGIN CORVINT REPOSITORY DATA')));
 assert.ok(rpcContexts.every(c=>JSON.stringify(conversation(c)).includes('BEGIN CORVINT REPOSITORY DATA')));
 const tuiCapture=join(scratch,'tui-capture'),tuiWitness=join(scratch,'tui-group');
 const tuiFlags=flags.filter(flag=>flag!=='-p'&&flag!=='--mode'&&flag!=='json');
 nativeGroups.add(tuiWitness);
 t.after(()=>nativeGroups.delete(tuiWitness));
 const tui=await run(await tuiFixture(scratch),[process.env.PI_BIN??'pi',...tuiFlags],{cwd:repo,env:{...env,TERM:'xterm-256color',CORVINT_PI_CAPTURE:tuiCapture,CORVINT_PI_PTY_WITNESS:tuiWitness}});
 assert.equal(tui.code,0,tui.stderr);
 assert.match(tui.stdout,/Native Pi TUI prompt and clean shutdown passed/);
 const tuiContext=JSON.parse(readFileSync(tuiCapture,'utf8').trim().split('\n')[0]);
 assert.match(systemText(tuiContext),/BEGIN CORVINT REPOSITORY DATA/);
 const settingsPath=join(agent,'settings.json'), settings=JSON.parse(readFileSync(settingsPath,'utf8'));
 settings.packages=[{source:pkg,extensions:[]}];writeFileSync(settingsPath,JSON.stringify(settings));
 const disabled=await pi([...flags,'disabled']);assert.equal(disabled.code,0,disabled.stderr);
 assert.doesNotMatch(systemText(JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1))),/BEGIN CORVINT/);
 settings.packages=[pkg];writeFileSync(settingsPath,JSON.stringify(settings));
 const upgraded=await pi(['update',pkg]);assert.equal(upgraded.code,0,upgraded.stderr);
 const enabled=await pi([...flags,'enabled']);assert.equal(enabled.code,0,enabled.stderr);
 assert.match(systemText(JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1))),/BEGIN CORVINT/);
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
 assert.doesNotMatch(systemText(JSON.parse(readFileSync(capture,'utf8').trim().split('\n').at(-1))),/BEGIN CORVINT/);
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

const qualificationProvider=`
import {appendFileSync} from 'node:fs';
import {createAssistantMessageEventStream} from '@earendil-works/pi-ai';
export default function(pi){
 const capture=value=>appendFileSync(process.env.CORVINT_PI_CAPTURE,JSON.stringify(value)+'\\n');
 let session,calls=0,active=0;
 pi.on('session_start',(e,ctx)=>{session=ctx.sessionManager.getSessionId();capture({kind:e.type,reason:e.reason,session})});
 pi.on('session_tree',e=>capture({kind:e.type,newLeaf:e.newLeafId,oldLeaf:e.oldLeafId,session}));
 pi.on('session_before_compact',e=>capture({kind:e.type,session}));
 pi.on('session_compact',(e,ctx)=>capture({kind:e.type,reason:e.reason,willRetry:e.willRetry,fromExtension:e.fromExtension,session,entries:ctx.sessionManager.getEntries()}));
 pi.on('tool_execution_start',e=>{if(e.toolName==='corvint_context')capture({kind:'tool-start',id:e.toolCallId,active:++active,session})});
 pi.on('tool_execution_end',e=>{if(e.toolName==='corvint_context'){capture({kind:'tool-end',id:e.toolCallId,isError:e.isError,session});active--}});
 pi.registerCommand('fixture-tree',{description:'Navigate a fixture tree without summarization',handler:async(id,ctx)=>{const result=await ctx.navigateTree(id,{summarize:false});capture({kind:'tree-command',cancelled:result.cancelled})}});
 pi.registerProvider('corvint-qualification',{
  baseUrl:'http://127.0.0.1:1',apiKey:'fixture-only',api:'corvint-qualification',
  models:[{id:'fixture',name:'Offline qualification',reasoning:false,input:['text'],cost:{input:0,output:0,cacheRead:0,cacheWrite:0},contextWindow:200000,maxTokens:1000}],
  streamSimple(model,context){
   calls++;capture({kind:'provider',calls,session,context});
   const stream=createAssistantMessageEventStream();
   const output={role:'assistant',content:[{type:'text',text:'Offline fixture summary or response.'}],api:model.api,provider:model.provider,model:model.id,usage:{input:1,output:1,cacheRead:0,cacheWrite:0,totalTokens:2,cost:{input:0,output:0,cacheRead:0,cacheWrite:0,total:0}},stopReason:'stop',timestamp:Date.now()};
   if(process.env.CORVINT_PI_QUALIFY==='overflow'&&calls===2){output.stopReason='error';output.errorMessage='context_length_exceeded: maximum context length exceeded'}
   if(process.env.CORVINT_PI_QUALIFY==='retry'&&calls===1){output.stopReason='error';output.errorMessage='429 rate limit offline fixture'}
   if(process.env.CORVINT_PI_QUALIFY==='concurrent'&&calls===1){output.stopReason='toolUse';output.content=[{type:'toolCall',id:'parallel-one',name:'corvint_context',arguments:{task:'main.go'}},{type:'toolCall',id:'parallel-two',name:'corvint_context',arguments:{task:'One'}}]}
   queueMicrotask(()=>{stream.push(output.stopReason==='error'?{type:'error',reason:'error',error:output}:{type:'done',reason:output.stopReason,message:output});stream.end()});return stream;
  }
 });
}`;

test('AHI-024 AHI-025 exact native fork tree compaction retry and concurrent tool qualification',{skip:!!process.env.CORVINT_PI_INTERRUPT_WITNESS},async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-matrix-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const home=join(scratch,'home'),agent=join(home,'.pi/agent'),repo=join(scratch,'repo'),pkg=join(scratch,'package'),capture=join(scratch,'capture'),providerPath=join(scratch,'provider.ts');
 for(const dir of [home,agent,repo])mkdirSync(dir,{recursive:true});
 cpSync(resolve(process.env.CORVINT_PI_PACKAGE_SOURCE??here),pkg,{recursive:true,filter:path=>!path.endsWith('.test.mjs')});
 writeFileSync(providerPath,qualificationProvider);
 writeFileSync(join(agent,'settings.json'),JSON.stringify({retry:{enabled:true,maxRetries:1,baseDelayMs:1,maxAgentDelayMs:5},compaction:{enabled:false,keepRecentTokens:1,reserveTokens:128}}));
 writeFileSync(join(repo,'AGENTS.md'),'# Fixture\nUse main.go evidence.\n');writeFileSync(join(repo,'.gitignore'),'.corvint/\n.context-corvint/\n');
 writeFileSync(join(repo,'go.mod'),'module fixture.local/example\n\ngo 1.27.1\n');writeFileSync(join(repo,'main.go'),'package example\nfunc One() int { return 1 }\n');
 for(const args of [['init','-q'],['add','.'],['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture']]){const git=spawnSync('git',args,{cwd:repo,encoding:'utf8'});assert.equal(git.status,0,git.stderr)}
 const binary=process.env.CORVINT_PI_HOST_BIN??join(scratch,'corvint');
 if(!process.env.CORVINT_PI_HOST_BIN){const build=await run('go',['build','-o',binary,'./cmd/corvint'],{cwd:root,env:{...process.env,GOTOOLCHAIN:'local'}});assert.equal(build.code,0,build.stderr)}
 const env={PATH:process.env.PATH,HOME:home,LANG:'en_US.UTF-8',PI_CODING_AGENT_DIR:agent,PI_OFFLINE:'1',CORVINT_BIN:binary,CORVINT_PI_CAPTURE:capture};
 const pi=(args,extra={})=>run(process.env.PI_BIN??'pi',args,{cwd:repo,env:{...env,...extra}});
 assert.equal((await pi(['--version'])).stdout.trim(),'0.99.1');assert.equal((await pi(['install',pkg])).code,0);
 const flags=['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--provider','corvint-qualification','--model','fixture','--thinking','off','-e',providerPath];
 let buffer='',child,driver,driverError,serial=0,settled;const waiting=new Map(),rpcEvents=[];
 const request=(type,fields={})=>new Promise((resolve,reject)=>{const id='matrix-'+(++serial);waiting.set(id,{resolve,reject});child.stdin.write(JSON.stringify({id,type,...fields})+'\n')});
 const turn=async message=>{const done=new Promise(resolve=>{settled=resolve});await request('prompt',{message});await done;settled=undefined};
 const rpc=await run(process.env.PI_BIN??'pi',[...flags,'--mode','rpc'],{cwd:repo,env,stdin:true,onSpawn(value){
  child=value;driver=(async()=>{
   await request('get_state');await turn('seed first main.go evidence');await turn('seed second One evidence');
   const entries=await request('get_entries');const target=entries.entries.find(e=>e.type==='message'&&e.message.role==='assistant').id;
   await request('prompt',{message:'/fixture-tree '+target});await turn('after native tree navigation');
   const forks=await request('get_fork_messages');const fork=await request('fork',{entryId:forks.messages.at(-1).entryId});assert.equal(fork.cancelled,false);
   await turn('after native session fork');
   const compact=await request('compact',{customInstructions:'Summarize this offline fixture.'});assert.equal(typeof compact.summary,'string');
   await turn('after native compaction');const stored=await request('get_entries');assert.doesNotMatch(JSON.stringify(stored),/BEGIN CORVINT REPOSITORY DATA/,'automatic recovery must remain ephemeral');
   child.stdin.end();
  })().catch(error=>{driverError=error;child.stdin.end()});
 },onStdout(data){buffer+=data;for(;;){const end=buffer.indexOf('\n');if(end<0)break;const line=buffer.slice(0,end);buffer=buffer.slice(end+1);if(!line)continue;const event=JSON.parse(line);rpcEvents.push(event);if(event.type==='response'&&waiting.has(event.id)){const pending=waiting.get(event.id);waiting.delete(event.id);if(event.success)pending.resolve(event.data);else pending.reject(Error(event.error))}if(event.type==='agent_settled')settled?.()}}});
 await driver;if(driverError)throw driverError;assert.equal(rpc.code,0,rpc.stderr);assert.doesNotMatch(rpc.stderr,/Corvint unavailable|Failed to load extension/);
 const lifecycle=readFileSync(capture,'utf8').trim().split('\n').map(JSON.parse);
 assert.ok(lifecycle.some(e=>e.kind==='session_tree'&&e.newLeaf!==e.oldLeaf));
 const sessions=lifecycle.filter(e=>e.kind==='session_start');assert.ok(sessions.some(e=>e.reason==='fork'));assert.ok(new Set(sessions.map(e=>e.session)).size>=2);
 assert.ok(lifecycle.some(e=>e.kind==='session_compact'&&e.reason==='manual'&&e.fromExtension===false));
 const compactAt=lifecycle.findIndex(e=>e.kind==='session_compact'),afterCompact=lifecycle.slice(compactAt+1).find(e=>e.kind==='provider');
 assert.match(JSON.stringify(conversation(afterCompact.context)),/BEGIN CORVINT REPOSITORY DATA/);
 for(const index of [lifecycle.findIndex(e=>e.kind==='session_tree'),lifecycle.findIndex(e=>e.kind==='session_start'&&e.reason==='fork')]){const next=lifecycle.slice(index+1).find(e=>e.kind==='provider');assert.equal(JSON.stringify(conversation(next.context)).split('BEGIN CORVINT REPOSITORY DATA').length,2,'tree/fork recovery reaches the next real provider request exactly once')}
 for(const scenario of ['retry','concurrent','overflow']){
  if(scenario==='overflow')writeFileSync(join(agent,'settings.json'),JSON.stringify({...JSON.parse(readFileSync(join(agent,'settings.json'),'utf8')),retry:{enabled:true,maxRetries:1,baseDelayMs:1},compaction:{enabled:true,keepRecentTokens:1,reserveTokens:128}}));
  const witness=join(scratch,scenario+'.jsonl');
  const result=await pi([...flags.filter(flag=>flag!=='--no-tools'),'--tools',scenario==='concurrent'?'corvint_context':'','--mode','json','-p',...(scenario==='overflow'?['seed history before overflow']:[]),'exercise '+scenario+' fixture'],{CORVINT_PI_QUALIFY:scenario,CORVINT_PI_CAPTURE:witness});
  assert.equal(result.code,0,result.stderr);assert.doesNotMatch(result.stderr,/Corvint unavailable|Failed to load extension/);
  const observed=readFileSync(witness,'utf8').trim().split('\n').map(JSON.parse),providers=observed.filter(e=>e.kind==='provider');assert.equal(providers.length,scenario==='overflow'?5:2,JSON.stringify(observed));assert.ok(providers.filter(e=>scenario!=='overflow'||[1,2,5].includes(e.calls)).every(e=>systemText(e.context).includes('BEGIN CORVINT REPOSITORY DATA')),'native per-turn evidence survives retry/tool continuation');
  if(scenario==='overflow'){
   const compact=observed.findIndex(e=>e.kind==='session_compact');assert.ok(compact>=0,JSON.stringify(observed));
   assert.ok(observed.slice(0,compact).some(e=>e.kind==='session_before_compact'));
   assert.doesNotMatch(JSON.stringify(observed[compact].entries),/BEGIN CORVINT REPOSITORY DATA/,'overflow recovery is not persisted into session entries');
   assert.equal(observed[compact].reason,'overflow');assert.equal(observed[compact].willRetry,true);assert.equal(observed[compact].fromExtension,false);
   const retry=observed.slice(compact+1).find(e=>e.kind==='provider');assert.ok(retry);assert.equal(JSON.stringify(conversation(retry.context)).split('BEGIN CORVINT REPOSITORY DATA').length,2);
   const events=result.stdout.trim().split('\n').map(JSON.parse);assert.ok(events.some(e=>e.type==='compaction_start'&&e.reason==='overflow'));assert.ok(events.some(e=>e.type==='compaction_end'&&e.willRetry));
  }else if(scenario==='retry'){const events=result.stdout.trim().split('\n').map(JSON.parse);assert.equal(events.filter(e=>e.type==='auto_retry_start').length,1);assert.ok(events.some(e=>e.type==='auto_retry_end'&&e.success));assert.equal(events.filter(e=>e.type==='agent_settled').length,1)}
  else{assert.ok(observed.some(e=>e.kind==='tool-start'&&e.active===2),'two native context tools must actually overlap');assert.equal(observed.filter(e=>e.kind==='tool-end'&&!e.isError).length,2);const results=providers[1].context.messages.filter(m=>m.role==='toolResult');assert.equal(results.length,2);assert.ok(results.every(m=>!m.isError&&m.content.some(c=>c.text?.includes('Expansion handle:'))))}
 }
});

for(const loseClaimReceipt of [false,true])test('LCP PWV native workflow owner and operator lease survive fork reload and resume'+(loseClaimReceipt?' with pending receipt':''),{skip:!!process.env.CORVINT_PI_INTERRUPT_WITNESS},async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-coexist-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const home=join(scratch,'home'),agent=join(home,'.pi/agent'),repo=join(scratch,'repo'),pkg=join(scratch,'package'),capture=join(scratch,'capture'),providerPath=join(scratch,'provider.ts');
 for(const dir of [home,agent,repo])mkdirSync(dir,{recursive:true});
 cpSync(resolve(process.env.CORVINT_PI_PACKAGE_SOURCE??here),pkg,{recursive:true,filter:path=>!path.endsWith('.test.mjs')});
 writeFileSync(providerPath,qualificationProvider.replace("let session,calls=0,active=0;","let session,calls=0,active=0; pi.registerCommand('fixture-reload',{description:'Reload native fixture',handler:async(_,ctx)=>{await ctx.reload()}});"));
 writeFileSync(join(repo,'.gitignore'),'.corvint/\n');writeFileSync(join(repo,'intent.md'),'# Intent\nPreserve operator ownership.\n');
 const git=args=>{const r=spawnSync('git',['-c','user.name=Fixture','-c','user.email=fixture@example.invalid',...args],{cwd:repo,encoding:'utf8'});assert.equal(r.status,0,r.stderr);return r.stdout.trim()};
 git(['init','-q','-b','main']);
 const templates=join(root,'internal/tasks/cli/testdata/external-agents');mkdirSync(join(repo,'.taskman'));
 for(const name of ['queue.json','policy.json']){let content=readFileSync(join(templates,name),'utf8');if(name==='queue.json')content=content.replace('"fixture":false','"fixture":true');writeFileSync(join(repo,'.taskman',name),content)}
 git(['add','.']);git(['commit','-qm','fixture']);
 const binary=process.env.CORVINT_PI_HOST_BIN;assert.ok(binary,'set CORVINT_PI_HOST_BIN to the exact candidate Core binary');
 const env={PATH:process.env.PATH,HOME:home,LANG:'en_US.UTF-8',PI_CODING_AGENT_DIR:agent,PI_OFFLINE:'1',CORVINT_BIN:binary,CORVINT_TASKS_BIN:process.env.CORVINT_TASKS_BIN??'corvint-tasks',CORVINT_PI_CAPTURE:capture};
 const receipts=[];
 const realTasks=env.CORVINT_TASKS_BIN,transport=join(scratch,'tasks-transport.cjs'),transportLog=join(scratch,'transport.jsonl'),lostReceipt=join(scratch,'lost-receipt.json');
 if(loseClaimReceipt){
  // Drop only the transport response after the real native writer has returned.
  writeFileSync(transport,`#!${process.execPath}\nconst {spawnSync}=require('node:child_process');const {appendFileSync,writeFileSync,existsSync}=require('node:fs');const args=process.argv.slice(2);const r=spawnSync(${JSON.stringify(realTasks)},args,{encoding:'utf8',timeout:10000});appendFileSync(${JSON.stringify(transportLog)},JSON.stringify({args,status:r.status,stdout:r.stdout})+'\\n');if(args[0]==='claim'&&r.status===0&&!existsSync(${JSON.stringify(lostReceipt)})){writeFileSync(${JSON.stringify(lostReceipt)},r.stdout);process.exit(0)}process.stdout.write(r.stdout??'');process.stderr.write(r.stderr??'');process.exit(r.status??1);\n`,{mode:0o700});
  env.CORVINT_TASKS_BIN=transport;
 }
 const pendingPath=join(repo,'.git','corvint-pi-operations','pending.json');let pendingBytes;
 const claimDispatches=()=>loseClaimReceipt?readFileSync(transportLog,'utf8').trim().split('\n').map(JSON.parse).filter(e=>e.args[0]==='claim').length:0;
 const native=args=>{const r=spawnSync(realTasks,args,{cwd:repo,env,encoding:'utf8',timeout:10000});assert.equal(r.status,0,r.stdout+r.stderr);const receipt=JSON.parse(r.stdout);receipts.push({component:'tasks',args,receipt});return receipt};
 native(['init','--request-id','coexist-init']);const created=native(['ticket','create','--request-id','coexist-ticket','--payload',readFileSync(join(templates,'ticket-create.json'),'utf8').trim()]);const ticketId=created.items[0].ticketId;
 git(['add','.taskman']);git(['commit','-qm','fixture ticket']);
 const plan=join(scratch,'plan.json');writeFileSync(plan,JSON.stringify({base:git(['rev-parse','HEAD']),intents:['intent.md'],checks:[{id:'check',argv:['git','diff','--check'],timeoutSeconds:5}]}));
 const pi=args=>run(process.env.PI_BIN??'pi',args,{cwd:repo,env});assert.equal((await pi(['install',pkg])).code,0);
 const claim={operation:'claim',requestId:'coexist-claim',input:{ticketId,expectedRevision:'1',holder:'operator-only'}};
 let child,driver,driverError,buffer='',serial=0,settled;const waiting=new Map();let original,forked,attempt,beforeAudit;
 const request=(type,fields={})=>new Promise((resolve,reject)=>{const id='coexist-'+(++serial);waiting.set(id,{resolve,reject});child.stdin.write(JSON.stringify({id,type,...fields})+'\n')});
 const turn=async message=>{const done=new Promise(resolve=>{settled=resolve});await request('prompt',{message});await done;settled=undefined};
 const command=message=>request('prompt',{message});
 const taskResult=async()=>{const all=await request('get_entries');return JSON.parse(all.entries.filter(e=>e.type==='custom_message'&&e.customType==='corvint-tasks').at(-1).content[0].text)};
 const policy=session=>{const r=spawnSync(binary,['dogfood','status','--session-key',workflowSessionKey(session.sessionId)],{cwd:repo,env,encoding:'utf8',timeout:10000});assert.equal(r.status,0,r.stdout+r.stderr);const receipt=JSON.parse(r.stdout);receipts.push({component:'core',session:session.sessionId,receipt});return receipt.policy};
 const unchanged=()=>{if(pendingBytes)assert.equal(readFileSync(pendingPath,'utf8'),pendingBytes,'pending replay identity survives host transitions');assert.deepEqual(native(['attempt','show',attempt.attemptId]).items[0],attempt,'host transitions must not claim, renew, release or complete the operator lease');assert.deepEqual(native(['receipt','audit']),beforeAudit,'host recovery adds no native Tasks receipts')};
 const result=await run(process.env.PI_BIN??'pi',['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--provider','corvint-qualification','--model','fixture','--thinking','off','-e',providerPath,'--mode','rpc'],{cwd:repo,env,stdin:true,onSpawn(value){child=value;driver=(async()=>{
  original=await request('get_state');await command('/corvint-workflow begin '+plan);await command('/corvint-tasks '+JSON.stringify(claim));let claimed=await taskResult();receipts.push({component:'pi-claim',receipt:claimed});
  if(loseClaimReceipt){assert.equal(claimed.ok,false);assert.equal(claimed.mutation,'unknown');assert.equal(claimDispatches(),1);pendingBytes=readFileSync(pendingPath,'utf8');assert.equal(JSON.parse(pendingBytes).requestId,claim.requestId);receipts.push({component:'private-pending',receipt:JSON.parse(pendingBytes)});claimed={raw:JSON.parse(readFileSync(lostReceipt,'utf8'))};receipts.push({component:'lost-native-claim',receipt:claimed.raw})}else assert.equal(claimed.ok,true,JSON.stringify(claimed));
  assert.equal(policy(original).lifecycle,'active');assert.equal(policy(original).satisfied,false);
  attempt=native(['attempt','show',claimed.raw.items[0].attemptId]).items[0];assert.equal(attempt.lease.holder,'operator-only');beforeAudit=native(['receipt','audit']);
  await turn('seed active workflow history');const forks=await request('get_fork_messages');await request('fork',{entryId:forks.messages.at(-1).entryId});forked=await request('get_state');assert.notEqual(forked.sessionId,original.sessionId);
  await command('/corvint-workflow status');assert.equal(policy(forked).lifecycle,'inactive');assert.equal(policy(original).lifecycle,'active');await command('/corvint-tasks '+JSON.stringify({...claim,resume:true}));const stale=await taskResult();assert.equal(stale.ok,false,JSON.stringify(stale));assert.equal(stale.mutation,'not-attempted');receipts.push({component:'pi-stale-replay',receipt:stale});if(loseClaimReceipt)assert.equal(claimDispatches(),1,'fork refusal must not redispatch the native claim');unchanged();
  await turn('fork must not acquire workflow ownership');await command('/fixture-reload');assert.equal(policy(forked).lifecycle,'inactive');assert.equal(policy(original).lifecycle,'active');unchanged();
  await request('switch_session',{sessionPath:original.sessionFile});await command('/corvint-workflow status');await turn('original owner resumes incomplete workflow');assert.equal(policy(original).lifecycle,'active');assert.equal(policy(original).satisfied,false);unchanged();
  if(loseClaimReceipt){
   await command('/corvint-tasks '+JSON.stringify({...claim,resume:true}));const recovered=await taskResult();receipts.push({component:'pi-original-replay',receipt:recovered});
   if(process.env.CORVINT_PI_RECOVERY_EVIDENCE)writeFileSync(process.env.CORVINT_PI_RECOVERY_EVIDENCE+'.pending',JSON.stringify({receipts,transport:readFileSync(transportLog,'utf8')},null,2)+'\n');
   assert.equal(recovered.ok,true,JSON.stringify(recovered));assert.equal(recovered.raw.items[0].replayed,true);assert.equal(recovered.raw.items[0].attemptId,attempt.attemptId);assert.equal(recovered.raw.items[0].generation,attempt.generation);assert.equal(claimDispatches(),2);assert.equal(existsSync(pendingPath),false);
   const terminal=JSON.parse(readFileSync(join(repo,'.git','corvint-pi-operations',digest(claim.requestId)+'.json'),'utf8'));assert.equal(terminal.receiptSha256,digest(recovered.raw));assert.equal(terminal.intentSha256,JSON.parse(pendingBytes).intentSha256);pendingBytes=undefined;unchanged();
  }
  const entries=await request('get_entries');assert.doesNotMatch(JSON.stringify(entries),/BEGIN CORVINT REPOSITORY DATA/);child.stdin.end();
 })().catch(error=>{driverError=error;child.stdin.end()})},onStdout(data){buffer+=data;for(;;){const end=buffer.indexOf('\n');if(end<0)break;const line=buffer.slice(0,end);buffer=buffer.slice(end+1);if(!line)continue;const e=JSON.parse(line);if(e.type==='response'&&waiting.has(e.id)){const p=waiting.get(e.id);waiting.delete(e.id);e.success?p.resolve(e.data):p.reject(Error(e.error))}if(e.type==='agent_settled')settled?.()}}});
 await driver;if(driverError)throw driverError;assert.equal(result.code,0,result.stderr);assert.doesNotMatch(result.stderr,/Failed to load extension/);
 const events=readFileSync(capture,'utf8').trim().split('\n').map(JSON.parse);for(const reason of ['fork','reload','resume'])assert.ok(events.some(e=>e.kind==='session_start'&&e.reason===reason),reason);
 if(process.env.CORVINT_PI_RECOVERY_EVIDENCE)writeFileSync(process.env.CORVINT_PI_RECOVERY_EVIDENCE+(loseClaimReceipt?'.pending':''),JSON.stringify({receipts,lifecycle:events.filter(e=>e.kind==='session_start'),...(loseClaimReceipt?{transport:readFileSync(transportLog,'utf8')}: {})},null,2)+'\n');
});
