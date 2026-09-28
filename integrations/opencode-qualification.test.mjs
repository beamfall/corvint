import assert from 'node:assert/strict'
import {createHash} from 'node:crypto'
import {spawn,spawnSync} from 'node:child_process'
import {cpSync,mkdirSync,mkdtempSync,readFileSync,writeFileSync,readdirSync,lstatSync,rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {dirname,join} from 'node:path'
import {fileURLToPath,pathToFileURL} from 'node:url'
import test,{after} from 'node:test'

const here=dirname(fileURLToPath(import.meta.url))
const hash=bytes=>createHash('sha256').update(bytes).digest('hex')
const producerArchitecture=()=>{const p=spawnSync('python3',['-B','-c',"import sys;sys.path.insert(0,sys.argv[1]);from opencode_qualification import node_architecture;print(node_architecture())",join(here,'../script')],{encoding:'utf8'});assert.equal(p.status,0,p.stderr);return p.stdout.trim()}
const fixture=JSON.parse(readFileSync(join(here,'fixtures/opencode-native-report.json'),'utf8'))
const tests=['TestHostAdapterJavaScriptHosts','TestHostAdapterJavaScriptHarnessInterruption']
const passed={exitCode:0,stdout:tests.flatMap(Test=>['run','pass'].map(Action=>JSON.stringify({Action,Test,Package:'github.com/Beamfall/corvint/cmd/corvint'}))).join('\n'),stderr:''}
const python=`import json,sys
sys.path.insert(0,sys.argv[1])
import opencode_qualification as q
x=json.load(sys.stdin)
q.begin_record(x['record'],x['backup'])
r=q.build_record(x['native'],x['focused'],x['before'],x['after'])
if x.get('interrupt'):
 def interrupt(*args): raise KeyboardInterrupt()
 q.os.replace=interrupt
q.write_record(x['record'],r)
`

async function setup(t){
 const root=mkdtempSync(join(tmpdir(),'corvint-qualification-handoff-'));t.after(()=>rmSync(root,{recursive:true,force:true}))
 const pkg=join(root,'integrations/opencode');mkdirSync(dirname(pkg),{recursive:true});cpSync(join(here,'opencode'),pkg,{recursive:true})
 const sourceFiles=Object.fromEntries(readdirSync(pkg,{recursive:true}).filter(p=>lstatSync(join(pkg,p)).isFile()).sort().map(p=>['integrations/opencode/'+p,hash(readFileSync(join(pkg,p)))]))
 const native=structuredClone(fixture)
 Object.assign(native,{sourceFiles,hostSHA256:hash(readFileSync(process.execPath)),corvintSHA256:hash(readFileSync(process.execPath)),os:process.platform,architecture:producerArchitecture()})
 const tuple=Object.fromEntries(['adapterVersion','hostVersion','os','architecture'].map(k=>[k,native[k]]))
 const before={...Object.fromEntries(['sourceCommit','sourceFiles','hostSHA256','corvintSHA256'].map(k=>[k,native[k]])),tuple,inputs:{'script/qualify-opencode-native.py':native.harnessSHA256}}
 const record=join(root,'integrations/opencode-qualification.json')
 const payload={record,backup:join(root,'previous.json'),native,focused:structuredClone(passed),before,after:structuredClone(before)}
 const produce=()=>spawnSync('python3',['-B','-c',python,join(here,'../script')],{input:JSON.stringify(payload),encoding:'utf8',timeout:10000})
 const {qualificationStatus}=await import(pathToFileURL(join(pkg,'src/qualification.js')).href)
 const options={root,hostVersion:'2.0.18',corvintBinary:process.execPath,environment:process.env}
 return {root,pkg,payload,record,produce,options,status:(extra={})=>qualificationStatus({...options,...extra})}
}

test('AHI-032 native report producer publishes the exact record consumed by corvint_status',async t=>{
 const f=await setup(t);assert.equal((await f.status()).integrationSupport,'UNQUALIFIED')
 const result=f.produce();assert.equal(result.status,0,result.stderr)
 const record=JSON.parse(readFileSync(f.record,'utf8'));assert.equal(record.conformance.compaction,'PASS')
 assert.equal((await f.status()).integrationSupport,'FULL')
 assert.equal((await f.status()).executionAuthority,'NONE');assert.equal((await f.status()).frontier,'UNAVAILABLE');assert.equal((await f.status()).legacyReceiptSupport,'FALLBACK')
 assert.equal((await f.status({hostVersion:'2.0.19'})).integrationSupport,'UNQUALIFIED')
 record.hostSHA256='0'.repeat(64);writeFileSync(f.record,JSON.stringify(record));assert.equal((await f.status()).integrationSupport,'UNQUALIFIED')
 assert.equal(f.produce().status,0);writeFileSync(join(f.pkg,'README.md'),'changed package');assert.equal((await f.status()).integrationSupport,'UNQUALIFIED')
})

test('AHI-032 failed, missing, skipped or stale gate evidence invalidates previous qualification',async t=>{
 for(const mutate of [
  p=>{delete p.native.checks['native-compaction']},p=>{p.native.checks.latency=false},p=>{p.native.checks.latency='PASS'},
  p=>{p.focused=null},p=>{p.focused.exitCode=1},p=>{p.focused.stdout=''},p=>{p.focused.stdout=p.focused.stdout.replace('"pass"','"skip"')},
  p=>{p.after.hostSHA256='0'.repeat(64)},p=>{p.after.inputs['script/qualify-opencode-native.py']='0'.repeat(64)},
  p=>{p.native.hostVersion='2.0.19'},p=>{p.native.sourceFiles={}},
 ]){
  const f=await setup(t);const first=f.produce();assert.equal(first.status,0,first.stderr);mutate(f.payload)
  assert.notEqual(f.produce().status,0);assert.equal((await f.status()).integrationSupport,'UNQUALIFIED')
  assert.equal(JSON.parse(readFileSync(f.payload.backup,'utf8')).result,'PASS')
 }
})

test('AHI-032 interrupted atomic publication never exposes a partial or previous FULL record',async t=>{
 const f=await setup(t);const first=f.produce();assert.equal(first.status,0,first.stderr)
 f.payload.interrupt=true;assert.notEqual(f.produce().status,0)
 assert.equal(JSON.parse(readFileSync(f.record,'utf8')).result,'INCOMPLETE')
 assert.equal((await f.status()).integrationSupport,'UNQUALIFIED')
 assert.deepEqual(readdirSync(dirname(f.record)).filter(n=>n.startsWith('.opencode-qualification-')),[])
})

const activeRunners=new Set()
const stopRunners=()=>{for(const p of activeRunners)p.kill('SIGTERM')}
process.on('SIGINT',stopRunners);process.on('SIGTERM',stopRunners)
after(()=>{stopRunners();process.off('SIGINT',stopRunners);process.off('SIGTERM',stopRunners)})

test('AHI-032 qualification command interruption reaps its detached gate descendant',async t=>{
 const root=mkdtempSync(join(tmpdir(),'corvint-qualification-command-'));t.after(()=>rmSync(root,{recursive:true,force:true}))
 const witness=join(root,'witness.json')
 const childCode=`import json,os,subprocess,sys,time\nfrom pathlib import Path\np=subprocess.Popen([sys.executable,'-c','import time;time.sleep(60)'],start_new_session=True)\nPath(${JSON.stringify(witness)}).write_text(json.dumps({'child':p.pid}))\ntime.sleep(60)`
 const runnerCode=`import importlib.util,os,sys\nfrom pathlib import Path\nsys.path.insert(0,sys.argv[1])\ns=importlib.util.spec_from_file_location('qualify',sys.argv[1]+'/qualify-opencode.py')\nm=importlib.util.module_from_spec(s);s.loader.exec_module(m)\nm.CommandRunner().run([sys.executable,'-c',sys.argv[3]],Path(sys.argv[2]),os.environ,Path(sys.argv[2])/'gate',timeout=60)`
 const p=spawn('python3',['-B','-c',runnerCode,join(here,'../script'),root,childCode],{stdio:'ignore'});activeRunners.add(p)
 const ended=new Promise(resolve=>p.on('close',code=>{activeRunners.delete(p);resolve(code)}))
 t.after(async()=>{if(p.exitCode===null){p.kill('SIGTERM');await ended}})
 let pid
 for(let i=0;i<1000&&!pid;i++){
  try{pid=JSON.parse(readFileSync(witness,'utf8')).child}catch{}
  if(!pid)await new Promise(r=>setTimeout(r,10))
 }
 assert.ok(pid,'gate descendant never started');p.kill('SIGTERM');assert.equal(await ended,143)
 const live=()=>{const r=spawnSync('ps',['-p',String(pid),'-o','stat='],{encoding:'utf8'});return r.status===0&&r.stdout.trim()&&!r.stdout.trim().startsWith('Z')}
 for(let i=0;i<100&&live();i++)await new Promise(r=>setTimeout(r,20))
 assert.ok(!live(),'owned descendant survived qualification interruption')
})


test('AHI-032 Python platform names map to the architecture accepted by the Node consumer',()=>{
 const code="import sys,json;sys.path.insert(0,sys.argv[1]);from opencode_qualification import node_architecture;print(json.dumps([node_architecture(x) for x in ['x86_64','AMD64','aarch64','arm64']]))"
 const result=spawnSync('python3',['-B','-c',code,join(here,'../script')],{encoding:'utf8'})
 assert.equal(result.status,0,result.stderr);assert.deepEqual(JSON.parse(result.stdout),['x64','x64','arm64','arm64'])
 assert.equal(producerArchitecture(),process.arch)
})
