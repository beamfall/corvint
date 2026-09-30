// SPDX-License-Identifier: AGPL-3.0-or-later
// Private replay metadata only. Native Tasks remains the sole task-store writer.
import { createHash } from 'node:crypto';
import { constants } from 'node:fs';
import { mkdir, open, readdir, lstat, unlink, realpath } from 'node:fs/promises';
import { join } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
const exec=promisify(execFile);
export const digest=value=>createHash('sha256').update(typeof value==='string'?value:canonical(value)).digest('hex');
export function canonical(value) {
 if(Array.isArray(value))return '['+value.map(canonical).join(',')+']';
 if(value&&typeof value==='object')return '{'+Object.keys(value).sort().map(k=>JSON.stringify(k)+':'+canonical(value[k])).join(',')+'}';
 return JSON.stringify(value);
}
export async function worktreeIdentity(ctx) {
 const git=async args=>(await exec('git',args,{cwd:ctx.cwd,timeout:2000,maxBuffer:8192,env:{PATH:process.env.PATH,HOME:process.env.HOME,GIT_CONFIG_NOSYSTEM:'1'}})).stdout.trim();
 const root=await realpath(await git(['rev-parse','--show-toplevel']));
 const gitDir=await realpath(await git(['rev-parse','--absolute-git-dir']));
 const branch=await git(['symbolic-ref','--short','HEAD']);
 const head=await git(['rev-parse','HEAD']);
 const session=ctx.sessionManager?.getSessionId();
 if(typeof session!=='string'||!session)throw Error('session-unavailable');
 return {root,gitDir,branch,head,sessionSha256:digest(session)};
}
async function load(path) {
 const h=await open(path,constants.O_RDONLY|constants.O_NOFOLLOW);
 try {if((await h.stat()).size>4096)throw Error('ledger-invalid');const v=JSON.parse(await h.readFile('utf8'));if(v.profile!=='corvint-pi-operation/0'||! /^[a-f0-9]{64}$/.test(v.intentSha256)||! /^[a-f0-9]{64}$/.test(v.scopeSha256)||typeof v.requestId!=='string')throw Error('ledger-invalid');return v}finally{await h.close()}
}
// Exclusive creation and fsync precede dispatch. An interrupted/partial write fails
// closed on readback; it is never interpreted as permission to issue another ID.
async function durableCreate(path,value) {
 const h=await open(path,constants.O_WRONLY|constants.O_CREAT|constants.O_EXCL|constants.O_NOFOLLOW,0o600);
 try{await h.writeFile(canonical(value)+'\n');await h.sync()}finally{await h.close()}
 const d=await open(join(path,'..'),constants.O_RDONLY);try{await d.sync()}finally{await d.close()}
}
export function createOperationLedger(gitDir) {
 const dir=join(gitDir,'corvint-pi-operations'),active=join(dir,'pending.json');
 async function prepare() {
  await mkdir(dir,{mode:0o700});
 }
 async function directory() {
  try{await prepare()}catch(e){if(e.code!=='EEXIST')throw e}
  const s=await lstat(dir);if(!s.isDirectory()||s.isSymbolicLink())throw Error('ledger-invalid');
 }
 return {
  async inspect(){try{return await load(active)}catch(e){if(e.code==='ENOENT')return null;throw e}},
  async begin({requestId,intentSha256,scopeSha256,issuedAt,argvSha256},resume=false) {
   await directory();
   const entry={profile:'corvint-pi-operation/0',requestId,intentSha256,scopeSha256,issuedAt,...(argvSha256?{argvSha256}:{})};
   const done=join(dir,digest(requestId)+'.json');
   try {
    const prior=await load(done);if(prior.intentSha256!==intentSha256||prior.scopeSha256!==scopeSha256)throw Error('request-id-conflict');
    // A crash after the terminal fsync but before unlink leaves both files.
    // Only the matching durable terminal record can retire that pending marker.
    try{const pending=await load(active);if(pending.requestId===requestId&&pending.intentSha256===intentSha256&&pending.scopeSha256===scopeSha256){await unlink(active);const d=await open(dir,constants.O_RDONLY);try{await d.sync()}finally{await d.close()}}}catch(e){if(e.code!=='ENOENT')throw e}
    return {...prior,completed:true};
   }catch(e){if(e.code!=='ENOENT')throw e}
   if((await readdir(dir)).length>=128){try{await load(active)}catch(e){if(e.code==='ENOENT')throw Error('ledger-full');throw e}}
   try{await durableCreate(active,entry);return {...entry,resume:false}}catch(e){if(e.code!=='EEXIST')throw e}
   const prior=await load(active);
   if(prior.requestId!==requestId)throw Error('pending-operation');
   if(prior.intentSha256!==intentSha256||prior.scopeSha256!==scopeSha256)throw Error('request-id-conflict');
   if(!resume)throw Error('reconciliation-required');
   return {...prior,resume:true};
  },
  async finish(entry,receipt) {
   const prior=await load(active);if(prior.intentSha256!==entry.intentSha256||prior.requestId!==entry.requestId)throw Error('ledger-invalid');
   // No raw payload, command output, or transcript persists here.
   await durableCreate(join(dir,digest(entry.requestId)+'.json'),{...prior,receiptSha256:digest(receipt)});
   try{await unlink(active)}catch(e){if(e.code!=='ENOENT')throw e}
   const d=await open(dir,constants.O_RDONLY);try{await d.sync()}finally{await d.close()}
  },
 };
}
