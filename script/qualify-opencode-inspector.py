#!/usr/bin/env python3
"""AHI-033: stock OpenCode terminal -> context RPC -> pinned source; no model call."""
import argparse
import atexit
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pty
import re
import select
import signal
import struct
import subprocess
import tempfile
import termios
import time

P = argparse.ArgumentParser()
P.add_argument('--host', required=True)
P.add_argument('--corvint', required=True)
P.add_argument('--output', required=True)
P.add_argument('--interrupt-probe', action='store_true')
A = P.parse_args()
SOURCE = Path(__file__).resolve().parents[1]
OUT = Path(A.output).resolve()
OUT.mkdir(parents=True, exist_ok=True)
RUN = Path(tempfile.mkdtemp(prefix='ui-', dir=OUT))
CHILD = None
OWNED = set()
MASTER = None
RAW = b''


def states():
    rows = subprocess.check_output(['ps', '-axo', 'pid=,ppid=,stat='], text=True).splitlines()
    return {int(pid): (int(parent), state) for pid, parent, state in (row.split() for row in rows)}


def observe():
    if CHILD is None:
        return
    rows, pending = states(), [CHILD.pid]
    while pending:
        OWNED.update(pending)
        pending = [pid for pid, (parent, _) in rows.items() if parent in pending]


