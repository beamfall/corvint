import test from "node:test"
import assert from "node:assert/strict"
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, symlinkSync, linkSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { createCorvintRunner, cockpitReadArguments } from "./src/runtime.js"
import { collectCockpit, projectCockpit, inspectProof, readPrivateFile, checkStatus } from "./src/cockpit.js"
import { invalidateInspection, INSPECTOR_RPC } from "./src/inspector.js"
const oid = "a".repeat(40), other = "b".repeat(40), key = "c".repeat(64), digest = "d".repeat(64)
const affected = () => ({ ok: true, mutates: false, profile: "affected-plan/0", revision: oid, plan: { dirty: ["math/add.go", "untracked.py"], scope: "UNKNOWN", selected: [{ unitId: "go:math", tests: ["math/add_test.go"], witness: { dirtyPath: "math/add.go", kind: "REVERSE_IMPORT", via: ["go:math", "go:caller"] } }], unknown: [{ reason: "LANGUAGE_FRONTIER", detail: "untracked.py" }] }, range: { base: other, paths: ["math/add.go"] }, advice: { checks: [{ kind: "mandatory", command: "false; touch SHOULD_NOT_EXIST", source: "AGENTS.md", reason: "declaration" }], unknown: [] } })
function fixture(t) {
 const root=mkdtempSync(path.join(tmpdir(),"corvint-cockpit-"));t.after(()=>rmSync(root,{recursive:true,force:true}))
 const gitdir=path.join(root,".git"), directory=path.join(gitdir,"corvint/local-completion"), generation=path.join(directory,key,digest+"-001")
 mkdirSync(generation,{recursive:true});writeFileSync(path.join(directory,"owner"),key+"\n")
 const stdout=path.join(generation,"check-001-0.log"),stderr=path.join(generation,"check-001-1.log")
 writeFileSync(stdout,"REAL_RECORDED_OUTPUT\n\x1b[31mnot a command\u202e\n");writeFileSync(stderr,"")
 const repo={ok:true,root,gitdir,target:oid,tree:other}
 const policy={lifecycle:"active",satisfied:false,unmet:["reports-not-produced"],base:other,planDigest:digest,intents:["intent.md"],checks:[{id:"unit",qualified:true,argv:["echo","inert; NOT_EXECUTED"],testedCommit:oid,currentTarget:oid,exit:0,cancelled:false,timedOut:false,secretScreened:false,stdout,stderr}]}
 const run=async read=>read.kind==="repository"?repo:read.kind==="resolve"?{ok:true,commit:other}:read.kind==="affected"?affected():{ok:true,mutates:false,profile:"corvint-local-completion/0",tool:"dogfood-status",policy:structuredClone(policy)}
 return {root,gitdir,directory,generation,stdout,stderr,repo,policy,run}
}

