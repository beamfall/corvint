// SPDX-License-Identifier: AGPL-3.0-or-later
import { spawn } from 'node:child_process';

// Each invocation owns a group, including descendants which retain inherited pipes.
export function createCommandRunner({binary,env=process.env,timeoutMs=15000,maxBytes=65536,spawnImpl=spawn}) {
 if(typeof binary!=='string'||!binary||binary.includes('\0')||!Number.isInteger(timeoutMs)||timeoutMs<1||timeoutMs>120000||!Number.isInteger(maxBytes)||maxBytes<1||maxBytes>1048576)throw Error('invalid-runner-options');
 const active=new Set();let closed=false;
 function run({cwd,args,input='',signal}={}) {
  const empty=fault=>Promise.resolve({exitCode:null,stdout:'',stderr:'',fault});
  if(closed)return empty('closed');
  if(!['darwin','linux'].includes(process.platform))return empty('unsupported-platform');
  if(typeof cwd!=='string'||!cwd||cwd.includes('\0')||!Array.isArray(args)||args.length>128||args.some(a=>typeof a!=='string'||a.includes('\0'))||Buffer.byteLength(JSON.stringify(args))>65536||typeof input!=='string'||Buffer.byteLength(input)>maxBytes)return empty('invalid-request');
  if(signal?.aborted)return empty('aborted');
  let stop;const done=new Promise(resolve=>{
   let child,deadline,killTimer,finishTimer,fault,exitCode=null,used=0,settled=false,stopping=false;
   const chunks={stdout:[],stderr:[]};
   const kill=kind=>{if(child?.pid)try{process.kill(-child.pid,kind)}catch(error){if(error.code!=='ESRCH')fault??='cleanup-failed'}};
   const decode=name=>{try{return new TextDecoder('utf-8',{fatal:true}).decode(Buffer.concat(chunks[name]))}catch{fault??='invalid-encoding';return ''}};
   const finish=()=>{if(settled)return;settled=true;clearTimeout(deadline);clearTimeout(killTimer);clearTimeout(finishTimer);signal?.removeEventListener('abort',abort);const stdout=decode('stdout'),stderr=decode('stderr');resolve({exitCode,stdout,stderr,...(fault?{fault}:{})});};
   stop=reason=>{
    fault??=reason;if(stopping)return;stopping=true;clearTimeout(deadline);kill('SIGTERM');
    // A leader's close event is not evidence that its descendants have exited.
    killTimer=setTimeout(()=>{kill('SIGKILL');child?.stdin?.destroy();child?.stdout?.destroy();child?.stderr?.destroy();finishTimer=setTimeout(finish,25)},100);
   };
   const abort=()=>stop('aborted');
   try{child=spawnImpl(binary,args,{cwd,env:{...env},shell:false,detached:true,stdio:['pipe','pipe','pipe']})}catch{fault='spawn-failed';finish();return;}
   const collect=name=>chunk=>{const bytes=Buffer.from(chunk);const remaining=Math.max(0,maxBytes-used);if(remaining)chunks[name].push(bytes.subarray(0,remaining));used+=bytes.length;if(used>maxBytes)stop('output-too-large')};
   child.stdout.on('data',collect('stdout'));child.stderr.on('data',collect('stderr'));
   child.on('error',()=>stop('spawn-failed'));
   child.on('exit',code=>{exitCode=code;stop(undefined)});
   child.on('close',code=>{exitCode??=code;if(!stopping)stop(undefined)});
   child.stdin.on('error',()=>stop('stdin-failed'));
   signal?.addEventListener('abort',abort,{once:true});
   deadline=setTimeout(()=>stop('timeout'),timeoutMs);
   if(signal?.aborted)abort();
   try{child.stdin.end(input)}catch{stop('stdin-failed')}
  });
  const item={done,stop:reason=>stop?.(reason)};active.add(item);done.finally(()=>active.delete(item));return done;
 }
 async function cancel(reason='aborted'){const owned=[...active];for(const item of owned)item.stop(reason);await Promise.all(owned.map(item=>item.done));}
 async function close(){closed=true;await cancel('closed');}
 return {run,close,cancel};
}
