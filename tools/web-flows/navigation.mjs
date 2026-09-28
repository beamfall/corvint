import { spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { ownedLifecycle } from './lifecycle.mjs';
import { navigationPolicy } from './navigation-policy.mjs';
const require=createRequire(import.meta.url);
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const MAX=5*1024*1024;
async function input(){const chunks=[];let size=0;for await(const c of process.stdin){size+=c.length;if(size>MAX+65536)throw Error('input-budget');chunks.push(c);}return JSON.parse(Buffer.concat(chunks));}
async function body(response,max=MAX){const chunks=[];let size=0;for await(const c of response.body??[]){size+=c.length;if(size>max)throw Error('response-budget');chunks.push(c);}return Buffer.concat(chunks);}
const locator=(page,l)=>l.test_id?page.getByTestId(l.test_id):page.getByRole(l.role,{name:l.name,exact:true});
async function visible(page,l,condition='visible'){
 const target=locator(page,l);
 // A missing hidden target is unknown rather than evidence of the expected state.
 await target.waitFor({state:'attached',timeout:1200});
 if(await target.count()!==1)throw Error('ambiguous-locator');
 await target.waitFor({state:condition,timeout:1200});
}
async function main(){
 const incoming=await input();
 const e={schema:'application-navigation-execution-receipt/0',authority:'CALLER_REPORTED',revision:incoming.packet.revision,binding:incoming.binding,runId:incoming.runId,
  packetDigest:incoming.packetDigest,executionDigest:incoming.executionDigest,originsDigest:incoming.originsDigest,providerDigest:'',nodeVersion:process.version,
  playwrightVersion:'',browser:'',status:'incomplete',steps:[],traffic:[],gaps:[],browserClosed:false,serverExited:false,cleanup:false};
 const policy=navigationPolicy(incoming,e), lifecycle=ownedLifecycle(e,(_e,reason)=>policy.gap(reason));
 const m=incoming.manifest;
 const local=p=>new URL(p,m.origin).href;
 async function fetchGuarded(url,method='GET',data,headers){
  if(!policy.request(url,method))throw Error('request-blocked');
  if(data?.length>8192){policy.gap('request-blocked');throw Error('request-budget');}
  const r=await fetch(url,{method,body:['GET','HEAD'].includes(method)?undefined:data,headers,redirect:'manual',signal:AbortSignal.timeout(2000)});
  if(r.status>=300&&r.status<400){policy.gap('request-blocked');throw Error('redirect-denied');}
  return r;
 }
 async function identity(){
  const r=await fetchGuarded(local(m.identityPath));
  const value=JSON.parse((await body(r,65536)).toString());
  return r.status===200&&value.runId===incoming.runId&&value.frontendDigest===incoming.binding.frontendDigest&&value.backendDigest===incoming.binding.backendDigest&&value.fixture===incoming.binding.fixture;
 }
 try {
  const {chromium}=await import('playwright');
  e.playwrightVersion=require('playwright/package.json').version;
  if(e.playwrightVersion!=='1.63.0')throw Error('runtime-unqualified');
  const child=spawn(m.server[0],m.server.slice(1),{cwd:incoming.root,env:{PATH:process.env.PATH,HOME:process.env.HOME,TMPDIR:process.env.TMPDIR,CORVINT_FLOW_RUN_ID:incoming.runId},stdio:'ignore'});
  lifecycle.ownServer(child);child.on('error',()=>policy.gap('execution-failed'));
  let ready=false;
  for(let i=0;i<100;i++){if(child.exitCode!==null||child.signalCode!==null||lifecycle.stopping)break;try{ready=await identity();break;}catch{await sleep(50);}}
  if(!ready)throw Error('identity-unavailable');
  const browser=await lifecycle.launch(()=>chromium.launch({headless:true,args:['--force-webrtc-ip-handling-policy=disable_non_proxied_udp']}));
  if(lifecycle.stopping)throw Error('interrupted');
  e.browser=`chromium-${browser.version()}`;
  const expected=require(join(dirname(require.resolve('playwright-core/package.json')),'browsers.json')).browsers.find(b=>b.name==='chromium').browserVersion;
  if(browser.version()!==expected)throw Error('browser-unqualified');
  const context=await browser.newContext({serviceWorkers:'block',acceptDownloads:false});
  context.setDefaultTimeout(1500);
  let page;
  try {
   await context.route('**/*',async route=>{
    try {
     const request=route.request();
     if(!page || request.frame()!==page.mainFrame() || context.pages().length!==1)throw Error('unattributed-request');
     const r=await fetchGuarded(request.url(),request.method(),request.postDataBuffer(),request.headers());
     const bytes=await body(r),headers=Object.fromEntries(r.headers);delete headers['content-length'];delete headers['content-encoding'];
     await route.fulfill({status:r.status,headers,body:bytes});
    }catch{policy.gap('request-blocked');await route.abort().catch(()=>{});}
   });
   await context.routeWebSocket('**/*',ws=>{policy.gap('websocket-blocked');ws.close();});
   await context.exposeBinding('__corvintForm',async(source,url,method)=>{if(!page||source.frame!==page.mainFrame()){policy.gap('form-blocked');return false;}return policy.request(url,method,true);});
   // Await admission before redispatch, so GET forms and direct submit() cannot race the guard.
   await context.addInitScript(()=>{
    const submit=HTMLFormElement.prototype.submit, requestSubmit=HTMLFormElement.prototype.requestSubmit;
    const admitted=new WeakSet();
    const approve=form=>window.__corvintForm(new URL(form.action||location.href,location.href).href,(form.method||'GET').toUpperCase());
    document.addEventListener('submit',event=>{
     const form=event.target;
     if(admitted.delete(form))return;
     event.preventDefault();event.stopImmediatePropagation();
     void approve(form).then(ok=>{if(ok){admitted.add(form);requestSubmit.call(form,event.submitter||undefined);}});
    },true);
    HTMLFormElement.prototype.submit=function(){void approve(this).then(ok=>{if(ok)submit.call(this);});};
    HTMLFormElement.prototype.requestSubmit=function(...args){return requestSubmit.apply(this,args);};
   });
   page=await context.newPage();
   page.on('dialog',d=>d.dismiss().catch(()=>{}));page.on('download',d=>{policy.gap('request-blocked');void d.cancel();});
   context.on('page',other=>{if(other!==page){policy.gap('request-blocked');void other.close();}});
   const executable=new Map(incoming.execution.steps.map(s=>[`${s.flow_id}/${s.step_id}`,s]));
   async function step(t,recovery=false){
    const result={flow_id:t.flow_id,step_id:t.step_id,outcome:'failed',reason:'step-failed',verification:t.verification};e.steps.push(result);
    policy.activate(t);
    try {
     const x=executable.get(`${t.flow_id}/${t.step_id}`),route=t.state.slice(3);
     policy.preflight();
     if(x.operation==='navigate')await page.goto(local(route),{waitUntil:'domcontentloaded'});
     else if(page.url()!==local(route))throw Error('state-mismatch');
     policy.preflight();await visible(page,t.ready);
     policy.preflight();
     if(x.operation==='fill')await locator(page,t.locator).fill(incoming.fixtures[t.input_fixture]);
     if(x.operation==='click')await locator(page,t.locator).click();
     for(const ob of x.observations){policy.preflight();await visible(page,ob.locator,ob.condition);}
     policy.preflight();
     result.outcome=recovery?'recovered':'passed';result.reason='none';return true;
    }catch{if(policy.failed)result.reason='request-blocked';return false;}
    finally{policy.deactivate();}
   }
   let complete=true;
   for(const t of incoming.packet.steps){
    if(!await step(t)){
     complete=false;
     const recovery=incoming.packet.recovery.find(r=>r.flow_id===t.flow_id&&r.step_id===t.recovery);
     if(recovery&&!policy.failed)await step(recovery,true);
     break;
    }
   }
   policy.deactivate();
   if(!policy.failed&&!await identity()){policy.gap('identity-drift');complete=false;}
   if(complete&&!policy.failed)e.status='passed';
  }finally{policy.deactivate();await context.close();}
 }catch{policy.gap('execution-failed');}
 finally{await lifecycle.close();lifecycle.dispose();}
 if(policy.failed||!e.browserClosed||!e.serverExited)e.status='incomplete';
 const output=JSON.stringify(e);if(Buffer.byteLength(output)>MAX)throw Error('output-budget');process.stdout.write(output+'\n');
}
main().catch(()=>{process.stderr.write('navigation observer failed\n');process.exitCode=2;});