test("AHI-034 closed read argv cannot execute client commands or providers",()=>{
 assert.equal(cockpitReadArguments("/repo",{kind:"verify",argv:["sh","-c","id"]}),undefined)
 assert.equal(cockpitReadArguments("/repo",{kind:"affected",base:"HEAD;id"}),undefined)
 assert.equal(cockpitReadArguments("/repo",{kind:"completion",key:"../owner"}),undefined)
 assert.equal(cockpitReadArguments("/repo",{kind:"resolve",ref:"main\n--output=x"}),undefined)
 assert.deepEqual(cockpitReadArguments("/repo",{kind:"resolve",ref:"--output=evil"}).args.slice(-2),["--end-of-options","--output=evil^{commit}"])
 for(const name of ["cockpitRefresh","cockpitSnapshot","cockpitProof"])assert.equal(INSPECTOR_RPC.methods[name].input.additionalProperties,false)
 assert.equal(INSPECTOR_RPC.methods.cockpitProof.input.properties.path,undefined)
})
test("AHI-034 affected paths and witness chains stay advisory; qualified check does not complete workflow",t=>{
 const f=fixture(t),view=projectCockpit(f.repo,other,affected(),f.policy)
 assert.deepEqual(view.files,["math/add.go","untracked.py"])
 assert.deepEqual(view.impacts[0].via,["go:math","go:caller"])
 assert.equal(view.checks[0].status,"PASS");assert.match(view.workflow,/obligations open/)
 assert.ok(view.gaps.includes("Workflow: reports-not-produced"));assert.match(view.gaps.join("\n"),/LANGUAGE_FRONTIER/)
 f.policy.unmet.push("uncommitted-work");const edited=projectCockpit(f.repo,other,affected(),f.policy);assert.equal(edited.checks[0].status,"STALE");assert.match(edited.checks[0].reason,/committed target: true/);f.policy.unmet.pop()
 assert.equal(view.declarations[0].command,"false; touch SHOULD_NOT_EXIST")
 const large=affected();large.plan.dirty=Array.from({length:80},(_,i)=>`file-${i}`);large.plan.unknown=Array.from({length:200},()=>({reason:"unknown",detail:"partial"}))
 const bounded=projectCockpit(f.repo,other,large,f.policy);assert.equal(bounded.files.length,64);assert.equal(bounded.gaps.length,128);assert.match(bounded.gaps[0],/16 changed paths omitted/);assert.match(bounded.gaps.at(-1),/further gaps omitted/)
 for(const [change,status] of [[{qualified:false,testedCommit:other},"STALE"],[{qualified:false,cancelled:true},"CANCELLED"],[{qualified:false,timedOut:true},"TIMEOUT"],[{qualified:false,exit:1},"FAIL"],[{qualified:false,testedCommit:"",exit:-1},"NOT RUN"]])assert.equal(checkStatus({...f.policy.checks[0],...change}),status)
})
test("AHI-034 real private logs remain inert, escaped and explicitly caller-owned",async t=>{
 const f=fixture(t),result=await collectCockpit(f.run,"main")
 assert.equal(result.view.base,other)
 const output=await inspectProof(f.run,result.binding,"unit")
 assert.match(output,/REAL_RECORDED_OUTPUT\n\\u001b/);assert.doesNotMatch(output,/\x1b|\u202e/u);assert.match(output,/no content attestation/)
 assert.match(output,/inert; NOT_EXECUTED/)
 await assert.rejects(inspectProof(f.run,result.binding,"../unknown"),/check-not-in-current-receipt/)
 writeFileSync(f.stdout,"x".repeat(70000));assert.match(await inspectProof(f.run,result.binding,"unit"),/truncated at 64 KiB/)
 f.policy.checks[0].secretScreened=true;const withheld=await collectCockpit(f.run)
 await assert.rejects(inspectProof(f.run,withheld.binding,"unit"),/withheld/)
})
test("AHI-034 private reads reject traversal, symlink parents, symlink leaves and hard links",t=>{
 const f=fixture(t)
 assert.throws(()=>readPrivateFile(f.gitdir,"../outside",100),/unsafe-private-path/)
 symlinkSync(f.generation,path.join(f.gitdir,"linked"));assert.throws(()=>readPrivateFile(f.gitdir,"linked/check-001-0.log",100),/unsafe-private-file/)
 symlinkSync(f.stdout,path.join(f.generation,"link.log"));assert.throws(()=>readPrivateFile(f.gitdir,path.relative(f.gitdir,path.join(f.generation,"link.log")),100),/unsafe-private-file/)
 linkSync(f.stdout,path.join(f.generation,"hard.log"));assert.throws(()=>readPrivateFile(f.gitdir,path.relative(f.gitdir,f.stdout),100),/unsafe-private-file/)
 assert.throws(()=>readPrivateFile(f.gitdir,"corvint/local-completion/owner",4),/too-large/)
})
test("AHI-034 refresh rejects changed Git identity and uses the resolved immutable base",async t=>{
 const f=fixture(t);let repositoryCalls=0,resolvedBase
 const run=async read=>{if(read.kind==="affected")resolvedBase=read.base;if(read.kind==="repository"&&++repositoryCalls===2)return {...f.repo,target:other};return f.run(read)}
 await assert.rejects(collectCockpit(run,"main"),/repository-changed/);assert.equal(resolvedBase,other)
})
test("AHI-034 replaced owners and rerun checks cannot publish old proof bytes",async t=>{
 const f=fixture(t),result=await collectCockpit(f.run)
 writeFileSync(path.join(f.directory,"owner"),"e".repeat(64));await assert.rejects(inspectProof(f.run,result.binding,"unit"),/verification-changed/)
 writeFileSync(path.join(f.directory,"owner"),key)
 let calls=0
 const race=async read=>{if(read.kind==="completion"&&++calls===2)f.policy.checks[0].stdout=path.join(f.generation,"check-002-0.log");return f.run(read)}
 await assert.rejects(inspectProof(race,result.binding,"unit"),/verification-changed-during-read/)
})
test("AHI-034 proof paths must belong to the current generation and observed edits abort reads",async t=>{
 const f=fixture(t);f.policy.checks[0].stdout=path.join(f.root,"private.txt");const result=await collectCockpit(f.run)
 await assert.rejects(inspectProof(f.run,result.binding,"unit"),/unsafe-verification-log/)
 const state={active:true,cockpit:result.view,cockpitBinding:result.binding,cockpitRequest:new AbortController()};invalidateInspection(state,"edited")
 assert.equal(state.cockpit.state,"stale");assert.equal(state.cockpitBinding,undefined);assert.equal(state.cockpitRequest.signal.aborted,true)
})
test("AHI-034 runner rejects mutating or oversized read output and owns cancelled processes",async t=>{
 const f=fixture(t),script=path.join(f.root,"core")
 const write=body=>writeFileSync(script,`#!${process.execPath}\n${body}\n`,{mode:0o755})
 const runner=createCorvintRunner({corvintBinary:script})
 const invoke=signal=>runner({root:f.root,event:"cockpit",input:{},read:{kind:"affected",base:oid},signal})
 write(`console.log(${JSON.stringify(JSON.stringify({...affected(),mutates:true}))})`);assert.equal((await invoke()).code,"malformed-cockpit-output")
 write('process.stdout.write("x".repeat(1048577))');assert.equal((await invoke()).code,"output-too-large")
 write('setInterval(()=>{},1000)');const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),100)
 try {assert.equal((await invoke(controller.signal)).code,"host-aborted")}finally{clearTimeout(timer)}
})


