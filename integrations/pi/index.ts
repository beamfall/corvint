// SPDX-License-Identifier: AGPL-3.0-or-later
import { VERSION, type ExtensionAPI } from '@earendil-works/pi-coding-agent';
import { createRunner, decodeObject } from './runtime.js';
import { createCommandRunner } from './process.js';
import { createCoreService, registerCoreTools } from './core.js';
import { createTasksService, registerTasksTools } from './tasks.js';
import { worktreeIdentity, canonical } from './operations.js';
import { registerCockpit } from './cockpit.js';
import { registerWorkflow } from './workflow.js';
import * as uiKit from '@earendil-works/pi-tui';
import register from './extension.js';

type Options = { coreBinary?: string; tasksBinary?: string };

// Local installation may pin candidate paths without replacing official binaries.
export function registerWithOptions(pi: ExtensionAPI, options: Options = {}) {
 const notice=(ctx:any,code:string)=>{
  const text=`Corvint unavailable: ${code}. No completion or authority is established.`;
  if(ctx.hasUI)ctx.ui.notify(text,'warning');else process.stderr.write(text+'\n');
 };
 if(VERSION!=='0.99.1'){process.stderr.write('Corvint unavailable: unsupported-host-version.\n');return;}
 const binary=options.coreBinary??process.env.CORVINT_BIN;
 const tasksBinary=options.tasksBinary??process.env.CORVINT_TASKS_BIN??'corvint-tasks';
 const coreRunner=createCommandRunner({binary:binary??'corvint'});
 const tasksRunner=createCommandRunner({binary:tasksBinary});
 const coreNative=createCoreService({runner:coreRunner});
 const tasks=createTasksService({runner:tasksRunner});
 let generation=0;
 const core={
  clear(){generation++;coreNative.clear();},close:()=>coreNative.close(),
  async read(operation:string,input:any,ctx:any){
   if(!ctx.isProjectTrusted())return coreNative.read(operation,input,ctx);
   const epoch=generation;let result:any;
   try{
    const before=canonical(await worktreeIdentity(ctx));
    result=await coreNative.read(operation,input,ctx);
    if(epoch!==generation||before!==canonical(await worktreeIdentity(ctx)))return {...result,fault:'stale-context'};
    return result;
   }catch{return {...result,operation,fault:'identity-unavailable'};}
  }
 };
 let workflow:any;
 const tools=register(pi,{runner:createRunner({binary}),version:VERSION,lifecycle:[core,tasks],onInterrupt:async()=>{core.clear();tasks.clear();await Promise.all([coreRunner.cancel(),tasksRunner.cancel(),workflow?.clear()]);}});
 workflow=registerWorkflow(pi,{runner:createCommandRunner({binary:binary??'corvint',timeoutMs:1700,maxBytes:8000}),version:VERSION,notice});
 const context={
  async read(_operation:string,input:any,ctx:any){
   const result=await tools.readContext(input,ctx);
   return {operation:'context',raw:{stdout:result.content.map((part:any)=>part.text??'').join('\n')},receipt:result.details,fault:result.isError?result.details.corvint.fault:undefined};
  },
  async expand({packetId,selector,maxBytes}:any,ctx:any){
   let selection;
   try{selection=decodeObject(selector);if(Object.keys(selection).some(key=>!['result','evidence','lines','requirement'].includes(key)))throw Error();}
   catch{return {operation:'expand',fault:'invalid-input'};}
   const result=await tools.expandSource({...selection,handle:packetId,maxBytes},ctx);
   return {operation:'expand',raw:{stdout:result.content.map((part:any)=>part.text??'').join('\n')},fault:result.isError?result.details.corvint.fault:undefined};
  }
 };
 registerCoreTools(pi,{service:core,notice});
 registerTasksTools(pi,{service:tasks,notice});
 const cockpit=registerCockpit(pi,{core,tasks,context,notice,identity:worktreeIdentity,uiKit});
 pi.on('session_shutdown',async()=>{cockpit.close();await workflow.close();});
 return {core,tasks,workflow,cockpit};
}
export default function(pi: ExtensionAPI) { return registerWithOptions(pi); }
