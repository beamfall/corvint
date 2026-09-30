// SPDX-License-Identifier: AGPL-3.0-or-later
import { canonical, digest, worktreeIdentity, createOperationLedger } from './operations.js';
const reads={help:['help'],audit:['receipt','audit'],queue:['queue','status'],roadmap:['roadmap'],ticket:['ticket','show'],blockers:['ticket','blockers'],gates:['gate','list'],gate:['gate','show'],releases:['release','list'],release:['release','show'],readiness:['release','readiness'],attempt:['attempt','show'],plan:['plan','preview']};
const writes=['ticket prioritize','claim','renew','release','submit','gate run','complete'];
const ident=v=>typeof v==='string'&&/^[A-Za-z0-9][A-Za-z0-9:._/-]{0,511}$/.test(v)&&!v.includes('..');
const count=v=>typeof v==='string'&&/^(0|[1-9][0-9]{0,17})$/.test(v);
const oid=v=>typeof v==='string'&&/^[0-9a-f]{40}([0-9a-f]{24})?$/.test(v);
const ticket=v=>ident(v)&&v.startsWith('ticket:');
function object(v,keys,required=keys) {if(!v||typeof v!=='object'||Array.isArray(v)||Object.keys(v).some(k=>!keys.includes(k))||required.some(k=>!Object.hasOwn(v,k)))throw Error('invalid-input')}
function ensure(ok){if(!ok)throw Error('invalid-input')}
function failure(fault,extra={}){return {ok:false,fault,mutation:'not-attempted',...extra}}
export function tasksResult(result) {return {content:[{type:'text',text:JSON.stringify(result)}],details:result,structuredContent:result,isError:!result.ok}}
function readArgs(operation,input) {
 const args=reads[operation];if(!args)throw Error('unsupported-operation');
 if(['ticket','blockers','gate','release','readiness','attempt'].includes(operation)){object(input,['id']);ensure(ident(input.id));return [...args,input.id]}
 if(operation==='roadmap'){object(input,['offset','limit'],[]);const out=[...args];for(const k of ['offset','limit'])if(input[k]!==undefined){ensure(Number.isSafeInteger(input[k])&&input[k]>=0&&input[k]<=(k==='limit'?100:100000));out.push('--'+k,String(input[k]))}return out}
 object(input,[]);return [...args];
}
function mutationArgs(operation,input,requestId,issuedAt,identity) {
 if(!writes.includes(operation))throw Error('unsupported-mutation');
 let args=operation.split(' ');
 if(operation==='ticket prioritize') {
  object(input,['ticketId','expectedRevision','priority','order']);ensure(ticket(input.ticketId)&&count(input.expectedRevision)&&['P0','P1','P2','P3'].includes(input.priority)&&count(input.order));
  return {args:[...args,'--target',input.ticketId,'--expected-revision',input.expectedRevision,'--request-id',requestId,'--issued-at',issuedAt,'--role','OPERATOR','--payload-stdin'],input:canonical({order:input.order,priority:input.priority})+'\n'};
 }
 if(operation==='claim') {
  object(input,['ticketId','expectedRevision','holder']);ensure(ticket(input.ticketId)&&count(input.expectedRevision)&&ident(input.holder));
  args.push(input.ticketId,'--holder',input.holder,'--branch',identity.branch,'--base',identity.head);
 } else {
  const extras={renew:[],release:['reason'],submit:['tree'],'gate run':['gate'],complete:['commit']}[operation];
  object(input,['ticketId','expectedRevision','attemptId','generation','holder',...extras]);
  ensure(ticket(input.ticketId)&&count(input.expectedRevision)&&ident(input.attemptId)&&count(input.generation)&&ident(input.holder));
  args.push('--attempt',input.attemptId,'--generation',input.generation);
  for(const key of extras){ensure(['tree','commit'].includes(key)?oid(input[key]):ident(input[key]));args.push('--'+key,input[key])}
  if(operation==='gate run')args.push('--worktree',identity.root);
 }
 args.push('--request-id',requestId,'--role','OPERATOR');
 return {args};
}
function decode(result) {
 if(result.fault)return failure(result.fault,{mutation:'unknown'});
 let raw;try{ensure(typeof result.stdout==='string'&&Buffer.byteLength(result.stdout)<=65536);raw=JSON.parse(result.stdout);ensure(raw&&raw.profile==='taskman-command-result/0'&&['OK','REFUSED','ERROR','NOT_RUN'].includes(raw.outcome)&&Array.isArray(raw.items)&&Array.isArray(raw.codes)&&Array.isArray(raw.warnings)&&Array.isArray(raw.command))}catch{return failure('invalid-native-receipt',{mutation:'unknown'})}
 return {ok:raw.outcome==='OK'&&result.exitCode===0,raw,exitCode:result.exitCode,mutation:raw.mutation??'not-reported',...(raw.outcome==='OK'&&result.exitCode===0?{}:{fault:'native-refusal'})};
}
export function createTasksService({runner,identity=worktreeIdentity,ledger=createOperationLedger}) {
 let epoch=0,busy=false,lastBranch;
 const scope=i=>digest({root:i.root,gitDir:i.gitDir,branch:i.branch,sessionSha256:i.sessionSha256,branchGeneration:epoch});
 const trusted=ctx=>typeof ctx.isProjectTrusted==='function'&&ctx.isProjectTrusted();
 async function identify(ctx) {const i=await identity(ctx);if(lastBranch!==undefined&&lastBranch!==i.branch)epoch++;lastBranch=i.branch;return i}
 async function native(args,ctx,input) {return decode(await runner.run({cwd:ctx.cwd,args,input,signal:ctx.signal}))}
 async function read(operation,input={},ctx) {
  if(!trusted(ctx))return failure('untrusted-project');
  try {
   const args=readArgs(operation,input),before=await identify(ctx),e=epoch;
   if(ctx.signal?.aborted)return failure('aborted');
   const result=await native(args,ctx);
   const after=await identify(ctx);
   if(e!==epoch||scope(before)!==scope(after))return failure('stale-context');
   return result;
  }catch(e){return failure(['invalid-input','unsupported-operation'].includes(e.message)?e.message:'read-unavailable')}
 }
 async function reconcile(input,ctx) {
  const evidence={audit:await read('audit',{},ctx),ticket:await read('ticket',{id:input.ticketId},ctx),queue:await read('queue',{},ctx)};
  if(input.attemptId)evidence.attempt=await read('attempt',{id:input.attemptId},ctx);
  return evidence;
 }
 // This is called only by the registered explicit command, never by a model tool.
 // A local host command is an operator assertion, not authenticated human identity.
 async function command(request,ctx) {
  if(!trusted(ctx))return failure('untrusted-project');
  if(ctx.isIdle?.()===false||ctx.hasPendingMessages?.()===true)return failure('host-not-idle');
  if(busy)return failure('operation-in-flight');
  busy=true;let entry,log,dispatched=false;
  try {
   object(request,['operation','input','requestId','resume'],['operation','input','requestId']);
   ensure(ident(request.requestId)&&request.requestId.length<=128&&(request.resume===undefined||typeof request.resume==='boolean'));
   const before=await identify(ctx),e=epoch;
   const issuedAt=new Date().toISOString().replace(/\.\d{3}Z$/,'Z');
   mutationArgs(request.operation,request.input,request.requestId,issuedAt,before);
   const capability=await read('help',{},ctx);
   if(!capability.ok||!capability.raw.items[0]?.implemented?.includes(request.operation))return failure('capability-unavailable');
   const evidence=await reconcile(request.input,ctx);
   if(Object.values(evidence).some(r=>!r.ok))return failure('reconciliation-unavailable',{reconciliation:evidence});
   const record=evidence.ticket.raw.items[0];
   if(!record||record.ticketId!==request.input.ticketId)return failure('ticket-mismatch',{reconciliation:evidence});
   log=ledger(before.gitDir);
   const pending=await log.inspect();
   // Resume may see the revision produced by the original mutation. Native same-ID
   // replay, never a fresh request, resolves that uncertainty.
   const isResume=request.resume===true&&pending?.requestId===request.requestId;
   if(!isResume&&record.revision!==request.input.expectedRevision)return failure('stale-ticket',{reconciliation:evidence});
   if(evidence.attempt) {
    const a=evidence.attempt.raw.items[0];
    if(!a||a.ticketId!==request.input.ticketId||a.lease?.holder!==request.input.holder||a.branch!==before.branch)return failure('attempt-owner-mismatch',{reconciliation:evidence});
    if(!isResume&&(a.generation!==request.input.generation||a.ticketRevision!==record.acceptanceRevision||!Number.isFinite(Date.parse(a.lease.expiresAt))||Date.parse(a.lease.expiresAt)<=Date.now()||['COMPLETED','FAILED','CANCELLED'].includes(a.phase)))return failure('stale-attempt',{reconciliation:evidence});
   }
   const after=await identify(ctx);
   if(e!==epoch||scope(before)!==scope(after)||before.head!==after.head||ctx.signal?.aborted)return failure('stale-context');
   const intentSha256=digest({operation:request.operation,input:request.input,head:before.head});
   const originalCall=mutationArgs(request.operation,request.input,request.requestId,issuedAt,before);
   entry=await log.begin({requestId:request.requestId,intentSha256,scopeSha256:scope(before),issuedAt,argvSha256:digest(originalCall)},request.resume===true);
   if(entry.completed)return {ok:entry.terminal?.ok===true,...(entry.terminal?.ok===true?{}:{fault:entry.terminal?.fault??'native-outcome-unknown'}),nativeOutcome:entry.terminal?.nativeOutcome??'UNKNOWN',exitCode:entry.terminal?.exitCode??null,mutation:'previously-observed',requestId:entry.requestId,receiptSha256:entry.receiptSha256,reconciliation:evidence};
   // Native GateRun executes the gate before its same-ID receipt replay lookup.
   // An uncertain execution cannot be retried safely by this adapter.
   if(request.operation==='gate run'&&entry.resume)return failure('gate-replay-unavailable',{mutation:'unknown',requestId:entry.requestId,reconciliation:evidence});
   const call=mutationArgs(request.operation,request.input,request.requestId,entry.issuedAt,before);
   if(entry.argvSha256!==digest(call))throw Error('request-id-conflict');
   const fresh=await identify(ctx);
   if(!trusted(ctx)||e!==epoch||scope(before)!==scope(fresh)||fresh.head!==before.head||ctx.signal?.aborted)return failure('stale-context',{requestId:entry.requestId,mutation:'pending-reconciliation'});
   dispatched=true;
   const result=await native(call.args,ctx,call.input);
   const final=await identify(ctx);
   if(e!==epoch||scope(before)!==scope(final))return failure('stale-context',{requestId:entry.requestId,mutation:'unknown'});
   if(result.raw){await log.finish(entry,result.raw,{ok:result.ok,nativeOutcome:result.raw.outcome,exitCode:result.exitCode,fault:result.ok?null:'native-refusal'});return {...result,mutation:result.raw.items[0]?.receipt?'receipt-observed':'native-result-observed',requestId:entry.requestId,reconciliation:evidence}}
   return {...result,requestId:entry.requestId,mutation:'unknown',reconciliation:evidence};
  } catch(e) {return failure(['invalid-input','unsupported-mutation','pending-operation','request-id-conflict','reconciliation-required','ledger-full','ledger-invalid'].includes(e.message)?e.message:'operation-unavailable',{mutation:dispatched?'unknown':entry?'pending-reconciliation':'not-attempted',...(entry?{requestId:entry.requestId}:{})})}
  finally{busy=false}
 }
 return {read,command,clear(){epoch++},close:()=>runner.close?.()};
}
export function registerTasksTools(pi,{service,notice=()=>{}}) {
 pi.registerTool({name:'corvint_tasks',label:'Corvint Tasks reads',description:'Bounded native Tasks reads. Raw receipts retain unknowns; no task-store writes or initialization.',parameters:{type:'object',properties:{operation:{type:'string',enum:Object.keys(reads)},input:{type:'object'}},required:['operation'],additionalProperties:false},execute:async(_id,args,signal,_update,ctx)=>tasksResult(await service.read(args.operation,args.input??{},{...ctx,signal}))});
 pi.registerCommand('corvint-tasks',{description:'Explicit native Tasks operation JSON: operation, input, requestId; resume:true reconciles the same uncertain request. Local operator assertion, not authenticated identity.',handler:async(text,ctx)=>{
  let request;try{if(Buffer.byteLength(text)>8192)throw Error();request=JSON.parse(text)}catch{notice(ctx,'invalid-input');return}
  const result=await service.command(request,ctx);
  if(!result.ok)notice(ctx,result.fault);
  pi.sendMessage({customType:'corvint-tasks',content:[{type:'text',text:JSON.stringify(result)}],display:true},{triggerTurn:false});
 }});
 return {clear:()=>service.clear()};
}
