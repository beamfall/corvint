import assert from 'node:assert/strict';
import {spawn,execFileSync} from 'node:child_process';
import {mkdtemp,writeFile,copyFile,readFile,mkdir,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createServer,createConnection} from 'node:net';
import {createServer as createHTTPServer} from 'node:http';
const core=process.env.CORVINT_CORE_BIN,observer=process.env.CORVINT_FLOW_BIN;
if(!core||!observer)throw Error('compiled binaries required');
const assets=fileURLToPath(new URL('..',import.meta.url)),fixture=fileURLToPath(new URL('./navigation-fixture/',import.meta.url));
const children=new Set(),servers=new Set(),sockets=new Set(),directories=[];let interrupted=false;
const stop=()=>{interrupted=true;for(const c of children)c.kill('SIGTERM');for(const socket of sockets)socket.destroy();for(const server of servers)server.close();};
process.once('SIGINT',stop);process.once('SIGTERM',stop);process.on('exit',stop);
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
function run(command,args,cwd){return new Promise((resolve,reject)=>{
 if(interrupted)return reject(Error('interrupted'));
 const c=spawn(command,args,{cwd,stdio:['ignore','pipe','pipe']});children.add(c);
 let out='',err='';const timer=setTimeout(()=>c.kill('SIGTERM'),30000);
 c.stdout.on('data',b=>{out+=b;if(out.length>6*1024*1024)c.kill('SIGTERM');});c.stderr.on('data',b=>{err+=b;});
 c.once('error',reject);c.once('close',code=>{clearTimeout(timer);children.delete(c);resolve({code,out,err});});
 });}
