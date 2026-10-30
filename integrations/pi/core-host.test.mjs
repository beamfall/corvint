// SPDX-License-Identifier: Apache-2.0
// Native read-chain witness (PWV-V0-012); this does not claim Pi host/OS support.
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {createHash} from 'node:crypto';
import {mkdtemp,mkdir,writeFile,readFile,readdir,lstat,readlink,rm,symlink} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createCommandRunner} from './process.js';
import {createCoreService} from './core.js';
import {createTasksService} from './tasks.js';
const exec=promisify(execFile);

// Every byte under the root, including .git, so an index refresh or store
// initialization by any read is observed rather than assumed absent.
async function snapshot(root) {
 const out={};
 async function walk(dir) {
  for(const name of (await readdir(dir)).sort()) {
   const path=join(dir,name),rel=path.slice(root.length+1),info=await lstat(path);
   if(info.isDirectory()){out[rel+'/']=String(info.mode);await walk(path)}
   else if(info.isSymbolicLink())out[rel]='link:'+await readlink(path);
   else out[rel]=info.mode+':'+createHash('sha256').update(await readFile(path)).digest('hex');
  }
 }
 await walk(root);return out;
}

test('PWV-V0-012 named task -> governing clause -> immutable source -> impact -> selected tests, without mutation',async t=>{
 const cwd=await mkdtemp(join(tmpdir(),'pi-core-native-'));t.after(()=>rm(cwd,{recursive:true,force:true}));
 const env={PATH:process.env.PATH,HOME:process.env.HOME,GIT_CONFIG_NOSYSTEM:'1'};
 const git=async args=>(await exec('git',args,{cwd,env,timeout:10000,maxBuffer:65536})).stdout.trim();
 await git(['init','-q','-b','main']);await git(['config','user.name','Fixture']);await git(['config','user.email','fixture@example.invalid']);
 await mkdir(join(cwd,'docs'));await mkdir(join(cwd,'calc'));
 await writeFile(join(cwd,'go.mod'),'module example.com/chain\n\ngo 1.22\n');
 await writeFile(join(cwd,'AGENTS.md'),'# Agents\n\nRounding behaviour is governed by docs/rounding.md.\n');
 await writeFile(join(cwd,'docs/rounding.md'),'# Rounding contract\n\n- `RND-001`: Round MUST round half away from zero.\n');
 await writeFile(join(cwd,'calc/round.go'),'package calc\n\n// Round implements RND-001.\nfunc Round(x float64) int { return int(x + 0.5) }\n');
 await writeFile(join(cwd,'calc/round_test.go'),'package calc\n\nimport "testing"\n\nfunc TestRound(t *testing.T) {\n\tif Round(2.5) != 3 {\n\t\tt.Fatal()\n\t}\n}\n');
 await git(['add','.']);await git(['commit','-qm','base']);
 const base=await git(['rev-parse','HEAD']);
 await writeFile(join(cwd,'calc/round.go'),'package calc\n\n// Round implements RND-001.\nfunc Round(x float64) int {\n\tif x < 0 {\n\t\treturn -Round(-x)\n\t}\n\treturn int(x + 0.5)\n}\n');
 await git(['commit','-qam','round negative halves away from zero']);
 const head=await git(['rev-parse','HEAD']),blob=async path=>git(['rev-parse',`HEAD:${path}`]);
 const runner=createCommandRunner({binary:process.env.CORVINT_BIN??'corvint',env,timeoutMs:60000,maxBytes:1048576});t.after(()=>runner.close());
 const core=createCoreService({runner}),ctx={cwd,isProjectTrusted:()=>true};
 const before=await snapshot(cwd);
 const ok=result=>{assert.equal(result.fault,undefined,JSON.stringify(result).slice(0,2000));assert.deepEqual(result.receipt,JSON.parse(result.raw.stdout),'native receipt is preserved verbatim');return result.receipt};

 const task='Fix Round in calc/round.go to satisfy the RND-001 rounding contract';
 const context=ok(await core.read('context',{task,limit:8},ctx));
 assert.equal(context.mutates,false);
 const governing=context.results.find(r=>r.kind==='governing');
 assert.equal(governing?.id,'AGENTS.md','the governing clause is the project instruction file');
 assert.equal(governing.evidence[0].authority,'project-instructions');
 assert.equal(governing.evidence[0].blob_hash,await blob('AGENTS.md'),'governing evidence is pinned to the immutable blob');
 const named=context.results.find(r=>r.id==='calc/round.go');
 assert.ok(named,'the named source is selected');
 assert.equal(named.evidence[0].blob_hash,await blob('calc/round.go'),'named source evidence is pinned to the immutable blob');
 assert.ok(context.coverage&&typeof context.coverage==='object','coverage and its unknowns stay visible');

 const impact=ok(await core.read('impact',{paths:['calc/round.go'],limit:5},ctx));
 assert.equal(impact.mutates,false);
 const tests=impact.context.results.filter(r=>r.kind==='test').map(r=>r.id);
 assert.deepEqual(tests,['calc/round_test.go']);
 assert.equal(impact.context.results.find(r=>r.id==='calc/round.go').evidence[0].blob_hash,await blob('calc/round.go'));

 const affected=ok(await core.read('affected',{base},ctx));
 assert.equal(affected.mutates,false);
 assert.equal(affected.revision,head);
 // The default affected document is the compact affected-plan/1 (AFP-V0-035):
 // each selection carries its test count; impact above names the test file.
 assert.equal(affected.profile,'affected-plan/1');
 assert.deepEqual(affected.plan.selected.map(s=>[s.unitId,s.testCount,s.witness.kind]),[['go:example.com/chain/calc',1,'DIRECT_SOURCE_CHANGE']]);
 assert.equal(affected.advice.status,'PLAN_ONLY','no selected test is executed');
 assert.ok(affected.advice.unknown.some(u=>u.startsWith('NO_REPOSITORY_GATE_DECLARED')),'missing gate evidence stays an explicit unknown');

 // Tasks inspection in a repository without a store must not initialize one.
 const tasksRunner=createCommandRunner({binary:process.env.CORVINT_TASKS_BIN??'corvint-tasks',env,timeoutMs:15000});t.after(()=>tasksRunner.close());
 const tasks=createTasksService({runner:tasksRunner});t.after(()=>tasks.close());
 const tctx={...ctx,sessionManager:{getSessionId:()=>'fixture-session'}};
 const help=await tasks.read('help',{},tctx);
 assert.equal(help.ok,true,JSON.stringify(help).slice(0,2000));
 assert.ok(Array.isArray(help.raw.items[0].implemented),'native capability inventory is read, not assumed');
 const queue=await tasks.read('queue',{},tctx);
 assert.ok(queue.raw||queue.fault,'native queue result or typed refusal is retained');

 // PWV-V0-013 and PWV-V0-014: the added native reads keep the same no-mutation contract.
 for(const operation of ['overview','features']) {
  const guide=await core.read(operation,{},ctx);
  assert.equal(guide.fault,undefined,JSON.stringify(guide).slice(0,2000));
  assert.deepEqual(guide.receipt,JSON.parse(guide.raw.stdout),`${operation} receipt is preserved verbatim`);
 }
 const witness=ok(await core.read('witness',{base,head},ctx));
 assert.equal(witness.profile,'corvint-witness/0');
 // AGW-V0-007: a head other than the checked-out HEAD is compiled at that commit and bound COMMIT-PINNED, never re-pointed.
 const pinned=ok(await core.read('witness',{base,head:base},ctx));
 assert.equal(pinned.range.head,base);assert.equal(pinned.range.headBinding,'COMMIT-PINNED');
 assert.equal(witness.range.headBinding,undefined,'the checked-out head carries no head binding');
 // A head that does not descend from --base is refused natively.
 const reversed=await core.read('witness',{base:head,head:base},ctx);
 assert.equal(reversed.fault,'native-command-failed');assert.equal(reversed.receipt,undefined);
 assert.equal(reversed.raw.exitCode,2);assert.match(reversed.raw.stderr,/ancestor/);

 assert.deepEqual(await snapshot(cwd),before,'reads leave worktree, Git directory and absent task store byte-identical');
 assert.equal(Object.keys(before).some(p=>p.startsWith('.taskman')||p.startsWith('.git/taskman')||p.startsWith('.corvint')),false);
});

