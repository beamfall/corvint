#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Private bounded transport transcript; owns only its configured server group."""
import json
import hashlib
from pathlib import Path
import os
import signal
import subprocess
import sys
import threading
import time

server = subprocess.Popen(sys.argv[2:], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, start_new_session=True)
log = open(sys.argv[1], 'w')
lock = threading.Lock()
used = 0

def record(direction, message):
    global used
    row = json.dumps({'direction': direction, 'message': message}, ensure_ascii=True) + '\n'
    with lock:
        if used + len(row) <= 262144:
            log.write(row); log.flush(); used += len(row)
        elif used < 262144:
            log.write(json.dumps({'transcript': 'TRUNCATED'}) + '\n'); log.flush(); used = 262144

record('provenance', {'serverExecutableSha256': hashlib.sha256(Path(sys.argv[2]).read_bytes()).hexdigest(), 'serverArgv': sys.argv[2:]})

def pump(source, dest, direction):
    try:
        while True:
            headers = bytearray()
            while not headers.endswith(b'\r\n\r\n'):
                char = source.read(1)
                if not char: return
                headers.extend(char)
                if len(headers) > 8192: raise ValueError('header-bound')
            length = next(int(row.split(b':', 1)[1]) for row in headers.split(b'\r\n')
                          if row.lower().startswith(b'content-length:'))
            if not 0 <= length <= 1048576: raise ValueError('message-bound')
            body = bytearray()
            while len(body) < length:
                chunk = source.read(length - len(body))
                if not chunk: raise ValueError('truncated-frame')
                body.extend(chunk)
            record(direction, json.loads(body))
            dest.write(headers + body); dest.flush()
    except (BrokenPipeError, ValueError, StopIteration) as err:
        record(direction, {'proxyError': str(err)})
    finally:
        try: dest.close()
        except OSError: pass

def retire(*_):
    if server.poll() is None:
        os.killpg(server.pid, signal.SIGTERM)
        try: server.wait(timeout=2)
        except subprocess.TimeoutExpired:
            os.killpg(server.pid, signal.SIGKILL); server.wait()
    # Group retirement is checked even if the direct child already exited.
    try:
        os.killpg(server.pid, 0)
    except ProcessLookupError:
        record('lifecycle', {'serverExit': server.returncode, 'groupRetired': True})
    else:
        os.killpg(server.pid, signal.SIGKILL)
        record('lifecycle', {'serverExit': server.returncode, 'groupRetired': 'forced'})

def force_retire():
    try: os.killpg(server.pid, signal.SIGKILL)
    except ProcessLookupError: pass
kill_timer = None
def interrupt(*_):
    global kill_timer
    # Never reenter Popen.wait from a signal handler while its lock is held.
    try: os.killpg(server.pid, signal.SIGTERM)
    except ProcessLookupError: pass
    if kill_timer is None:
        kill_timer = threading.Timer(1.5, force_retire)
        kill_timer.daemon = True
        kill_timer.start()
signal.signal(signal.SIGTERM, interrupt)
signal.signal(signal.SIGINT, interrupt)
thread = threading.Thread(target=pump, args=(server.stdout, sys.stdout.buffer, 'server-to-client'), daemon=True)
thread.start()
def stderr():
    while True:
        b = server.stderr.read(4096)
        if not b: return
        record('server-stderr', b.decode(errors='replace'))
threading.Thread(target=stderr, daemon=True).start()
input_stream = os.fdopen(os.dup(sys.stdin.fileno()), 'rb', buffering=0)
input_thread = threading.Thread(target=pump, args=(input_stream, server.stdin, 'client-to-server'), daemon=True)
input_thread.start()
try:
    server.wait()
finally:
    retire(); thread.join(timeout=1)
    if kill_timer is not None: kill_timer.cancel()
    log.close()
sys.exit(server.returncode if server.returncode is not None and server.returncode >= 0 else 1)
