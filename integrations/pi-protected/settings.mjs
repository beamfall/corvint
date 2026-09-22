// SPDX-License-Identifier: AGPL-3.0-or-later
import { readFileSync } from 'node:fs';
import { decodeObject } from '../pi/runtime.js';
const levels=new Set(['off','minimal','low','medium','high','xhigh']);
const apis=new Set(['openai-completions','openai-responses','anthropic-messages','google-generative-ai']);
export function readSettings(path) {
 let raw;try{raw=readFileSync(path,'utf8')}catch(e){if(e.code==='ENOENT')return {};throw e;}
 if(Buffer.byteLength(raw)>65536)throw Error('protected-settings-bound');
 const data=decodeObject(raw);
 const allowed=new Set(['provider','model','thinkingLevel','theme','endpoint']);
 if(Object.keys(data).some(k=>!allowed.has(k)))throw Error('unsupported-protected-setting');
 for(const k of ['provider','model','thinkingLevel','theme'])if(data[k]!==undefined&&(typeof data[k]!=='string'||!data[k]||data[k].length>256))throw Error('invalid-protected-setting');
 if(data.thinkingLevel&&!levels.has(data.thinkingLevel))throw Error('invalid-thinking-level');
 if(data.theme&&!['dark','light'].includes(data.theme))throw Error('unsupported-protected-theme');
 if(data.endpoint!==undefined) {
  const e=data.endpoint;
  if(!e||typeof e!=='object'||Array.isArray(e)||Object.keys(e).some(k=>!['api','baseUrl','contextWindow','maxTokens','input'].includes(k))||!apis.has(e.api))throw Error('invalid-protected-endpoint');
  if(e.input!==undefined&&(!Array.isArray(e.input)||!e.input.includes('text')||new Set(e.input).size!==e.input.length||e.input.some(k=>!['text','image'].includes(k))))throw Error('invalid-protected-input');
  if(typeof e.baseUrl!=='string'||e.baseUrl.length>2048)throw Error('invalid-protected-endpoint');
  let url;try{url=new URL(e.baseUrl)}catch{throw Error('invalid-protected-endpoint')}
  if(url.username||url.password||url.hash||!(url.protocol==='https:'||(url.protocol==='http:'&&['127.0.0.1','[::1]','localhost'].includes(url.hostname))))throw Error('invalid-protected-endpoint');
  for(const k of ['contextWindow','maxTokens'])if(!Number.isSafeInteger(e[k])||e[k]<1||e[k]>2000000)throw Error('invalid-protected-model-limit');
  if(e.maxTokens>e.contextWindow)throw Error('invalid-protected-model-limit');
  if(!data.provider||!data.model)throw Error('endpoint-model-required');
 }
 return data;
}
export function guardAuth(raw) {
 if(raw===undefined)return raw;
 if(Buffer.byteLength(raw)>1048576)throw Error('protected-auth-bound');
 const data=decodeObject(raw);
 for(const credential of Object.values(data)) {
  if(!credential||typeof credential!=='object'||Array.isArray(credential)||Object.hasOwn(credential,'env'))throw Error('unsupported-protected-credential');
  if(credential.type==='api_key') {
   if(typeof credential.key!=='string'||!credential.key||credential.key.startsWith('!'))throw Error('command-credentials-unavailable');
  }else if(credential.type!=='oauth'||typeof credential.access!=='string'||typeof credential.refresh!=='string'||!Number.isFinite(credential.expires))throw Error('unsupported-protected-credential');
 }
 return raw;
}
export function guardedAuthBackend(backend) {
 const apply=fn=>raw=>{const result=fn(guardAuth(raw));if(result.next!==undefined)guardAuth(result.next);return result;};
 return {withLock:fn=>backend.withLock(apply(fn)),withLockAsync:(fn,options)=>backend.withLockAsync(async raw=>{const result=await fn(guardAuth(raw));if(result.next!==undefined)guardAuth(result.next);return result;},options)};
}
export function createCredentials(backend) {
 const storage=guardedAuthBackend(backend);
 const transaction=(fn,options)=>storage.withLockAsync(raw=>fn(raw===undefined?{}:decodeObject(raw)),options);
 return {
  read:(provider,options)=>transaction(data=>({result:Object.hasOwn(data,provider)?data[provider]:undefined}),options),
  list:options=>transaction(data=>({result:Object.entries(data).map(([providerId,c])=>({providerId,type:c.type}))}),options),
  modify:(provider,fn,options)=>transaction(async data=>{
   const before=Object.hasOwn(data,provider)?data[provider]:undefined,next=await fn(before);
   return next===undefined?{result:before}:{result:next,next:JSON.stringify({...data,[provider]:next})};
  },options),
  delete:(provider,options)=>transaction(data=>{delete data[provider];return {result:undefined,next:JSON.stringify(data)}},options),
 };
}
export function parseArguments(argv) {
 const options={mode:'tui'};const keys={'--mode':'mode','--provider':'provider','--model':'model','--thinking':'thinkingLevel','--data-dir':'dataDir','--session':'session'};
 const seen=new Set();
 for(let i=0;i<argv.length;i++) {
  const key=Object.hasOwn(keys,argv[i])?keys[argv[i]]:undefined;if(!key||seen.has(key)||typeof argv[i+1]!=='string'||!argv[i+1]||argv[i+1].startsWith('--'))throw Error('unsupported-protected-argument');
  seen.add(key);options[key]=argv[++i];
 }
 if(!['tui','rpc'].includes(options.mode)||options.thinkingLevel&&!levels.has(options.thinkingLevel))throw Error('unsupported-protected-mode');
 return options;
}
