// SPDX-License-Identifier: AGPL-3.0-or-later
// PWV-V0-011: the registered surface equals the closed capability inventory in the spec.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import register from './extension.js';
import {coreOperations,createCoreService,registerCoreTools} from './core.js';
import {createTasksService,registerTasksTools} from './tasks.js';
import {registerCockpit} from './cockpit.js';
import {registerWorkflow} from './workflow.js';
import {digest} from './operations.js';

const spec=readFileSync(new URL('../../docs/specs/pi-workflow-v0.md',import.meta.url),'utf8');
const section=spec.slice(spec.indexOf('\n## Capability contract\n'));
const inventory=JSON.parse(/```json\n([\s\S]*?)\n```/.exec(section)[1]);
const pkg=JSON.parse(readFileSync(new URL('./package.json',import.meta.url),'utf8'));
const compatibility=JSON.parse(readFileSync(new URL('./compatibility.json',import.meta.url),'utf8'));
const sorted=values=>[...values].sort();
const ticketId='ticket:fixture:main:APP-0001',tree='b'.repeat(40);

// Mirrors index.ts registration, which needs the host package to import.
function surface(t) {
 const tools=new Map(),commands=new Map(),events=new Set();
 const pi={on:name=>events.add(name),registerTool:tool=>tools.set(tool.name,tool),registerCommand:(name,command)=>commands.set(name,command),sendMessage(){}};
 const listeners=new Set([...process.listeners('SIGINT'),...process.listeners('SIGTERM')]);
 t.after(()=>{for(const signal of ['SIGINT','SIGTERM'])for(const listener of process.listeners(signal))if(!listeners.has(listener))process.removeListener(signal,listener);});
 const runner={async run(){return {exitCode:2,stdout:'',stderr:''}},async close(){},async cancel(){}};
 const core=createCoreService({runner}),tasks=createTasksService({runner,identity:async()=>({})});
 register(pi,{runner,version:'0.99.1',lifecycle:[core,tasks]});
 registerWorkflow(pi,{runner,version:'0.99.1',notice(){}});
 registerCoreTools(pi,{service:core});
 registerTasksTools(pi,{service:tasks});
 registerCockpit(pi,{core,tasks,context:{},identity:async()=>({}),uiKit:{}});
 return {tools,commands,events};
}

test('PWV-V0-011 registered tools, commands, events and operations equal the closed inventory',t=>{
 assert.equal(inventory.profile,'corvint-pi-capabilities/0');
 assert.deepEqual(inventory.tuple,{adapterVersion:pkg.corvintIntegration.adapterVersion,host:pkg.corvintIntegration.host,hostVersion:compatibility.testedHostVersions[0],support:compatibility.support,surface:compatibility.surface});
 assert.deepEqual(compatibility.testedHostVersions,[inventory.tuple.hostVersion]);
 const {tools,commands,events}=surface(t);
 assert.deepEqual(sorted(tools.keys()),Object.keys(inventory.tools));
 assert.deepEqual(sorted(commands.keys()),Object.keys(inventory.commands));
 assert.deepEqual(sorted(events),inventory.events);
 assert.deepEqual(sorted(coreOperations),inventory.coreReadOperations);
 assert.deepEqual(sorted(tools.get('corvint_core_read').parameters.properties.operation.enum),inventory.coreReadOperations);
 assert.deepEqual(sorted(tools.get('corvint_tasks').parameters.properties.operation.enum),inventory.tasksReadOperations);
 assert.deepEqual(sorted(compatibility.unavailableCapabilities),inventory.unavailable);
 assert.equal(Object.values(inventory.deferred).every(owner=>/^V1-\d{4}$/.test(owner)),true);
 const states=new Set(['NATIVE_READ','LIFECYCLE','EXPLICIT_RECORD','LOCAL_POLICY','OPERATOR_NATIVE_WRITE']);
 assert.equal([...Object.values(inventory.tools),...Object.values(inventory.commands)].every(state=>states.has(state)),true);
 assert.equal(Object.entries(inventory.tools).some(([,state])=>state==='OPERATOR_NATIVE_WRITE'),false,'no model-callable tool owns a Tasks write');
});

test('PWV-V0-011 Tasks writes are negotiated per call against native help; others refuse without a native call',async t=>{
 const dir=await mkdtemp(join(tmpdir(),'pi-capabilities-'));t.after(()=>rm(dir,{recursive:true,force:true}));
 let implemented=[];const calls=[];
 const runner={async run({args}){calls.push(args.join(' '));return {exitCode:0,stdout:JSON.stringify({profile:'taskman-command-result/0',command:args,items:args[0]==='help'?[{implemented}]:[],outcome:'OK',codes:[],warnings:[]})}},async close(){}};
 const identity=async()=>({root:dir,gitDir:dir,branch:'main',head:'a'.repeat(40),sessionSha256:digest('session'),filesystemSha256:digest('repo')});
 const service=createTasksService({runner,identity});
 const ctx={cwd:dir,sessionManager:{getSessionId:()=>'session'},isProjectTrusted:()=>true,isIdle:()=>true};
 const attempt={ticketId,expectedRevision:'1',attemptId:'attempt-1',generation:'1',holder:'pi'};
 const inputs={'ticket prioritize':{ticketId,expectedRevision:'1',priority:'P1',order:'2'},claim:{ticketId,expectedRevision:'1',holder:'pi'},renew:attempt,release:{...attempt,reason:'handoff'},submit:{...attempt,tree},'gate run':{...attempt,gate:'verify'},complete:{...attempt,commit:tree}};
 assert.deepEqual(sorted(Object.keys(inputs)),inventory.tasksWriteOperations);
 assert.equal(inventory.tasksWriteAdmission,'native-help-implemented-per-call');
 for(const operation of inventory.tasksWriteOperations) {
  implemented=inventory.tasksWriteOperations.filter(x=>x!==operation);calls.length=0;
  const result=await service.command({operation,input:inputs[operation],requestId:'request-1'},ctx);
  assert.equal(result.fault,'capability-unavailable',operation);
  assert.equal(result.mutation,'not-attempted');
  assert.deepEqual(calls,['help'],`${operation} dispatches nothing before native admission`);
 }
 for(const operation of ['complete-manual','ticket create','ticket refine','init','index']) {
  calls.length=0;
  const result=await service.command({operation,input:{ticketId,expectedRevision:'1'},requestId:'request-2'},ctx);
  assert.equal(result.fault,'unsupported-mutation',operation);
  assert.deepEqual(calls,[]);
 }
});
