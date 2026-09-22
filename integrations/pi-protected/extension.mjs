// SPDX-License-Identifier: AGPL-3.0-or-later
import register from '../pi/extension.js';

// The ordinary adapter owns context recovery, tools and cleanup. Protected Stop
// runs only after the native SDK declares the turn idle, never on an early token.
export default function registerProtected(pi,{ordinary,qualified,installed,version}) {
 let stopHandler,pending,lastStop,continued=false,epoch=0;
 const runner={
  tool:request=>ordinary.tool(request),
  async close(){await Promise.all([ordinary.close(),qualified.close()])},
  async run(request){
   const requestEpoch=epoch;
   const stop=request.event==='stop';if(stop)lastStop=undefined;
   const eligible=installed&&['session-start','user-prompt','stop','session-end'].includes(request.event)&&!(request.event==='session-end'&&'outcome' in request.input);
   let result=await (eligible?qualified:ordinary).run({...request,input:stop&&eligible?{...request.input,stopHookActive:continued}:request.input});
   if(eligible&&result.fault&&!request.signal?.aborted&&requestEpoch===epoch){
    const fault=result.fault;
    result=await ordinary.run(request);
    if(!result.fault)result={...result,degradations:[...result.degradations,fault]};
   }
   if(stop)lastStop={result,epoch:requestEpoch};
   return result;
  },
 };
 register({...pi,on(name,handler){if(name==='agent_end')stopHandler=handler;else pi.on(name,handler)}},{runner,version});
 for(const name of ['session_start','session_tree','session_shutdown'])pi.on(name,()=>{epoch++;pending=undefined;lastStop=undefined;continued=false});
 pi.on('input',event=>{if(event.source!=='extension'){epoch++;continued=false}});
 pi.on('agent_end',event=>{const terminal=event.messages.findLast(m=>m.role==='assistant');pending={event,epoch,success:terminal&&['stop','length'].includes(terminal.stopReason)&&!event.willRetry}});
 pi.on('agent_settled',async(_,ctx)=>{
  const work=pending;pending=undefined;if(!work||work.epoch!==epoch)return;
  lastStop=undefined;
  await stopHandler(work.event,ctx);
  const result=lastStop?.epoch===work.epoch?lastStop.result:undefined;
  if(work.epoch!==epoch||!ctx.isProjectTrusted()||ctx.signal?.aborted||result?.fault||!result?.qualified)return;
  const q=result.qualified,unresolved=q.frontier?.state==='OPEN'||q.policy?.unmet?.length>0;
  if(continued&&unresolved)pi.sendMessage({customType:'corvint-status',content:`Corvint ${q.support}/${q.qualification}: remediation remains unresolved. Frontier ${q.frontier.state}; local policy ${q.policy?.unmet?.length?'unmet':'satisfied or inactive'}. No further automatic continuation; this is not a closure claim.`,display:true},{triggerTurn:false});
  if(!work.success||continued||!ctx.isIdle()||ctx.hasPendingMessages()||!result.shouldContinue)return;
  continued=true;
  pi.sendUserMessage('Corvint requests one permitted remediation for unmet local policy or an open protected Frontier. Inspect the current evidence, address the unmet requirement if possible, and report anything unresolved. This is the single bounded follow-up and is not evidence of completion.',{deliverAs:'followUp'});
 });
}
