// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import { fork } from 'node:child_process';
import { once } from 'node:events';
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, symlinkSync, utimesSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHostScratch, sweepHostScratch } from './host-scratch.mjs';

// Fixtures use IPC readiness, never a wait for a filesystem side effect.
async function fixture(t,root) {
 const path=join(root,'child.mjs');
 const module=new URL('./host-scratch.mjs',import.meta.url).href;
 writeFileSync(path,`import {createHostScratch,installHostScratchCleanup} from ${JSON.stringify(module)}; installHostScratchCleanup(); const path=createHostScratch(process.argv[2]); process.send(path); process.on('message',()=>process.exit(0));`);
 const child=fork(path,[root],{stdio:['ignore','pipe','pipe','ipc']});
 t.after(()=>{if(child.exitCode===null&&child.signalCode===null)child.kill('SIGKILL')});
 const [scratch]=await once(child,'message');return {child,scratch};
}

test('LVT-V0-009 scratch is removed on normal exit and signals',async t=>{
 const root=mkdtempSync(join(tmpdir(),'pi-scratch-test-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 for(const signal of [null,'SIGINT','SIGTERM','SIGHUP']) {
  const {child,scratch}=await fixture(t,root);assert.ok(existsSync(join(scratch,'.harness-owner.json')));
  const exited=once(child,'exit');if(signal)child.kill(signal);else child.send('exit');
  const [code]=await exited;assert.equal(code,signal?{SIGINT:130,SIGTERM:143,SIGHUP:129}[signal]:0);
  assert.equal(existsSync(scratch),false);
 }
});

test('LVT-V0-009 sweep recovers killed scratch and preserves ambiguous or live owners',async t=>{
 const root=mkdtempSync(join(tmpdir(),'pi-sweep-test-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 const {child,scratch}=await fixture(t,root), exited=once(child,'exit');child.kill('SIGKILL');await exited;
 assert.ok(existsSync(scratch),'SIGKILL leaves scratch for recovery');
 const now=Date.now(),old=now-2*86400000;
 const age=path=>{const file=join(path,'.harness-owner.json');const owner=JSON.parse(readFileSync(file));owner.created=old;writeFileSync(file,JSON.stringify(owner));utimesSync(path,old/1000,old/1000)};
 age(scratch);
 const live=createHostScratch(root);age(live);
 const malformed=createHostScratch(root);age(malformed);writeFileSync(join(malformed,'.harness-owner.json'),'invalid');
 const unmarked=mkdtempSync(join(root,'corvint-pi-host-'));utimesSync(unmarked,old/1000,old/1000);
 const publicDir=createHostScratch(root);age(publicDir);chmodSync(publicDir,0o755);
 const link=join(root,'corvint-pi-host-link');symlinkSync(scratch,link);
 const markerLink=createHostScratch(root);age(markerLink);rmSync(join(markerLink,'.harness-owner.json'));symlinkSync(join(scratch,'.harness-owner.json'),join(markerLink,'.harness-owner.json'));utimesSync(markerLink,old/1000,old/1000);
 const fresh=createHostScratch(root);
 const ambiguous=[];
 if(process.platform==='linux') {
  for(const namespace of [undefined,'pid:[0]']) {
   const path=createHostScratch(root);age(path);
   const file=join(path,'.harness-owner.json'),owner=JSON.parse(readFileSync(file));
   owner.pid=child.pid;owner.pidNamespace=namespace;writeFileSync(file,JSON.stringify(owner));
   ambiguous.push(path);
  }
 }
 assert.deepEqual(sweepHostScratch(root,now),[scratch]);
 for(const path of [live,malformed,unmarked,publicDir,markerLink,fresh,...ambiguous])assert.ok(existsSync(path),path);
 assert.equal(sweepHostScratch(root,now).length,0);
});
