// SPDX-License-Identifier: AGPL-3.0-or-later
import { createHash } from 'node:crypto';
import { VERSION, type ExtensionAPI } from '@earendil-works/pi-coding-agent';
import { createRunner } from './runtime.js';
export default function(pi: ExtensionAPI) {
 const runner=createRunner();
 let interrupting=false, starting=false, startupExitCode=0;
 const signalInterrupt=(signal:'SIGINT'|'SIGTERM',listener:()=>void)=>{
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
 let generation=0;const seen=new Set<string>();
 const session=(ctx:any)=>createHash('sha256').update(String(ctx.sessionManager.getSessionId())).digest('hex');
 const notice=(ctx:any,code:string)=>{const text=`Corvint unavailable: ${code}. Frontier authority remains unavailable.`;if(ctx.hasUI)ctx.ui.notify(text,'warning');else pi.sendMessage({customType:'corvint-fault',content:text,display:true},{triggerTurn:false});};
 async function event(ctx:any,name:string,input:any={}) {
  if(startupExitCode)return;
  if(!ctx.isProjectTrusted()){notice(ctx,'untrusted-project');return;}
  const result=await runner.run({cwd:ctx.cwd,event:name,input:{...input,sessionIdSha256:session(ctx)},hostVersion:VERSION,signal:ctx.signal});
  if(startupExitCode)return;
  if(result.fault){notice(ctx,result.fault);return;}
  const key=`${generation}:${name}:${result.receiptId}`;if(name!=='user-prompt'&&seen.has(key))return;seen.add(key);if(seen.size>128)seen.delete(seen.values().next().value);
  return result.context;
 }
 pi.on('session_start',async(e,ctx)=>{starting=e.reason==='startup'&&!ctx.hasUI;if(starting)process.on('SIGTERM',terminate);try{await runner.close();generation++;seen.clear();await event(ctx,'session-start',{startSource:({startup:'startup',reload:'resume',new:'clear',resume:'resume',fork:'resume'} as any)[e.reason]});}finally{starting=false;process.removeListener('SIGTERM',terminate);}});
 pi.on('input',async()=>{if(startupExitCode){process.stderr.write('Corvint unavailable: aborted.\n');process.exitCode=startupExitCode;return {action:'handled' as const};}});
 pi.on('session_compact',async(_,ctx)=>{await event(ctx,'session-start',{startSource:'compact'});});
 pi.on('before_agent_start',async(e,ctx)=>{const context=await event(ctx,'user-prompt',{task:e.prompt});if(context)return {systemPrompt:e.systemPrompt+'\n'+context};});
 pi.on('tool_result',async(_,ctx)=>{await event(ctx,'post-tool');});
 pi.on('agent_end',async(e,ctx)=>{const terminal=e.messages.findLast(m=>m.role==='assistant');if(!terminal||!['stop','error','aborted','toolUse','length'].includes(terminal.stopReason))notice(ctx,'invalid-adapter-response');await event(ctx,'stop');});
 pi.on('session_shutdown',async(_,ctx)=>{await runner.close();await event(ctx,'session-end');await runner.close();seen.clear();process.removeListener('SIGINT',interrupt);process.removeListener('SIGTERM',terminate);});
 pi.registerCommand('corvint-context',{description:'Read bounded Corvint context',handler:async(text,ctx)=>{const context=await event(ctx,'user-prompt',{task:text});if(context)pi.sendMessage({customType:'corvint-context',content:context,display:true},{triggerTurn:false});}});
 pi.registerCommand('corvint-outcome',{description:'Record an explicit outcome JSON',handler:async(text,ctx)=>{let input;try{input=JSON.parse(text);}catch{notice(ctx,'invalid-input');return;}await event(ctx,'session-end',input);}});
}
