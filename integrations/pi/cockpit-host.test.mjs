// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdtempSync,writeFileSync,rmSync} from 'node:fs';
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

test('installed Pi 0.99.1 native TUI opens, refreshes and closes /corvint through the repository PTY fixture',()=>{
 const scratch=mkdtempSync(join(tmpdir(),'corvint-cockpit-tui-'));
 try {
  const extension=join(scratch,'cockpit-extension.mjs'),fixture=join(scratch,'pi-tui-fixture');
  const moduleUrl=pathToFileURL(new URL('./cockpit.js',import.meta.url).pathname).href;
  writeFileSync(extension,`import * as uiKit from '@earendil-works/pi-tui';\nimport {registerCockpit} from ${JSON.stringify(moduleUrl)};\nexport default function(pi){registerCockpit(pi,{core:{read:async operation=>({operation,receipt:'native-tui-receipt',raw:{stdout:'authority: repository-owned'}})},tasks:{read:async operation=>({operation,raw:{stdout:'tasks read'}})},uiKit})}\n`);
  const build=spawnSync('go',['build','-o',fixture,'./tools/pi-tui-fixture'],{cwd:new URL('../..',import.meta.url).pathname,env:{...process.env,GOCACHE:join(scratch,'go-cache')},encoding:'utf8',timeout:30000});
  assert.equal(build.status,0,build.stderr);
  const run=spawnSync(fixture,['/Users/russelllewis/.bun/bin/pi','--offline','--approve','--no-context-files','--no-skills','--no-prompt-templates','--no-themes','--no-tools','--no-session','-e',extension],{
   cwd:scratch,env:{...process.env,PI_CODING_AGENT_DIR:join(scratch,'agent'),CORVINT_PI_TUI_COMMAND:'/corvint native-tui',CORVINT_PI_TUI_EXPECT:'native-tui-receipt'},encoding:'utf8',timeout:30000,maxBuffer:2**20
  });
  assert.equal(run.status,0,run.stderr);assert.match(run.stdout,/Native Pi TUI prompt and clean shutdown passed/);
 } finally {rmSync(scratch,{recursive:true,force:true});}
});
