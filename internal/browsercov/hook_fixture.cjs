// SPDX-License-Identifier: Apache-2.0
// LPCV-V0-8301: offline CDP transport, real hook and incomplete-note writer.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const hook = process.argv[2], dir = process.argv[3];
let socket, attached, holdTake=false;
class WebSocket {
 constructor() {socket=this;queueMicrotask(()=>this.onopen());}
 send(raw) {
  const m=JSON.parse(raw);
  if(holdTake&&m.method==='Profiler.takePreciseCoverage')return;
  let result={};
  if(m.method==='Browser.getVersion')result={product:'fixture-chromium'};
  if(m.method==='Profiler.takePreciseCoverage')result={result:[{scriptId:'app',url:'http://localhost/app.js',functions:[]},{scriptId:'anon',url:'',functions:[]}]};
  if(m.method==='Debugger.getScriptSource')result={scriptSource:'return 7'};
  queueMicrotask(()=>this.onmessage({data:JSON.stringify({id:m.id,result})}));
  if(m.method==='Runtime.runIfWaitingForDebugger')attached();
 }
 close() {this.onclose();}
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
 // LPCV-V0-8301 review fix: a successful take clears untaken, but parsed app
 // code can run again; losing the transport afterwards is still incomplete,
 // and every later take refuses instead of reporting nothing as complete.
 holdTake=false;
 const resumed=await api.cdpCoverage(1);
 await attach('already-parsed');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'already-parsed');
 assert.equal((await resumed.take()).length,1);
 const beforeLoss=fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8');
 socket.close();
 assert.match(fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8').slice(beforeLoss.length),/"kind":"cdp-detached"/);
 await assert.rejects(resumed.take(),/DevTools connection closed/);
 // Control: an intentional close after the final take records no loss.
 const intentional=await api.cdpCoverage(1);
 await attach('intentional');
 event('Debugger.scriptParsed',{scriptId:'app',url:'http://localhost/app.js'},'intentional');
 assert.equal((await intentional.take()).length,1);
 const beforeClose=fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8');
 intentional.close();
 assert.equal(fs.readFileSync(path.join(dir,'incomplete-777.jsonl'),'utf8'),beforeClose);
})().catch(error=>{console.error(error);process.exitCode=1});