import { createCockpitClient } from "./src/cockpit-client.js"
test("AHI-034 collection rejects check reruns, dirty state and newly appearing owners",async t=>{
 for (const [missingOwner,change] of [[false,f=>{f.policy.checks[0].stdout=path.join(f.generation,"check-002-0.log")}],[false,f=>f.policy.unmet.push("uncommitted-work")],[true,f=>writeFileSync(path.join(f.directory,"owner"),key)]]) {
  const f=fixture(t);if(missingOwner)rmSync(path.join(f.directory,"owner"))
  const run=async read=>{const result=await f.run(read);if(read.kind==="affected")change(f);return result}
  await assert.rejects(collectCockpit(run),/verification-changed-during-refresh/)
 }
})
test("AHI-034 UI pins a resolved base across branch movement and rejects reordered snapshots",async t=>{
 const f=fixture(t),controller=new AbortController(),states=[],bases=[];let branch=other,server=projectCockpit(f.repo,other,affected(),f.policy),delayed=false,waiting=[]
 const rpc={cockpitRefresh:async input=>{bases.push(input.base);server={...server,base:input.base==='main'?branch:input.base}},cockpitSnapshot:async()=>delayed?await new Promise(resolve=>waiting.push(resolve)):server}
 const client=createCockpitClient({rpc,signal:controller.signal,publish:x=>states.push(x)})
 await client.refresh({sessionID:'a',location:{directory:f.root}},'main');branch=oid
 await client.refresh({sessionID:'a',location:{directory:f.root}})
 assert.deepEqual(bases,['main',other]);assert.equal(states.at(-1).base,other)
 delayed=true;const old=client.observe(),newer=client.observe();waiting[1]({...server,state:'stale'});await newer;waiting[0]({...server,state:'ready'});await old
 assert.equal(states.at(-1).state,'stale')
 const abandoned=client.observe();client.cancel();waiting[2]({...server,state:'ready'});await abandoned;assert.equal(states.at(-1).state,'stale')
})
test("AHI-034 refresh acknowledgment cannot overwrite a newer stale server snapshot",async t=>{
 const f=fixture(t),states=[];let finish,reads=0
 const stale={...projectCockpit(f.repo,other,affected(),f.policy),state:'stale'}
 const rpc={cockpitRefresh:async()=>await new Promise(resolve=>{finish=resolve}),cockpitSnapshot:async()=>{reads++;return stale}}
 const client=createCockpitClient({rpc,signal:new AbortController().signal,publish:x=>states.push(x)})
 const refresh=client.refresh({sessionID:'a',location:{directory:f.root}},'main')
 await client.observe();assert.equal(reads,0)
 finish({...stale,state:'ready'});await refresh
 assert.equal(reads,1);assert.equal(states.at(-1).state,'stale');client.cancel()
})

test("AHI-034 a failed first refresh in another scope cannot retain the previous base",async t=>{
 const f=fixture(t),bases=[];let fail=false,server=projectCockpit(f.repo,other,affected(),f.policy)
 const rpc={cockpitRefresh:async input=>{bases.push(input.base);if(fail)throw new Error('unavailable')},cockpitSnapshot:async()=>server}
 const client=createCockpitClient({rpc,signal:new AbortController().signal,publish:()=>{}})
 const scope={sessionID:'a',location:{directory:f.root}}
 await client.refresh(scope,'main')
 fail=true;await client.refresh({...scope,sessionID:'b'},'');await client.refresh({...scope,sessionID:'b'})
 assert.deepEqual(bases,['main','',''])
 fail=false;await client.refresh(scope,'main');fail=true
 const moved={...scope,location:{directory:f.root+'/other'}}
 await client.refresh(moved);await client.refresh(moved)
 assert.deepEqual(bases.slice(-3),['main','','']);client.cancel()
})
