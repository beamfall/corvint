// SPDX-License-Identifier: AGPL-3.0-or-later
// Explicit offline qualification: exact Pi 0.99.1 and candidate Core with Pi LCP admission.
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here=dirname(fileURLToPath(import.meta.url));
const groups=new Set();
const cleanup=()=>{for(const pid of groups)try{process.kill(-pid,'SIGKILL')}catch{}};
process.on('exit',cleanup);
process.once('SIGTERM',()=>{cleanup();process.exit(143)});
process.once('SIGINT',()=>{cleanup();process.exit(130)});
function run(binary,args,{cwd,env}={}) {
 return new Promise((resolve,reject)=>{
  const child=spawn(binary,args,{cwd,env,detached:true,stdio:['ignore','pipe','pipe']});groups.add(child.pid);
  let stdout='',stderr='',failure;const stop=()=>{try{process.kill(-child.pid,'SIGKILL')}catch{}};
  const timer=setTimeout(()=>{failure=Error('native workflow qualification timeout');stop()},30000);
  const read=name=>chunk=>{if(name==='stdout')stdout+=chunk;else stderr+=chunk;if(stdout.length+stderr.length>2**20){failure=Error('native workflow qualification output bound');stop()}};
  child.stdout.on('data',read('stdout'));child.stderr.on('data',read('stderr'));
  child.on('error',err=>{failure=err});child.on('close',code=>{clearTimeout(timer);stop();groups.delete(child.pid);if(failure)reject(failure);else resolve({code,stdout,stderr})});
 });
}
const extension=(processModule)=>`
import {appendFileSync} from 'node:fs';
import {createAssistantMessageEventStream} from '@earendil-works/pi-ai';
import {createCommandRunner} from ${JSON.stringify(processModule)};
import {registerWorkflow} from ${JSON.stringify(join(here,'workflow.js'))};
export default function(pi){
 const capture=value=>appendFileSync(process.env.WORKFLOW_CAPTURE,JSON.stringify(value)+'\\n');
 const mode=process.env.WORKFLOW_CASE;let calls=0,queued=false,boundary=0;
 pi.on('agent_before_settle',async(e,ctx)=>{
  boundary++;capture({kind:'boundary',boundary,outcome:e.outcome,continuing:e.continue,pending:e.context.pendingMessages.length});
  if(mode==='queued'&&!queued){queued=true;await pi.sendUserMessage('queued fixture input',{deliverAs:'followUp'});capture({kind:'queued',pending:ctx.hasPendingMessages()})}
 });
 const native=createCommandRunner({binary:process.env.CORVINT_BIN,timeoutMs:1700,maxBytes:8000});
 const runner={async run(req){const result=await native.run(req);capture({kind:'native',verb:req.args[1],event:req.args[req.args.indexOf('--event')+1],boundary,fault:result.fault,exit:result.exitCode});return result},close:()=>native.close()};
 registerWorkflow(pi,{runner,version:'0.99.1',notice:(_,code)=>capture({kind:'notice',code})});
 pi.on('agent_settled',()=>capture({kind:'settled'}));
 pi.registerProvider('corvint-workflow-fixture',{baseUrl:'http://127.0.0.1:1',apiKey:'fixture-only',api:'corvint-workflow-fixture',models:[{id:'fixture',name:'Offline workflow fixture',reasoning:false,input:['text'],cost:{input:0,output:0,cacheRead:0,cacheWrite:0},contextWindow:200000,maxTokens:1000}],streamSimple(model,ctx){
  calls++;capture({kind:'provider',calls,messages:ctx.messages});
  const stream=createAssistantMessageEventStream();const stopReason=mode==='error'?'error':mode==='aborted'?'aborted':'stop';
  const output={role:'assistant',content:[{type:'text',text:'fixture response'}],api:model.api,provider:model.provider,model:model.id,usage:{input:1,output:1,cacheRead:0,cacheWrite:0,totalTokens:2,cost:{input:0,output:0,cacheRead:0,cacheWrite:0,total:0}},stopReason,timestamp:Date.now(),...(stopReason!=='stop'?{errorMessage:'offline fixture terminal '+stopReason}:{})};
  queueMicrotask(()=>{stream.push(stopReason==='stop'?{type:'done',reason:stopReason,message:output}:{type:'error',reason:stopReason,error:output});stream.end()});return stream;
 }});
}`;

