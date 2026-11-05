# SPDX-License-Identifier: Apache-2.0
"""Opt-in test adapter. Reads one invocation on stdin; never installs tools.
The ptrace stream observes launches/exits. Uninstrumented build/runner processes
stay incomplete even when descendant profiles exist. All raw bytes are retained
outside the repository. This adapter confers no qualification or exclusion.
"""
import base64
import glob
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

tools = Path(os.environ['CORVINT_NATIVE_LIVE_TOOLS'])
evidence = Path(os.environ['CORVINT_NATIVE_LIVE_EVIDENCE'])
source = Path(__file__).resolve().parent
family, work = sys.argv[1], Path(sys.argv[2])
work.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
env.update(CARGO_HOME=str(tools/'cargo'), RUSTUP_HOME=str(tools/'rustup'), DOTNET_CLI_HOME=str(tools/'dotnet-home'), NUGET_PACKAGES=str(tools/'nuget'), DOTNET_SKIP_FIRST_TIME_EXPERIENCE='1', DOTNET_CLI_TELEMETRY_OPTOUT='1')
(tools/'dotnet-home/.dotnet').mkdir(parents=True, exist_ok=True)
(tools/'dotnet-home/.dotnet/8.0.427.dotnetFirstUseSentinel').touch()
env['DOTNET_ADD_GLOBAL_TOOLS_TO_PATH']='false'
env['DOTNET_GENERATE_ASPNET_CERTIFICATE']='false'
env['LD_LIBRARY_PATH'] = str(tools/'swift-deps/usr/lib/x86_64-linux-gnu')
java = tools/'jdk-21.0.13+9/bin/java'
junit = tools/'junit-platform-console-standalone-1.11.4.jar'
agent = tools/'org.jacoco.agent-0.8.13-runtime.jar'
cli = tools/'org.jacoco.cli-0.8.13-nodeps.jar'
cargo = tools/'rustup/toolchains/stable-x86_64-unknown-linux-gnu/bin/cargo'
swift = tools/'swift-6.4.2-RELEASE-ubuntu24.04/usr/bin/swift'
dotnet = tools/'dotnet/dotnet'
env['PATH'] = os.pathsep.join([str(cargo.parent),str(swift.parent),env['PATH']])
def run(argv, **kwargs):
    result = subprocess.run([str(v) for v in argv], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=300, **kwargs)
    if result.returncode:
        sys.stderr.buffer.write(result.stdout + result.stderr)
        result.check_returncode()
    return result
