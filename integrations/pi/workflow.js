// SPDX-License-Identifier: AGPL-3.0-or-later
import { createHash } from 'node:crypto';
import { realpathSync } from 'node:fs';
import { decodeObject } from './runtime.js';

const PROFILE='corvint-dogfood-event/0';
const TUPLE={host:'pi',hostVersion:'0.99.1',surface:'extension',adapterVersion:'0.3.0'};
const sha=value=>createHash('sha256').update(value).digest('hex');
const object=v=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const keys=(v,names)=>object(v)&&Object.keys(v).sort().join('\0')===[...names].sort().join('\0');
const hex=(v,n=64)=>typeof v==='string'&&new RegExp(`^[0-9a-f]{${n}}$`).test(v);
const states=new Set(['inactive','active','satisfied','cancelled']);
const unmetCodes=new Set(['worktree-owner-mismatch','uncommitted-work','selected-check-unverified','reports-not-produced','report-set-stale','review-required','final-check-required','policy-condition-unavailable']);

// Sort UTF-8 keys explicitly, including numeric-looking keys; JSON.stringify's object
// enumeration alone does not implement the native canonical envelope encoding.
export function canonicalWorkflowJSON(value,depth=0) {
 if(depth>64)throw Error('invalid-workflow-envelope');
 if(value===null||typeof value==='boolean')return JSON.stringify(value);
 if(typeof value==='string'){if(!value.isWellFormed())throw Error('invalid-workflow-envelope');return JSON.stringify(value)}
 if(typeof value==='number'){if(!Number.isFinite(value)||(Number.isInteger(value)&&!Number.isSafeInteger(value)))throw Error('invalid-workflow-envelope');return JSON.stringify(value)}
 if(Array.isArray(value))return '['+value.map(v=>canonicalWorkflowJSON(v,depth+1)).join(',')+']';
 if(!object(value))throw Error('invalid-workflow-envelope');
 return '{'+Object.keys(value).sort((a,b)=>Buffer.compare(Buffer.from(a),Buffer.from(b))).map(k=>canonicalWorkflowJSON(k)+':'+canonicalWorkflowJSON(value[k],depth+1)).join(',')+'}';
}
export function workflowSessionKey(session) {
 if(typeof session!=='string'||!session||!session.isWellFormed()||Buffer.byteLength(session)>4096)throw Error('missing-session-identity');
 return sha('corvint-local-completion-session/pi/0\0'+session);
}
export function decodeWorkflowEnvelope(raw,event,input) {
 if(!['stop','session-start'].includes(event)||typeof raw!=='string'||Buffer.byteLength(raw)>8000)throw Error('invalid-workflow-envelope');
 const v=decodeObject(raw), expected=['profile','ok','mutates','support','event','adapter','repository','requestSha256','degradations','frontier','policy','completion','resultDigest'];
 if(event==='session-start')expected.push('context');
 if(!keys(v,expected)||v.profile!==PROFILE||v.ok!==true||v.mutates!==false||v.support!=='FALLBACK'||v.event!==event||!keys(v.adapter,Object.keys(TUPLE))||Object.entries(TUPLE).some(([k,x])=>v.adapter[k]!==x))throw Error('invalid-workflow-envelope');
 const {resultDigest,...basis}=v;
 if(resultDigest!=='dogfood-event:sha256:'+sha(PROFILE+'\0'+canonicalWorkflowJSON(basis))||v.requestSha256!==sha(canonicalWorkflowJSON(input)))throw Error('invalid-workflow-binding');
 if(!keys(v.frontier,['state','shouldContinue','reason'])||v.frontier.state!=='UNAVAILABLE'||v.frontier.shouldContinue!==false||v.frontier.reason!=='frontier-authority-unavailable')throw Error('invalid-workflow-envelope');
 if(!Array.isArray(v.degradations)||!v.degradations.includes('frontier-authority-unavailable')||new Set(v.degradations).size!==v.degradations.length||v.degradations.some(x=>!['frontier-authority-unavailable','compaction-critical-evidence-overflow','compaction-dirty-set-over-budget','compaction-untracked-paths-not-rehydratable'].includes(x)))throw Error('invalid-workflow-envelope');
 const r=v.repository,n=r?.objectFormat==='sha1'?40:r?.objectFormat==='sha256'?64:0;
 if(!keys(r,['commitRevision','treeRevision','objectFormat','worktreeState','dirtyPathCount','dirtyPathsSha256'])||!n||!hex(r.commitRevision,n)||!hex(r.treeRevision,n)||!['clean','dirty'].includes(r.worktreeState)||!Number.isSafeInteger(r.dirtyPathCount)||r.dirtyPathCount<0||!hex(r.dirtyPathsSha256))throw Error('invalid-workflow-envelope');
 const p=v.policy;
 if(!keys(p,['lifecycle','satisfied','unmet','base','target','planDigest','reportSetDigest'])||!states.has(p.lifecycle)||typeof p.satisfied!=='boolean'||!Array.isArray(p.unmet)||p.unmet.some(x=>!unmetCodes.has(x))||['base','target','planDigest','reportSetDigest'].some(k=>typeof p[k]!=='string')||(p.satisfied&&(p.lifecycle!=='satisfied'||p.unmet.length)))throw Error('invalid-workflow-envelope');
 const c=v.completion,owner=Object.hasOwn(c??{},'owner');
 if(!keys(c,owner?['decision','reason','owner']:['decision','reason'])||(owner&&!hex(c.owner)))throw Error('invalid-workflow-envelope');
 let decision='release',reason=event==='stop'?'local-policy-'+p.lifecycle:'not-stop-event';
 if(event==='stop'&&(p.lifecycle==='active'||p.lifecycle==='satisfied'&&!p.satisfied)){decision=input.stopHookActive?'release':'block';reason=input.stopHookActive?'local-policy-continuation-limit':'local-policy-incomplete'}
 if(owner){if(event!=='stop'||p.lifecycle!=='inactive'||p.satisfied)throw Error('invalid-workflow-envelope');decision='release';reason='local-policy-other-session-active'}
 if(c.decision!==decision||c.reason!==reason||event==='session-start'&&!object(v.context))throw Error('invalid-workflow-envelope');
 return v;
}

