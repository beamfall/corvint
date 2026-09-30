// SPDX-License-Identifier: AGPL-3.0-or-later
// Private replay metadata only. Native Tasks remains the sole task-store writer.
import { createHash } from 'node:crypto';
import { constants } from 'node:fs';
import { mkdir, open, readdir, lstat, unlink, realpath } from 'node:fs/promises';
import { join, isAbsolute } from 'node:path';
import { createCommandRunner } from './process.js';
import { workflowIdentity } from './workflow.js';
export const digest=value=>createHash('sha256').update(typeof value==='string'?value:canonical(value)).digest('hex');
export function canonical(value) {
 if(Array.isArray(value))return '['+value.map(canonical).join(',')+']';
 if(value&&typeof value==='object')return '{'+Object.keys(value).sort().map(k=>JSON.stringify(k)+':'+canonical(value[k])).join(',')+'}';
 return JSON.stringify(value);
}
const identityArgs=['rev-parse','--show-toplevel','--absolute-git-dir','HEAD','--symbolic-full-name','HEAD'];
const filesystemIdentity=i=>({root:i.root,gitDir:i.gitDir,rootDevice:i.rootDevice,rootInode:i.rootInode,gitDevice:i.gitDevice,gitInode:i.gitInode,markerDevice:i.markerDevice,markerInode:i.markerInode});
export function createWorktreeIdentity({runner,realpathImpl=realpath}={}) {
 if(!runner){const env={GIT_CONFIG_NOSYSTEM:'1'};for(const key of ['PATH','HOME','LANG','LC_ALL','TMPDIR'])if(process.env[key]!==undefined)env[key]=process.env[key];runner=createCommandRunner({binary:'git',env,timeoutMs:1700,maxBytes:16384})}
 const pending=new Set();let generation=0,closed=false;
 async function identify(ctx) {
  if(closed)throw Error('identity-closed');
  if(!ctx.isProjectTrusted?.())throw Error('untrusted-project');
  if(ctx.signal?.aborted)throw Error('aborted');
  const epoch=generation,cwd=ctx.cwd,session=ctx.sessionManager?.getSessionId(),signal=ctx.signal;
  if(typeof session!=='string'||!session||!session.isWellFormed()||Buffer.byteLength(session)>4096)throw Error('session-unavailable');
  const frozen={...ctx,cwd,sessionManager:{getSessionId:()=>session}};
  const check=()=>{if(closed||epoch!==generation||cwd!==ctx.cwd||session!==ctx.sessionManager?.getSessionId()||!ctx.isProjectTrusted?.())throw Error('stale-context');if(signal?.aborted||ctx.signal?.aborted)throw Error('aborted')};
  const beforeIdentity=workflowIdentity(frozen),before=filesystemIdentity(beforeIdentity);check();
  const result=await runner.run({cwd,args:[...identityArgs],signal});check();
  if(result.fault||result.exitCode!==0||typeof result.stdout!=='string'||Buffer.byteLength(result.stdout)>16384||!result.stdout.endsWith('\n'))throw Error('identity-unavailable');
  const lines=result.stdout.slice(0,-1).split('\n');
  if(lines.length!==4||lines.some(v=>!v||v.includes('\0')||v.includes('\r'))||!isAbsolute(lines[0])||!isAbsolute(lines[1])||! /^[a-f0-9]{40}([a-f0-9]{24})?$/.test(lines[2])||!(lines[3]==='HEAD'||/^refs\/heads\/[^\x00-\x20\x7f]+$/.test(lines[3])))throw Error('identity-unavailable');
  check();const [root,gitDir]=await Promise.all([realpathImpl(lines[0]),realpathImpl(lines[1])]);check();
  const afterIdentity=workflowIdentity(frozen),after=filesystemIdentity(afterIdentity);check();
  const headMarker=lines[3]==='HEAD'?lines[2]:'ref: '+lines[3];
  if(root!==before.root||gitDir!==before.gitDir||canonical(before)!==canonical(after)||beforeIdentity.head.trim()!==headMarker||afterIdentity.head.trim()!==headMarker)throw Error('stale-context');
  // HEAD is a native Git observation. Filesystem HEAD text cannot prove a later
  // same-branch commit; callers revalidate exact OIDs at their dispatch boundary.
  return {...after,branch:lines[3]==='HEAD'?'HEAD':lines[3].slice('refs/heads/'.length),detached:lines[3]==='HEAD',head:lines[2],sessionSha256:digest(session),filesystemSha256:digest(after)};
 }
 function read(ctx){const job=identify(ctx);pending.add(job);job.then(()=>pending.delete(job),()=>pending.delete(job));return job}
 async function cancel(){generation++;const owned=[...pending];await runner.cancel?.();await Promise.allSettled(owned)}
 async function close(){closed=true;await cancel();await runner.close?.()}
 return {read,cancel,close};
}
export async function worktreeIdentity(ctx) {
 const owner=createWorktreeIdentity();try{return await owner.read(ctx)}finally{await owner.close()}
}
function validTerminal(v) {
 return v&&Object.keys(v).sort().join(',')==='exitCode,fault,nativeOutcome,ok'&&typeof v.ok==='boolean'&&['OK','REFUSED','ERROR','NOT_RUN'].includes(v.nativeOutcome)&&(v.exitCode===null||Number.isInteger(v.exitCode))&&v.fault===(v.ok?null:'native-refusal')&&v.ok===(v.nativeOutcome==='OK'&&v.exitCode===0);
}
async function load(path) {
 const h=await open(path,constants.O_RDONLY|constants.O_NOFOLLOW);
 try {if((await h.stat()).size>4096)throw Error('ledger-invalid');const v=JSON.parse(await h.readFile('utf8'));if(v.profile!=='corvint-pi-operation/0'||! /^[a-f0-9]{64}$/.test(v.intentSha256)||! /^[a-f0-9]{64}$/.test(v.scopeSha256)||typeof v.requestId!=='string')throw Error('ledger-invalid');if(Object.hasOwn(v,'terminal')&&!validTerminal(v.terminal))throw Error('ledger-invalid');return v}finally{await h.close()}
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
  async finish(entry,receipt,terminal) {
   if(terminal!==undefined&&!validTerminal(terminal))throw Error('ledger-invalid');
   const prior=await load(active);if(prior.intentSha256!==entry.intentSha256||prior.requestId!==entry.requestId)throw Error('ledger-invalid');
   // No raw payload, command output, or transcript persists here.
   await durableCreate(join(dir,digest(entry.requestId)+'.json'),{...prior,receiptSha256:digest(receipt),...(terminal===undefined?{}:{terminal})});
   try{await unlink(active)}catch(e){if(e.code!=='ENOENT')throw e}
   const d=await open(dir,constants.O_RDONLY);try{await d.sync()}finally{await d.close()}
  },
 };
}
