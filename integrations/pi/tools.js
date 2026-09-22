// SPDX-License-Identifier: AGPL-3.0-or-later
import { decodeObject } from './runtime.js';

const string={type:'string'};
const strings={type:'array',items:string,maxItems:256};
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const observationFields=['observedEvidenceHandles','changedPaths','verification'];

export function toolObservations(metadata) {
 if(metadata===undefined)return {};
 if(!metadata||typeof metadata!=='object'||Array.isArray(metadata))throw Error('metadata');
 const input={};
 for(const key of observationFields)if(Object.hasOwn(metadata,key)) {
  const value=metadata[key];
  if(!Array.isArray(value)||value.length>256)throw Error('observation bound');
  if(key==='verification') {
   if(value.some(v=>!v||JSON.stringify(Object.keys(v).sort())!==JSON.stringify(['commandSha256','status'])||! /^[0-9a-f]{64}$/.test(v.commandSha256)||!['passed','failed','not-run','unknown'].includes(v.status)))throw Error('verification');
  } else if(value.some(v=>typeof v!=='string'||!v||Buffer.byteLength(v)>4096))throw Error('observation');
  input[key]=value;
 }
 if(Buffer.byteLength(JSON.stringify(input))>120000)throw Error('observation bound');
 return input;
}

export function registerTools(pi,{runner,version,notice}) {
 const packets=new Map();let next=0,generation=0;
 const clear=()=>{generation++;packets.clear()};
 const scope=ctx=>JSON.stringify([ctx.cwd,ctx.sessionManager.getSessionId()]);
 const failure=(code,record=false)=>({content:[{type:'text',text:`Corvint unavailable: ${code}. No completion or authority is established.${record?' A failed record attempt may have written; inspect the local trace store before retrying.':''}`}],details:{corvint:{fault:code},uncertainMutation:record},isError:true});
 async function invoke(operation,input,signal,ctx) {
  if(!ctx.isProjectTrusted())return {fault:'untrusted-project',mutation:'not-attempted'};
  if(signal?.aborted)return {fault:'aborted',mutation:'not-attempted'};
  const epoch=generation,identity=scope(ctx);
  const result=await runner.tool({cwd:ctx.cwd,operation,input,hostVersion:version,signal});
  if(epoch!==generation||identity!==scope(ctx))return {fault:'stale-context',mutation:result.mutation??'unknown'};
  return result;
 }
 async function context(args,signal,ctx) {
  const result=await invoke('context',args,signal,ctx);
  if(result.fault)return failure(result.fault);
  const handle=`packet-${++next}`;
  packets.set(handle,{...result.packet,scope:scope(ctx)});
  if(packets.size>4)packets.delete(packets.keys().next().value);
  return {content:[{type:'text',text:`Expansion handle: ${handle}\nEvidence handle: ${result.packet.evidenceHandle}\n${result.context}`}],details:{handle,corvint:{observedEvidenceHandles:[result.packet.evidenceHandle]}}};
 }
 async function expand(args,signal,ctx) {
  const {handle,...selection}=args;
  const packet=packets.get(handle);
  if(!packet||packet.scope!==scope(ctx))return failure('stale-context');
  const result=await invoke('expand',{...selection,packet:packet.json,packetSha256:packet.sha256,commit:packet.commit},signal,ctx);
  return result.fault?failure(result.fault):{content:[{type:'text',text:result.context}],details:{}};
 }
 async function record(args,signal,ctx) {
  const result=await invoke('record',args,signal,ctx);
  return result.fault?failure(result.fault,result.mutation!=='not-attempted'):{content:[{type:'text',text:result.context}],details:{mutation:result.mutation}};
 }
 const definitions=[
  ['corvint_context','Corvint context','Read revision-pinned context. Use the returned packet handle and evidence row indexes with corvint_expand.',object({task:{...string,minLength:1,maxLength:2000}}),context],
  ['corvint_expand','Corvint source','Expand exact immutable source from a returned context packet. Select either inclusive lines (e.g. 10:25) or a requirement ID; stale packets refuse.',object({handle:string,result:{type:'integer',minimum:0},evidence:{type:'integer',minimum:0},lines:string,requirement:string,maxBytes:{type:'integer',minimum:1,maximum:8192}},['handle','result','evidence']),expand],
  ['corvint_record_outcome','Record explicit outcome','Explicitly persist a caller-reported local outcome at a clean revision through Corvint record. Requires user-directed learning; task and verification text are secret-screened by the core. This does not verify correctness or grant authority.',object({task:{...string,minLength:1,maxLength:2000},openedPaths:strings,changedPaths:{...strings,minItems:1},verification:{...strings,minItems:1},outcome:{type:'string',enum:['passed','failed','blocked']}},['task','changedPaths','verification','outcome']),record],
 ];
 for(const [name,label,description,parameters,handler] of definitions) {
  pi.registerTool({name,label,description,parameters,promptSnippet:description,execute:async(_id,args,signal,_update,ctx)=>{const result=await handler(args,signal,ctx);if(result.isError)throw Error(result.content[0].text);return result;}});
 }
 pi.registerCommand('corvint-record',{description:'Explicitly persist an outcome JSON through Corvint record',handler:async(text,ctx)=>{
  let input;try{if(Buffer.byteLength(text)>131072)throw Error('bound');input=decodeObject(text)}catch{notice(ctx,'invalid-input');return;}
  const result=await record(input,ctx.signal,ctx);
  if(result.isError){notice(ctx,result.details.corvint.fault,result.details.uncertainMutation?' The record may have been written; inspect the local trace store before retrying.':'');return;}
  pi.sendMessage({customType:'corvint-record',content:result.content,display:true},{triggerTurn:false});
 }});
 return {clear};
}