test('LCP-V0-008 actual Pi one remediation, recursive release, error/abort and queued boundary bypass',async t=>{
 const binary=process.env.CORVINT_PI_WORKFLOW_BIN;
 assert.ok(binary&&existsSync(binary),'set CORVINT_PI_WORKFLOW_BIN to the exact candidate Core binary');
 const processModule=resolve(process.env.CORVINT_PI_PROCESS_MODULE??join(here,'process.js'));
 assert.ok(existsSync(processModule),'shared process runner must be built before native qualification');
 const scratch=mkdtempSync(join(tmpdir(),'pi-workflow-host-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const extensionPath=join(scratch,'fixture.ts');writeFileSync(extensionPath,extension(processModule));
 for(const mode of ['completed','error','aborted','queued']){
  const base=join(scratch,mode),home=join(base,'home'),repo=join(base,'repo'),agent=join(home,'.pi/agent'),capture=join(base,'events.jsonl');
  for(const dir of [home,repo,agent])mkdirSync(dir,{recursive:true});
  writeFileSync(join(agent,'settings.json'),JSON.stringify({retry:{enabled:false},quietStartup:true}));
  writeFileSync(join(repo,'.gitignore'),'.corvint/\n');
  writeFileSync(join(repo,'intent.md'),'# Fixture intent\n\n## Requirements\n\n- `FIXTURE-001`: preserve fixture behavior.\n');
  writeFileSync(join(repo,'main.go'),'package fixture\n');
  const git=args=>{const r=spawnSync('git',['-c','user.name=Fixture','-c','user.email=fixture@example.invalid',...args],{cwd:repo,encoding:'utf8'});assert.equal(r.status,0,r.stderr);return r.stdout.trim()};
  git(['init','-q']);git(['add','.']);git(['commit','-qm','fixture']);
  const plan=join(base,'plan.json');writeFileSync(plan,JSON.stringify({base:git(['rev-parse','HEAD']),intents:['intent.md'],checks:[{id:'fixture-check',argv:['git','diff','--check'],timeoutSeconds:5}]}));
  const env={PATH:process.env.PATH,HOME:home,LANG:'en_US.UTF-8',PI_CODING_AGENT_DIR:agent,PI_OFFLINE:'1',CORVINT_BIN:binary,WORKFLOW_CAPTURE:capture,WORKFLOW_CASE:mode};
  const version=await run(process.env.PI_BIN??'pi',['--version'],{cwd:repo,env});assert.equal(version.stdout.trim(),'0.99.1');
  const result=await run(process.env.PI_BIN??'pi',['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--mode','json','--provider','corvint-workflow-fixture','--model','fixture','--thinking','off','-e',extensionPath,'-p','/corvint-workflow begin '+plan,'complete the offline fixture'],{cwd:repo,env});
  const events=readFileSync(capture,'utf8').trim().split('\n').map(JSON.parse);
  assert.ok(events.some(e=>e.kind==='native'&&e.verb==='begin'&&e.exit===0),JSON.stringify({mode,events,result}));
  assert.ok(!events.some(e=>e.kind==='notice'&&['invalid-workflow-response','native-event-unavailable'].includes(e.code)),JSON.stringify({mode,events,result}));
  const providers=events.filter(e=>e.kind==='provider'),stops=events.filter(e=>e.kind==='native'&&e.event==='stop');
  if(mode==='completed'){
   assert.equal(result.code,0,result.stderr);assert.equal(providers.length,2,JSON.stringify(events));assert.equal(stops.length,2);
   assert.equal(providers[1].messages.filter(m=>JSON.stringify(m).includes('corvint-workflow-remediation')||JSON.stringify(m).includes('explicitly enrolled Corvint workflow remains incomplete')).length,1);
  }else if(mode==='queued'){
   assert.equal(result.code,0,result.stderr);assert.equal(providers.length,3,JSON.stringify(events));
   assert.ok(events.some(e=>e.kind==='queued'),JSON.stringify(events));assert.ok(stops.every(e=>e.boundary!==1),'queued boundary must not consult local stop policy');
  }else{assert.equal(providers.length,1,JSON.stringify(events));assert.equal(stops.length,0,JSON.stringify(events));}
  assert.equal(events.filter(e=>e.kind==='settled').length,1,JSON.stringify(events));
  assert.ok(events.some(e=>e.kind==='notice'&&e.code==='local-policy-incomplete'),JSON.stringify(events));
 }
});
