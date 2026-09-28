#!/usr/bin/env python3
"""Run both OpenCode qualification gates and publish the exact status record."""
import argparse
import atexit
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True

from opencode_qualification import begin_record, build_record, identities, require, write_record


class CommandRunner:
    def __init__(self):
        self.child = None
        atexit.register(self.close)
        for sig in (signal.SIGINT, signal.SIGTERM):
            signal.signal(sig, self.interrupted)

    def interrupted(self, sig, _frame):
        self.close()
        raise SystemExit(128 + sig)

    def close(self):
        child, self.child = self.child, None
        if child is None:
            return
        rows = subprocess.check_output(['ps', '-axo', 'pid=,ppid='], text=True).splitlines()
        parents = {int(p): int(parent) for p, parent in (row.split() for row in rows)}
        owned, pending = [], [child.pid]
        while pending:
            pending = [p for p, parent in parents.items() if parent in pending]
            owned.extend(pending)
        if child.poll() is None:
            os.killpg(child.pid, signal.SIGTERM)
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                pass
        for pid in reversed(owned):
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        try:
            os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        except PermissionError:
            live = subprocess.check_output(['ps', '-axo', 'pgid=,stat='], text=True).splitlines()
            if any(int(pgid) == child.pid and not state.startswith('Z') for pgid, state in (row.split() for row in live)):
                raise
        child.wait(timeout=10)

    def run(self, argv, directory, environment, output, timeout=1800):
        with output.with_suffix('.stdout').open('w') as out, output.with_suffix('.stderr').open('w') as err:
            self.child = subprocess.Popen(argv, cwd=directory, env=environment, stdout=out, stderr=err, start_new_session=True)
            try:
                code = self.child.wait(timeout=timeout)
            finally:
                self.close()
        streams = []
        for suffix in ('.stdout', '.stderr'):
            path = output.with_suffix(suffix)
            require(path.stat().st_size <= 4 * 1024 * 1024, 'gate output exceeds bound')
            streams.append(path.read_text())
        return {'exitCode': code, 'stdout': streams[0], 'stderr': streams[1], 'argv': argv}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host', required=True, help='actual OpenCode 2.0.18 executable, not a shell launcher')
    parser.add_argument('--corvint', required=True)
    parser.add_argument('--output', required=True, help='retained evidence directory outside this checkout')
    args = parser.parse_args()
    source = Path(__file__).resolve().parents[1]
    host, corvint, output = (Path(p).resolve() for p in (args.host, args.corvint, args.output))
    require(output != source and source not in output.parents, 'evidence output must be outside the checkout')
    output.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix='qualification-', dir=output))
    record = source / 'integrations/opencode-qualification.json'
    begin_record(record, run / 'previous-record.json')
    runner = CommandRunner()
    try:
        require(sys.platform in ('darwin', 'linux'), 'unsupported process cleanup platform')
        with host.open('rb') as f:
            require(f.read(2) != b'#!', '--host must name the actual executable, not its shell launcher')
        before = identities(source, host, corvint)
        env = {k: v for k, v in os.environ.items() if not k.startswith('CORVINT_TEST_') and k not in ('NODE_OPTIONS', 'NODE_PATH')}
        env.update(GOTOOLCHAIN='local', GOWORK='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off')
        version = runner.run(['go', 'env', 'GOVERSION'], source, env, run / 'go-version')
        require(version['exitCode'] == 0 and version['stdout'].strip() == 'go1.27.1', 'Go 1.27.1 is required')
        focused = runner.run(['go', 'test', '-json', '-count=1', '-timeout', '30m', './cmd/corvint', '-run', '^TestHostAdapterJavaScript(Hosts|HarnessInterruption)$'], source, env, run / 'focused')
        require(focused['exitCode'] == 0, 'focused host suite failed; inspect ' + str(run))
        native = runner.run([sys.executable, str(source / 'script/qualify-opencode-native.py'), '--host', str(host), '--corvint', str(corvint), '--output', str(run / 'native')], source, env, run / 'native-command', timeout=600)
        require(native['exitCode'] == 0, 'native campaign failed; inspect ' + str(run))
        native_report = json.loads((run / 'native/report.json').read_text())
        result = build_record(native_report, focused, before, identities(source, host, corvint))
        result['evidenceDirectory'] = str(run)
        write_record(run / 'report.json', result)
        write_record(output / 'report.json', result)
        write_record(record, result)
        print(json.dumps({'result': 'PASS', 'record': str(record), 'evidence': str(run)}))
    except Exception as error:
        write_record(record, {'profile': 'opencode-native-integration/1', 'result': 'FAIL', 'reason': str(error), 'evidenceDirectory': str(run)})
        raise
    finally:
        runner.close()


if __name__ == '__main__':
    main()
