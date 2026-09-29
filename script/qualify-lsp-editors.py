#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Run actual editor clients in disposable state; never infer full qualification."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]

class HarnessInterrupted(RuntimeError):
    pass

def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

def inventory():
    rows = subprocess.run(['/bin/ps', '-axo', 'pid=,ppid=,command='], capture_output=True, text=True, check=True).stdout
    found = {}
    for line in rows.splitlines():
        fields = line.strip().split(None, 2)
        if len(fields) == 3: found[int(fields[0])] = (int(fields[1]), fields[2])
    return found

def cleanup_owned(proc, d, report):
    processes = inventory()
    owned = {pid for pid, (_, cmd) in processes.items() if str(d) in cmd and
             (cmd.startswith('/Applications/Visual Studio Code.app/') or str(ROOT / 'tools/lsp-editors/wire-proxy.py') in cmd)}
    if proc is not None and proc.pid in processes: owned.add(proc.pid)
    while True:
        expanded = owned | {pid for pid, (parent, _) in processes.items() if parent in owned}
        if expanded == owned: break
        owned = expanded
    report['ownedCleanupPids'] = sorted(owned)
    if proc is not None and proc.poll() is None:
        try: os.killpg(proc.pid, signal.SIGTERM)
        except ProcessLookupError: pass
        try: out, err = proc.communicate(timeout=3)
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid, signal.SIGKILL); out, err = proc.communicate(timeout=3)
        report.update(exitCode=proc.returncode, stdout=out.decode(errors='replace')[-8192:], stderr=err.decode(errors='replace')[-8192:])
    for attempt in range(3):
        current = inventory()
        for pid in owned:
            if pid in current and pid in processes and current[pid][1] == processes[pid][1]:
                try: os.kill(pid, signal.SIGTERM if attempt == 0 else signal.SIGKILL)
                except ProcessLookupError: pass
        time.sleep(.2)
        current = inventory()
        for pid, (parent, cmd) in current.items():
            if parent in owned or (str(d) in cmd and cmd.startswith('/Applications/Visual Studio Code.app/')):
                owned.add(pid); processes[pid] = (parent, cmd)
    remaining = {pid: current[pid][1] for pid in owned if pid in current}
    report['remainingOwnedProcesses'] = remaining
    report['cleanup'] = 'OWNED_EDITOR_PROCESSES_RETIRED' if not remaining else 'OWNED_EDITOR_PROCESS_REMAINS'
    if remaining: report['cleanupHold'] = str(d)

def validate_wire(rows, server_digest, root_uri):
    errors = []
    if any('transcript' in row for row in rows): errors.append('transcript-truncated')
    if not any(row.get('direction') == 'provenance' and row.get('message', {}).get('serverExecutableSha256') == server_digest for row in rows): errors.append('server-digest-not-witnessed')
    def select(direction, method):
        return [(i, row.get('message', {})) for i, row in enumerate(rows) if row.get('direction') == direction and isinstance(row.get('message'), dict) and row['message'].get('method') == method]
    init, initialized, shutdown, exit_rows = [select('client-to-server', method) for method in ['initialize', 'initialized', 'shutdown', 'exit']]
    if any(len(found) != 1 for found in [init, initialized, shutdown, exit_rows]): errors.append('lifecycle-methods-missing-or-duplicate')
    else:
        i, initialize = init[0]; j, _ = initialized[0]; k, stop = shutdown[0]; z, _ = exit_rows[0]
        responses = [(n, row.get('message', {})) for n, row in enumerate(rows) if row.get('direction') == 'server-to-client' and isinstance(row.get('message'), dict) and 'method' not in row['message']]
        init_responses = [(n,m) for n,m in responses if m.get('id') == initialize.get('id')]
        stop_responses = [(n,m) for n,m in responses if m.get('id') == stop.get('id')]
        if len(init_responses) != 1 or len(stop_responses) != 1: errors.append('lifecycle-response-missing-or-duplicate')
        else:
            ir, reply = init_responses[0]; sr, stopped = stop_responses[0]
            if not i < ir < j < k < sr < z: errors.append('lifecycle-order-invalid')
            caps = reply.get('result', {}).get('capabilities', {})
            if caps.get('positionEncoding') not in ['utf-8', 'utf-16', 'utf-32']: errors.append('position-encoding-missing')
            if 'error' in stopped or stopped.get('result', 'MISSING') is not None: errors.append('shutdown-response-invalid')
        params = initialize.get('params', {})
        roots = [item.get('uri') for item in params.get('workspaceFolders', []) or []]
        if params.get('rootUri') != root_uri and root_uri not in roots: errors.append('workspace-root-not-witnessed')
    if not any(row.get('direction') == 'lifecycle' and row.get('message', {}).get('serverExit') == 0 and row.get('message', {}).get('groupRetired') is True for row in rows): errors.append('server-retirement-not-witnessed')
    return {'valid': not errors, 'errors': errors}

