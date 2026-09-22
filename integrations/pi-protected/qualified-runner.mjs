// SPDX-License-Identifier: AGPL-3.0-or-later
import {spawn,execFile} from 'node:child_process';
import {lstatSync,readFileSync} from 'node:fs';
import {dirname,join} from 'node:path';
import {createHash} from 'node:crypto';
import {decodeObject} from '../pi/runtime.js';

const profile='corvint-qualified-lifecycle/2',digest=/^[0-9a-f]{64}$/;
const keys=['profile','ok','mutates','event','requestProvenance','requestSha256','support','qualification','degradations','qualifiedHost','repository','policy','completion','decision','authority','frontier','resultDigest'];
const closed=(value,keys)=>value&&typeof value==='object'&&!Array.isArray(value)&&JSON.stringify(Object.keys(value).sort())===JSON.stringify([...keys].sort());
const hash=value=>createHash('sha256').update(value).digest('hex');
// Go's canonical map ordering uses UTF-8 bytes, including for non-ASCII keys.
function canonical(value) {
 if(Array.isArray(value))return '['+value.map(canonical).join(',')+']';
 if(value&&typeof value==='object')return '{'+Object.keys(value).sort((a,b)=>Buffer.compare(Buffer.from(a),Buffer.from(b))).map(key=>JSON.stringify(key)+':'+canonical(value[key])).join(',')+'}';
 return JSON.stringify(value);
}
export function decodeQualified(raw,event,requestSHA256) {
 if(Buffer.byteLength(raw)>8000)throw Error('qualified-output-bound');
 const value=decodeObject(raw),expected=[...keys,...(['session-start','user-prompt'].includes(event)?['context']:[])].sort();
 if(JSON.stringify(Object.keys(value).sort())!==JSON.stringify(expected)||value.profile!==profile||value.event!==event||value.ok!==true||value.mutates!==false||value.requestProvenance!=='caller-asserted'||!digest.test(value.requestSha256)||!/^qualified-lifecycle:sha256:[0-9a-f]{64}$/.test(value.resultDigest))throw Error('invalid-qualified-response');
 if(requestSHA256&&value.requestSha256!==requestSHA256)throw Error('qualified-request-mismatch');
 const {resultDigest,...basis}=value;
 if(resultDigest!=='qualified-lifecycle:sha256:'+hash(profile+'\0'+canonical(basis)))throw Error('qualified-result-mismatch');
 const h=value.qualifiedHost;
 if(!closed(h,['host','digest','evidenceSHA256','adapterSHA256','osBuild','architecture','supportScope','eventSurface','qualifiedSurfaces','hostSHA256','runtimeAdmissionEvidenceSHA256'])||h.host!=='pi'||h.eventSurface!=='unattributed'||h.architecture!=='arm64'||typeof h.osBuild!=='string'||!h.osBuild||!Array.isArray(h.qualifiedSurfaces)||!Array.isArray(value.degradations)||h.qualifiedSurfaces.some(s=>!closed(s,['surface','evidenceSHA256'])))throw Error('invalid-qualified-host');
 for(const name of ['hostSHA256','runtimeAdmissionEvidenceSHA256','adapterSHA256'])if(!digest.test(h[name]))throw Error('invalid-qualified-host');
 if(value.support==='FULL') {
  if(value.qualification!=='QUALIFIED'||h.supportScope!=='qualified-protected-pi-runtime'||!digest.test(h.digest)||!digest.test(h.evidenceSHA256)||h.qualifiedSurfaces.length!==2||h.qualifiedSurfaces[0].surface!=='pi-tui'||h.qualifiedSurfaces[1].surface!=='pi-rpc'||h.qualifiedSurfaces.some(s=>!digest.test(s.evidenceSHA256)))throw Error('invalid-qualified-host');
 }else if(value.support!=='FALLBACK'||value.qualification!=='UNQUALIFIED'||h.supportScope!=='candidate-protected-pi-runtime'||h.digest!==''||h.evidenceSHA256!==''||h.qualifiedSurfaces.length!==0||value.degradations[0]!=='native-tuple-unqualified')throw Error('invalid-qualified-host');
 for(const decision of [value.decision,value.completion])if(!closed(decision,['decision','reason'])||!['block','release'].includes(decision.decision)||typeof decision.reason!=='string'||!decision.reason)throw Error('invalid-qualified-decision');
 if(!closed(value.frontier,['state','universeSHA256','decision','reason'])||!['block','release'].includes(value.frontier.decision)||typeof value.frontier.reason!=='string'||!value.frontier.reason)throw Error('invalid-qualified-frontier');
 if(!closed(value.repository,['commitRevision','treeRevision','objectFormat','worktreeState','dirtyPathCount','dirtyPathsSha256'])||!closed(value.policy,['lifecycle','satisfied','unmet','base','target','planDigest','reportSetDigest']))throw Error('invalid-qualified-response');
 const codes=value.degradations.slice(value.support==='FALLBACK'?1:0);
 if(new Set(codes).size!==codes.length||codes.some(c=>event!=='session-start'||!['compaction-critical-evidence-overflow','compaction-dirty-set-over-budget','compaction-untracked-paths-not-rehydratable'].includes(c)))throw Error('invalid-qualified-degradations');
 if(event==='stop') {
  if(value.authority!=='VERIFIED'||!['OPEN','EMPTY'].includes(value.frontier.state)||!digest.test(value.frontier.universeSHA256)||(value.frontier.state==='EMPTY'&&value.frontier.decision!=='release'))throw Error('invalid-qualified-frontier');
 }else if(value.authority!=='NONE'||value.frontier.state!=='NOT_EVALUATED'||value.frontier.universeSHA256!==''||value.frontier.decision!=='release'||value.frontier.reason!=='not-stop-event'||[value.decision,value.completion].some(d=>d.decision!=='release'||d.reason!=='not-stop-event'))throw Error('invalid-qualified-frontier');
 if(value.context&&(value.context.profile!=='corvint-dogfood-prompt/0'||raw.includes('END CORVINT REPOSITORY DATA')))throw Error('invalid-qualified-context');
 const context=value.context?`BEGIN CORVINT REPOSITORY DATA\n${raw.trim()}\nEND CORVINT REPOSITORY DATA`:'';
 if(Buffer.byteLength(context)>8000)throw Error('qualified-output-bound');
 return {fault:null,qualified:value,receiptId:value.resultDigest,context,degradations:codes,shouldContinue:event==='stop'&&value.decision.decision==='block'};
}

