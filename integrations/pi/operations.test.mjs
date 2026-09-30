// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,mkdir,writeFile,readFile,readdir} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createOperationLedger,digest} from './operations.js';
const intent={requestId:'one',intentSha256:digest('input'),scopeSha256:digest('scope'),issuedAt:'2026-09-30T00:00:00Z'};
async function setup(t){const dir=await mkdtemp(join(tmpdir(),'pi-ledger-'));t.after(()=>rm(dir,{recursive:true,force:true}));return {dir,ledger:createOperationLedger(dir)}}
test('PWV private ledger persists before dispatch, deduplicates and retains only digests',async t=>{
 const {dir,ledger}=await setup(t);const entry=await ledger.begin(intent);assert.equal((await ledger.inspect()).requestId,'one');
 await assert.rejects(ledger.begin(intent),/reconciliation-required/);
 await assert.rejects(ledger.begin({...intent,requestId:'two'}),/pending-operation/);
 await assert.rejects(ledger.begin({...intent,intentSha256:digest('changed')},true),/request-id-conflict/);
 const resume=await createOperationLedger(dir).begin(intent,true);assert.equal(resume.resume,true);
 await ledger.finish(entry,{private:'not persisted'});
 assert.equal(await ledger.inspect(),null);assert.equal((await ledger.begin(intent)).completed,true);
 const bytes=await readFile(join(dir,'corvint-pi-operations',digest('one')+'.json'),'utf8');assert.doesNotMatch(bytes,/not persisted/);
});
test('PWV interrupted persistence refuses reuse and simultaneous requests dispatch at most once',async t=>{
 const {dir,ledger}=await setup(t);await mkdir(join(dir,'corvint-pi-operations'));await writeFile(join(dir,'corvint-pi-operations','pending.json'),'{');
 await assert.rejects(ledger.begin(intent,true));await assert.rejects(ledger.inspect());
 await rm(join(dir,'corvint-pi-operations'),{recursive:true});
 const results=await Promise.allSettled([ledger.begin(intent),ledger.begin({...intent,requestId:'two'})]);
 assert.equal(results.filter(r=>r.status==='fulfilled').length,1);assert.equal((await readdir(join(dir,'corvint-pi-operations'))).length,1);
});
test('PWV inspect is a read and does not initialize a ledger',async t=>{const {dir,ledger}=await setup(t);assert.equal(await ledger.inspect(),null);assert.deepEqual(await readdir(dir),[])});
test('PWV recovery retires a pending marker only with its durable matching terminal record',async t=>{
 const {dir,ledger}=await setup(t);const entry=await ledger.begin(intent);await ledger.finish(entry,{receipt:'native'});
 await writeFile(join(dir,'corvint-pi-operations','pending.json'),JSON.stringify(entry));
 assert.equal((await ledger.begin(intent,true)).completed,true);assert.equal(await ledger.inspect(),null);
});