def digest(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def b64(b): return base64.b64encode(b).decode()
def save(p,b): p.write_bytes(b)
if len(sys.argv)>3 and sys.argv[3]=='setup':
    root=work/'project'
    shutil.copytree(source/family,root,dirs_exist_ok=True)
    trace=work/'trace';run(['gcc','-O2',source/'trace.c','-o',trace])
    if family=='jvm':
        (root/'classes').mkdir(exist_ok=True)
        run([java.parent/'javac','-g','-cp',str(junit)+os.pathsep+str(agent),'-d',root/'classes',*sorted((root/'p').glob('*.java'))])
        request=dict(runner='junit-platform',root=str(root),executable=str(java),project='classes',reporter=str(junit),reporterSha256=digest(junit))
    elif family=='dotnet':
        result=run([dotnet,'build',root/'Probe.csproj','--nologo','--source','https://api.nuget.org/v3/index.json']);save(work/'setup.stdout',result.stdout);save(work/'setup.stderr',result.stderr)
        request=dict(runner='dotnet-vstest-xunit',root=str(root),executable=str(dotnet),project=str(root/'Probe.csproj'))
    elif family=='rust':
        run([cargo,'generate-lockfile','--offline','--manifest-path',root/'Cargo.toml'])
        request=dict(runner='cargo-test',root=str(root),executable=str(cargo),project=str(root/'Cargo.toml'))
    else:
        request=dict(runner='swift-testing',root=str(root),executable=str(swift),project='')
    request.update(executableSha256=digest(Path(request['executable'])),timeoutSeconds=300)
    print(json.dumps(request));sys.exit()
if len(sys.argv)>3 and sys.argv[3]=='rebuild':
    root=work/'project'
    run([java.parent/'javac','-g','-cp',str(junit)+os.pathsep+str(agent),'-d',root/'classes',*sorted((root/'p').glob('*.java'))])
    sys.exit()
packet=json.load(sys.stdin);r=packet['request'];inv=packet['invocation']
out=Path(r['reportDir']);out.mkdir(parents=True,exist_ok=False)
env.update(inv.get('environment') or {})
if family=='jvm':env['JAVA_TOOL_OPTIONS']+=' "-Dcovlive.classes='+str(work/'project/classes')+'"'
counter=work/'invocation-count';seq=int(counter.read_text())+1 if counter.exists() else 1;counter.write_text(str(seq))
prefix=evidence/(family+'-'+str(seq));prefix.parent.mkdir(parents=True,exist_ok=True)
for path,data in (inv.get('files') or {}).items():
    dest=out/path;dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(base64.b64decode(data))
for old in (work/'project/.build').glob('**/*.profraw'):
    old.unlink()
result=subprocess.run([str(work/'trace'),str(prefix)+'.trace',r['executable'],*inv['argv']],cwd=r['root'],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=300)
save(Path(str(prefix)+'.stdout'),result.stdout);save(Path(str(prefix)+'.stderr'),result.stderr)
reports={}
for pattern in (inv.get('reportPaths') or [])+(inv.get('reportPatterns') or []):
    for p in sorted(out.glob(pattern)):
        reports[p.name]=b64(p.read_bytes());save(Path(str(prefix)+'-'+p.name),p.read_bytes())
exports=[]
env.pop('JAVA_TOOL_OPTIONS', None)
if family=='jvm' and (out/'jacoco.exec').exists():
    info=run([java,'-jar',cli,'execinfo',out/'jacoco.exec']).stdout;save(Path(str(prefix)+'.execinfo'),info)
    run([java,'-jar',cli,'report',out/'jacoco.exec','--classfiles',work/'project/classes','--xml',out/'coverage.xml'])
    exports=[('jacoco', (out/'coverage.xml').read_bytes())]
elif family=='dotnet':
    exports=[('cobertura',Path(p).read_bytes()) for p in glob.glob(str(out/'**/coverage.cobertura.xml'),recursive=True)]
else:
    profiles=sorted(out.glob('*.profraw'))
    if family=='swift':profiles+=sorted((work/'project/.build').glob('**/*.profraw'))
    if profiles:
        if family=='rust':
            llvm=tools/'rustup/toolchains/stable-x86_64-unknown-linux-gnu/lib/rustlib/x86_64-unknown-linux-gnu/bin'
            binaries=[p for p in (work/'project/target/debug/deps').glob('covlive_probe-*') if p.is_file() and os.access(p,os.X_OK) and not p.suffix]
        else:
            llvm=swift.parent
            # Swift Testing 6.4's native build engine loads instrumented test
            # shared libraries; it does not produce an XCTest executable.
            binaries=sorted({p.resolve() for p in (work/'project/.build').glob('**/*.so') if p.name in ('Logic.so','LogicTests.so')})
        run([llvm/'llvm-profdata','merge','-sparse',*profiles,'-o',out/'merged.profdata'])
        if binaries:
            args=[llvm/'llvm-cov','export','-format=lcov','-instr-profile='+str(out/'merged.profdata'),binaries[0]]
            for b in binaries[1:]:args+=['-object',b]
            # The fixture's complete original source set is explicit. SwiftPM
            # generated discovery glue is not a repository source mapping.
            if family=='swift':args+=sorted((work/'project/Sources').glob('**/*.swift'))+sorted((work/'project/Tests').glob('**/*.swift'))
            exports=[('lcov',run(args).stdout)]
for i,(fmt,b) in enumerate(exports):save(Path(str(prefix)+f'.coverage-{i}.{fmt}'),b)
# Preserve raw terminal and coverage bytes. Unknown launchers cannot be upgraded
# from the existence of their children's coverage. JVM sessions name real PIDs.
processes={};complete=False
for line in Path(str(prefix)+'.trace').read_text().splitlines():
    f=line.split(' ',2)
    if f[0]=='COMPLETE':complete=True
    elif f[0]=='START':processes[int(f[1])]=dict(ID=int(f[1]),Parent=int(f[2]),Exited=False,Instrumented=False,Reports=[])
    elif f[0]=='EXEC' and int(f[1]) in processes:
        p=processes[int(f[1])]
        p.update(Instrumented=False,Reports=[])
        if family=='jvm' and Path(f[2])==java and len(exports)==1 and ('Session "'+f[1]+'"').encode() in info:
            p.update(Instrumented=True,Reports=[dict(Format=exports[0][0],Data=b64(exports[0][1]))])
    elif f[0]=='EXIT' and int(f[1]) in processes:processes[int(f[1])]['Exited']=True
capture=dict(Input=dict(runner=r['runner'],target=r.get('target',''),stdout=b64(result.stdout),stderr=b64(result.stderr),reports=reports,exitCode=result.returncode,selectors=r.get('selectors'),expected=r.get('expectedTests')),TreeComplete=complete,Processes=list(processes.values()))
save(Path(str(prefix)+'.capture.json'),(json.dumps(capture,indent=2)+'\n').encode())
print(json.dumps(capture))