export function registerWorkflow(pi,{runner,version,notice=(ctx,code)=>{const text=`Corvint workflow unresolved: ${code}. Frontier authority remains unavailable.`;if(ctx.hasUI)ctx.ui.notify(text,'warning');else process.stderr.write(text+'\n')}}) {
 let generation=0,used=false,unresolved=false,knownIncomplete=false,recovery,closed=false,scope,activitySession;
 const pending=new Set(),jobs=new Set();
 const identify=ctx=>({root:realpathSync(ctx.cwd),session:workflowSessionKey(ctx.sessionManager.getSessionId())});
 const same=(a,b)=>a?.root===b?.root&&a?.session===b?.session;
 const clear=()=>{generation++;recovery=undefined;scope=undefined;for(const c of pending)c.abort();return Promise.allSettled([...jobs]);};
 const allowed=ctx=>{if(closed)return false;if(version!==TUPLE.hostVersion){notice(ctx,'unsupported-host-version');return false}if(!ctx.isProjectTrusted()){notice(ctx,'untrusted-project');return false}return !ctx.signal?.aborted};
 async function invoke(ctx,args,input,validate) {
  if(!allowed(ctx))return;
  let identity;try{identity=identify(ctx)}catch{notice(ctx,'missing-session-identity');return}
  const epoch=generation,c=new AbortController(),signal=ctx.signal;pending.add(c);
  const abort=()=>c.abort();signal?.addEventListener('abort',abort,{once:true});
  try{
   const request=typeof input==='function'?input(identity):input;
   const job=runner.run({cwd:identity.root,args:typeof args==='function'?args(identity):args,input:request?canonicalWorkflowJSON(request):'',signal:c.signal});
   jobs.add(job);let result;try{result=await job}finally{jobs.delete(job)}
   if(c.signal.aborted||closed||epoch!==generation||!allowed(ctx)||!same(identity,identify(ctx)))return;
   if(result.fault||result.exitCode!==0){unresolved=true;notice(ctx,'native-event-unavailable');return}
   const value=validate(result.stdout,request);
   scope=identity;return value;
  }catch{unresolved=true;notice(ctx,'invalid-workflow-response')}
  finally{pending.delete(c);signal?.removeEventListener('abort',abort)}
 }
 const event=(ctx,name,extra={})=>invoke(ctx,['dogfood','event','--host','pi','--host-version',version,'--surface','extension','--adapter-version','0.3.0','--event',name,'--input','-','--budget-bytes','8000'],id=>({sessionIdSha256:id.session,...extra}),(raw,input)=>decodeWorkflowEnvelope(raw,name,input));
 async function recover(ctx,startSource){
  const value=await event(ctx,'session-start',{startSource});
  if(value){knownIncomplete=unresolved=value.policy.lifecycle==='active'||value.policy.lifecycle==='satisfied'&&!value.policy.satisfied;if(['active','satisfied'].includes(value.policy.lifecycle))recovery={scope,context:value.context}}
 }
 pi.on('session_start',async(e,ctx)=>{clear();try{const next=identify(ctx).session;if(next!==activitySession){used=false;knownIncomplete=unresolved=false;activitySession=next}}catch{}await recover(ctx,({startup:'startup',reload:'resume',new:'clear',resume:'resume',fork:'resume'})[e.reason]??'resume')});
 pi.on('session_tree',async(_,ctx)=>{clear();await recover(ctx,'resume')});
 pi.on('session_compact',async(_,ctx)=>{clear();await recover(ctx,'compact')});
 pi.on('input',(e,ctx)=>{if(e.source==='interactive'||e.source==='rpc'){const saved=recovery,prior=scope;clear();try{const current=identify(ctx);if(same(prior,current))scope=current;else knownIncomplete=false;if(saved&&same(saved.scope,current))recovery=saved}catch{knownIncomplete=false}used=false;unresolved=knownIncomplete}});
 pi.on('context',(e,ctx)=>{
  const saved=recovery;recovery=undefined;
  if(!saved||!allowed(ctx))return;
  try{if(!same(saved.scope,identify(ctx)))return}catch{return}
  return {messages:[...e.messages,{role:'custom',customType:'corvint-workflow-recovery',content:'BEGIN CORVINT REPOSITORY DATA\n'+canonicalWorkflowJSON(saved.context)+'\nEND CORVINT REPOSITORY DATA',display:false,timestamp:Date.now()}]};
 });
 pi.on('agent_before_settle',async(e,ctx)=>{
  if(e.outcome!=='completed'||e.continue!==false||!Array.isArray(e.context?.pendingMessages)||e.context.pendingMessages.length||ctx.hasPendingMessages()||ctx.signal?.aborted)return;
  const value=await event(ctx,'stop',{stopHookActive:used,changedPaths:[]});
  if(!value||!allowed(ctx)||ctx.hasPendingMessages()||ctx.signal?.aborted)return;
  knownIncomplete=unresolved=value.policy.lifecycle==='active'||value.policy.lifecycle==='satisfied'&&!value.policy.satisfied;
  if(value.completion.decision!=='block'||used)return;
  used=true;
  // Only a fixed instruction and the validated receipt handle enter the session.
  // Source packets and native status/verification output stay outside its transcript.
  return {entries:[{type:'custom_message',customType:'corvint-workflow-remediation',content:'The explicitly enrolled Corvint workflow remains incomplete. Report its unresolved work and use the operator-approved workflow to address it. Do not claim completion or run expensive verification or recording automatically. Receipt: '+value.resultDigest,display:true}],continue:true};
 });
 pi.on('agent_settled',(_,ctx)=>{if(unresolved)notice(ctx,'local-policy-incomplete')});
 const close=async()=>{if(closed)return;closed=true;await clear();await runner.close()};
 pi.on('session_shutdown',close);
 pi.registerCommand('corvint-workflow',{description:'Explicit local workflow: begin PLAN_FILE or status',handler:async(text,ctx)=>{
  const match=/^(status|begin)(?:\s+(.+))?$/.exec(text.trim());
  if(!match||match[1]==='status'&&match[2]||match[1]==='begin'&&!match[2]){notice(ctx,'expected-begin-plan-or-status');return}
  const action=match[1],file=match[2];
  if(action==='begin')clear();
  if(file&&(Buffer.byteLength(file)>4096||/[\0\r\n]/.test(file))){notice(ctx,'invalid-plan-path');return}
  const value=await invoke(ctx,id=>['dogfood',action,...(file?['--plan',file]:[]),'--session-key',id.session],undefined,raw=>{
   if(Buffer.byteLength(raw)>8000)throw Error();
   const v=decodeObject(raw);
   if(!keys(v,['ok','profile','tool','mutates','policy','claim'])||v.ok!==true||v.profile!=='corvint-local-completion/0'||v.tool!=='dogfood-'+action||v.mutates!==(action==='begin')||v.claim!=='caller-owned-selected-workflow-only'||!object(v.policy)||!states.has(v.policy.lifecycle)||typeof v.policy.satisfied!=='boolean')throw Error();return v;
  });
  if(value){knownIncomplete=unresolved=value.policy.lifecycle==='active'||value.policy.lifecycle==='satisfied'&&!value.policy.satisfied;const summary=`Corvint workflow: ${value.policy.lifecycle}; selected policy satisfied: ${value.policy.satisfied}. Frontier authority remains unavailable.`;if(ctx.hasUI)ctx.ui.notify(summary,'info');else process.stderr.write(summary+'\n')}
 }});
 return {clear,close};
}
