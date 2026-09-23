// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,existsSync,rmSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {run} from './process.mjs';
const here=dirname(fileURLToPath(import.meta.url));
const binary=join(here,'build/release/pi-protected'),control=join(here,'build/node');
test('PPI-V0-002 hardened SEA rejects startup injection with live positive controls',async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-startup-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const data=join(scratch,'data'),home=join(scratch,'home'),marker=join(scratch,'injected'),preload=join(scratch,'preload.cjs');
 mkdirSync(data);mkdirSync(home);
 const env={PATH:'/usr/bin:/bin',HOME:home,TMPDIR:tmpdir()};
 writeFileSync(preload,`require('node:fs').writeFileSync(${JSON.stringify(marker)},'executed')`);
 const positive=await run(control,['--eval','0'],{env:{...env,NODE_OPTIONS:`--require=${preload}`}});
 assert.equal(positive.code,0,positive.stderr);assert.ok(existsSync(marker));rmSync(marker);
 const c=join(scratch,'canary.c'),dylib=join(scratch,'canary.dylib'),conf=join(scratch,'openssl.cnf');
 writeFileSync(c,`#include <stdio.h>\n__attribute__((constructor)) static void load(void){FILE *f=fopen(${JSON.stringify(marker)},"w");if(f){fputs("executed",f);fclose(f);}}\n`);
 const built=await run('/usr/bin/clang',['-dynamiclib',c,'-o',dylib]);assert.equal(built.code,0,built.stderr);
 writeFileSync(conf,`nodejs_conf = initialization\nopenssl_conf = initialization\n[initialization]\nproviders = provider_section\n[provider_section]\ncanary = canary_section\n[canary_section]\nmodule = ${dylib}\nactivate = 1\n`);
 for(const injected of [{DYLD_INSERT_LIBRARIES:dylib},{OPENSSL_CONF:conf}]){
  await run(control,['--eval','0'],{env:{...env,...injected}});
  assert.ok(existsSync(marker),'native injection control did not execute');rmSync(marker);
 }
 for(const injected of [{NODE_OPTIONS:`--require=${preload}`},{BUN_OPTIONS:`--preload=${preload}`,BUN_BE_BUN:'1'},{DYLD_INSERT_LIBRARIES:dylib},{OPENSSL_CONF:conf}]) {
  const result=await run(binary,['--mode','rpc','--data-dir',data],{env:{...env,...injected},cwd:scratch,onSpawn(child){child.stdin.end('{"type":"get_state","id":"ready"}\n')}});
  assert.equal(result.code,0,result.stderr);assert.match(result.stdout,/"id":"ready"/);assert.equal(existsSync(marker),false);
 }
 for(const args of [['--require',preload],['--eval',readFileSync(preload,'utf8')],['--node-options',`--require=${preload}`]]) {
  const result=await run(binary,args,{env,cwd:scratch});assert.notEqual(result.code,0);assert.equal(existsSync(marker),false);
 }
 const forged=await run(binary,['--mode','rpc','--data-dir',data],{argv0:`--require=${preload}`,env,cwd:scratch,onSpawn(child){child.stdin.end('{"type":"get_state","id":"ready"}\n')}});
 assert.equal(forged.code,0,forged.stderr);assert.match(forged.stdout,/"id":"ready"/);assert.equal(existsSync(marker),false);
 let buffered='',sent=false;
 const inspect=await run(binary,['--mode','rpc','--data-dir',data],{env,cwd:scratch,onSpawn(child){child.stdin.write('{"type":"get_state","id":"before"}\n')},onStdout(bytes,child){
  buffered+=bytes;if(!sent&&buffered.includes('"id":"before"')){sent=true;child.kill('SIGUSR1');child.stdin.write('{"type":"get_state","id":"after"}\n')}
  if(buffered.includes('"id":"after"'))child.stdin.end();
 }});
 assert.equal(inspect.code,0,inspect.stderr);assert.match(inspect.stdout,/"id":"after"/);assert.doesNotMatch(inspect.stderr,/Debugger listening/);
});

test('PPI-V0-004 interrupted harness reaps a signal-ignoring descendant',async t=>{
 const scratch=mkdtempSync(join(tmpdir(),'pi-cleanup-'));t.after(()=>rmSync(scratch,{recursive:true,force:true}));
 const fixture=join(scratch,'owner.mjs');
 const shell="trap '' TERM\nsleep 86400 &\necho $!\nwait";
 writeFileSync(fixture,`import {run} from ${JSON.stringify(new URL('./process.mjs',import.meta.url).href)};await run('/bin/sh',['-c',${JSON.stringify(shell)}],{onStdout(data){process.stdout.write(data)}});`);
 let descendant;
 const result=await run(process.execPath,[fixture],{onStdout(bytes,child){descendant=Number(bytes.toString().trim());if(Number.isSafeInteger(descendant)&&descendant>1)child.kill('SIGTERM')}});
 assert.equal(result.code,143);assert.ok(descendant>1);
 const until=Date.now()+2000;let alive=true;
 while(alive&&Date.now()<until){try{process.kill(descendant,0);await new Promise(r=>setTimeout(r,20))}catch(e){if(e.code==='ESRCH')alive=false;else throw e}}
 assert.equal(alive,false,'descendant survived owner interruption');
});