def validate_semantic(rows, observation, uri, disk_unchanged):
    errors = []
    client_rows = [row['message'] for row in rows if row.get('direction') == 'client-to-server' and isinstance(row.get('message'), dict)]
    opened = [m for m in client_rows if m.get('method') == 'textDocument/didOpen' and m.get('params', {}).get('textDocument', {}).get('uri') == uri]
    changed = [m for m in client_rows if m.get('method') == 'textDocument/didChange' and m.get('params', {}).get('textDocument', {}).get('uri') == uri]
    overlay = 'package p\n/*😀*/ var x int\nvar y = x\n'
    if not opened: errors.append('didOpen-not-witnessed')
    if not any(any(change.get('text') == overlay and 'range' not in change for change in m['params'].get('contentChanges', [])) for m in changed): errors.append('full-unsaved-didChange-not-witnessed')
    if any(m.get('method') == 'textDocument/didSave' for m in client_rows): errors.append('unexpected-save')
    requests = [m for m in client_rows if m.get('method') == 'textDocument/definition' and m.get('params', {}).get('textDocument', {}).get('uri') == uri]
    if len(requests) != 1 or requests[0]['params'].get('position') != {'line': 2, 'character': 8}: errors.append('definition-query-not-witnessed')
    semantic = observation.get('semantic', {})
    if not semantic.get('bufferModified'): errors.append('buffer-not-unsaved')
    if semantic.get('error'): errors.append('client-definition-error')
    encoding = observation.get('initializeResult', {}).get('capabilities', {}).get('positionEncoding')
    expected = {'utf-8': 13, 'utf-16': 11, 'utf-32': 10}.get(encoding)
    result = semantic.get('definition')
    locations = result if isinstance(result, list) else [result] if isinstance(result, dict) else []
    target = {'line': 1, 'character': expected}
    if expected is None or not any(location.get('uri') == uri and location.get('range', {}).get('start') == target for location in locations): errors.append('definition-target-encoding-mismatch')
    if len(requests) == 1:
        witnessed = [row['message'].get('result') for row in rows if row.get('direction') == 'server-to-client' and isinstance(row.get('message'), dict) and row['message'].get('id') == requests[0].get('id') and 'method' not in row['message']]
        if witnessed != [result]: errors.append('client-result-not-wire-bound')
    indexed = [(i, row['message']) for i, row in enumerate(rows) if row.get('direction') == 'client-to-server' and isinstance(row.get('message'), dict)]
    open_rows = [(i, m) for i, m in indexed if m.get('method') == 'textDocument/didOpen' and m.get('params', {}).get('textDocument', {}).get('uri') == uri]
    change_rows = [(i, m) for i, m in indexed if m.get('method') == 'textDocument/didChange' and m.get('params', {}).get('textDocument', {}).get('uri') == uri]
    query_rows = [(i, m) for i, m in indexed if m.get('method') == 'textDocument/definition' and m.get('params', {}).get('textDocument', {}).get('uri') == uri]
    fixture_changes = [(i, m) for i, m in change_rows if any(c.get('text') == overlay and 'range' not in c for c in m['params'].get('contentChanges', []))]
    if len(open_rows) != 1 or len(fixture_changes) != 1 or len(query_rows) != 1:
        errors.append('snapshot-events-missing-or-duplicate')
    else:
        oi, opened_doc = open_rows[0]; ci, changed_doc = fixture_changes[0]; qi, query = query_rows[0]
        replies = [(i, row['message']) for i, row in enumerate(rows) if row.get('direction') == 'server-to-client' and isinstance(row.get('message'), dict) and row['message'].get('id') == query.get('id') and 'method' not in row['message']]
        if len(replies) != 1 or not oi < ci < qi < replies[0][0]:
            errors.append('semantic-event-order-invalid')
        else:
            ri = replies[0][0]
            if any(qi < i < ri for i, _ in change_rows): errors.append('document-changed-before-definition-result')
        opened_version = opened_doc['params']['textDocument'].get('version')
        changed_version = changed_doc['params']['textDocument'].get('version')
        if type(opened_version) is not int or type(changed_version) is not int or changed_version <= opened_version:
            errors.append('document-version-not-increasing')
        if semantic.get('version') != changed_version: errors.append('client-request-snapshot-version-mismatch')
        before_query = [(i, m) for i, m in change_rows if i < qi]
        if not before_query or before_query[-1][0] != ci: errors.append('definition-not-current-fixture-snapshot')
    if not disk_unchanged: errors.append('disk-changed')
    return {'valid': not errors, 'errors': errors, 'negotiatedEncoding': encoding, 'expectedTargetStart': target, 'definition': result, 'diskUnchanged': disk_unchanged}

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--client', choices=['neovim', 'vscode'])
    p.add_argument('--client-bin')
    p.add_argument('--client-module', help='Pinned vscode-languageclient package directory; no downloads')
    p.add_argument('--server', help='Operator-selected executable; never auto-installed')
    p.add_argument('--server-arg', action='append', default=[])
    p.add_argument('--server-source-commit', help='Caller source claim, NOT_PROVEN by this harness')
    p.add_argument('--report')
    p.add_argument('--timeout', type=int, default=20)
    p.add_argument('--self-check', action='store_true')
    p.add_argument('--semantic-development', action='store_true', help='One unsaved Unicode definition observation; not qualification')
    a = p.parse_args()
    if a.self_check:
        assert (ROOT / 'tools/lsp-editors/neovim.lua').is_file()
        assert (ROOT / 'tools/lsp-editors/extension.js').is_file()
        subprocess.run([sys.executable, str(ROOT / 'tools/lsp-editors/check-proxy.py')], check=True)
        subprocess.run([sys.executable, str(ROOT / 'tools/lsp-editors/check-harness.py')], check=True)
        subprocess.run([sys.executable, str(ROOT / 'tools/lsp-editors/check-semantic.py')], check=True)
        print('harness assets present; actual editor qualification NOT_RUN')
        return 0
    if not a.client or not a.server or not a.report or not 1 <= a.timeout <= 120:
        p.error('--client, --server, --report and timeout 1..120 required')
    report_path = Path(a.report).resolve()
    server = Path(a.server).resolve(strict=True)
    client = Path(a.client_bin or shutil.which('nvim' if a.client == 'neovim' else 'code') or '').resolve()
    if not client.is_file() or not os.access(client, os.X_OK) or not os.access(server, os.X_OK):
        p.error('client and server must be executable files')
    report = {'profile': 'corvint-editor-development-probe/0', 'qualification': 'UNQUALIFIED',
              'client': a.client, 'clientExecutableSha256': digest(client),
              'harnessCommit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
              'harnessSha256': digest(__file__),
              'harnessAssetsSha256': {name: digest(ROOT / 'tools/lsp-editors' / name) for name in ['wire-proxy.py', 'neovim.lua', 'extension.js']},
              'harnessWorktreeDirty': bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True)),
              'serverSourceClaim': {'commit': a.server_source_commit, 'binding': 'NOT_PROVEN'},
              'serverExecutableSha256': digest(server), 'serverArgv': [str(server)] + a.server_arg,
              'cases': {k: 'NOT_RUN' for k in ['unicode', 'rapid-edit', 'multi-root', 'stale-response',
                         'server-crash', 'interruption-descendants', 'semantic-outcome']}}
    version = subprocess.run([str(client), '--version'], capture_output=True, text=True, timeout=15)
    report['clientVersionProbe'] = {'exitCode': version.returncode, 'stdout': version.stdout[:8192], 'stderr': version.stderr[:8192]}
    if a.client == 'vscode':
        app = next((parent for parent in client.parents if parent.name.endswith('.app')), None)
        if app is not None:
            binary = app / 'Contents/MacOS/Code'
            report['editorApplicationExecutableSha256'] = digest(binary)
    d = Path(tempfile.mkdtemp(prefix='corvint-editor-')).resolve()
    result, wire = d / 'client-result.json', d / 'wire.jsonl'
    proc = None
    report['temporaryDirectory'] = str(d)
    def interrupted(signum, frame):
        raise HarnessInterrupted('harness interrupted by signal ' + str(signum))
    prior = {sig: signal.signal(sig, interrupted) for sig in (signal.SIGINT, signal.SIGTERM)}
    try:
        workspace = d / 'workspace'; workspace.mkdir()
        document = workspace / 'main.go'
        disk_text = 'package p\nvar disk int\n' if a.semantic_development else 'package main\nfunc main() {}\n'
        document.write_text(disk_text)
        if a.semantic_development: (workspace / 'go.mod').write_text('module example.com/corvint/editorprobe\n\ngo 1.27.1\n')
        result = d / 'client-result.json'
        env = os.environ.copy()
        wire = d / 'wire.jsonl'
        resolved_args = [str(workspace) if value == '{workspace}' else value for value in a.server_arg]
        report['serverArgvTemplate'] = report['serverArgv']
        report['serverArgv'] = [str(server)] + resolved_args
        server_argv = [sys.executable, str(ROOT / 'tools/lsp-editors/wire-proxy.py'), str(wire), str(server)] + resolved_args
        env.update(CORVINT_EDITOR_RESULT=str(result), CORVINT_EDITOR_ROOT=str(workspace),
                   CORVINT_EDITOR_DOCUMENT=str(document), CORVINT_EDITOR_TIMEOUT_MS=str(a.timeout * 1000),
                   CORVINT_EDITOR_SERVER_ARGV=json.dumps(server_argv), CORVINT_EDITOR_SEMANTIC='1' if a.semantic_development else '0')
        for key in ['XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME']:
            env[key] = str(d / key)
        if a.client == 'neovim':
            argv = [str(client), '--headless', '-u', 'NONE', '-i', 'NONE', '-n',
                    '-c', 'lua dofile(' + json.dumps(str(ROOT / 'tools/lsp-editors/neovim.lua')) + ')']
        else:
            if not a.client_module:
                p.error('--client-module required for vscode; no implicit npm install')
            module = Path(a.client_module).resolve(strict=True)
            pkg = json.loads((module / 'package.json').read_text())
            if pkg['name'] != 'vscode-languageclient' or pkg['version'] != '10.1.2':
                p.error('vscode-languageclient exact 10.1.2 required')
            report['languageClientVersion'] = pkg['version']
            env['CORVINT_EDITOR_CLIENT_MODULE'] = str(module)
            extension = d / 'extension'; extension.mkdir()
            shutil.copy(ROOT / 'tools/lsp-editors/extension.js', extension)
            (extension / 'package.json').write_text(json.dumps({'name':'corvint-qualification',
                'version':'0.0.0','publisher':'corvint-local','engines':{'vscode':'^1.91.0'},
                'main':'./extension.js','activationEvents':['*']}))
            userdata = d / 'userdata'; (userdata / 'User').mkdir(parents=True)
            (userdata / 'User/settings.json').write_text(json.dumps({'security.workspace.trust.enabled':False,
                'update.mode':'none','extensions.autoUpdate':False,'telemetry.telemetryLevel':'off'}))
            argv = [str(client), '--new-window', '--wait', '--disable-extensions',
                    '--user-data-dir', str(userdata), '--extensions-dir', str(d / 'extensions'),
                    '--extensionDevelopmentPath=' + str(extension), str(workspace)]
        report['clientCommand'] = argv
        proc = subprocess.Popen(argv, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        out, err = proc.communicate(timeout=a.timeout + 10)
        report.update(exitCode=proc.returncode, stdout=out.decode(errors='replace')[-8192:],
                      stderr=err.decode(errors='replace')[-8192:])
    except (Exception, KeyboardInterrupt) as exc:
        report['failure'] = {'type': type(exc).__name__, 'message': str(exc)}
    finally:
        # A second interruption must not bypass bounded retirement and report preservation.
        for sig in prior: signal.signal(sig, signal.SIG_IGN)
        try:
            cleanup_owned(proc, d, report)
        except Exception as exc:
            report.update(cleanup='UNKNOWN', cleanupError=str(exc), cleanupHold=str(d))
            if proc is not None and proc.poll() is None:
                try: os.killpg(proc.pid, signal.SIGKILL); proc.wait(timeout=3)
                except (ProcessLookupError, subprocess.TimeoutExpired): pass
        for key, file in [('observation', result), ('wireTranscript', wire)]:
            try:
                if key == 'observation': report[key] = json.loads(file.read_text())
                else: report[key] = [json.loads(row) for row in file.read_text().splitlines()]
            except (OSError, ValueError) as exc:
                report[key] = {} if key == 'observation' else []
                report.setdefault('evidenceErrors', []).append(key + ': ' + str(exc))
        if not isinstance(report['observation'], dict):
            report.setdefault('evidenceErrors', []).append('observation: expected object')
            report['observation'] = {}
        try: report['serverExecutableSha256AfterRun'] = digest(server)
        except OSError as exc: report.setdefault('evidenceErrors', []).append('server digest: ' + str(exc))
        try: report['wireEvidence'] = validate_wire(report['wireTranscript'], report['serverExecutableSha256'], (d / 'workspace').as_uri())
        except (AttributeError, TypeError, ValueError) as exc: report['wireEvidence'] = {'valid': False, 'errors': ['invalid-wire-structure: ' + str(exc)]}
        report['cases']['initialize-lifecycle-root-encoding'] = report['observation']
        if a.semantic_development:
            try:
                report['semanticDevelopment'] = validate_semantic(report['wireTranscript'], report['observation'], document.as_uri(), document.read_text() == disk_text)
            except (OSError, AttributeError, TypeError, ValueError) as exc:
                report['semanticDevelopment'] = {'valid': False, 'errors': ['invalid-semantic-evidence: ' + str(exc)]}
            report['cases']['unsaved-unicode-definition-development'] = report['semanticDevelopment']
        try: report_path.write_text(json.dumps(report, indent=2) + '\n')
        except OSError as exc:
            report.update(reportWriteError=str(exc), cleanupHold=str(d))
            (d / 'harness-report.json').write_text(json.dumps(report, indent=2) + '\n')
            print('Report preserved at ' + str(d / 'harness-report.json'), file=sys.stderr)
        if report.get('cleanup') == 'OWNED_EDITOR_PROCESSES_RETIRED' and not report.get('reportWriteError'): shutil.rmtree(d)
        for sig, handler in prior.items(): signal.signal(sig, handler)
    return 0 if (not report.get('failure') and not report.get('evidenceErrors') and not report.get('reportWriteError') and
                 report['observation'].get('status') == 'LIFECYCLE_OBSERVED' and report.get('exitCode') == 0 and
                 report['cleanup'] == 'OWNED_EDITOR_PROCESSES_RETIRED' and report['wireEvidence']['valid'] and
                 (not a.semantic_development or report['semanticDevelopment']['valid']) and
                 report.get('serverExecutableSha256AfterRun') == report['serverExecutableSha256']) else 1

if __name__ == '__main__':
    sys.exit(main())