// PWV-V0-013: without a store, native search and list refuse UNINITIALIZED and initialize nothing.
// Successful reads on an initialized store are qualified in tasks-host.test.mjs.
test('PWV-V0-013 search and list refuse UNINITIALIZED in a repository without a task store, without mutation',async t=>{
 const cwd=await mkdtemp(join(tmpdir(),'pi-tasks-absent-'));t.after(()=>rm(cwd,{recursive:true,force:true}));
 const env={PATH:process.env.PATH,HOME:process.env.HOME,GIT_CONFIG_NOSYSTEM:'1'};
 const git=async args=>(await exec('git',args,{cwd,env,timeout:10000,maxBuffer:65536})).stdout.trim();
 await git(['init','-q','-b','main']);await git(['config','user.name','Fixture']);await git(['config','user.email','fixture@example.invalid']);
 await writeFile(join(cwd,'README.md'),'fixture\n');await git(['add','.']);await git(['commit','-qm','base']);
 const runner=createCommandRunner({binary:process.env.CORVINT_TASKS_BIN??'corvint-tasks',env,timeoutMs:15000});t.after(()=>runner.close());
 const tasks=createTasksService({runner});t.after(()=>tasks.close());
 const ctx={cwd,isProjectTrusted:()=>true,sessionManager:{getSessionId:()=>'fixture-session'}};
 const before=await snapshot(cwd);
 for(const [operation,input] of [['search',{text:'round',limit:5,summary:true}],['list',{limit:5}]]) {
  const result=await tasks.read(operation,input,ctx);
  assert.equal(result.ok,false);assert.equal(result.fault,'native-refusal');
  assert.equal(result.raw.outcome,'REFUSED');assert.deepEqual(result.raw.codes,['UNINITIALIZED'],JSON.stringify(result.raw).slice(0,2000));
 }
 assert.deepEqual(await snapshot(cwd),before,'refused reads leave the repository byte-identical');
 assert.equal(Object.keys(await snapshot(cwd)).some(p=>p.startsWith('.taskman')||p.startsWith('.git/taskman')),false);
});

