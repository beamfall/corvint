// SPDX-License-Identifier: AGPL-3.0-or-later
import { spawn } from 'node:child_process';
import { isAbsolute } from 'node:path';
export const EVENTS = new Set(['session-start','user-prompt','file-change','post-tool','stop','session-end']);
const KEYS=['profile','event','host','surface','hostVersion','adapterVersion','support','receiptId','context','degradations','fault','shouldContinue'].sort();
const CODES=new Set(['invalid-input','unsupported-event','unsupported-host-version','core-unavailable','invalid-core-response','output-too-large','deadline','untrusted-project']);
const DEGRADATIONS=new Set(['compaction-critical-evidence-overflow','compaction-dirty-set-over-budget','compaction-untracked-paths-not-rehydratable','frontier-authority-unavailable','outcome-persistence-unavailable']);
export function validEnvelope(v,event) {
 if(!v || typeof v!=='object' || Array.isArray(v) || JSON.stringify(Object.keys(v).sort())!==JSON.stringify(KEYS) || v.profile!=='corvint-pi-adapter/0' || v.event!==event || v.host!=='pi'||v.surface!=='extension'||v.adapterVersion!=='0.1.0'||v.support!=='FALLBACK'||v.shouldContinue!==false||typeof v.context!=='string'||!Array.isArray(v.degradations)||new Set(v.degradations).size!==v.degradations.length||!v.degradations.every(x=>DEGRADATIONS.has(x)))return false;
 if(v.fault!==null)return CODES.has(v.fault)&&v.receiptId===null&&v.context===''&&v.degradations.length===0&&(v.hostVersion==='0.85.1'||(v.hostVersion===null&&['invalid-input','unsupported-event','unsupported-host-version','deadline'].includes(v.fault)));
 if(v.context!=='' && (!['session-start','user-prompt'].includes(event)||!v.context.startsWith('BEGIN CORVINT REPOSITORY DATA\n')||!v.context.endsWith('\nEND CORVINT REPOSITORY DATA')))return false;
 return v.hostVersion==='0.85.1'&&typeof v.receiptId==='string'&&/^harness-receipt:sha256:[0-9a-f]{64}$/.test(v.receiptId);
}
export function decodeEnvelope(raw) {
 const value=JSON.parse(raw);
 // JSON.parse discards duplicate keys; retain the closed top-level key count.
 const tokens=raw.match(/"(?:[^"\\]|\\.)*"|[{}\[\]:,]|[^\s{}\[\]:,]+/g)??[];
 let depth=0,count=0;
 for(let i=0;i<tokens.length;i++){const t=tokens[i];if(t==='{'||t==='[')depth++;else if(t==='}'||t===']')depth--;else if(depth===1&&t.startsWith('"')&&tokens[i+1]===':')count++;}
 if(count!==KEYS.length)throw Error('closed envelope keys');
 return value;
}
export function createRunner({binary,env=process.env,spawnImpl=spawn}={}) {
 let conflict=false;
 if(binary===undefined){binary=env.CORVINT_BIN;conflict=binary!==undefined&&!binary;}
 if(binary!==undefined&&(!binary||!isAbsolute(binary)))conflict=true;
 const pending=new Set();
 async function run({cwd,event,input,hostVersion,signal}) {
  if(conflict)return {fault:'invalid-binary-config'};
  if(!binary)return {fault:'missing-binary'};
  if(process.platform==='win32')return {fault:'cleanup-failed'};
  if(signal?.aborted)return {fault:'aborted'};
  let bytes;try{bytes=JSON.stringify({hostVersion,input});}catch{return {fault:'invalid-input'}}
  if(Buffer.byteLength(bytes)>131072)return {fault:'invalid-input'};
  const allowed={};for(const key of ['PATH','HOME','LANG','LC_ALL','TMPDIR'])if(env[key]!==undefined)allowed[key]=env[key];
  return await new Promise(resolve=>{
   let child,reason,stdout=[],size=0,stderrSize=0,closed=false,exit,settled=false,deadline,killTimer,poll,hard;
   const alive=()=>{if(!child?.pid)return false;try{process.kill(-child.pid,0);return true}catch(e){return e.code!=='ESRCH'}};
   const send=sig=>{if(child?.pid)try{process.kill(-child.pid,sig)}catch{}};
   const finish=()=>{
    if(settled)return;settled=true;for(const t of [deadline,killTimer,poll,hard])clearTimeout(t);signal?.removeEventListener('abort',abort);pending.delete(cancel);
    if(reason)return resolve({fault:reason});
    if(exit!==0)return resolve({fault:'core-unavailable'});
    try{const raw=new TextDecoder('utf-8',{fatal:true}).decode(Buffer.concat(stdout));const v=decodeEnvelope(raw);resolve(validEnvelope(v,event)?v:{fault:'invalid-adapter-response'});}catch{resolve({fault:'invalid-adapter-response'})}
   };
   const check=()=>{clearTimeout(poll);if(closed&&!alive())finish();else poll=setTimeout(check,15)};
   const cancel=(code='aborted')=>{if(settled)return;reason??=code;send('SIGTERM');if(!killTimer)killTimer=setTimeout(()=>send('SIGKILL'),100);check();};
   const abort=()=>cancel();pending.add(cancel);signal?.addEventListener('abort',abort,{once:true});
   try{child=spawnImpl(binary,['adapter','pi',event],{cwd,env:allowed,shell:false,detached:true,stdio:['pipe','pipe','pipe']});}catch{reason='missing-binary';closed=true;finish();return;}
   deadline=setTimeout(()=>cancel('deadline'),1700);
   hard=setTimeout(()=>{reason='cleanup-failed';send('SIGKILL');check();},2000);
   child.on('error',()=>{reason='missing-binary';closed=true;if(!alive())finish();else cancel(reason)});
   child.stdout.on('data',chunk=>{size+=chunk.length;if(size>8000)cancel('output-too-large');else stdout.push(chunk)});
   child.stderr.on('data',chunk=>{stderrSize+=chunk.length;if(stderrSize>4096)cancel('output-too-large')});
   child.on('close',code=>{closed=true;exit=code;if(alive())cancel('cleanup-failed');else finish()});
   child.stdin.on('error',()=>{});child.stdin.end(bytes);
  });
 }
 return {run,async close(){for(const cancel of pending)cancel();while(pending.size)await new Promise(r=>setTimeout(r,10));return;}};
}
