// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync,existsSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {run,ownNativeGroup} from './process.mjs';
import {readSettings,guardAuth,guardedAuthBackend,createCredentials,parseArguments} from './settings.mjs';
const here=dirname(fileURLToPath(import.meta.url));
const binary=join(here,'build/release/pi-protected');

test('PPI-V0-003 configuration rejects executable resources and shell credentials',async()=>{
 const dir=mkdtempSync(join(tmpdir(),'pi-settings-'));
 try {
  const file=join(dir,'settings.json');
  const valid={provider:'fixture',model:'fixture',endpoint:{api:'openai-completions',baseUrl:'http://127.0.0.1:8000',contextWindow:20000,maxTokens:1000}};
  writeFileSync(file,JSON.stringify(valid));assert.deepEqual(readSettings(file),valid);
  for(const value of [{extensions:['x']},{...valid,endpoint:{...valid.endpoint,baseUrl:'http://example.com'}},{...valid,endpoint:{...valid.endpoint,baseUrl:123}},{...valid,endpoint:{...valid.endpoint,apiKey:'!touch /tmp/never'}}]) {
   writeFileSync(file,JSON.stringify(value));assert.throws(()=>readSettings(file));
  }
  writeFileSync(file,'{"theme":"dark","theme":"light"}');assert.throws(()=>readSettings(file));
  assert.throws(()=>guardAuth('{"fixture":{"type":"api_key","key":"!echo bad"}}'));
  assert.equal(guardAuth('{"fixture":{"type":"api_key","key":"literal"}}'),'{"fixture":{"type":"api_key","key":"literal"}}');
  let written=false;
  const backend=guardedAuthBackend({withLock(fn){const value=fn('{}');written=!!value.next;return value.result}});
  assert.throws(()=>backend.withLock(()=>({next:'{"fixture":{"type":"api_key","key":"!bad"}}'})));assert.equal(written,false);
  let stored='{}';
  const credentials=createCredentials({async withLockAsync(fn){const value=await fn(stored);if(value.next!==undefined)stored=value.next;return value.result}});
  await assert.rejects(credentials.modify('fixture',async()=>({type:'api_key',key:'!bad'})));assert.equal(stored,'{}');
  await assert.rejects(credentials.modify('fixture',async()=>({type:'api_key',key:'literal',env:{AWS_PROFILE:'unsafe'}})));assert.equal(stored,'{}');
  await credentials.modify('fixture',async()=>({type:'api_key',key:'literal$HOME'}));
  assert.equal((await credentials.read('fixture')).key,'literal$HOME');
  assert.deepEqual(await credentials.list(),[{providerId:'fixture',type:'api_key'}]);
  await credentials.delete('fixture');assert.equal(await credentials.read('fixture'),undefined);
  for(const argv of [['constructor','x'],['toString','x'],['__proto__','x'],['--require','x'],['--mode','print'],['--mode','rpc','--mode','tui'],['--data-dir']])assert.throws(()=>parseArguments(argv));
 }finally{rmSync(dir,{recursive:true,force:true})}
});

