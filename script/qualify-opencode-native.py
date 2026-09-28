#!/usr/bin/env python3
"""AHI-032: unmodified OpenCode + real Corvint, using a loopback scripted provider.

No model-quality claim. Outputs contain only the generated fixture's conversation.
Fault/privacy/normalization adversaries also require TestHostAdapterJavaScriptHosts.
"""
import argparse
import atexit
import hashlib
import http.server
import json
import math
import os
from pathlib import Path
import platform
import re
import signal
import subprocess
import tempfile
import threading
import time

os.sys.dont_write_bytecode = True
from opencode_qualification import node_architecture

P = argparse.ArgumentParser()
P.add_argument('--host', required=True)
P.add_argument('--corvint', required=True)
P.add_argument('--output', required=True)
P.add_argument('--interrupt-probe', action='store_true')
A = P.parse_args()
A.host = str(Path(A.host).resolve())
A.corvint = str(Path(A.corvint).resolve())
SOURCE = Path(__file__).resolve().parents[1]
OUT = Path(A.output).resolve()
OUT.mkdir(parents=True, exist_ok=True)
RUN = Path(tempfile.mkdtemp(prefix='run-', dir=OUT))
CHILD = None
SERVER = None
GROUP_CURSOR = 0
STOP = threading.Event()


def descendants(pid):
    rows = subprocess.check_output(['ps', '-axo', 'pid=,ppid='], text=True).splitlines()
    children = {}
    for row in rows:
        child, parent = map(int, row.split())
        children.setdefault(parent, []).append(child)
    result, pending = [], [pid]
    while pending:
        pending = [child for parent in pending for child in children.get(parent, [])]
        result.extend(pending)
    return result


def process_states():
    result={}
    for row in subprocess.check_output(['ps','-axo','pid=,pgid=,stat='],text=True).splitlines():
        pid,group,state=row.split()
        result[int(pid)]=(int(group),state)
    return result


def signal_group(group, sig):
    try:
        os.killpg(group,sig)
    except ProcessLookupError:
        pass
    except PermissionError:
        if any(pgid==group and not state.startswith('Z') for pgid,state in process_states().values()):
            raise


