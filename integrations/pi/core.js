// SPDX-License-Identifier: AGPL-3.0-or-later
const fields={query:['task','limit'],context:['task','subject','limit'],impact:['paths','base','limit'],affected:['base'],review:['base'],prove:['task','base','limit'],'cem-status':['map','base','target'],'ocm-status':['map','cem','base','target'],frontier:['cem','ocm','base','target'],'dogfood-status':['sessionKey']};
export const coreOperations=Object.freeze(Object.keys(fields));
function argumentsFor(operation,input) {
 if(!Object.hasOwn(fields,operation)||!input||typeof input!=='object'||Array.isArray(input)||Object.keys(input).some(k=>!fields[operation].includes(k)))throw Error('invalid-input');
 for(const [key,value] of Object.entries(input)) {
  if(key==='limit'){if(!Number.isInteger(value)||value<1||value>20)throw Error('invalid-input');continue;}
  if(key==='paths'){if(!Array.isArray(value)||value.length<1||value.length>100||value.some(p=>typeof p!=='string'||!p||p.startsWith('-')||p.includes('\0')||p.length>1024))throw Error('invalid-input');continue;}
  if(typeof value!=='string'||!value||value.includes('\0')||value.length>4096)throw Error('invalid-input');
  if(['base','target'].includes(key)&&! /^[0-9a-f]{40}$/.test(value))throw Error('invalid-input');
  if(key==='sessionKey'&&! /^[0-9a-f]{64}$/.test(value))throw Error('invalid-input');
 }
 const required={query:['task'],context:['task'],review:['base'],'cem-status':['map','base','target'],'ocm-status':['map','base','target'],frontier:['cem','ocm','base','target']}[operation]??[];
 if(required.some(k=>input[k]===undefined))throw Error('invalid-input');
 if(operation==='impact'&&Number(!!input.paths)+Number(!!input.base)!==1)throw Error('invalid-input');
 if(operation==='prove'&&Number(!!input.task)+Number(!!input.base)!==1)throw Error('invalid-input');
 const args=operation.endsWith('-status')?[operation.slice(0,-7),'status']:[operation];
 for(const [key,value] of Object.entries(input)){if(key==='paths')continue;const flag=key==='sessionKey'?'session-key':key==='base'&&['cem-status','ocm-status','frontier'].includes(operation)?'expected-base':key;args.push(`--${flag}`,String(value));}
 if(input.paths)args.push('--',...input.paths);
 if(operation==='frontier')args.push('--json');
 return args;
}
export function createCoreService({runner}) {
 let generation=0;
 return {
  clear(){generation++;},
  async read(operation,input={},ctx={}) {
   if(typeof ctx.isProjectTrusted!=='function'||!ctx.isProjectTrusted())return {operation,fault:'untrusted-project'};
   let args;try{args=argumentsFor(operation,input)}catch{return {operation,fault:'invalid-input'}}
   const started=generation;let raw;try{raw=await runner.run({cwd:ctx.cwd,args,signal:ctx.signal})}catch{return {operation,fault:'runner-failed'}}
   const result={operation,raw,exitCode:raw.exitCode};
   if(started!==generation)return {...result,fault:'stale-context'};
   if(raw.fault)return {...result,fault:raw.fault};
   try{const receipt=JSON.parse(raw.stdout);if(!receipt||typeof receipt!=='object'||Array.isArray(receipt))throw Error();result.receipt=receipt;}catch{return {...result,fault:raw.exitCode===0?'malformed-output':'native-command-failed'}}
   if(raw.exitCode!==0&&!(operation==='frontier'&&raw.exitCode===1))result.fault='native-command-failed';
   return result;
  },
  close:()=>runner.close(),
 };
}
export function registerCoreTools(pi,{service,notice=()=>{}}) {
 pi.registerTool({name:'corvint_core_read',label:'Corvint native read',description:'Read native Core evidence. Preserve limitations and unknowns; no test execution, indexing, completion or authority is granted.',parameters:{type:'object',properties:{operation:{type:'string',enum:coreOperations},input:{type:'object'}},required:['operation','input'],additionalProperties:false},execute:async(_id,args,signal,_update,ctx)=>{
  const result=await service.read(args.operation,args.input,{cwd:ctx.cwd,isProjectTrusted:()=>ctx.isProjectTrusted(),signal});
  if(result.fault)notice(ctx,result.fault);
  return {content:[{type:'text',text:JSON.stringify(result)}],details:{corvint:result},isError:!!result.fault};
 }});
 return {clear:()=>service.clear(),close:()=>service.close()};
}
