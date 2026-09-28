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
P.add_argument('--theme', choices=['dark', 'light'], default='dark')
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
        stdout, stderr = CHILD.communicate(timeout=90)
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


def frame():
    try:
        return json.loads((RUN/'screen.json').read_text())
    except (FileNotFoundError, json.JSONDecodeError):
        return None


def text():
    current = frame()
    return '\n'.join(''.join(span['text'] for span in line) for line in current['lines']) if current else ANSI.sub('', RAW.decode(errors='replace'))


def capture(name):
    (RUN/(name+'.txt')).write_text(text())
    (RUN/(name+'.bin')).write_bytes(RAW)
    (RUN/(name+'.json')).write_text(json.dumps(frame()))


def click_text(value, before_column=None):
    for y, line in enumerate(text().splitlines(), 1):
        if value in line:
            x = line.index(value)+1
            if before_column is not None and x >= before_column:
                continue
            send(('\x1b[<0;%d;%dM\x1b[<0;%d;%dm' % (x,y,x,y)).encode())
            return
    raise RuntimeError('Clickable text not visible: '+value)



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
    (root/'multiply.go').write_text('package inspector\n\n'+''.join('// Before citation %02d\n'%i for i in range(35))+'// Multiply preserves the NATIVE_MULTIPLY_WITNESS contract.\nfunc Multiply(a, b int) int { return a * b }\n'+''.join('// After citation %02d\n'%i for i in range(45)))
    (root/'examples').mkdir()
    (root/'examples/multiply.go').write_text('package examples\n\n// Multiply uses NATIVE_SECOND_WITNESS.\nfunc Multiply(a, b int) int { return a * b }\n')
    (root/'multiply_test.go').write_text('package inspector\n\nimport "testing"\n\nfunc TestMultiply(t *testing.T) { if Multiply(3, 4) != 12 { t.Fatal("wrong product") }; t.Log("COCKPIT_PROOF_WITNESS") }\n')
    (root/'AGENTS.md').write_text('# Inspector fixture\nUse Add in add.go for addition.\n')
    (root/'.gitignore').write_text('opencode.json\n.corvint/\n.context-corvint/\n')
    command(['git', 'add', '.'], root)
    command(['git', '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture'], root)
    base_revision = command(['git', 'rev-parse', 'HEAD'], root).strip()
    verification_key = 'c' * 64
    verification_plan = RUN/'verification-plan.json'
    verification_plan.write_text(json.dumps({'base': base_revision, 'intents': ['AGENTS.md'], 'checks': [{'id': 'unit', 'argv': ['go', 'test', '-v', './...'], 'timeoutSeconds': 60}]}))
    command([A.corvint, 'dogfood', 'begin', '--plan', str(verification_plan), '--session-key', verification_key], root, env)
    with (root/'multiply.go').open('a') as changed_source:
        changed_source.write('// Committed change for the cockpit witness.\n')
    command(['git', 'add', 'multiply.go'], root)
    command(['git', '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'change'], root)
    command([A.corvint, 'dogfood', 'verify', '--session-key', verification_key, '--check', 'unit'], root, env)
    command([A.corvint, 'index', '--if-stale'], root, env)
    (root/'opencode.json').write_text(json.dumps({'plugins': [{'package': (SOURCE/'integrations/opencode/src').as_uri(), 'options': {'corvintBinary': A.corvint, 'queryTimeoutMs': 10000}}]}))
    log = RUN/'witness.jsonl'
    log.write_text('')
    (helper/'tui.tsx').write_text(r'''import { Plugin } from "@opencode/plugin/tui"
import { appendFileSync, writeFileSync, renameSync } from "node:fs"
const log = value => appendFileSync(LOG, JSON.stringify(value)+"\n")
export default Plugin.define({id:"inspector-witness",setup(ctx){
 let last = "", pending = ""
 const capture = buffer => {
  pending = JSON.stringify({width:buffer.width,height:buffer.height,lines:buffer.getSpanLines().map(line=>line.spans.map(span=>({text:span.text,width:span.width,fg:span.fg.toInts(),bg:span.bg.toInts(),attributes:span.attributes})))})
 }
 const flush = () => {
  if(pending!==last){writeFileSync(SCREEN+".tmp",pending);renameSync(SCREEN+".tmp",SCREEN);last=pending}
 }
 ctx.renderer.on("frame",flush)
 ctx.renderer.addPostProcessFn(capture)
 log({stage:"loaded",version:ctx.app.version})
 const stop=ctx.ui.slot({append:"app",render:()=>{
 ctx.keymap.layer(()=>({mode:"global",commands:[
 {bind:"f6",run:async()=>{try{const s=await ctx.client.session.create({title:"Inspector fixture",location:{directory:ROOT}});ctx.ui.router.navigate({type:"session",sessionID:s.id});log({stage:"session",id:s.id})}catch(e){log({error:String(e)})}}},
 {bind:"f7",run:()=>{ctx.keymap.dispatch("corvint.context","Locate Multiply in multiply.go");log({stage:"requested"})}},
 {bind:"f8",run:()=>ctx.keymap.dispatch("corvint.context")},
 ]}));return null}})
 return ()=>{ctx.renderer.off("frame",flush);ctx.renderer.removePostProcessFn(capture);stop()}
}})
'''.replace('LOG', json.dumps(str(log))).replace('SCREEN', json.dumps(str(RUN/'screen.json'))).replace('ROOT', json.dumps(str(root))))
    config = RUN/'config/opencode'
    config.mkdir(exist_ok=True)
    (config/'cli.json').write_text(json.dumps({'plugins': [str(helper)], 'theme': {'mode': A.theme}, 'mouse': True}))
    MASTER, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 48, 160, 0, 0))
    CHILD = subprocess.Popen([A.host, '--standalone'], cwd=root, env=env, stdin=slave, stdout=slave, stderr=slave, start_new_session=True)
    os.close(slave)
    read_until(lambda: 'Ask anything' in text(), 'home')
    send(b'\x1b[17~')
    read_until(lambda: 'Corvint context' in text(), 'sidebar')
    send(b'\x1b[18~')
    read_until(lambda: 'Ready' in text() and '2 of 2 locations' in text(), 'context evidence')
    send(b'f')
    read_until(lambda: 'Why included:' in text() and frame()['width'] == 160, 'wide master detail')
    send(b'\x1b[B'); send(b'\r')
    read_until(lambda: 'NATIVE_MULTIPLY_WITNESS' in text() and 'cited line 39' in text(), 'selected cited source')
    parser_cache = RUN/'data/opentui/tree-sitter'
    if list(parser_cache.glob('languages/*')) or list(parser_cache.glob('queries/*')):
        raise RuntimeError('Plain source reading fetched a parser into the cold cache')
    capture('plain-terminal')
    send(b'h')
    read_until(lambda: 'Syntax enabled' in text(), 'explicit host syntax opt-in')
    capture('wide-terminal')
    send(b'\x1b[5~')
    read_until(lambda: 'Before citation 05' in text(), 'keyboard source scrolling')
    send(b'l')
    read_until(lambda: 'NATIVE_MULTIPLY_WITNESS' in text(), 'return to cited line')
    if A.interrupt_probe:
        (OUT/'ready.json').write_text(json.dumps({'pid': os.getpid(), 'run': str(RUN)}))
        while True:
            observe()
            time.sleep(.1)
    send(b'\t')
    send(b'/')
    read_until(lambda: 'Find evidence' in text(), 'search dialog')
    send(b'rgx/\t\x1b')
    read_until(lambda: 'Find evidence' not in text() and '2 of 2 locations' in text(), 'dialog cancel isolation')
    send(b'/')
    read_until(lambda: 'Find evidence' in text(), 'reopen search dialog')
    send(b'rgx/nonexistent')
    read_until(lambda: 'rgx/nonexistent' in text() and 'Find evidence' in text(), 'dialog focus isolation')
    send(b'\r')
    read_until(lambda: 'No matching evidence.' in text(), 'empty filter')
    send(b'x')
    read_until(lambda: '2 of 2 locations' in text(), 'clear filter')
    send(b'/')
    read_until(lambda: 'Find evidence' in text(), 'second search')
    send(b'examples\r')
    read_until(lambda: '1 of 2 locations' in text() and 'NATIVE_MULTIPLY_WITNESS' not in text(), 'matching filter clears old source')
    click_text('multiply.go:4')
    read_until(lambda: 'NATIVE_SECOND_WITNESS' in text(), 'pointer source opening')
    send(b'x'); send(b'\x1b[B'); send(b'\r')
    read_until(lambda: 'NATIVE_MULTIPLY_WITNESS' in text(), 'selection after filter')
    RAW = b''
    fcntl.ioctl(MASTER, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 72, 0, 0))
    os.kill(CHILD.pid, signal.SIGWINCH)
    read_until(lambda: frame()['width'] == 72 and 'NATIVE_MULTIPLY_WITNESS' in text() and 'Tab list/source' in text(), 'short narrow source')
    capture('narrow-terminal')
    send(b'\t')
    read_until(lambda: '2 of 2 locations' in text(), 'compact list navigation')
    send(b'\x1b[A'); send(b'\r')
    read_until(lambda: 'NATIVE_SECOND_WITNESS' in text(), 'narrow keyboard source selection')
    send(b'g')
    read_until(lambda: 'Gaps and limitations' in text(), 'keyboard gap navigation')
    capture('gaps-terminal')
    send(b'i')
    read_until(lambda: 'Evidence details' in text() and 'Receipt:' in text(), 'identity disclosure')
    # A new context request invalidates the displayed source before the next expansion.
    send(b'\x1b[18~')
    read_until(lambda: 'Ready' in text() and 'Receipt:' in text(), 'fresh query')
    send(b'\t'); send(b'\t')
    read_until(lambda: 'Enter Open pinned source' in text() and 'NATIVE_SECOND_WITNESS' not in text(), 'source invalidation')
    fcntl.ioctl(MASTER, termios.TIOCSWINSZ, struct.pack('HHHH', 48, 160, 0, 0))
    os.kill(CHILD.pid, signal.SIGWINCH)
    send(b'\x1b[19~')
    read_until(lambda: 'Corvint · Change' in text() and '1 files' in text(), 'change cockpit')
    if 'obligations open' not in text():
        raise RuntimeError('Qualified fixture check incorrectly implied workflow completion')
    send(b'f')
    read_until(lambda: 'Recorded intent scopes' in text(), 'wide change details')
    capture('cockpit-files')
    send(b'\r')
    read_until(lambda: 'Why selected:' in text() and 'multiply_test.go' in text(), 'affected dependency witness')
    capture('cockpit-impact')
    send(b'3')
    read_until(lambda: 'PASS · unit' in text(), 'observed verification result')
    click_text('PASS · unit', before_column=40)
    read_until(lambda: 'COCKPIT_PROOF_WITNESS' in text(), 'recorded verification output')
    capture('cockpit-proof')
    send(b'b')
    read_until(lambda: 'Compare from revision' in text(), 'base selection dialog')
    send(b'HEAD\x1b')
    read_until(lambda: 'Compare from revision' not in text(), 'base dialog cancel isolation')
    fcntl.ioctl(MASTER, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 72, 0, 0))
    os.kill(CHILD.pid, signal.SIGWINCH)
    read_until(lambda: frame()['width'] == 72 and 'Tab list/details' in text(), 'compact cockpit proof')
    capture('cockpit-narrow')
    send(b'e')
    read_until(lambda: 'Corvint · Evidence' in text() and 'c Change' in text(), 'cockpit to governing context')
    send(b'c')
    read_until(lambda: 'Corvint · Change' in text() and '1 files' in text(), 'return to cockpit')
    with (root/'multiply.go').open('a') as changed_source:
        changed_source.write('// External edit invalidates the recorded check.\n')
    send(b'r')
    read_until(lambda: '1 files' in text(), 'refresh external edit')
    send(b'3')
    read_until(lambda: ('STALE · unit' in text() or 'UNVERIFIED · unit' in text()) and 'PASS · unit' not in text(), 'old verification is not current after edit')
    capture('cockpit-stale')
    survivors = cleanup()
    if survivors:
        raise RuntimeError('Owned processes survived: '+str(survivors))
    # Interrupt this same harness after it has reached the real host and evidence path.
    interrupted_out = RUN/'interrupted'
    CHILD = subprocess.Popen([os.sys.executable, '-B', __file__, '--host', A.host, '--corvint', A.corvint, '--output', str(interrupted_out), '--theme', A.theme, '--interrupt-probe'], start_new_session=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
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
    report = {'profile': 'corvint-opencode-inspector-witness/0', 'result': 'PASS', 'host': version, 'theme': A.theme, 'sourceCommit': command(['git', 'rev-parse', 'HEAD'], SOURCE).strip(), 'sourceSHA256': {str(p.relative_to(SOURCE)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted((SOURCE/'integrations/opencode/src').glob('*')) if p.is_file()}, 'checks': ['native-sidebar', 'context-rpc', 'evidence-panel', 'pinned-source', 'keyboard-source-selection', 'narrow-width', 'keyboard-gaps', 'native-frame-capture', 'cited-line', 'source-scroll', 'filter', 'empty-filter', 'dialog-focus', 'pointer-open', 'short-height', 'source-invalidation', 'cold-cache-plain-source', 'explicit-syntax-opt-in', 'cockpit-files', 'cockpit-impact', 'cockpit-proof', 'cockpit-unsatisfied-workflow', 'cockpit-base-dialog', 'cockpit-narrow', 'cockpit-context', 'cockpit-stale-check', 'interruption-no-descendants'], 'root': str(RUN), 'interruption': interruption, 'authority': 'NONE', 'qualification': 'UI witness only; does not promote integration support'}
    if A.theme == 'dark':
        light_out = OUT/'light'
        command([os.sys.executable, '-B', __file__, '--host', A.host, '--corvint', A.corvint, '--output', str(light_out), '--theme', 'light'], SOURCE)
        light = json.loads((light_out/'report.json').read_text())
        if light['result'] != 'PASS':
            raise RuntimeError('Light-theme witness failed')
        report['checks'].append('light-theme')
        report['lightReport'] = str(light_out/'report.json')
    (OUT/'report.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'result': 'PASS', 'report': str(OUT/'report.json')}))
except Exception as error:
    (RUN/'terminal.bin').write_bytes(RAW)
    print(str(error), file=os.sys.stderr)
    raise