def cleanup():
    global CHILD, MASTER
    observe()
    if CHILD is not None:
        try:
            os.killpg(CHILD.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            CHILD.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(CHILD.pid, signal.SIGKILL)
            CHILD.wait(timeout=5)
        CHILD = None
    for pid in OWNED:
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    if MASTER is not None:
        os.close(MASTER)
        MASTER = None
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        rows = states()
        survivors = [pid for pid in OWNED if pid in rows and not rows[pid][1].startswith('Z')]
        if not survivors:
            break
        time.sleep(.05)
    (RUN/'cleanup.json').write_text(json.dumps({'observed': sorted(OWNED), 'survivors': survivors}))
    return survivors


def interrupted(number, _):
    cleanup()
    raise SystemExit(128 + number)


atexit.register(cleanup)
for number in (signal.SIGINT, signal.SIGTERM):
    signal.signal(number, interrupted)


def command(argv, cwd, env=None):
    global CHILD
    CHILD = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
    try:
        stdout, stderr = CHILD.communicate(timeout=45)
        if CHILD.returncode:
            raise RuntimeError(str(argv)+': '+stderr)
        return stdout
    finally:
        if CHILD.poll() is None:
            os.killpg(CHILD.pid, signal.SIGKILL)
            CHILD.wait()
        CHILD = None


ANSI = re.compile(r'\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\)|[()][A-Z0-9]|.)')


def read_until(predicate, label, timeout=20):
    global RAW
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline and CHILD.poll() is None:
        observe()
        if select.select([MASTER], [], [], .1)[0]:
            try:
                RAW += os.read(MASTER, 65536)
            except OSError:
                break
        if predicate():
            return
    (RUN/'terminal.bin').write_bytes(RAW)
    raise RuntimeError('Native UI witness missing: '+label)


def text():
    return ANSI.sub('', RAW.decode(errors='replace'))


def send(value):
    os.write(MASTER, value)


try:
    root, helper = RUN/'repo', RUN/'helper'
    root.mkdir(); helper.mkdir()
    env = {key: value for key, value in os.environ.items() if key in ('PATH', 'HOME', 'USER', 'TMPDIR', 'SHELL')}
    for key, name in [('CONFIG', 'config'), ('DATA', 'data'), ('CACHE', 'cache'), ('STATE', 'state')]:
        (RUN/name).mkdir()
        env['XDG_'+key+'_HOME'] = str(RUN/name)
    env.update(TERM='xterm-256color', OPENCODE_DISABLE_AUTOUPDATE='1')
    version = command([A.host, '--version'], root, env).strip()
    if version != 'opencode v2.0.18':
        raise RuntimeError('Unqualified UI host: '+version)
    command(['git', 'init', '-q'], root)
    (root/'go.mod').write_text('module example.com/inspector\n\ngo 1.27.1\n')
    (root/'add.go').write_text('package inspector\n\n// Add preserves the NATIVE_INSPECTOR_WITNESS contract.\nfunc Add(a, b int) int { return a + b }\n')
    (root/'multiply.go').write_text('package inspector\n\n// Multiply preserves the NATIVE_MULTIPLY_WITNESS contract.\nfunc Multiply(a, b int) int { return a * b }\n')
    (root/'examples').mkdir()
    (root/'examples/multiply.go').write_text('package examples\n\n// Multiply uses NATIVE_SECOND_WITNESS.\nfunc Multiply(a, b int) int { return a * b }\n')
    (root/'AGENTS.md').write_text('# Inspector fixture\nUse Add in add.go for addition.\n')
    (root/'.gitignore').write_text('opencode.json\n.corvint/\n.context-corvint/\n')
    command(['git', 'add', '.'], root)
    command(['git', '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture'], root)
    command([A.corvint, 'index', '--if-stale'], root, env)
    (root/'opencode.json').write_text(json.dumps({'plugins': [{'package': (SOURCE/'integrations/opencode/src').as_uri(), 'options': {'corvintBinary': A.corvint, 'queryTimeoutMs': 10000}}]}))
    log = RUN/'witness.jsonl'
    log.write_text('')
    (helper/'tui.tsx').write_text(r'''import { Plugin } from "@opencode/plugin/tui"
import { appendFileSync } from "node:fs"
const log = value => appendFileSync(LOG, JSON.stringify(value)+"\n")
export default Plugin.define({id:"inspector-witness",setup(ctx){
 log({stage:"loaded",version:ctx.app.version})
 return ctx.ui.slot({append:"app",render:()=>{
 ctx.keymap.layer(()=>({mode:"global",commands:[
 {bind:"f6",run:async()=>{try{const s=await ctx.client.session.create({title:"Inspector fixture",location:{directory:ROOT}});ctx.ui.router.navigate({type:"session",sessionID:s.id});log({stage:"session",id:s.id})}catch(e){log({error:String(e)})}}},
 {bind:"f7",run:()=>{ctx.keymap.dispatch("corvint.context","Locate Multiply in multiply.go");log({stage:"requested"})}},
 ]}));return null}})
}})
'''.replace('LOG', json.dumps(str(log))).replace('ROOT', json.dumps(str(root))))
    config = RUN/'config/opencode'
    config.mkdir(exist_ok=True)
    (config/'cli.json').write_text(json.dumps({'plugins': [str(helper)]}))
    MASTER, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 48, 160, 0, 0))
    CHILD = subprocess.Popen([A.host, '--standalone'], cwd=root, env=env, stdin=slave, stdout=slave, stderr=slave, start_new_session=True)
    os.close(slave)
    read_until(lambda: 'Ask anything' in text(), 'home')
    send(b'\x1b[17~')
    read_until(lambda: 'Corvint context' in text(), 'sidebar')
    send(b'\x1b[18~')
    read_until(lambda: 'Why included:' in text(), 'context evidence')
    send(b'\x1b[B')
    send(b'\r')
    read_until(lambda: 'NATIVE_MULTIPLY_WITNESS' in text(), 'selected pinned source')
    wide = text()
    (RUN/'wide-terminal.txt').write_text(wide)
    (RUN/'wide-terminal.bin').write_bytes(RAW)
    if A.interrupt_probe:
        (OUT/'ready.json').write_text(json.dumps({'pid': os.getpid(), 'run': str(RUN)}))
        while True:
            observe()
            time.sleep(.1)
    RAW = b''
    fcntl.ioctl(MASTER, termios.TIOCSWINSZ, struct.pack('HHHH', 42, 72, 0, 0))
    os.kill(CHILD.pid, signal.SIGWINCH)
    read_until(lambda: 'NATIVE_MULTIPLY_WITNESS' in text(), 'narrow pinned source')
    send(b'\x1b[A'); send(b'\r')
    read_until(lambda: 'NATIVE_SECOND_WITNESS' in text(), 'narrow keyboard source selection')
    send(b'g')
    read_until(lambda: 'Gaps and limitations' in text(), 'keyboard gap navigation')
    (RUN/'narrow-terminal.txt').write_text(text())
    (RUN/'narrow-terminal.bin').write_bytes(RAW)
    survivors = cleanup()
    if survivors:
        raise RuntimeError('Owned processes survived: '+str(survivors))
    # Interrupt this same harness after it has reached the real host and evidence path.
    interrupted_out = RUN/'interrupted'
    CHILD = subprocess.Popen([os.sys.executable, '-B', __file__, '--host', A.host, '--corvint', A.corvint, '--output', str(interrupted_out), '--interrupt-probe'], start_new_session=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    deadline = time.monotonic()+60
    while not (interrupted_out/'ready.json').exists() and time.monotonic()<deadline and CHILD.poll() is None:
        observe(); time.sleep(.1)
    if not (interrupted_out/'ready.json').exists():
        raise RuntimeError('Interruption witness did not reach native UI')
    probe = json.loads((interrupted_out/'ready.json').read_text())
    CHILD.send_signal(signal.SIGTERM)
    stdout, stderr = CHILD.communicate(timeout=15)
    exit_code = CHILD.returncode
    CHILD = None
    interruption = json.loads((Path(probe['run'])/'cleanup.json').read_text())
    if exit_code != 143 or not interruption['observed'] or interruption['survivors']:
        raise RuntimeError('Interruption cleanup failed: '+stderr)
    report = {'profile': 'corvint-opencode-inspector-witness/0', 'result': 'PASS', 'host': version, 'sourceCommit': command(['git', 'rev-parse', 'HEAD'], SOURCE).strip(), 'sourceSHA256': {str(p.relative_to(SOURCE)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted((SOURCE/'integrations/opencode/src').glob('*')) if p.is_file()}, 'checks': ['native-sidebar', 'context-rpc', 'evidence-panel', 'pinned-source', 'keyboard-source-selection', 'narrow-width', 'keyboard-gaps', 'interruption-no-descendants'], 'root': str(RUN), 'interruption': interruption, 'authority': 'NONE', 'qualification': 'UI witness only; does not promote integration support'}
    (OUT/'report.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'result': 'PASS', 'report': str(OUT/'report.json')}))
except Exception as error:
    (RUN/'terminal.bin').write_bytes(RAW)
    print(str(error), file=os.sys.stderr)
    raise
