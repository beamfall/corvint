// SPDX-License-Identifier: AGPL-3.0-or-later
import { createHash } from 'node:crypto';
import { decodeObject } from './runtime.js';

const STARTS={startup:'startup',reload:'resume',new:'clear',resume:'resume',fork:'resume'};
const OUTCOME_KEYS=new Set(['outcome','taskSha256','openedPaths','changedPaths','verification']);

export default function register(pi, {runner, version}) {
 let interrupting=false, starting=false, startupExitCode=0, generation=0, recovery;
 const seen=new Set();
 const identity=ctx=>createHash('sha256').update(String(ctx.sessionManager.getSessionId())).digest('hex');
 const notice=(ctx,code)=>{
  const text=`Corvint unavailable: ${code}. Frontier authority remains unavailable.`;
  if(ctx.hasUI)ctx.ui.notify(text,'warning');
  else process.stderr.write(text+'\n');
 };
 const signalInterrupt=(signal,listener)=>{
  if(starting&&!startupExitCode)startupExitCode=signal==='SIGINT'?130:143;
  if(interrupting)return;
  interrupting=true;
  void runner.close().then(()=>{
   const others=process.listeners(signal).filter(candidate=>candidate!==listener);
   // Restore only the default signal action; never replay into Pi's handlers.
   if(others.length===0){process.removeListener(signal,listener);process.kill(process.pid,signal);}
   interrupting=false;
  }).catch(()=>{process.stderr.write('Corvint unavailable: cleanup-failed.\n');});
 };
 const interrupt=()=>signalInterrupt('SIGINT',interrupt);
 const terminate=()=>signalInterrupt('SIGTERM',terminate);
 process.on('SIGINT',interrupt);
 process.on('SIGTERM',terminate);

 async function event(ctx,name,input={}) {
  if(startupExitCode)return;
  if(!ctx.isProjectTrusted()){notice(ctx,'untrusted-project');return;}
  const epoch=generation;
  const result=await runner.run({cwd:ctx.cwd,event:name,input:{...input,sessionIdSha256:identity(ctx)},hostVersion:version,signal:ctx.signal});
  if(startupExitCode||epoch!==generation)return;
  if(result.fault){notice(ctx,result.fault);return;}
  const key=`${generation}:${name}:${result.receiptId}`;
  if(name!=='user-prompt'&&name!=='session-start'&&name!=='session-end'&&seen.has(key))return;
  seen.add(key);
  if(seen.size>128)seen.delete(seen.values().next().value);
  for(const code of result.degradations)if(code!=='frontier-authority-unavailable')notice(ctx,code);
  return result;
 }
 async function recover(ctx,startSource) {
  recovery=undefined;
  const result=await event(ctx,'session-start',{startSource});
  if(result?.context)recovery={context:result.context,cwd:ctx.cwd,session:identity(ctx)};
 }
 async function transition(ctx,startSource) {
  generation++;
  seen.clear();
  recovery=undefined;
  await runner.close();
  await recover(ctx,startSource);
 }
 pi.on('session_start',async(e,ctx)=>{
  starting=e.reason==='startup'&&!ctx.hasUI;
  try{await transition(ctx,STARTS[e.reason]);}finally{starting=false;}
 });
 pi.on('input',async()=>{
  if(startupExitCode){process.stderr.write('Corvint unavailable: aborted.\n');process.exitCode=startupExitCode;return {action:'handled'};}
 });
 pi.on('session_compact',async(_,ctx)=>{await recover(ctx,'compact');});
 pi.on('session_tree',async(_,ctx)=>{await transition(ctx,'resume');});
 pi.on('before_agent_start',async(e,ctx)=>{
  const result=await event(ctx,'user-prompt',{task:e.prompt});
  if(!result){recovery=undefined;return;}
  if(result.context)return {systemPrompt:e.systemPrompt+'\n'+result.context};
 });
 pi.on('context',async(e,ctx)=>{
  const pending=recovery;
  recovery=undefined;
  if(!pending||!ctx.isProjectTrusted()||ctx.signal?.aborted)return;
  if(pending.cwd!==ctx.cwd||pending.session!==identity(ctx))return;
  // Pi's context hook changes this request only, including an automatic compaction retry.
  return {messages:[...e.messages,{role:'custom',customType:'corvint-recovery',content:pending.context,display:false,timestamp:Date.now()}]};
 });
 pi.on('tool_result',async(_,ctx)=>{await event(ctx,'post-tool');});
 pi.on('agent_end',async(e,ctx)=>{
  const terminal=e.messages.findLast(m=>m.role==='assistant');
  if(!terminal||!['stop','error','aborted','toolUse','length'].includes(terminal.stopReason))notice(ctx,'invalid-adapter-response');
  await event(ctx,'stop');
 });
 pi.on('session_shutdown',async(_,ctx)=>{
  generation++;
  recovery=undefined;
  try{await runner.close();await event(ctx,'session-end');}
  finally{await runner.close();seen.clear();process.removeListener('SIGINT',interrupt);process.removeListener('SIGTERM',terminate);}
 });
 pi.registerCommand('corvint-context',{description:'Read bounded Corvint context',handler:async(text,ctx)=>{
  const result=await event(ctx,'user-prompt',{task:text});
  if(result?.context)pi.sendMessage({customType:'corvint-context',content:result.context,display:true},{triggerTurn:false});
 }});
 pi.registerCommand('corvint-outcome',{description:'Submit an explicit outcome JSON (persistence unavailable)',handler:async(text,ctx)=>{
  let input;
  try{
   if(Buffer.byteLength(text)>131072)throw Error('input bound');
   input=decodeObject(text);
   if(Object.keys(input).some(key=>!OUTCOME_KEYS.has(key)))throw Error('input keys');
  }catch{notice(ctx,'invalid-input');return;}
  await event(ctx,'session-end',input);
 }});
}
