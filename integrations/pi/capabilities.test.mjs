// SPDX-License-Identifier: Apache-2.0
// PWV-V0-011: the registered surface equals the closed capability inventory in the spec.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,readdirSync} from 'node:fs';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import register from './extension.js';
import {coreOperations,createCoreService,registerCoreTools} from './core.js';
import {createTasksService,registerTasksTools} from './tasks.js';
import {registerCockpit} from './cockpit.js';
import {registerWorkflow} from './workflow.js';
import {digest} from './operations.js';
import {PI_ADAPTER_VERSION} from './runtime.js';

const spec=readFileSync(new URL('../../docs/specs/pi-workflow-v0.md',import.meta.url),'utf8');
const section=spec.slice(spec.indexOf('\n## Capability contract\n'));
const inventory=JSON.parse(/```json\n([\s\S]*?)\n```/.exec(section)[1]);
const pkg=JSON.parse(readFileSync(new URL('./package.json',import.meta.url),'utf8'));
const compatibility=JSON.parse(readFileSync(new URL('./compatibility.json',import.meta.url),'utf8'));
const sorted=values=>[...values].sort();
// tasks.js does not export its write set; read the shipped literal so the inventory binds to it.
const shippedWrites=JSON.parse(/^const writes=(\[[^\]\n]*\]);$/m.exec(readFileSync(new URL('./tasks.js',import.meta.url),'utf8'))[1].replaceAll("'",'"'));
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

// AHI-024/PWV-V0-011 (V1-2670): a half bump of the adapter version (manifests moved, emitters did not)
// fails here at PR time instead of in the nightly. Every emitting or admitting literal must equal the package.
test('AHI-024 every Pi adapter version literal equals the package manifest version',()=>{
 const version=pkg.version;
 const read=path=>readFileSync(new URL(path,import.meta.url),'utf8');
 const workflow=read('./workflow.js');
 // AHI-240 (V1-2854): workflow.js carries no literal of its own; it emits runtime.js's constant.
 const workflowTuple=[...workflow.matchAll(/adapterVersion:([A-Za-z_'.0-9]+)/g)].map(m=>m[1]);
 const workflowArgv=[...workflow.matchAll(/'--adapter-version',([A-Za-z_'.0-9]+)/g)].map(m=>m[1]);
 const native=/^const piAdapterVersion = "([^"]*)"$/m.exec(read('../../cmd/corvint/pi_adapter.go'));
 const matrix=JSON.parse(read('../compatibility.json')).entries.filter(e=>e.host==='pi');
 assert.equal(pkg.corvintIntegration.adapterVersion,version);
 assert.equal(compatibility.adapterVersion,version);
 assert.deepEqual(matrix.map(e=>e.adapterVersion),[version]);
 assert.equal(PI_ADAPTER_VERSION,version);
 assert.deepEqual(workflowTuple,['PI_ADAPTER_VERSION']);
 assert.deepEqual(workflowArgv,['PI_ADAPTER_VERSION']);
 assert.ok(native,'cmd/corvint/pi_adapter.go piAdapterVersion literal not found');
 assert.equal(native[1],version);
});

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
 assert.deepEqual(sorted(shippedWrites),inventory.tasksWriteOperations,'shipped tasks.js writes equal the inventory');
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

test('PWV-V0-015 every model tool declares annotations that follow its inventory state',t=>{
 const {tools}=surface(t);
 const hints={NATIVE_READ:{readOnlyHint:true,destructiveHint:false,idempotentHint:true,openWorldHint:false},EXPLICIT_RECORD:{readOnlyHint:false,destructiveHint:false,idempotentHint:false,openWorldHint:false}};
 for(const [name,state] of Object.entries(inventory.tools)) {
  assert.ok(hints[state],`${name} has a state with declared hints`);
  assert.deepEqual({...tools.get(name).annotations},hints[state],name);
 }
});

test('PWV-V0-017 the package ships exactly the inventory skills, each routing only to registered tools',t=>{
 const {tools,commands}=surface(t);
 assert.deepEqual(pkg.pi.skills,['./skills']);
 assert.ok(pkg.files.includes('skills'));
 const dirs=readdirSync(new URL('./skills/',import.meta.url),{withFileTypes:true}).filter(e=>e.isDirectory()).map(e=>e.name).sort();
 assert.deepEqual(dirs,inventory.skills);
 for(const name of dirs) {
  const text=readFileSync(new URL(`./skills/${name}/SKILL.md`,import.meta.url),'utf8');
  const front=/^---\n([\s\S]*?)\n---\n/.exec(text);
  assert.ok(front,`${name} has frontmatter`);
  const fields=Object.fromEntries(front[1].split('\n').map(line=>[line.slice(0,line.indexOf(':')),line.slice(line.indexOf(':')+1).trim()]));
  assert.equal(fields.name,name);
  assert.ok(fields.description&&fields.description.length<=1024,`${name} description`);
  assert.equal(fields.license,'Apache-2.0');
  for(const [,tool] of text.matchAll(/`(corvint_[a-z_]+)`/g))assert.ok(tools.has(tool),`${name} routes to registered tool ${tool}`);
  for(const [,command] of text.matchAll(/`\/(corvint-[a-z-]+)/g))assert.ok(commands.has(command),`${name} routes to registered command ${command}`);
  assert.doesNotMatch(text,/corvint-tasks (claim|submit|complete|release|renew|ticket create)/,`${name} names no native write`);
  assert.doesNotMatch(text,/`(git|go|make|npm|npx|node|bun|pi|corvint|corvint-tasks|corvint-mcp) [^`]*`/,`${name} names no native command outside the registered tools`);
  assert.doesNotMatch(text,/\b(run|execute)\s+(the\s+)?(selected\s+)?(tests|checks)\b/i,`${name} does not direct the model to run commands itself`);
 }
});