const commit=root=>execFileSync('git',['-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qam','fixture'],{cwd:root,stdio:'ignore'});
async function prepare({mode='good',effect='write-irreversible',origin=true,externalOrigin='http://127.0.0.1:1'}={}){
 const root=await mkdtemp(join(tmpdir(),'corvint-navigation-'));directories.push(root);
 await mkdir(join(root,'flows'));await mkdir(join(root,'execution'));
 for(const f of ['server.cjs','index.html'])await copyFile(join(fixture,f),join(root,f));
 await writeFile(join(root,'flow.spec.ts'),"test('fixture', async () => {});\n");
 const server=createServer();servers.add(server);await new Promise(r=>server.listen(0,'127.0.0.1',r));const port=server.address().port;await new Promise(r=>server.close(r));servers.delete(server);
 const manifest={profile:'application-flow-intent/0',application:'fixture',origin:`http://127.0.0.1:${port}`,sources:['index.html','server.cjs'],tests:['flow.spec.ts'],backendSource:'server.cjs',fixture:'navigation',identityPath:'/identity',resetPath:'/reset',server:[process.execPath,'server.cjs',String(port),mode,externalOrigin],scenarios:[{id:'unused',role:'editor',path:'/',basis:'declared',actions:[],checks:[{id:'unused',kind:'visible',selector:'#unused',want:true}]}]};
 const ids=['open','fill','save'],expected=['ready','filled','saved'];
 const flow={schema:'application-flow-intent/1',flow_id:'save-item',revision:1,kind:'ui',actor:'editor',preconditions:[],steps:ids.map(step_id=>({step_id,action:'Prose is never executed'})),outcomes:expected.map(outcome_id=>({outcome_id,behavior:'Declared fixture outcome',matcher:'toBeVisible',locator:'#declared',value:'visible'})),variations:[{variation_id:'happy',preconditions:[],steps:ids,observable_facts:['saved'],outcomes:expected,projects:['chromium']}],links:[],navigation:{precondition_flows:[],steps:ids.map((step_id,i)=>({step_id,state:'/',locator:i===1?{role:'textbox',name:'Item'}:{test_id:i===0?'ready':'save'},ready:{test_id:'ready'},...(i===1?{input_fixture:'item'}:{}),expect:[expected[i]],effect:i===2?effect:'read'}))}};
 const execution={schema:'application-navigation-execution-input/0',steps:ids.map((step_id,i)=>({flow_id:'save-item',step_id,operation:['navigate','fill','click'][i],observations:[{outcome_id:expected[i],condition:'visible',locator:{test_id:i===2?'saved':'ready'}}]}))};
 await writeFile(join(root,'manifest.json'),JSON.stringify(manifest));await writeFile(join(root,'flows/save-item.json'),JSON.stringify(flow));await writeFile(join(root,'execution/steps.json'),JSON.stringify(execution));
 if(origin)await writeFile(join(root,'flows/origins.json'),JSON.stringify({schema:'application-flow-origins/0',disposable:[manifest.origin]}));
 execFileSync('git',['init','-q'],{cwd:root});execFileSync('git',['add','.'],{cwd:root});commit(root);
 await writeFile(join(root,'fixtures.json'),JSON.stringify({item:'private-navigation-value'}));
 return {root,manifest,flow,execution};
}
async function packet(root,max='write-irreversible'){
 const r=await run(core,['--root',root,'flows','navigate','--flows','flows','--goal','save-item','--max-effect',max],root);assert.equal(r.code,0,r.err);await writeFile(join(root,'packet.json'),r.out);
}
function args(root,max='write-irreversible'){return ['--experimental','--trusted-local','--observe','--root',root,'--manifest','manifest.json','--assets',assets,'--flows','flows','--navigation',join(root,'packet.json'),'--execution','execution/steps.json','--fixtures',join(root,'fixtures.json'),'--max-effect',max];}
async function writes(root){try{return (await readFile(join(root,'writes.log'),'utf8')).trim().split('\n');}catch{return [];}}
async function update(f){
 await writeFile(join(f.root,'flows/save-item.json'),JSON.stringify(f.flow));
 await writeFile(join(f.root,'execution/steps.json'),JSON.stringify(f.execution));
 execFileSync('git',['add','flows','execution'],{cwd:f.root});commit(f.root);
}
function descendants(pid){
 const rows=execFileSync('ps',['-axo','pid=,ppid=,comm='],{encoding:'utf8'}).trim().split('\n').map(line=>/^\s*(\d+)\s+(\d+)\s+(.+)$/.exec(line)).filter(Boolean).map(m=>({pid:Number(m[1]),parent:Number(m[2]),name:m[3]}));
 const ids=new Set([pid]);for(let i=0;i<10;i++)for(const row of rows)if(ids.has(row.parent))ids.add(row.pid);return rows.filter(row=>row.pid!==pid&&ids.has(row.pid));
}
async function interrupt(f,signal){
 const argv=args(f.root);if(signal==='timeout')argv.push('--timeout','1800ms');
 const child=spawn(observer,argv,{cwd:f.root,stdio:['ignore','pipe','pipe']});children.add(child);
 let out='',err='';child.stdout.on('data',b=>out+=b);child.stderr.on('data',b=>err+=b);
 const done=new Promise(resolve=>child.once('close',code=>{children.delete(child);resolve(code);}));
 let owned=[];
 try{
  const deadline=Date.now()+1200;
  while(Date.now()<deadline){owned=descendants(child.pid);if(owned.some(p=>/chrome|chromium/i.test(p.name)))break;await sleep(20);}
  assert.ok(owned.some(p=>/chrome|chromium/i.test(p.name)),'real browser must start before interruption');await sleep(150);owned=descendants(child.pid);assert.ok(owned.some(p=>/chrome|chromium/i.test(p.name)));
  if(signal!=='timeout')child.kill(signal);
  const code=await Promise.race([done,sleep(5000).then(()=>{throw Error('interruption deadline');})]);assert.notEqual(code,0);assert.ok(!out.includes('"status":"passed"'));if(signal==='timeout')assert.match(err,/timed out/);
  for(const p of owned){let present=true;for(let i=0;i<60;i++){try{process.kill(p.pid,0);}catch{present=false;break;}await sleep(20);}assert.equal(present,false,`descendant ${p.pid} survived ${signal}`);}
  return owned.length;
 }finally{child.kill('SIGTERM');await done;}
}
async function sentinel(){
 let http=0,upgrade=0;
 const server=createHTTPServer((_req,res)=>{http++;res.writeHead(200,{'access-control-allow-origin':'*'});res.end('sentinel');});
 servers.add(server);server.on('connection',socket=>{sockets.add(socket);socket.once('close',()=>sockets.delete(socket));});
 server.on('upgrade',(_request,socket)=>{upgrade++;socket.destroy();});
 await new Promise(r=>server.listen(0,'127.0.0.1',r));const port=server.address().port,origin=`http://127.0.0.1:${port}`;
 assert.equal(await (await fetch(origin+'/calibrate')).text(),'sentinel');
 await new Promise((resolve,reject)=>{
  const socket=createConnection(port,'127.0.0.1');sockets.add(socket);
  const timer=setTimeout(()=>{socket.destroy();reject(Error('sentinel upgrade calibration timeout'));},1000);
  socket.once('connect',()=>socket.write('GET /calibrate HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n'));
  socket.once('error',reject);socket.once('close',()=>{clearTimeout(timer);sockets.delete(socket);resolve();});
 });
 assert.equal(http,1);assert.equal(upgrade,1);http=0;upgrade=0;
 return {origin,counts:()=>({http,upgrade})};
}
try {
 const external=await sentinel();
 const good=await prepare();await packet(good.root);
 const result=await run(observer,args(good.root),good.root);assert.equal(result.code,0,result.err+' '+result.out);
 const receipt=JSON.parse(result.out);assert.equal(receipt.status,'passed');assert.equal(receipt.steps.length,3);assert.equal(receipt.cleanup,true);assert.deepEqual(await writes(good.root),['/write']);assert.ok(!result.out.includes('private-navigation-value'));
 await packet(good.root,'read');await rm(join(good.root,'writes.log'));
 const denied=await run(observer,args(good.root,'read'),good.root);assert.equal(denied.code,2);assert.deepEqual(await writes(good.root),[]);
 const hidden=await prepare({effect:'read'});await packet(hidden.root,'read');
 const blocked=await run(observer,args(hidden.root,'read'),hidden.root);assert.equal(blocked.code,1,blocked.err+' '+blocked.out);assert.equal(JSON.parse(blocked.out).status,'incomplete');assert.deepEqual(await writes(hidden.root),[]);
 const cases=['compiled-goal','default-read-zero-writes','declared-read-post-zero-writes'];
 for(const mode of ['absent','uncommitted','unlisted']){
  const f=await prepare({origin:mode==='unlisted'});
  if(mode!=='absent')await writeFile(join(f.root,'flows/origins.json'),JSON.stringify({schema:'application-flow-origins/0',disposable:[mode==='unlisted'?'http://127.0.0.1:9':f.manifest.origin]}));
  if(mode==='unlisted')commit(f.root);
  await packet(f.root);const r=await run(observer,args(f.root),f.root);assert.equal(r.code,2,r.out);assert.deepEqual(await writes(f.root),[]);cases.push('origin-'+mode);
 }
 for(const mode of ['background','redirect','cross','websocket']){
  const f=await prepare({mode,effect:'read',externalOrigin:external.origin});await packet(f.root,'read');const r=await run(observer,args(f.root,'read'),f.root);assert.equal(r.code,1,r.err+' '+r.out);assert.deepEqual(await writes(f.root),[]);assert.deepEqual(external.counts(),{http:0,upgrade:0},mode+' escaped before refusal');cases.push(mode+'-blocked');
 }
 for(const target of ['form','direct']){
  const f=await prepare({effect:'read'});f.flow.navigation.steps[2].locator={test_id:target};await update(f);await packet(f.root,'read');
  const r=await run(observer,args(f.root,'read'),f.root);assert.equal(r.code,1,r.err+' '+r.out);assert.deepEqual(await writes(f.root),[]);cases.push('get-'+target+'-blocked');
 }
 for(const [mode,byRole,expectedCode] of [['good',false,0],['good',true,0],['hidden-remove',false,1],['hidden-missing',false,1]]){
  const f=await prepare({mode});f.execution.steps[2].observations=[{outcome_id:'saved',condition:'hidden',locator:byRole?{role:'button',name:'Hidden status'}:{test_id:'hidden-target'}}];
  await update(f);await packet(f.root);const r=await run(observer,args(f.root),f.root);assert.equal(r.code,expectedCode,r.err+' '+r.out);
  cases.push('hidden-'+mode+(byRole?'-role':'-test-id'));
 }
 const recovered=await prepare({mode:'recover'});
 recovered.flow.steps.push({step_id:'recover',action:'Recovery prose'});
 recovered.flow.navigation.steps[2].recovery='recover';
 recovered.flow.navigation.steps.push({step_id:'recover',state:'/',locator:{test_id:'recovered'},ready:{test_id:'recovered'},expect:['ready'],effect:'read'});
 recovered.execution.steps.push({flow_id:'save-item',step_id:'recover',operation:'observe',observations:[{outcome_id:'ready',condition:'visible',locator:{test_id:'recovered'}}]});
 await update(recovered);await packet(recovered.root);const recoveryResult=await run(observer,args(recovered.root),recovered.root);assert.equal(recoveryResult.code,1,recoveryResult.err);assert.equal(JSON.parse(recoveryResult.out).steps.at(-1).outcome,'recovered');cases.push('bounded-recovery');
 const pre=await prepare();
 const setup=structuredClone(pre.flow);setup.flow_id='setup';setup.variations[0].variation_id='setup.happy';setup.steps=setup.steps.slice(0,1);setup.navigation.steps=setup.navigation.steps.slice(0,1);setup.variations[0].steps=['open'];setup.variations[0].outcomes=['ready'];
 pre.flow.steps=pre.flow.steps.slice(1);pre.flow.navigation.steps=pre.flow.navigation.steps.slice(1);pre.flow.navigation.precondition_flows=['setup'];pre.flow.variations[0].steps=['fill','save'];
 pre.execution.steps[0].flow_id='setup';await writeFile(join(pre.root,'flows/setup.json'),JSON.stringify(setup));await update(pre);await packet(pre.root);
 const preResult=await run(observer,args(pre.root),pre.root);assert.equal(preResult.code,0,preResult.err+' '+preResult.out);assert.equal(JSON.parse(preResult.out).steps[0].flow_id,'setup');cases.push('precondition-order');
 const unknown=await prepare();await packet(unknown.root);await writeFile(join(unknown.root,'fixtures.json'),'{}');const missing=await run(observer,args(unknown.root),unknown.root);assert.equal(missing.code,2);assert.deepEqual(await writes(unknown.root),[]);cases.push('unknown-fixture');
 const slow=await prepare({mode:'slow'});await packet(slow.root);
 const cleanup={};for(const signal of ['SIGINT','SIGTERM','timeout'])cleanup[signal]=await interrupt(slow,signal);
 console.log(JSON.stringify({navigation:'PASS',cases,externalAttempts:external.counts(),cleanup}));
}finally{stop();await Promise.all([...children].map(c=>new Promise(r=>c.once('close',r))));for(const d of directories)await rm(d,{recursive:true,force:true});}