def cleanup():
    global CHILD, SERVER, GROUP_CURSOR
    STOP.set()
    registered=rows(RUN/'groups.jsonl')
    groups=registered[GROUP_CURSOR:]
    for item in groups:
        signal_group(item['pgid'],signal.SIGTERM)
    if groups:time.sleep(0.1)

    if CHILD is not None:
        owned = descendants(CHILD.pid)
        for pid in reversed(owned):
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        try:
            os.killpg(CHILD.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        CHILD.wait(timeout=10)
        CHILD = None
    for item in groups:
        signal_group(item['pgid'],signal.SIGKILL)
    GROUP_CURSOR=len(registered)
    if SERVER is not None:
        SERVER.shutdown()
        SERVER.server_close()
        SERVER = None


def interrupted(signum, _frame):
    cleanup()
    raise SystemExit(128 + signum)


atexit.register(cleanup)
for signum in (signal.SIGINT, signal.SIGTERM):
    signal.signal(signum, interrupted)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def run(argv, cwd, env):
    global CHILD
    CHILD = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
    try:
        stdout, stderr = CHILD.communicate(timeout=45)
        if CHILD.returncode:
            raise RuntimeError(str(argv)+": "+stderr)
        return stdout
    finally:
        if CHILD is not None:
            try:
                os.killpg(CHILD.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            CHILD.wait(timeout=10)
            CHILD = None


def rows(path):
    return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []


class Provider(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_POST(self):
        data = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        with (RUN/'requests.jsonl').open('a') as f:
            f.write(json.dumps(data)+'\n')
        if A.interrupt_probe:
            STOP.wait(45)
            return
        step = 99 if getattr(self.server, 'smoke', False) else self.server.step
        self.server.step += 1
        task = 'Locate Add in add.go.'
        supplied = re.search(r'cv1:[0-9a-f]+:[0-9a-f]+:all:add.go', json.dumps(data))
        selected = supplied.group().replace(':all:', ':1-3:') if supplied else 'missing-supplied-handle'
        verification = [{'commandSha256': hashlib.sha256(b'git diff --check').hexdigest(), 'status': 'passed'}]
        calls = [
            ('execute', {'code': 'return await tools.corvint_context('+json.dumps({'task': 'Identify the active work queue and required workflow gates'})+')'}),
            ('execute', {'code': 'return await tools.corvint_expand('+json.dumps({'handle': selected})+')'}),
            ('edit', {'path': 'add.go', 'oldString': '// before', 'newString': '// after'}),
            ('execute', {'code': 'return await tools.qualification_verify({})'}),
            ('execute', {'code': 'return await tools.corvint_record_outcome('+json.dumps({'task': task, 'changedPaths': ['add.go'], 'verification': verification, 'outcome': 'passed'})+')'}),
            ('execute', {'code': 'await tools.qualification_prompts({}); for(let i=0;i<21;i++)await tools.qualification_noop({}); return \"native samples complete\"'}),
        ]
        if getattr(self.server, 'benchmark', False):
            calls = [calls[-1]]
        else:
            calls = calls[:-1]
        if step < len(calls):
            name, args = calls[step]
            delta = {'role': 'assistant', 'tool_calls': [{'index': 0, 'id': 'call_'+str(step), 'type': 'function', 'function': {'name': name, 'arguments': json.dumps(args)}}]}
            reason = 'tool_calls'
        else:
            delta, reason = {'role': 'assistant', 'content': 'QUALIFICATION_DONE'}, 'stop'
        self.send_response(200)
        self.send_header('Content-Type', 'text/event-stream')
        self.end_headers()
        for value, finish in [(delta, None), ({}, reason)]:
            chunk = {'id': 'local-'+str(step), 'object': 'chat.completion.chunk', 'created': 1, 'model': 'probe', 'choices': [{'index': 0, 'delta': value, 'finish_reason': finish}]}
            if step == 4 and finish:
                chunk['usage'] = {'prompt_tokens': 130000, 'completion_tokens': 5000, 'total_tokens': 135000}
            self.wfile.write(('data: '+json.dumps(chunk)+'\n\n').encode())
        self.wfile.write(b'data: [DONE]\n\n')


def identities():
    return {'sourceCommit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=SOURCE,text=True).strip(),'hostSHA256':digest(A.host),'corvintSHA256':digest(A.corvint),
            'harnessSHA256':digest(__file__),
            'sourceFiles':{str(p.relative_to(SOURCE)):digest(p) for p in sorted((SOURCE/'integrations/opencode').rglob('*')) if p.is_file()}}


baseline = identities()
home, repo, probe = [RUN/name for name in ('home', 'repo', 'probe')]
for path in (home, repo, probe):
    path.mkdir()
env = {'PATH': os.environ['PATH'], 'HOME': str(home), 'XDG_CONFIG_HOME': str(home/'config'), 'XDG_DATA_HOME': str(home/'data'), 'XDG_CACHE_HOME': str(home/'cache'), 'XDG_STATE_HOME': str(home/'state'), 'TMPDIR': str(RUN), 'NO_COLOR': '1', 'SECRET_DO_NOT_LEAK': 'qualification-sentinel', 'GEMINI_API_KEY': 'qualification-sentinel'}
(repo/'go.mod').write_text('module example.com/qualification\n\ngo 1.27.1\n')
source = 'package qualification\n// before\nfunc Add(a, b int) int { return a + b }\n' + ''.join('// unrelated fixture line %d: retained for the manual full-file read baseline.\n'%n for n in range(180))
(repo/'add.go').write_text(source)
(repo/'AGENTS.md').write_text('# Project instructions\nFor repository workflow changes, inspect add.go and run git diff --check.\n')
(repo/'oversized.txt').write_text('excluded fixture\n'*150000)
for args in [['git','init','-q'],['git','add','.'],['git','-c','user.name=Qualification','-c','user.email=qualification@localhost','-c','commit.gpgsign=false','commit','-qm','fixture']]:
    run(args, repo, env)
tree = run(['git','rev-parse','HEAD^{tree}'],repo,env).strip()
blob = run(['git','rev-parse','HEAD:add.go'],repo,env).strip()
# Select the declaration range so the exact source operation stays under its public cap.
handle = 'cv1:'+tree+':'+blob+':1-3:add.go'
wrapper = RUN/'corvint-probe'
wrapper.write_text('''#!/usr/bin/env python3
import json,os,signal,subprocess,sys,time
from pathlib import Path
child=None
def stop(signum,_frame):
 if child is not None:
  child.terminate()
  try:child.wait(timeout=1)
  except subprocess.TimeoutExpired:child.kill();child.wait()
 raise SystemExit(128+signum)
signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
with Path(GROUPS).open('a') as f: f.write(json.dumps({'pgid':os.getpgrp(),'pid':os.getpid()})+'\\n')
if INTERRUPT:
 child=subprocess.Popen(['/bin/sleep','60'])
 Path(ACTIVE).write_text(json.dumps({'wrapper':os.getpid(),'child':child.pid}))
 child.wait()
raw=sys.stdin.read(); start=time.monotonic()
child=subprocess.Popen([BINARY,*sys.argv[1:]],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
out,err=child.communicate(raw)
with Path(CAPTURE).open('a') as f:
 f.write(json.dumps({'argv':sys.argv[1:],'input':json.loads(raw or '{}'),'output':out,'exit':child.returncode,'milliseconds':(time.monotonic()-start)*1000,'leakedKeys':[k for k,v in os.environ.items() if v=='qualification-sentinel']})+'\\n')
sys.stdout.write(out);sys.stderr.write(err);sys.exit(child.returncode)
'''.replace('BINARY',repr(str(Path(A.corvint).resolve()))).replace('CAPTURE',repr(str(RUN/'invocations.jsonl'))).replace('GROUPS',repr(str(RUN/'groups.jsonl'))).replace('ACTIVE',repr(str(RUN/'active.json'))).replace('INTERRUPT',repr(A.interrupt_probe)))
wrapper.chmod(0o700)
probe_source = r'''import {appendFileSync,createReadStream} from 'node:fs';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const log=x=>appendFileSync(EVENTS,JSON.stringify(x)+'\n');
export default {id:'qualification-probe',async setup(ctx){
 const image=createHash('sha256');for await(const chunk of createReadStream(process.execPath))image.update(chunk);
 log({kind:'setup',version:ctx.app.version,executableSHA256:image.digest('hex'),architecture:process.arch,os:process.platform});
 const stop=new AbortController();
 await ctx.session.hook('title',e=>{e.result='Qualification'});
 await ctx.session.hook('compaction',async e=>{e.result={summary:'Fixture compaction summary. Inspect the supplied current repository evidence.'};await ctx.session.synthetic({sessionID:e.sessionID,text:'After compaction, locate Add in add.go.',description:'qualification follow-up',resume:true});log({kind:'compaction-hook'})});
 await ctx.session.hook('prompt',e=>log({kind:'prompt',text:e.prompt.text}));
 await ctx.session.hook('context',e=>log({kind:'context',system:e.system}));
 await ctx.tool.hook('execute.after',e=>log({kind:'tool',name:e.tool,status:e.status,result:e.result}));
 await ctx.tool.transform(editor=>{
  editor.add({name:'qualification_prompts',description:'Measure awaited native prompt hooks on idle fixture sessions.',input:{type:'object',properties:{},additionalProperties:false},async execute(){
   for(let i=0;i<21;i++){
    const session=await ctx.session.create({});
    await ctx.session.prompt({sessionID:session.id,text:'Locate Add in add.go.',resume:false});
   }
   return {content:'native prompt samples complete'};
  }});
  editor.add({name:'qualification_noop',description:'Measure native post-tool observations.',input:{type:'object',properties:{},additionalProperties:false},async execute(){return {content:'fixture observation'}}});
  editor.add({name:'qualification_verify',description:'Verify the fixture diff.',input:{type:'object',properties:{},additionalProperties:false},async execute(){
   execFileSync('git',['diff','--check'],{cwd:ctx.location.directory,timeout:5000});
   return {content:'git diff --check passed',metadata:{corvint:{verification:[{commandSha256:VERIFY,status:'passed'}]}}};
  }});

 });
 const watching=(async()=>{for await(const e of ctx.event.subscribe({signal:stop.signal}))if(e.type.startsWith('session.execution.')||e.type.startsWith('session.compaction.'))log({kind:'event',type:e.type})})();
 return async()=>{stop.abort();await watching;log({kind:'cleanup'})};
}};
'''.replace('EVENTS',json.dumps(str(RUN/'events.jsonl'))).replace('VERIFY',json.dumps(hashlib.sha256(b'git diff --check').hexdigest()))
(probe/'index.js').write_text(probe_source)
SERVER = http.server.ThreadingHTTPServer(('127.0.0.1',0),Provider)
SERVER.daemon_threads = True
SERVER.step = 0
SERVER.handle = handle
threading.Thread(target=SERVER.serve_forever,daemon=True).start()
# A transparent timing observer wraps registration boundaries without changing their values.
# The real adapter still receives real host events and mutates the real awaited prompt/context.
instrumented = RUN/'instrumented'
instrumented.mkdir()
(instrumented/'index.js').write_text("""import plugin from CANDIDATE;
import {appendFileSync,createReadStream} from 'node:fs';
import {createHash} from 'node:crypto';
const timing=(kind,ms,delivered)=>appendFileSync(TIMINGS,JSON.stringify({kind,ms,delivered})+'\\n');
export default {...plugin,async setup(ctx){
 const warn=console.warn;
 console.warn=(...args)=>{if(String(args[0]).startsWith('[corvint/opencode]'))appendFileSync(FAULTS,JSON.stringify({message:String(args[0])})+'\\n');warn(...args)};
 const wrap=(kind,fn)=>async(...args)=>{const t=performance.now();let complete=false;try{const result=await fn(...args);complete=true;return result}finally{
  const label=typeof kind==='function'?kind(...args):kind;
  const delivered=label==='session.prompt'?complete&&args[0].prompt.text.includes('harness-receipt:sha256:'):complete;
  timing(label,performance.now()-t,delivered);
 }};
 const dispose=await plugin.setup({...ctx,
  session:{...ctx.session,hook:(name,fn)=>ctx.session.hook(name,wrap('session.'+name,fn))},
  tool:{...ctx.tool,hook:(name,fn)=>ctx.tool.hook(name,wrap(call=>'tool.'+name+':'+call.tool,fn))},
  event:{...ctx.event,subscribe:async function* (options){for await(const event of ctx.event.subscribe(options)){
   const t=performance.now();yield event;timing('event.'+event.type,performance.now()-t);
  }}}
 });
 return async()=>{try{await dispose?.()}finally{console.warn=warn}};
}};
""".replace('CANDIDATE',json.dumps((SOURCE/'integrations/opencode/src/index.js').as_uri())).replace('TIMINGS',json.dumps(str(RUN/'timings.jsonl'))).replace('FAULTS',json.dumps(str(RUN/'faults.jsonl'))))
config = {'model':'local/probe','plugins':[{'package':instrumented.as_uri(),'options':{'corvintBinary':str(wrapper)}},{'package':probe.as_uri(),'options':{}}],'providers':{'local':{'name':'Qualification loopback provider','package':'@opencode/ai/providers/openai-compatible','settings':{'baseURL':'http://127.0.0.1:'+str(SERVER.server_port)+'/v1','apiKey':'local-fixture'},'models':{'probe':{'modelID':'probe','capabilities':{'tools':True,'input':['text'],'output':['text']},'limit':{'context':131072,'output':1024}}}}}}
# OpenCode config is untracked fixture setup, not a change to the pinned source baseline.
(repo/'.git/info/exclude').write_text('opencode.json\n')
(repo/'opencode.json').write_text(json.dumps(config))
with (RUN/'stdout.log').open('w') as out, (RUN/'stderr.log').open('w') as err:
    CHILD = subprocess.Popen([A.host,'run','--standalone','--format','json','--model','local/probe','--title','Qualification','Locate Add in add.go.'],cwd=repo,env=env,stdout=out,stderr=err,start_new_session=True)
    (OUT/'current.json').write_text(json.dumps({'runner':os.getpid(),'pid':CHILD.pid,'root':str(RUN)}))
    try:
        code = CHILD.wait(timeout=90)
    finally:
        cleanup()
invocations, events, requests = rows(RUN/'invocations.jsonl'), rows(RUN/'events.jsonl'), rows(RUN/'requests.jsonl')
packets = [json.loads(x['output']) for x in invocations if x['exit']==0 and x['output'].startswith('{')]
prompts = [x['text'] for x in events if x['kind']=='prompt']
expanded = [x for x in packets if x.get('handle')==handle]
checks = {
 'native-discovery':any(x.get('version')=='2.0.18' for x in events),
 'native-host-image':any(x.get('executableSHA256')==baseline['hostSHA256'] and x.get('architecture')==node_architecture() and x.get('os')==platform.system().lower() for x in events if x['kind']=='setup'),
 'awaited-current-prompt':bool(prompts) and 'harness-receipt:sha256:' in prompts[0] and any('harness-receipt:sha256:' in json.dumps(m) for m in requests[0]['messages'] if m['role']=='user'),
 'native-query':any(x.get('name')=='corvint_context' and x['status']=='completed' for x in events),
 'native-exact-expansion':bool(expanded) and expanded[0]['selection']['text']==''.join(source.splitlines(keepends=True)[:3]),
 'native-edit':'// after' in (repo/'add.go').read_text(),
 'verification-observation':any(x['input'].get('verification') and '--event' in x['argv'] and x['argv'][x['argv'].index('--event')+1]=='post-tool' for x in invocations),
 'explicit-outcome':any(x['input'].get('outcome')=='passed' and 'task' not in x['input'] for x in invocations),
 'advisory-completion':any(x.get('event')=='stop' and x.get('frontier',{}).get('state')=='UNAVAILABLE' and not x['frontier']['shouldContinue'] for x in packets),
 'native-compaction':any(x.get('type')=='session.compaction.ended' for x in events),
 'dirty-path-recovery':any('"startSource":"compact"' in json.dumps(x['input'],separators=(',',':')) for x in invocations) and any('add.go' in json.dumps(x['system']) and 'rehydration' in json.dumps(x['system']) for x in events if x['kind']=='context'),
 'environment-filter':all(not x['leakedKeys'] for x in invocations),
 'bounded-prompt':all(len(x.split('\n\n',1)[1].encode())+2<=8000 for x in prompts if '\n\n' in x),
 'native-exit':code==0,
}
# Capture wrappers are useful for normalized-request evidence but their Python startup is
# not shipped overhead. Measure the real executable in a separate native callback campaign.
STOP.clear()
SERVER=http.server.ThreadingHTTPServer(('127.0.0.1',0),Provider)
SERVER.daemon_threads=True; SERVER.step=0; SERVER.benchmark=True; SERVER.handle=handle
threading.Thread(target=SERVER.serve_forever,daemon=True).start()
config['providers']['local']['settings']['baseURL']='http://127.0.0.1:'+str(SERVER.server_port)+'/v1'
config['plugins'][0]['options']['corvintBinary']=A.corvint
(repo/'opencode.json').write_text(json.dumps(config))
timing_start=len(rows(RUN/'timings.jsonl'))
fault_start=len(rows(RUN/'faults.jsonl'))
run([A.host,'run','--standalone','--format','json','--model','local/probe','--title','Qualification timing','Locate Add in add.go.'],repo,env)
cleanup()
timings = rows(RUN/'timings.jsonl')[timing_start:]
query_rows=[x for x in timings if x['kind']=='session.prompt']
query_times=[x['ms'] for x in query_rows]
checks['timed-delivery']=bool(query_rows) and all(x['delivered'] for x in query_rows) and len(rows(RUN/'faults.jsonl'))==fault_start
lifecycle_times=[x['ms'] for x in timings if x['kind'] in ('event.session.created','event.session.execution.succeeded','event.session.execution.failed','event.session.deleted') or (x['kind'].startswith('tool.execute.after:') and not x['kind'].endswith(':execute'))]
metrics={'query':query_times[1:],'lifecycle':lifecycle_times[1:],'surface':'real stock host callbacks, including normalization, spawn, framing and delivery','observer':'transparent registration wrapper; candidate source unchanged; direct Corvint executable, no capture subprocess'}
def p95(values):
    return sorted(values)[math.ceil(len(values)*0.95)-1]
checks['latency']=len(metrics['query'])>=20 and len(metrics['lifecycle'])>=20 and p95(metrics['query'])<=500 and p95(metrics['lifecycle'])<=250
query_packets=[p for p in packets if p.get('event')=='user-prompt']
checks['snapshot']=all(p['repository']['treeRevision']==tree for p in packets if 'repository' in p) and any(p['repository']['worktreeState']=='mixed' for p in packets if 'repository' in p)
checks['governance']=any(any(e.get('path')=='AGENTS.md' for row in p.get('context',{}).get('results',[]) for e in row.get('evidence',[])) for p in query_packets)
checks['oversized-exclusion']=any(p.get('context',{}).get('exclusions',{}).get('count',0)>0 for p in query_packets)
injected_bytes=len(prompts[0].split('\n\n',1)[1].encode())+2
checks['critical-recall-and-bytes']=injected_bytes<len(source.encode()) and any(row.get('name')=='Add' and any(e.get('path')=='add.go' and e.get('line')==3 for e in row.get('evidence',[])) for row in query_packets[0]['context']['results'])
checks['supplied-evidence']=any(x['input'].get('observedEvidenceHandles') for x in invocations)
checks['bounded-stops']=1<=sum(p.get('event')=='stop' for p in packets)<=2
metrics['queryP95Ms']=p95(metrics['query']);metrics['lifecycleP95Ms']=p95(metrics['lifecycle'])
metrics['injectedBytes']=injected_bytes;metrics['manualBytes']=len(source.encode())
metrics['criticalRecall']={'injected':1,'manual':1,'total':1}
for installed in (True, False):
    STOP.clear()
    SERVER=http.server.ThreadingHTTPServer(('127.0.0.1',0),Provider)
    SERVER.daemon_threads=True; SERVER.step=0; SERVER.smoke=True; SERVER.handle=handle
    threading.Thread(target=SERVER.serve_forever,daemon=True).start()
    config['providers']['local']['settings']['baseURL']='http://127.0.0.1:'+str(SERVER.server_port)+'/v1'
    config['plugins']=[{'package':(SOURCE/'integrations/opencode/src').as_uri(),'options':{'corvintBinary':str(wrapper)}}] if installed else []
    (repo/'opencode.json').write_text(json.dumps(config))
    count=len(rows(RUN/'invocations.jsonl'))
    start=len(rows(RUN/'requests.jsonl'))
    run([A.host,'run','--standalone','--format','json','--model','local/probe','--title','Qualification','Locate Add in add.go.'],repo,env)
    observed=rows(RUN/'requests.jsonl')[start:]
    advertised=any('corvint_context' in json.dumps(x) and 'harness-receipt:sha256:' in json.dumps(x) for x in observed)
    checks['direct-install' if installed else 'uninstall']=advertised if installed else not advertised and len(rows(RUN/'invocations.jsonl'))==count
    cleanup()

# Interrupt while the adapter owns a wrapper and a live child, before its timeout.
interrupted_out=RUN/'interruption'
CHILD=subprocess.Popen([os.sys.executable,__file__,'--host',A.host,'--corvint',A.corvint,'--output',str(interrupted_out),'--interrupt-probe'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,start_new_session=True)
try:
    deadline=time.monotonic()+20
    active=None
    while time.monotonic()<deadline:
        matches=list(interrupted_out.glob('run-*/active.json'))
        if matches:
            active=json.loads(matches[0].read_text());break
        if CHILD.poll() is not None:break
        time.sleep(0.05)
    assert active is not None, 'interruption probe never had an active owned child'
    observed=descendants(CHILD.pid)
    CHILD.send_signal(signal.SIGTERM)
    stdout,stderr=CHILD.communicate(timeout=20)
    deadline=time.monotonic()+3
    while True:
        live=process_states()
        survivors=[pid for pid in observed if pid in live and not live[pid][1].startswith('Z')]
        if not survivors or time.monotonic()>=deadline:break
        time.sleep(0.05)
    checks['interruption-cleanup']=CHILD.returncode==143 and not survivors
    cleanup_evidence={'exit':CHILD.returncode,'observedDescendants':observed,'active':active,'survivors':survivors,'stderr':stderr[-2000:]}
finally:
    cleanup()

checks['frozen-identities']=identities()==baseline
report = {'profile':'opencode-native-integration/1','result':'PASS' if all(checks.values()) else 'FAIL','executionAuthority':'NONE','frontier':'UNAVAILABLE','legacyReceiptSupport':'FALLBACK','hostVersion':'2.0.18','adapterVersion':json.loads((SOURCE/'integrations/opencode/package.json').read_text())['version'],'os':platform.system().lower(),'architecture':node_architecture(),**baseline,'checks':checks,'root':str(RUN),'manualBaseline':{'method':'read complete named source file','bytes':len(source.encode()),'criticalEvidence':['add.go:Add at line 3']},'metrics':metrics,'cleanupEvidence':cleanup_evidence,'inputTokens':'NOT_OBSERVED (provider usage is synthetic compaction stimulus)','requiresFocusedSuite':'TestHostAdapterJavaScriptHosts'}
(RUN/'report.json').write_text(json.dumps(report,indent=2)+'\n')
(OUT/'report.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'result':report['result'],'checks':checks,'report':str(RUN/'report.json')}))
raise SystemExit(0 if all(checks.values()) else 1)
