"""Maintainer qualification evidence; this is not execution attestation."""
import hashlib
import json
import math
import os
from pathlib import Path
import subprocess
import tempfile

PROFILE = 'opencode-native-integration/1'
FOCUSED_TESTS = {'TestHostAdapterJavaScriptHosts', 'TestHostAdapterJavaScriptHarnessInterruption'}
CASES = {
    'install': ['native-discovery', 'native-host-image', 'direct-install', 'uninstall'],
    'snapshot': ['snapshot', 'oversized-exclusion'],
    'context': ['awaited-current-prompt', 'native-query', 'bounded-prompt', 'governance'],
    'expansion': ['native-exact-expansion'],
    'observations': ['native-edit', 'verification-observation', 'explicit-outcome', 'supplied-evidence'],
    'frontier': ['advisory-completion', 'bounded-stops'],
    'degradation': [],
    'privacy': ['environment-filter'],
    'normalization': [],
    'recursion': ['bounded-stops'],
    'compaction': ['native-compaction', 'dirty-path-recovery'],
    'latency': ['timed-delivery', 'latency'],
    'recall': ['critical-recall-and-bytes'],
    'cleanup': ['interruption-cleanup', 'native-exit'],
}


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def digest(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as source:
        for block in iter(lambda: source.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def node_architecture(machine=None):
    import platform
    machine = (machine or platform.machine()).lower()
    aliases = {'x86_64': 'x64', 'amd64': 'x64', 'aarch64': 'arm64', 'arm64': 'arm64'}
    require(machine in aliases, 'unsupported qualification architecture: ' + machine)
    return aliases[machine]


def identities(source, host, corvint):
    def git(*args):
        return subprocess.check_output(['git', *args], cwd=source, text=True).strip()
    require(not git('status', '--porcelain', '--untracked-files=all'), 'qualification requires a clean committed checkout')
    package = source / 'integrations/opencode'
    files = sorted(p for p in package.rglob('*') if p.is_file())
    inputs = git('ls-files', 'integrations', 'conformance/harness-event-v0',
                 'cmd/corvint/host_adapter_javascript_test.go', 'script/qualify-opencode.py',
                 'script/qualify-opencode-native.py', 'script/opencode_qualification.py').splitlines()
    manifest = json.loads((package / 'package.json').read_text())
    import platform
    return {'sourceCommit': git('rev-parse', 'HEAD'), 'hostSHA256': digest(host),
            'corvintSHA256': digest(corvint),
            'sourceFiles': {str(p.relative_to(source)): digest(p) for p in files},
            'inputs': {p: digest(source / p) for p in inputs},
            'tuple': {'hostVersion': '2.0.18', 'adapterVersion': manifest['version'],
                      'os': platform.system().lower(), 'architecture': node_architecture()}}


def write_record(path, record):
    path = Path(path)
    payload = (json.dumps(record, indent=2) + '\n').encode()
    require(len(payload) <= 131072, 'qualification record exceeds consumer bound')
    fd, temporary = tempfile.mkstemp(prefix='.opencode-qualification-', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(payload)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def begin_record(path, backup):
    path, backup = Path(path), Path(backup)
    require(not path.is_symlink(), 'qualification record must not be a symlink')
    if path.exists():
        require(path.is_file() and path.stat().st_size <= 131072, 'invalid previous qualification record')
        backup.write_bytes(path.read_bytes())
    write_record(path, {'profile': PROFILE, 'result': 'INCOMPLETE', 'reason': 'qualification-in-progress'})


def build_record(native, focused, before, after):
    require(before == after, 'qualification inputs changed during the gates')
    require(native.get('profile') == PROFILE and native.get('result') == 'PASS', 'native campaign did not pass')
    require(native.get('executionAuthority') == 'NONE' and native.get('frontier') == 'UNAVAILABLE'
            and native.get('legacyReceiptSupport') == 'FALLBACK', 'authority boundary mismatch')
    for key in ('sourceCommit', 'sourceFiles', 'hostSHA256', 'corvintSHA256'):
        require(native.get(key) == before[key], 'native identity mismatch: ' + key)
    require(all(native.get(k) == v for k, v in before['tuple'].items()), 'native tuple mismatch')
    require(native.get('harnessSHA256') == before['inputs']['script/qualify-opencode-native.py'], 'native collector changed')
    checks = native.get('checks', {})
    needed = {name for names in CASES.values() for name in names} | {'frozen-identities'}
    require(all(checks.get(name) is True for name in needed), 'missing or failed native check')
    require(isinstance(focused, dict) and focused.get('exitCode') == 0, 'focused suite did not pass')
    events = [json.loads(line) for line in focused.get('stdout', '').splitlines() if line]
    require(events and not any(e.get('Action') in ('fail', 'skip') for e in events), 'focused suite missing, failed or skipped')
    for action in ('run', 'pass'):
        observed = {e.get('Test') for e in events if e.get('Action') == action
                    and e.get('Package') == 'github.com/Beamfall/corvint/cmd/corvint'}
        require(FOCUSED_TESTS <= observed, 'focused suite missing required test: ' + action)
    metrics = native['metrics']
    for key, limit in [('query', 500), ('lifecycle', 250)]:
        values = metrics[key]
        require(len(values) >= 20 and all(type(x) in (float, int) and math.isfinite(x) and x >= 0 for x in values), 'invalid latency samples')
        p95 = sorted(values)[math.ceil(len(values) * .95) - 1]
        require(p95 == metrics[key + 'P95Ms'] and p95 <= limit, 'latency gate failed')
    recall = metrics['criticalRecall']
    require(recall['injected'] == recall['manual'] == recall['total'] > 0
            and 0 < metrics['injectedBytes'] < metrics['manualBytes'], 'recall/bytes gate failed')
    cleanup = native['cleanupEvidence']
    require(cleanup['exit'] == 143 and cleanup['observedDescendants'] and cleanup['survivors'] == [], 'cleanup evidence incomplete')
    result = dict(native)
    result['conformance'] = {name: 'PASS' for name in CASES}
    result['conformanceEvidence'] = {name: {'nativeChecks': checks, 'focusedTests': sorted(FOCUSED_TESTS)} for name, checks in CASES.items()}
    result['qualificationInputs'] = before['inputs']
    result['focusedSuite'] = {'exitCode': 0, 'tests': sorted(FOCUSED_TESTS),
                              'stdoutSHA256': hashlib.sha256(focused['stdout'].encode()).hexdigest(),
                              'stderrSHA256': hashlib.sha256(focused.get('stderr', '').encode()).hexdigest()}
    return result
