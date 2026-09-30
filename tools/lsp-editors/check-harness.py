#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Outer lifetime regression fixtures; these are never editor qualification."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time

harness = Path(__file__).resolve().parents[2] / 'script/qualify-lsp-editors.py'
with tempfile.TemporaryDirectory(prefix='editor-harness-check-') as td:
    d = Path(td)
    fixture, pids, result = d / 'fixture', d / 'pids', d / 'report.json'
    def configure(mode):
        fixture.write_text('#!/usr/bin/env python3\nimport json,os,pathlib,sys,time\n'
            'if "--version" in sys.argv: print("regression fixture");sys.exit(0)\n'
            'pathlib.Path(' + repr(str(pids)) + ').write_text(str(os.getpid()))\n'
            'pathlib.Path(os.environ["CORVINT_EDITOR_RESULT"]).write_text(' +
            (repr('{') if mode == 'malformed' else 'json.dumps({"status":"LIFECYCLE_OBSERVED"})') + ')\n' +
            ('time.sleep(30)\n' if mode == 'interrupt' else ''))
        fixture.chmod(0o755)
    args = [sys.executable, str(harness), '--client', 'neovim', '--client-bin', str(fixture),
            '--server', '/bin/cat', '--report', str(result)]
    for mode in ['missing-wire', 'malformed']:
        configure(mode)
        run = subprocess.run(args, capture_output=True, timeout=15)
        assert run.returncode == 1, (mode, run.stderr)
        report = json.loads(result.read_text())
        assert not report['wireEvidence']['valid']
        assert report['cleanup'] == 'OWNED_EDITOR_PROCESSES_RETIRED'
        if mode == 'malformed': assert report['evidenceErrors']
    for sig in [signal.SIGINT, signal.SIGTERM]:
        configure('interrupt'); pids.unlink(); result.unlink()
        proc = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            deadline = time.monotonic() + 5
            while not pids.exists() and time.monotonic() < deadline: time.sleep(.02)
            assert pids.exists(), 'fixture did not start'
            editor = int(pids.read_text()); proc.send_signal(sig)
            out, err = proc.communicate(timeout=15)
            assert proc.returncode == 1, err
            report = json.loads(result.read_text())
            assert report['failure']['type'] == 'HarnessInterrupted'
            assert report['cleanup'] == 'OWNED_EDITOR_PROCESSES_RETIRED'
            try: os.kill(editor, 0)
            except ProcessLookupError: pass
            else: raise AssertionError('owned fixture editor remained')
        finally:
            if pids.exists():
                try: os.killpg(int(pids.read_text()), signal.SIGKILL)
                except ProcessLookupError: pass
            if proc.poll() is None: proc.kill(); proc.wait()
print('outer INT/TERM, malformed evidence, and missing-wire false success regressions passed; real editor interruption NOT_RUN')
