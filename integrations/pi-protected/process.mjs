// SPDX-License-Identifier: AGPL-3.0-or-later
import {spawn} from 'node:child_process';
import {readFileSync} from 'node:fs';
const children=new Set();
const witnesses=new Set();
export function ownNativeGroup(path){witnesses.add(path);return ()=>witnesses.delete(path)}
function kill(child){if(child.pid)try{process.kill(-child.pid,'SIGKILL')}catch{}}
function cleanup(){for(const child of children)kill(child);for(const path of witnesses)try{const pid=Number(readFileSync(path,'utf8'));if(Number.isSafeInteger(pid)&&pid>1)kill({pid})}catch{}}
process.once('exit',cleanup);
for(const [signal,code] of [['SIGINT',130],['SIGTERM',143]])process.once(signal,()=>{cleanup();process.exit(code)});
export function run(command,args,options={}) {
 return new Promise((resolve,reject)=>{
  const {timeout=45000,onSpawn,onStdout,signal,...rest}=options;
  const child=spawn(command,args,{...rest,detached:true,stdio:['pipe','pipe','pipe']});
  children.add(child);let stdout='',stderr='',error;
  const abort=()=>{error=Error('owned-process-interrupted');kill(child)};
  const timer=setTimeout(abort,timeout);signal?.addEventListener('abort',abort,{once:true});
  if(signal?.aborted)abort();
  child.on('error',value=>{error=value});
  child.stdout.on('data',data=>{stdout+=data;if(stdout.length>2**20)abort();else try{onStdout?.(data,child)}catch(e){error=e;kill(child)}});
  child.stderr.on('data',data=>{stderr+=data;if(stderr.length>2**20)abort()});
  child.on('close',(code,termination)=>{clearTimeout(timer);signal?.removeEventListener('abort',abort);kill(child);children.delete(child);if(error)reject(error);else resolve({code,termination,stdout,stderr})});
  try{if(onSpawn)onSpawn(child);else child.stdin.end()}catch(e){error=e;kill(child)}
 });
}
