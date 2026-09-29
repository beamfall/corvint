#!/usr/bin/env python3
"""Experimental FPK-V0-020..026 public-CLI pilot; never promotes qualification.

Creates a disposable synthetic fixture and two isolated handoff arms. Does not
launch agents. --evaluate independently tests one repaired arm in another clone.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time


TASK = 'Finish NormalizeRetries: negative inputs become 0, 0 through 5 stay unchanged, and inputs above 5 become 5. Add boundary tests.'
OBLIGATIONS = ['RETRY-001: implement and verify every boundary before calling this task complete']
UNKNOWN = 'The earlier test only covered negative input. Upper-bound behavior and complete boundary coverage remain unverified.'
FILES = {
    '.gitignore': '.corvint/\n',
    'AGENTS.md': '# Fixture instructions\n\nFollow docs/specs/retry.md. Run the focused Go tests before reporting completion.\n',
    'go.mod': 'module example.test/recovery\n\ngo 1.27.0\n',
    'docs/specs/retry.md': '# Retry normalization\n\n## Requirements\n\n- RETRY-001: Negative inputs become zero; zero through five stay unchanged; inputs above five become five.\n',
    'retry/retry.go': 'package retry\n\nfunc NormalizeRetries(n int) int {\n\tif n < 0 { return 0 }\n\tif n > 6 { return 6 }\n\treturn n\n}\n',
    'retry/retry_test.go': 'package retry\n\nimport "testing"\n\nfunc TestNegative(t *testing.T) {\n\tif NormalizeRetries(-1) != 0 { t.Fatal("negative retry count") }\n}\n',
}
PATHS = ['AGENTS.md', 'docs/specs/retry.md', 'retry/retry.go', 'retry/retry_test.go']
# Evaluator-owned outcomes, frozen before dispatch and independent of agent tests.
OUTCOMES = [(-1, 0), (0, 0), (1, 1), (4, 4), (5, 5), (6, 5), (7, 5), (100, 5)]


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def save(path, value):
    path.write_bytes(canonical(value))


def interrupted(signum, _frame):
    raise SystemExit(128 + signum)


class Runner:
    def __init__(self, output):
        self.output = output
        self.records = []

    def run(self, argv, cwd, expected=0):
        start = time.monotonic()
        env = dict(os.environ, GOTOOLCHAIN='local', GOCACHE='/tmp/corvint-go-build-cache',
                   GOWORK='off', GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
                   GIT_AUTHOR_DATE='2026-09-29T00:00:00Z', GIT_COMMITTER_DATE='2026-09-29T00:00:00Z')
        # Synthetic repositories must not inherit a caller's Git repository override.
        for key in list(env):
            if key.startswith('GIT_') and key not in {
                'GIT_CONFIG_NOSYSTEM', 'GIT_CONFIG_GLOBAL', 'GIT_AUTHOR_DATE', 'GIT_COMMITTER_DATE'
            }:
                del env[key]
        process = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, start_new_session=True)
        failure = None
        try:
            stdout, stderr = process.communicate(timeout=90)
        except BaseException as error:
            failure = error
        finally:
            # Also retire descendants when the group leader has already exited.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            stdout, stderr = process.communicate()
        index = len(self.records)
        stem = f'command-{index:03}'
        (self.output / (stem + '.stdout')).write_bytes(stdout)
        (self.output / (stem + '.stderr')).write_bytes(stderr)
        self.records.append({'argv': argv, 'cwd': str(cwd), 'exit': process.returncode,
                             'elapsedSeconds': time.monotonic() - start,
                             'stdoutSha256': digest(stdout), 'stderrSha256': digest(stderr), 'artifact': stem,
                             'interruption': type(failure).__name__ if failure else None})
        save(self.output / 'commands.json', self.records)
        if failure is not None:
            raise failure
        if process.returncode != expected:
            raise RuntimeError(f'{argv[0]} exit {process.returncode}; inspect {stem}.stderr')
        return stdout

    def git(self, root, *args):
        return self.run(['git', *args], root).decode().strip()


def repository_files(root):
    return {str(p.relative_to(root)): digest(p.read_bytes()) for p in sorted(root.rglob('*'))
            if p.is_file() and '.git' not in p.relative_to(root).parts}


def clone(runner, source, destination):
    runner.run(['git', 'clone', '--no-hardlinks', '--quiet', str(source), str(destination)], source.parent)


def evaluate(runner, source, output):
    # Include an agent's uncommitted repair, but never write evaluator tests into its arm.
    repo = output / 'evaluation'
    clone(runner, source, repo)
    for name in ['retry/retry.go', 'retry/retry_test.go']:
        shutil.copyfile(source / name, repo / name)
    cases = ','.join('{%d,%d}' % pair for pair in OUTCOMES)
    test = 'package retry\nimport "testing"\nfunc TestIndependentRecoveryOutcome(t *testing.T) {\n'
    test += 'for _, c := range [][2]int{' + cases + '} { if got := NormalizeRetries(c[0]); got != c[1] { t.Errorf("input %d: got %d want %d", c[0], got, c[1]) } }\n}\n'
    (repo / 'retry/recovery_outcome_test.go').write_text(test)
    runner.run(['go', 'test', '-count=1', '-timeout', '60s', './retry'], repo)
    return {'behavior': 'PASS', 'cases': OUTCOMES, 'sourceFiles': repository_files(source),
            'obligationAndHistoricalAttribution': 'REQUIRES_INDEPENDENT_REVIEW'}


def prepare(runner, binary, output):
    root = output / 'builder'
    root.mkdir()
    runner.git(root, 'init', '--quiet', '--object-format=sha1')
    runner.git(root, 'config', 'user.email', 'fixture@example.test')
    runner.git(root, 'config', 'user.name', 'Recovery fixture')
    for name, body in FILES.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body)
    runner.git(root, 'add', '.')
    runner.git(root, 'commit', '--quiet', '-m', 'Interrupted builder fixture')
    revision = runner.git(root, 'rev-parse', 'HEAD')
    tree = runner.git(root, 'rev-parse', 'HEAD^{tree}')
    runner.run(['go', 'test', '-count=1', '-timeout', '60s', './retry'], root)
    handles = [{'path': path, 'blob_hash': runner.git(root, 'rev-parse', f'HEAD:{path}')} for path in PATHS]
    document = {
        'version': 'corvint-checkpoint/0', 'task': TASK, 'obligations': OBLIGATIONS,
        'repository': {'object_format': 'sha1', 'base_commit': revision, 'base_tree': tree,
                       'dirty_paths_sha256': digest(canonical([]))},
        'handles': handles, 'critical': handles, 'unknowns': UNKNOWN,
        'failed_approaches': 'No broader verification was performed.',
        'verification': [{'command': 'go test -count=1 -timeout 60s ./retry', 'observed_status': 'passed',
                          'provenance': 'builder fixture at ' + revision + '; historical observation, not current proof'}],
        'provenance': {},
    }
    checkpoint = output / 'checkpoint.json'
    save(checkpoint, document)
    notes = '# Structured builder notes\n\n' + TASK + '\n\n'
    notes += 'Builder commit: ' + revision + '\nBuilder tree: ' + tree + '\n\n'
    notes += 'Obligations: ' + '\n'.join(OBLIGATIONS) + '\n\nUnknowns: ' + UNKNOWN + '\n'
    notes += 'Failed approaches: ' + document['failed_approaches'] + '\n\nEvidence handles:\n'
    notes += ''.join('- ' + h['path'] + ' @ ' + h['blob_hash'] + ' (critical)\n' for h in handles)
    notes += '\nHistorical verification:\n' + json.dumps(document['verification'], indent=2) + '\n'
    (output / 'notes.md').write_text(notes)
    for arm in ['checkpoint-arm', 'notes-arm']:
        clone(runner, root, output / arm)
        if runner.git(output / arm, 'rev-parse', 'HEAD') != revision:
            raise RuntimeError('arm revision mismatch')
    cases = []
    previous = None
    for case in ['unchanged', 'repeat', 'stale', 'deleted']:
        if case == 'stale':
            path = root / 'retry/retry.go'
            path.write_text(path.read_text() + '\n// Another contributor touched this implementation.\n')
            runner.git(root, 'add', '.')
            runner.git(root, 'commit', '--quiet', '-m', 'Independent committed drift')
        if case == 'deleted':
            runner.git(root, 'rm', '--quiet', 'retry/retry_test.go')
            runner.git(root, 'commit', '--quiet', '-m', 'Evidence removed after checkpoint')
        before = repository_files(root)
        status = runner.git(root, 'status', '--porcelain')
        raw = runner.run([str(binary), '--root', str(root), 'prove', '--checkpoint', str(checkpoint)], root)
        receipt = json.loads(raw)
        (output / (case + '.json')).write_bytes(raw)
        expected = {p: 'unchanged' for p in PATHS}
        if case in ['stale', 'deleted']:
            expected['retry/retry.go'] = 'blob-changed'
        if case == 'deleted':
            expected['retry/retry_test.go'] = 'path-deleted'
        got = {h['path']: h['verdict'] for h in receipt['handles']}
        if got != expected or len(receipt['handles']) != len(PATHS):
            raise RuntimeError(f'{case}: unexpected handle verdicts')
        for handle in receipt['handles']:
            flags = {k: v for k, v in handle.items() if k not in ['path', 'verdict', 'rows']}
            want = {'commit_moved': True, 'tree_moved': True} if case in ['stale', 'deleted'] else {}
            if flags != want:
                raise RuntimeError(f'{case}: unexpected drift flags')
            if handle['verdict'] != 'path-deleted' and not handle.get('rows'):
                raise RuntimeError(f'{case}: missing rehydrated rows')
        missing = receipt['critical_missing']
        if case == 'deleted':
            if missing != [{'path': 'retry/retry_test.go', 'reason': 'path-deleted',
                            'recovery': 'Re-run query or impact at the current revision for this path and update the caller checkpoint.'}]:
                raise RuntimeError('deleted: wrong critical missing/recovery result')
        elif missing:
            raise RuntimeError(f'{case}: unexpected critical missing rows')
        for key in ['task', 'obligations', 'unknowns', 'failed_approaches', 'verification']:
            if receipt[key] != document[key]:
                raise RuntimeError(f'{case}: lost {key}')
        if receipt['obligations_authority'] != 'caller-reported-unverified':
            raise RuntimeError('obligations gained authority')
        if before != repository_files(root) or status != runner.git(root, 'status', '--porcelain'):
            raise RuntimeError('checkpoint read changed repository files')
        if case == 'repeat' and raw != previous:
            raise RuntimeError('unchanged fresh processes disagree')
        if case == 'unchanged':
            previous = raw
        cases.append({'case': case, 'status': 'PASS', 'receiptSha256': digest(raw), 'expectedVerdicts': expected})
    return {'profile': 'checkpoint-handoff-pilot/0', 'experimental': True, 'promotion': 'NOT_QUALIFIED',
            'binarySha256': digest(binary.read_bytes()), 'builderCommit': revision, 'builderTree': tree,
            'checkpointSha256': digest(checkpoint.read_bytes()), 'cases': cases,
            'freshAgentOutcome': 'NOT_OBSERVED', 'actualProcessInterruption': 'NOT_OBSERVED',
            'billedTokens': 'NOT_OBSERVED', 'cacheUsage': 'NOT_OBSERVED', 'cost': 'NOT_OBSERVED'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--corvint', type=Path)
    parser.add_argument('--output', type=Path, help='New directory; existing paths are refused. Default: a new temporary directory.')
    parser.add_argument('--evaluate', type=Path, help='Independently test a repaired arm; does not launch agents.')
    args = parser.parse_args()
    if bool(args.corvint) == bool(args.evaluate):
        parser.error('choose exactly one of --corvint or --evaluate')
    output = args.output.resolve() if args.output else Path(tempfile.mkdtemp(prefix='corvint-handoff-')) / 'run'
    output.mkdir(mode=0o700, parents=False, exist_ok=False)
    signal.signal(signal.SIGINT, interrupted)
    signal.signal(signal.SIGTERM, interrupted)
    runner = Runner(output)
    try:
        result = evaluate(runner, args.evaluate.resolve(), output) if args.evaluate else prepare(runner, args.corvint.resolve(), output)
        save(output / 'result.json', result)
        print(json.dumps({'output': str(output), 'result': result}))
    except BaseException as error:
        save(output / 'failure.json', {'type': type(error).__name__, 'error': str(error), 'qualification': 'FAILED'})
        raise


if __name__ == '__main__':
    main()