export async function assertProtectedConsumer(binary,sha,signal) {
 if(!/^\/Library\/CorvintAuthority\/versions\/[0-9a-f]{64}\/corvint$/.test(binary)||!digest.test(sha))throw Error('protected-runtime-uninstalled');
 const paths=[];
 for(let path=binary;;path=dirname(path)) {
  const stat=lstatSync(path);
  if(stat.uid!==0||(stat.mode&0o022)!==0||stat.isSymbolicLink()||(path===binary?(!stat.isFile()||stat.nlink!==1||stat.size>256*1024*1024):!stat.isDirectory()))throw Error('unsafe-protected-consumer');
  paths.push(path);if(path==='/')break;
 }
 // Node exposes ownership/modes but not macOS ACLs. Read only fixed-path OS metadata.
 await new Promise((resolve,reject)=>execFile('/bin/ls',['-lde',...paths],{env:{PATH:'/usr/bin:/bin',LC_ALL:'C'},timeout:500,maxBuffer:16384,signal,killSignal:'SIGKILL'},(error,stdout)=>{
  const lines=stdout.trimEnd().split('\n');
  if(error||lines.length!==paths.length||lines.some(line=>line.slice(0,12).includes('+')))reject(Error('unsafe-protected-consumer'));else resolve();
 }));
 signal?.throwIfAborted();
 if(createHash('sha256').update(readFileSync(binary)).digest('hex')!==sha)throw Error('protected-consumer-drift');
}

export function createQualifiedRunner({binary,consumerSHA256,spawnImpl=spawn,admit=assertProtectedConsumer,env=process.env}) {
 const pending=new Set();
 async function run({cwd,event,input,signal}) {
  if(!['session-start','user-prompt','stop','session-end'].includes(event)||signal?.aborted)return {fault:'qualified-lifecycle-unavailable'};
  const controller=new AbortController();const abort=()=>controller.abort();signal?.addEventListener('abort',abort,{once:true});pending.add(abort);
  const timer=setTimeout(abort,1700);
  try {
   await admit(binary,consumerSHA256,controller.signal);
   controller.signal.throwIfAborted();
   const request=canonical({profile,event,input});if(Buffer.byteLength(request)>131072)throw Error('input-bound');
   const childEnv={PATH:join(dirname(binary),'bin')};for(const k of ['HOME','LANG','LC_ALL','TMPDIR'])if(env[k]!==undefined)childEnv[k]=env[k];
   return await new Promise(resolve=>{
    let child,output=[],size=0,diagnostics=0,failed=false,closed=false,exit,settled=false,poll;
    const alive=()=>{if(!child?.pid)return false;try{process.kill(-child.pid,0);return true}catch(e){return e.code!=='ESRCH'}};
    const finish=()=>{
     if(settled)return;settled=true;clearTimeout(poll);controller.signal.removeEventListener('abort',kill);
     if(failed||exit!==0||controller.signal.aborted){resolve({fault:'qualified-lifecycle-unavailable'});return}
     try{resolve(decodeQualified(new TextDecoder('utf-8',{fatal:true}).decode(Buffer.concat(output)),event,hash(request)))}catch{resolve({fault:'invalid-qualified-response'})}
    };
    const check=()=>{clearTimeout(poll);if(closed&&!alive())finish();else poll=setTimeout(check,15)};
    const kill=()=>{failed=true;if(child?.pid)try{process.kill(-child.pid,'SIGKILL')}catch{}check()};
    controller.signal.addEventListener('abort',kill,{once:true});
    try{child=spawnImpl(binary,['qualified-event','--input','-'],{cwd,env:childEnv,detached:true,stdio:['pipe','pipe','pipe']})}catch{controller.signal.removeEventListener('abort',kill);resolve({fault:'qualified-core-unavailable'});return}
    child.on('error',()=>{failed=true;closed=true;if(alive())kill();else finish()});
    child.stdout.on('data',bytes=>{size+=bytes.length;if(size>8000)kill();else output.push(bytes)});
    child.stderr.on('data',bytes=>{diagnostics+=bytes.length;if(diagnostics>4096)kill()});
    child.stdin.on('error',()=>{});child.stdin.end(request);
    child.on('close',code=>{closed=true;exit=code;if(alive())kill();else finish()});
    if(controller.signal.aborted)kill();
   });
  }catch{return {fault:'qualified-lifecycle-unavailable'}}
  finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);pending.delete(abort)}
 }
 return {run,async close(){for(const cancel of pending)cancel();while(pending.size)await new Promise(r=>setTimeout(r,10))}};
}
