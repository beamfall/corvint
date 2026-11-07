// SPDX-License-Identifier: Apache-2.0
// LPCV-V0-8301: offline CDP transport, real hook and incomplete-note writer.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const hook = process.argv[2], dir = process.argv[3];
let socket, attached, holdTake=false, closeSource=false;
const sourceRequests=[];
class WebSocket {
 constructor() {socket=this;this.closed=false;queueMicrotask(()=>this.onopen());}
 send(raw) {
  const m=JSON.parse(raw);
  if(m.method==='Debugger.getScriptSource') {
   sourceRequests.push(m.params.scriptId);
   if(this.closed)return;
   if(closeSource) {queueMicrotask(()=>this.close());return;}
  }
  if(holdTake&&m.method==='Profiler.takePreciseCoverage')return;
  let result={};
  if(m.method==='Browser.getVersion')result={product:'fixture-chromium'};
  if(m.method==='Profiler.takePreciseCoverage')result={result:[{scriptId:'app',url:'http://localhost/app.js',functions:[]},{scriptId:'anon',url:'',functions:[]},...(closeSource?[{scriptId:'second',url:'http://localhost/second.js',functions:[]}]:[])]};
  if(m.method==='Debugger.getScriptSource')result={scriptSource:'return 7'};
  queueMicrotask(()=>this.onmessage({data:JSON.stringify({id:m.id,result})}));
  if(m.method==='Runtime.runIfWaitingForDebugger')attached();
 }
 close() {if(this.closed)return;this.closed=true;this.onclose();}
}
const sandbox={global:{},module:{exports:{}},process:{pid:777,env:{CORVINT_BROWSERCOV_DIR:dir}},require:name=>name==='worker_threads'?{isMainThread:false}:require(name),WebSocket,
 fetch:async()=>({json:async()=>({webSocketDebuggerUrl:'ws://fixture'})}),URL,setTimeout};
vm.runInNewContext(fs.readFileSync(hook,'utf8')+'\nmodule.exports.fixture={cdpCoverage,mayBeAppCode};',sandbox);
(async()=>{
 const api=sandbox.module.exports.fixture;
 // Anonymous browser scripts remain an explicit unobserved collection limit.
 assert.equal(api.mayBeAppCode(''),false);
 const coverage=await api.cdpCoverage(1);
 const event=(method,params,sessionId)=>socket.onmessage({data:JSON.stringify({method,params,sessionId})});
 const attach=async id=>{
  const ready=new Promise(resolve=>attached=resolve);
  event('Target.attachedToTarget',{sessionId:id,targetInfo:{type:'page'},waitingForDebugger:true});
  await ready;await new Promise(resolve=>setImmediate(resolve));
 };
 await attach('old-renderer');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'old-renderer');
 event('Target.detachedFromTarget',{sessionId:'old-renderer'});
 await attach('new-renderer');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'new-renderer');
 const entries=await coverage.take();
 assert.equal(entries.length,1);assert.equal(entries[0].url,'http://localhost/app.js');
 event('Target.detachedFromTarget',{sessionId:'new-renderer'});
 const notes=fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8').trim().split('\n').map(JSON.parse);
 assert.equal(notes.length,1);assert.equal(notes[0].kind,'cdp-detached');
 await attach('racing-renderer');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'racing-renderer');
 holdTake=true;
 const pendingTake=coverage.take();
 event('Target.detachedFromTarget',{sessionId:'racing-renderer'});
 assert.equal((await pendingTake).length,0,'detached renderer must reject its pending take');
 await attach('disconnected-renderer');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'disconnected-renderer');
 const disconnectedTake=coverage.take();socket.close();
 assert.equal((await disconnectedTake).length,0,'closed transport must reject its pending take');
 const raced=fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8');
 assert.match(raced,/"kind":"cdp-take"/);
 // Close with the first source request pending. A second coverage script
 // must reject locally instead of sending through the dead socket and hanging.
 holdTake=false;
 const sourceCoverage=await api.cdpCoverage(1);
 await attach('source-renderer');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'source-renderer');
 event('Debugger.scriptParsed',{scriptId:'second',url:'http://localhost/second.js'},'source-renderer');
 closeSource=true;sourceRequests.length=0;
 let timer;
 try {
  const entries=await Promise.race([sourceCoverage.take(),new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('source retrieval hung after transport close')),1000)})]);
  assert.equal(entries.length,2);
  assert.ok(entries.every(entry=>entry.source===''));
  assert.deepEqual(sourceRequests,['app'],'no subsequent request may reach the closed socket');
 } finally {clearTimeout(timer);}
 const sourceNotes=fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8').trim().split('\n').map(JSON.parse).filter(n=>n.kind==='cdp-source');
 assert.equal(sourceNotes.length,2);
 assert.ok(sourceNotes.every(n=>n.detail.includes('DevTools connection closed')));
})().catch(error=>{console.error(error);process.exitCode=1});