// PWV-V0-014 and AGW-V0-011: the witness evidence input reaches the native
// --evidence-in-repo admission; a document inside the checkout is read and
// reported as a source, a link that leaves the checkout is refused natively with
// a named reason, and a path the facade does not admit never reaches the binary.
test('PWV-V0-014 witness evidence is read only from inside the checkout, without mutation',async t=>{
 const cwd=await mkdtemp(join(tmpdir(),'pi-core-evidence-'));t.after(()=>rm(cwd,{recursive:true,force:true}));
 const outside=await mkdtemp(join(tmpdir(),'pi-core-evidence-outside-'));t.after(()=>rm(outside,{recursive:true,force:true}));
 const env={PATH:process.env.PATH,HOME:process.env.HOME,GIT_CONFIG_NOSYSTEM:'1'};
 const git=async args=>(await exec('git',args,{cwd,env,timeout:10000,maxBuffer:65536})).stdout.trim();
 await git(['init','-q','-b','main']);await git(['config','user.name','Fixture']);await git(['config','user.email','fixture@example.invalid']);
 await writeFile(join(cwd,'README.md'),'fixture\n');await git(['add','.']);await git(['commit','-qm','base']);
 const base=await git(['rev-parse','HEAD']);
 await writeFile(join(cwd,'README.md'),'fixture, changed\n');await git(['commit','-qam','change']);
 const head=await git(['rev-parse','HEAD']);
 await writeFile(join(cwd,'.git/info/exclude'),'.corvint/\n');
 await mkdir(join(cwd,'.corvint'));
 const projection=JSON.stringify({schema:'corvint-test-validity/0',source:'corvint-go-test-provider',tests:[]});
 await writeFile(join(cwd,'.corvint/run.json'),projection);await writeFile(join(outside,'run.json'),projection);
 await symlink(join(outside,'run.json'),join(cwd,'.corvint/escape.json'));
 const runner=createCommandRunner({binary:process.env.CORVINT_BIN??'corvint',env,timeoutMs:60000,maxBytes:1048576});t.after(()=>runner.close());
 const core=createCoreService({runner}),ctx={cwd,isProjectTrusted:()=>true};
 const before=await snapshot(cwd);

 const read=await core.read('witness',{base,head,evidence:['.corvint/run.json']},ctx);
 assert.equal(read.fault,undefined,JSON.stringify(read).slice(0,2000));
 const source=read.receipt.sources.find(s=>s.name==='evidence');
 assert.equal(source.path,'.corvint/run.json','the source names the repository-relative path as given');
 assert.equal(source.status,'UNBOUND');assert.match(source.detail,/carries no Playwright receipt or RSpec provider document/);
 const escape=await core.read('witness',{base,head,evidence:['.corvint/escape.json']},ctx);
 assert.equal(escape.fault,'native-command-failed');assert.equal(escape.receipt,undefined);
 assert.equal(escape.raw.exitCode,2);assert.match(escape.raw.stderr,/--evidence-in-repo \.corvint\/escape\.json: resolves through a symbolic link to .*, outside the checkout/);
 for(const [path,reason] of [['../run.json',/\.\./],[join(outside,'run.json'),/absolute/]]) {
  const refused=await core.read('witness',{base,head,evidence:[path]},ctx);
  assert.equal(refused.fault,'invalid-input');assert.match(refused.reason,reason);assert.equal(refused.raw,undefined,'a refused path never reaches the binary');
 }
 assert.deepEqual(await snapshot(cwd),before,'evidence reads leave worktree and Git directory byte-identical');
});
