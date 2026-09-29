#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Test harness transport and owned-group retirement; no product qualification."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time

proxy = Path(__file__).with_name('wire-proxy.py')
with tempfile.TemporaryDirectory(prefix='editor-proxy-check-') as td:
    d = Path(td)
    message = json.dumps({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {'fragmented': 'x' * 100000}}).encode()
    frame = b'Content-Length: ' + str(len(message)).encode() + b'\r\n\r\n' + message
    proc = subprocess.Popen([sys.executable, str(proxy), str(d / 'transcript'), '/bin/cat'],
                            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    out, err = proc.communicate(frame, timeout=5)
    assert out == frame, (out, err)
    rows = [json.loads(line) for line in (d / 'transcript').read_text().splitlines()]
    assert any(row['direction'] == 'client-to-server' for row in rows)
    assert any(row['direction'] == 'server-to-client' for row in rows)
    assert rows[-1]['message']['groupRetired'] is True
    # A child that ignores SIGTERM must still retire when the proxy is interrupted.
    server = d / 'server.py'
    server.write_text("import os,signal,subprocess,sys,time\nsignal.signal(signal.SIGTERM,signal.SIG_IGN)\np=subprocess.Popen([sys.executable,'-c','import signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); time.sleep(60)'])\nopen(sys.argv[1],'w').write(str(os.getpid())+' '+str(p.pid))\ntime.sleep(60)\n")
    pids = d / 'pids'
    proc = subprocess.Popen([sys.executable, str(proxy), str(d / 'interrupt'), sys.executable,
                             str(server), str(pids)], stdin=subprocess.PIPE,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    deadline = time.monotonic() + 5
    while not pids.exists() and time.monotonic() < deadline: time.sleep(.02)
    assert pids.exists(), 'fixture start timeout'
    group, child = map(int, pids.read_text().split())
    time.sleep(.1)
    try:
        proc.terminate(); proc.communicate(timeout=5)
    finally:
        if proc.poll() is None: proc.kill(); proc.wait()
        try: os.killpg(group, signal.SIGKILL)
        except ProcessLookupError: pass
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        try: os.kill(child, 0)
        except ProcessLookupError: break
        time.sleep(.02)
    else: raise AssertionError('owned descendant remains after proxy interruption')
    try: os.killpg(group, 0)
    except ProcessLookupError: pass
    else: raise AssertionError('owned server group remains')
print('proxy transparent frames, EOF and interrupted descendant cleanup passed; editor conformance NOT_RUN')
