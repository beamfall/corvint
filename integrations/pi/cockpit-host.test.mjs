// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {spawnSync,spawn} from 'node:child_process';
import {mkdtempSync,writeFileSync,readFileSync,existsSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {registerCockpit} from './cockpit.js';

const globalRoot='/Users/russelllewis/.bun/install/global/node_modules';

test('PWV cockpit component runs against installed Pi 0.99.1 and host-provided pi-tui APIs',async t=>{
 let coding,tui;
 try{
  coding=await import(`file://${globalRoot}/@earendil-works/pi-coding-agent/dist/index.js`);
  tui=await import(`file://${globalRoot}/@earendil-works/pi-tui/dist/index.js`);
 }catch(error){t.skip(`installed native Pi unavailable: ${error.message}`);return;}
 assert.equal(coding.VERSION,'0.99.1');
 for(const name of ['matchesKey','truncateToWidth','visibleWidth'])assert.equal(typeof tui[name],'function');
 const commands=new Map(),handlers=new Map();
 const pi={registerCommand:(n,c)=>commands.set(n,c),on:(n,h)=>handlers.set(n,h),sendMessage(){}};
 registerCockpit(pi,{core:{read:async operation=>({operation,raw:{stdout:'immutable source reason: exact selector\nauthority: accepted decision'},receipt:'native-host'})},tasks:{read:async operation=>({operation,raw:{stdout:'queue read only'}})},uiKit:tui});
 const ctx={mode:'tui',hasUI:true,cwd:process.cwd(),sessionManager:{getSessionId:()=> 'native'},ui:{custom:async factory=>{
  let completed;const host={requestRender(){}};
  const component=await factory(host,{fg:(_c,s)=>s,bold:s=>s},null,value=>{completed=value});
  assert.ok(component.render(30).every(line=>tui.visibleWidth(line)<=30));
  component.handleInput('r');await new Promise(resolve=>setImmediate(resolve));
  assert.match(component.render(100).join('\n'),/immutable source reason/);
  component.handleInput('\x1b');assert.equal(completed.profile,'corvint-cockpit/0');
 }}};
 await commands.get('corvint').handler('cockpit native host',ctx);
 assert.ok(handlers.has('session_compact'));assert.ok(handlers.has('session_tree'));
});

test('installed Pi 0.99.1 RPC host invokes the user /corvint command offline with structured fallback',()=>{
 const scratch=mkdtempSync(join(tmpdir(),'corvint-cockpit-host-'));
 try {
  const extension=join(scratch,'cockpit-extension.mjs');
  const moduleUrl=pathToFileURL(new URL('./cockpit.js',import.meta.url).pathname).href;
  writeFileSync(extension,`import {registerCockpit} from ${JSON.stringify(moduleUrl)};\nexport default function(pi){registerCockpit(pi,{core:{read:async operation=>({operation,receipt:'native-rpc',raw:{stdout:'authority: repository-owned\\nreason: immutable selector'}})},tasks:{read:async operation=>({operation,raw:{stdout:'tasks read'}})}})}\n`);
  const run=spawnSync('/Users/russelllewis/.bun/bin/pi',['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--no-session','--mode','rpc','-e',extension],{
   cwd:scratch,env:{...process.env,PI_CODING_AGENT_DIR:join(scratch,'agent')},input:'{"id":"commands","type":"get_commands"}\n{"id":"cockpit","type":"prompt","message":"/corvint native-host"}\n',encoding:'utf8',timeout:15000,maxBuffer:2**20
  });
  assert.equal(run.status,0,run.stderr);
  const events=run.stdout.trim().split('\n').map(line=>JSON.parse(line));
  const commands=events.find(e=>e.id==='commands');
  assert.equal(commands.success,true);assert.ok(commands.data.commands.some(command=>command.name==='corvint'&&command.source==='extension'));
  const response=events.find(e=>e.id==='cockpit');assert.equal(response.success,true);
  assert.ok(events.some(e=>JSON.stringify(e).includes('corvint-cockpit/0')),'structured cockpit result missing from native RPC events');
  assert.ok(events.some(e=>JSON.stringify(e).includes('native-rpc')),'native Core receipt missing from RPC result');
  for(const mode of ['json','text']) {
   const args=['--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--no-session','-e',extension,'--print','/corvint native-host'];
   if(mode==='json')args.splice(args.length-2,0,'--mode','json');
   const noninteractive=spawnSync('/Users/russelllewis/.bun/bin/pi',args,{cwd:scratch,env:{...process.env,PI_CODING_AGENT_DIR:join(scratch,`agent-${mode}`)},encoding:'utf8',timeout:15000,maxBuffer:2**20});
   assert.equal(noninteractive.status,0,noninteractive.stderr);
   assert.doesNotMatch(noninteractive.stdout,/\x1b/,'noninteractive cockpit emitted terminal controls');
  }
 } finally {rmSync(scratch,{recursive:true,force:true});}
});

test('installed Pi 0.99.1 native TUI qualifies cockpit matrix and interruption through a real PTY',async()=>{
 const scratch=mkdtempSync(join(tmpdir(),'corvint-cockpit-tui-'));
 try {
  const extension=join(scratch,'cockpit-extension.mjs'),fixture=join(scratch,'pi-tui-fixture');
  const moduleUrl=pathToFileURL(new URL('./cockpit.js',import.meta.url).pathname).href;
  writeFileSync(extension,`import * as uiKit from '@earendil-works/pi-tui';
import {writeFileSync} from 'node:fs';
import {spawn} from 'node:child_process';
import {registerCockpit} from ${JSON.stringify(moduleUrl)};
export default function(pi){
 pi.on('session_start',async(_e,ctx)=>{
  const result=ctx.ui.setTheme(process.env.CORVINT_TEST_THEME);
  if(!result.success)throw Error('native theme unavailable');
  writeFileSync(process.env.CORVINT_TEST_THEME_WITNESS,process.env.CORVINT_TEST_THEME);
  if(process.env.CORVINT_TEST_CHILD){const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'});writeFileSync(process.env.CORVINT_TEST_CHILD,String(child.pid));}
 });
 const read=async operation=>({operation,receipt:'native-'+operation,raw:{stdout:['INJECT'+String.fromCharCode(27)+'[31m'+String.fromCharCode(7,13,155),'LONG-'+ 'x'.repeat(20000)+'LONG-END',...Array.from({length:80},(_,i)=>'ROW-'+String(i).padStart(3,'0'))].join('\\n')}});
 registerCockpit(pi,{core:{read},tasks:{read},uiKit});
}
`);
  const build=spawnSync('go',['build','-o',fixture,'./tools/pi-tui-fixture'],{cwd:new URL('../..',import.meta.url).pathname,env:{...process.env,GOCACHE:join(scratch,'go-cache')},encoding:'utf8',timeout:30000});
  assert.equal(build.status,0,build.stderr);
  const args=['/Users/russelllewis/.bun/bin/pi','--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--no-session','-e',extension];
  const environment=theme=>({...process.env,PI_CODING_AGENT_DIR:join(scratch,'agent-'+theme),CORVINT_PI_TUI_MATRIX:'1',CORVINT_TEST_THEME:theme,CORVINT_TEST_THEME_WITNESS:join(scratch,'theme-'+theme)});
  for(const theme of ['dark','light']) {
   const run=spawnSync(fixture,args,{cwd:scratch,env:environment(theme),encoding:'utf8',timeout:30000,maxBuffer:2**20});
   assert.equal(run.status,0,run.stderr);assert.match(run.stdout,/cockpit matrix: resize 40\/120, five views/);
   assert.match(run.stdout,/Native Pi TUI prompt and clean shutdown passed/);
   assert.equal(readFileSync(join(scratch,'theme-'+theme),'utf8'),theme);
  }
  const witness=join(scratch,'group'),childWitness=join(scratch,'child');
  const interrupted=spawn(fixture,args,{cwd:scratch,env:{...environment('dark'),CORVINT_PI_PTY_WITNESS:witness,CORVINT_TEST_CHILD:childWitness},stdio:'ignore'});
  const ended=new Promise(resolve=>interrupted.once('exit',(code,signal)=>resolve({code,signal})));
  try {
   const deadline=Date.now()+10000;
   while(!existsSync(childWitness)&&Date.now()<deadline)await new Promise(resolve=>setTimeout(resolve,25));
   assert.ok(existsSync(childWitness),'actual Pi descendant never started');
   const group=Number(readFileSync(witness,'utf8')),child=Number(readFileSync(childWitness,'utf8'));
   interrupted.kill('SIGTERM');
   const result=await Promise.race([ended,new Promise((_,reject)=>setTimeout(()=>reject(Error('interruption timeout')),4000).unref())]);
   assert.equal(result.code,143);
   const retired=pid=>{try{process.kill(pid,0);return false}catch(error){assert.equal(error.code,'ESRCH');return true}};
   const retirementDeadline=Date.now()+2000;
   while(!retired(child)&&Date.now()<retirementDeadline)await new Promise(resolve=>setTimeout(resolve,25));
   assert.ok(retired(child),'Pi descendant survived fixture interruption');
   assert.ok(retired(-group),'PTY process group survived fixture interruption');
  } finally {if(interrupted.exitCode===null)interrupted.kill('SIGTERM');await ended;}

 } finally {rmSync(scratch,{recursive:true,force:true});}
});