test('PPI-V0-001 PPI-V0-002 native RPC uses embedded SDK with hostile global resources',async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-protected-host-'));
 t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const repo=join(scratch,'repo'),home=join(scratch,'home'),dataDir=join(scratch,'data');
 for(const path of [repo,home,dataDir,join(repo,'.pi/extensions'),join(home,'.pi/agent/extensions')])mkdirSync(path,{recursive:true});
 const marker=join(scratch,'injected'),canary=`require('node:fs').writeFileSync(${JSON.stringify(marker)},'injected')`;
 const preload=join(scratch,'preload.cjs');writeFileSync(preload,canary);
 for(const path of [join(repo,'.pi/extensions/hostile.js'),join(home,'.pi/agent/extensions/hostile.js')])writeFileSync(path,canary);
 writeFileSync(join(repo,'.pi/settings.json'),JSON.stringify({extensions:[preload],packages:[scratch]}));
 writeFileSync(join(home,'.pi/agent/settings.json'),JSON.stringify({extensions:[preload],packages:[scratch]}));
 writeFileSync(join(repo,'.gitignore'),'.corvint/\n.context-corvint/\n');
 writeFileSync(join(repo,'AGENTS.md'),'# Fixture\nUse evidence from main.go.\n');
 writeFileSync(join(repo,'go.mod'),'module fixture.local/example\n\ngo 1.27.1\n');
 writeFileSync(join(repo,'main.go'),'package example\nfunc One() int { return 1 }\n');
 // A 3001×1 solid PNG forces the embedded worker to resize without flooding RPC output.
 writeFileSync(join(repo,'fixture.png'),Buffer.from('iVBORw0KGgoAAAANSUhEUgAAC7kAAAABCAIAAAAudx9XAAAAH0lEQVR4nO3BAQEAAACCIP+vbkhAAQAAAAAAAADAhwEjLAAB5nWy6QAAAABJRU5ErkJggg==','base64'));
 writeFileSync(join(repo,'image-resize-worker.js'),canary);
 for(const args of [['init','-q'],['add','.'],['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','fixture']])assert.equal((await run('/usr/bin/git',args,{cwd:repo})).code,0);
 const requests=[];let imageIssued=false;
 const server=createServer((req,res)=>{
  let body='';req.on('data',chunk=>{body+=chunk;if(body.length>2**20)req.destroy()});
  req.on('end',()=>{
   const request=JSON.parse(body);requests.push(request);
   const imageCall=!imageIssued&&request.messages.some(m=>m.role==='user'&&JSON.stringify(m.content).includes('read fixture image'));
   if(imageCall)imageIssued=true;
   res.writeHead(200,{'Content-Type':'text/event-stream'});
   const delta=imageCall?{role:'assistant',tool_calls:[{index:0,id:'read-image',type:'function',function:{name:'read',arguments:JSON.stringify({path:'fixture.png'})}}]}:{role:'assistant',content:'PROTECTED_NATIVE_PI_OK_'+requests.length};
   res.write('data: '+JSON.stringify({id:'fixture',object:'chat.completion.chunk',model:'fixture',choices:[{index:0,delta,finish_reason:null}]})+'\n\n');
   res.write('data: '+JSON.stringify({id:'fixture',object:'chat.completion.chunk',model:'fixture',choices:[{index:0,delta:{},finish_reason:imageCall?'tool_calls':'stop'}],usage:{prompt_tokens:1,completion_tokens:1,total_tokens:2}})+'\n\n');
   res.end('data: [DONE]\n\n');
  });
 });
 t.after(()=>{server.closeAllConnections();server.close()});
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)});
 writeFileSync(join(dataDir,'settings.json'),JSON.stringify({provider:'fixture',model:'fixture',thinkingLevel:'off',endpoint:{api:'openai-completions',baseUrl:`http://127.0.0.1:${server.address().port}/v1`,contextWindow:200000,maxTokens:1000,input:['text','image']}}));
 writeFileSync(join(dataDir,'auth.json'),JSON.stringify({fixture:{type:'api_key',key:'fixture-only'}}));
 const env={PATH:'/usr/bin:/bin',HOME:home,TMPDIR:tmpdir(),TERM:'xterm-256color',NODE_OPTIONS:`--require=${preload}`,BUN_OPTIONS:`--preload=${preload}`,BUN_BE_BUN:'1',PI_PACKAGE_DIR:scratch,PI_CODING_AGENT_DIR:join(home,'.pi/agent')};
 // The native host cannot read either global or build-time SDK modules during this run.
 const profile=`(version 1)(allow default)(deny file-read* (subpath ${JSON.stringify(join(here,'node_modules'))}) (subpath ${JSON.stringify(join(process.env.HOME,'.bun/install/global/node_modules'))}))`;
 const events=[];let buffered='',settled=0;
 const result=await run('/usr/bin/sandbox-exec',['-p',profile,binary,'--mode','rpc','--data-dir',dataDir],{cwd:repo,env,onSpawn(c){c.stdin.write('{"id":"ready","type":"get_state"}\n')},onStdout(chunk,c){
  buffered+=chunk;
  for(;;){const i=buffered.indexOf('\n');if(i<0)break;const line=buffered.slice(0,i);buffered=buffered.slice(i+1);if(!line)continue;const e=JSON.parse(line);events.push(e);
   if(e.id==='ready')c.stdin.write('{"id":"first","type":"prompt","message":"inspect main.go"}\n');
   if(e.type==='agent_settled'&&++settled===1)c.stdin.write('{"id":"new","type":"new_session"}\n');
   if(e.id==='new')c.stdin.write('{"id":"second","type":"prompt","message":"inspect One"}\n');
   if(e.type==='agent_settled'&&settled===2)c.stdin.end();
  }
 }});
 assert.equal(result.code,0,result.stderr);assert.equal(settled,2,JSON.stringify(events));assert.equal(requests.length,2,JSON.stringify(events));
 assert.ok(events.some(e=>e.id==='new'&&e.success));assert.doesNotMatch(result.stderr,/Corvint unavailable|Failed to load/);
 for(const request of requests)assert.match(JSON.stringify(request.messages),/BEGIN CORVINT REPOSITORY DATA/);
 assert.equal(existsSync(marker),false);
 const witness=join(scratch,'tui-pgid'),releaseWitness=ownNativeGroup(witness);
 try {
  const tui=await run('/usr/bin/python3',[join(here,'tui-fixture.py'),'/usr/bin/sandbox-exec','-p',profile,binary,'--mode','tui','--data-dir',dataDir],{cwd:repo,env:{...env,CORVINT_PI_PTY_WITNESS:witness,CORVINT_PI_TUI_FIRST:'3'}});
  assert.equal(tui.code,0,tui.stderr);assert.match(tui.stdout,/clean shutdown passed/);
 }finally{releaseWitness()}
 assert.equal(requests.length,5);
 for(const request of requests)assert.match(JSON.stringify(request.messages),/BEGIN CORVINT REPOSITORY DATA/);
 assert.equal(existsSync(marker),false);
 let imageOutput='';
 const resized=await run('/usr/bin/sandbox-exec',['-p',profile,binary,'--mode','rpc','--data-dir',dataDir],{cwd:repo,env,onSpawn(c){c.stdin.write('{"type":"prompt","message":"read fixture image"}\n')},onStdout(chunk,c){imageOutput+=chunk;if(imageOutput.includes('"type":"agent_settled"'))c.stdin.end()}});
 assert.equal(resized.code,0,resized.stderr);assert.match(imageOutput,/"toolName":"read"/);assert.match(imageOutput,/"type":"image"/);assert.doesNotMatch(imageOutput,/external-module-unavailable|image-resize-failed|native-addon-unavailable/);
 assert.ok(requests.some(request=>JSON.stringify(request.messages).includes('data:image/png;base64,')),'native image must reach the provider');
 assert.equal(existsSync(marker),false);
});
