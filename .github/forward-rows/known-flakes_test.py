# SPDX-License-Identifier: Apache-2.0
"""Run with python3 -B .github/forward-rows/known-flakes_test.py (requires git and jq)."""
import datetime
import importlib.util
import io
import json
import os
import pathlib
import re
import shutil
import subprocess
import tempfile
import textwrap
import unittest
from unittest import mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("known_flakes", HERE / "known-flakes.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
PKG = "example.org/project/pkg"
TODAY = datetime.date(2026, 11, 8)
ROW = f"{PKG}\tTestKnown\tV1-4549\t2026-11-15\n".encode()


def event(action, test=None, package=PKG):
    e = {"Action": action, "Package": package}
    if test is not None:
        e["Test"] = test
    return json.dumps(e).encode() + b"\n"


def failure(test="TestKnown", package=PKG):
    return (event("start", package=package) + event("run", test, package)
            + event("fail", test, package) + event("fail", package=package))


def output(text, test=None, package=PKG):
    e = json.loads(event("output", test, package))
    e["Output"] = text
    return json.dumps(e).encode() + b"\n"


class KnownFlakes(unittest.TestCase):
    def test_CINV08720_unfinished_invocations_and_tests(self):
        allowed = m.exceptions(ROW, TODAY)
        listed = (event("start") + event("run", "TestKnown")
                  + event("fail", "TestKnown"))
        # The timeout shape matches cmd/internal/test2json/testdata/timeout.json:
        # a running test gets diagnostic output but no terminal test event.
        timeout = (event("run", "TestNew")
                   + output("=== RUN   TestNew\n", "TestNew")
                   + output("panic: test timed out after 1s\n", "TestNew")
                   + output("\n", "TestNew")
                   + output(f"FAIL\t{PKG}\t1.111s\n") + output("FAIL\n")
                   + event("fail"))
        for stream in (listed + timeout,
                       listed + event("run", "TestNew") + event("fail"),
                       listed + event("run", "TestNew") + event("pause", "TestNew")
                       + event("fail"),
                       failure() + event("start"),
                       failure() + event("start") + event("run", "TestNew"),
                       failure() + event("start", package=PKG + "other"),
                       listed + event("start") + event("fail"),
                       listed + event("pass", "TestNeverRan") + event("fail"),
                       listed + event("bench", "BenchmarkUnknown") + event("fail")):
            with self.subTest(stream=stream), self.assertRaises(m.Refusal):
                m.authorize(io.BytesIO(stream), {PKG}, allowed, {PKG: 1})

    def test_CINV08720_output_allowlist_refuses_crashes_and_unknown_forms(self):
        allowed = m.exceptions(ROW, TODAY)
        running = event("start") + event("run", "TestKnown")
        ending = event("fail", "TestKnown") + event("fail")
        for diagnostic in ("panic: oops [recovered]\n", "fatal error: unexpected signal\n",
                           "panic: test timed out after 1s\n", "future runtime diagnostic\n"):
            for test in (None, "TestKnown"):
                with self.subTest(diagnostic=diagnostic, test=test), self.assertRaises(m.Refusal):
                    m.authorize(io.BytesIO(running + output(diagnostic, test) + ending),
                                {PKG}, allowed, {PKG: 1})
        for stream in (running + output("--- FAIL: TestOther (0.00s)\n", "TestKnown") + ending,
                       failure() + output("FAIL\n"),
                       running + event("output", "TestKnown") + ending,
                       failure().replace(b'"Action": "fail"',
                                         b'"FutureFatal": true, "Action": "fail"'),
                       running + output("    log\n", "TestKnown").replace(
                           b'"Output":', b'"OutputType": "future-diagnostic", "Output":') + ending):
            with self.subTest(stream=stream), self.assertRaises(m.Refusal):
                m.authorize(io.BytesIO(stream), {PKG}, allowed, {PKG: 1})

    def test_CINV08720_complete_interleaved_and_parallel_lifecycles(self):
        allowed = m.exceptions(ROW, TODAY)
        other = PKG + "other"
        stream = (event("start") + event("start", package=other)
                  + event("run", "TestKnown") + output("=== RUN   TestKnown\n", "TestKnown")
                  + output("=== PAUSE TestKnown\n", "TestKnown") + event("pause", "TestKnown")
                  + event("run", "TestPassing") + event("pass", "TestPassing")
                  + event("run", "TestSkipped") + event("skip", "TestSkipped")
                  + event("cont", "TestKnown") + output("=== CONT  TestKnown\n", "TestKnown")
                  + output("    sample_test.go:12: expected failure\n", "TestKnown")
                  + output("--- FAIL: TestKnown (0.00s)\n", "TestKnown")
                  + event("fail", "TestKnown") + output("FAIL\n")
                  + event("skip", package=other)
                  + output(f"FAIL\t{PKG}\t0.001s\n") + event("fail")
                  + event("start") + event("run", "TestKnown")
                  + event("pass", "TestKnown") + output("PASS\n") + event("pass"))
        m.authorize(io.BytesIO(stream), {PKG}, allowed, {PKG: 2, other: 1})

    def test_CINV08720_listed_and_unlisted(self):
        allowed = m.exceptions(ROW, TODAY)
        m.authorize(io.BytesIO(failure()), {PKG}, allowed, {PKG: 1})
        for stream in (failure("TestNew"), failure("TestKnown/subtest"),
                       failure() + failure("TestNew"), failure(package=PKG + "other")):
            with self.subTest(stream=stream), self.assertRaises(m.Refusal):
                m.authorize(io.BytesIO(stream), {PKG}, allowed, {PKG: 1})

    def test_CINV08720_expected_invocation_counts(self):
        allowed = m.exceptions(ROW, TODAY)
        passing = event("start") + event("pass")
        other = PKG + "other"
        for stream, expected in (
                (failure(), {PKG: 2}),  # second slice emits no JSON at all
                (failure(), {PKG: 1, other: 1}),  # entirely missing package
                (failure() + passing, {PKG: 1}),  # excess invocation
                (failure() + event("start", package=other)
                 + event("skip", package=other), {PKG: 1})):  # unscheduled package
            with self.subTest(stream=stream, expected=expected), self.assertRaises(m.Refusal):
                m.authorize(io.BytesIO(stream), {PKG}, allowed, expected)
        m.authorize(io.BytesIO(failure() + passing), {PKG}, allowed, {PKG: 2})

    def test_CINV08720_expected_manifest_closed_form_and_bounds(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "expected"
            with self.assertRaises(OSError):
                m.expected_invocations(path)
            path.write_text((PKG + "\n") * 2)
            self.assertEqual(m.expected_invocations(path), {PKG: 2})
            for data in (b"", PKG.encode(), b"\n", (PKG + "\r\n").encode(),
                         (PKG + "\n\n").encode(), b"../pkg\n", b"-flag\n",
                         (PKG + " 2\n").encode(), b'{"future-schedule": []}\n',
                         b"example.org/pkg\x00\n", b"\xff\n"):
                path.write_bytes(data)
                with self.subTest(data=data), self.assertRaises(ValueError):
                    m.expected_invocations(path)
            path.write_text((PKG + "\n") * 2)
            with mock.patch.object(m, "MAX_INVOCATIONS", 1), self.assertRaises(m.Refusal):
                m.expected_invocations(path)
            with mock.patch.object(m, "MAX_EXPECTED_BYTES", 10), self.assertRaises(m.Refusal):
                m.expected_invocations(path)

    def test_CINV08720_expiry_boundary_and_malformed(self):
        m.exceptions(ROW, datetime.date(2026, 11, 14))
        for today in (datetime.date(2026, 11, 15), datetime.date(2026, 11, 16)):
            with self.subTest(today=today), self.assertRaisesRegex(m.Refusal, "expired"):
                m.exceptions(ROW, today)
        malformed = [ROW.rstrip(), ROW + ROW, ROW.replace(b"V1-4549", b""),
                     ROW.replace(b"V1-4549", b"closed"), ROW.replace(b"2026-11-15", b"2026-02-30"),
                     ROW.replace(b"2026-11-15", b"tomorrow"), ROW.replace(b"TestKnown", b"Test*"),
                     ROW.replace(b"TestKnown", b"TestKnown\textra"), ROW + b"broken\n", b"\xff\n"]
        for data in malformed:
            with self.subTest(data=data), self.assertRaises(ValueError):
                m.exceptions(data, TODAY)
        with self.assertRaisesRegex(m.Refusal, "byte bound"):
            m.exceptions(b"#" * (m.MAX_LIST + 1), TODAY)
        with self.assertRaisesRegex(m.Refusal, "entry bound"):
            m.exceptions(b"".join(ROW.replace(b"TestKnown", f"Test{i}".encode())
                                   for i in range(m.MAX_ENTRIES + 1)), TODAY)

    def test_CINV08720_missing_malformed_and_capped_evidence(self):
        allowed = m.exceptions(ROW, TODAY)
        for stream in (b"", event("fail"), event("fail", "TestKnown"),
                       failure() + event("fail"),  # second invocation has no named failure
                       event("fail", "TestKnown") + event("start") + event("fail"),
                       event("fail", "TestKnown") + event("pass"),
                       failure() + b"not JSON\n", b"[]\n", b"{}\n", event("output", package=""),
                       event("output").replace(b'"output"', b'NaN'),
                       b'{"Action":"pass","Action":"fail","Package":"x"}\n',
                       event("unknown") + failure()):
            with self.subTest(stream=stream), self.assertRaises(m.Refusal):
                m.authorize(io.BytesIO(stream), {PKG}, allowed, {PKG: 1})
        with mock.patch.object(m, "MAX_STREAM", 10), self.assertRaisesRegex(m.Refusal, "bound"):
            m.authorize(io.BytesIO(failure()), {PKG}, allowed, {PKG: 1})
        with mock.patch.object(m, "MAX_LINE", 10), self.assertRaisesRegex(m.Refusal, "bound"):
            m.authorize(io.BytesIO(failure()), {PKG}, allowed, {PKG: 1})
        m.authorize(io.BytesIO(failure() + failure()), {PKG}, allowed, {PKG: 2})

    def test_CINV08720_failed_package_list(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "packages"
            path.write_text(PKG + "\n")
            self.assertEqual(m.failed_packages(path), {PKG})
            for data in ("", PKG, PKG + "\n" + PKG + "\n", "\n", "bad package\n",
                         "".join(f"example.org/pkg{i}\n" for i in range(9))):
                path.write_text(data)
                with self.subTest(data=data), self.assertRaises(m.Refusal):
                    m.failed_packages(path)

    def test_CINV08720_immutable_list_and_diagnostic_script(self):
        # Exercise the actual scripts, using a committed fixture policy and authorizer. Changing
        # the worktree's list or authorizer cannot authorize a new test (the workflow reads both
        # from the event base), and the diagnostic report survives a policy refusal.
        with tempfile.TemporaryDirectory() as tmp, tempfile.TemporaryDirectory() as runner_temp:
            root = pathlib.Path(tmp)
            env = {"PATH": os.environ["PATH"], "HOME": tmp, "GIT_CONFIG_NOSYSTEM": "1",
                   "GIT_CONFIG_GLOBAL": os.devnull, "SHARD": "0", "SHARDS": "1",
                   "RUNNER_TEMP": runner_temp,
                   "GITHUB_STEP_SUMMARY": str(root / "summary"),
                   "GITHUB_OUTPUT": str(root / "outputs")}

            def git(*args):
                return subprocess.check_output(["git", *args], cwd=root, env=env,
                                               stderr=subprocess.STDOUT).decode().strip()

            git("init", "-q")
            policy = root / m.LIST_PATH
            policy.parent.mkdir(parents=True)
            # Clock-independent fixture only; the production seed remains seven days.
            policy.write_bytes(ROW.replace(b"2026-11-15", b"9999-12-31"))
            authorizer = policy.parent / "known-flakes.py"
            shutil.copyfile(HERE / "known-flakes.py", authorizer)
            git("add", ".")
            git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                "commit", "-qm", "fixture policy")
            revision = git("rev-parse", "HEAD")
            env["KNOWN_FLAKES_REVISION"] = revision
            # The tested tree's copies: a list naming TestNew and an authorizer that forgives
            # everything. Neither may count.
            policy.write_text(f"{PKG}\tTestNew\tV1-4549\t9999-12-31\n")
            authorizer.write_text("import sys\nsys.exit(0)\n")
            for script in ("flaky.sh", "outcomes.sh", "outcomes.jq"):
                shutil.copyfile(HERE / script, policy.parent / script)
            ci = (HERE.parent / "workflows" / "ci.yml").read_text()
            block = re.search(r'(?ms)^              if \[ "\$status" != 0 \].*?^              fi$', ci)
            self.assertIsNotNone(block, "actual workflow forgiveness block is missing")
            decision = textwrap.dedent(block[0])
            self.assertIn('git show "$KNOWN_FLAKES_REVISION:.github/forward-rows/known-flakes.py" > "$RUNNER_TEMP/known-flakes.py"', decision)
            self.assertNotIn("python3 .github/forward-rows/known-flakes.py", decision)
            env.update(outcomes_ok="1", status="1", flaky="")
            stream = root / "stream"
            outcomes = root / "outcomes"
            outcomes.mkdir()
            env.update(outcomes_dir=str(outcomes), capture=str(stream))
            for name, data in {
                "expected": PKG + "\n", "status": "1\n", "rerun-status": "0\n", "rerun-lines": PKG + "\n",
                "first.json": json.dumps({PKG: "fail"}), "rerun.json": json.dumps({PKG: "pass"}),
                "outcomes.json": '{"rerun":{"state":"RAN"}}',
            }.items():
                (outcomes / name).write_text(data)
            listed = event("start") + event("run", "TestKnown") + event("fail", "TestKnown")
            for data, want in ((failure("TestKnown"), 0), (failure("TestNew"), 1),
                               (listed + event("run", "TestNew")
                                + output("panic: test timed out after 1s\n", "TestNew")
                                + event("fail"), 1),
                               (failure() + event("start") + event("run", "TestNew"), 1),
                               (listed + output("future runtime diagnostic\n") + event("fail"), 1)):
                stream.write_bytes(data)
                diagnostic = subprocess.run(["bash", "-e", "-c", decision + '\nexit "$status"'],
                                            cwd=root, env=env, capture_output=True, text=True)
                self.assertEqual(diagnostic.returncode, want, diagnostic.stdout + diagnostic.stderr)
                self.assertEqual(json.loads((outcomes / "flaky.json").read_text())["verdict"], "FLAKY")
                self.assertIn("flaky=1", (root / "outputs").read_text())
                result = subprocess.run(["python3", "-B", str(HERE / "known-flakes.py"), str(stream),
                                         str(outcomes / "flaky-failed-packages"), str(outcomes / "expected")], cwd=root, env=env,
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, want, result.stdout + result.stderr)
            # Test code that ran earlier in the job can write $RUNNER_TEMP, where the authorizer is
            # copied: a planted module the authorizer imports must not run (python3 -I keeps the
            # script's directory off sys.path), so an unlisted failure still fails.
            planted = pathlib.Path(runner_temp) / "datetime.py"
            planted.write_text("import sys\nsys.exit(0)\n")
            stream.write_bytes(failure("TestNew"))
            diagnostic = subprocess.run(["bash", "-e", "-c", decision + '\nexit "$status"'],
                                        cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(diagnostic.returncode, 1, diagnostic.stdout + diagnostic.stderr)
            planted.unlink()
            # Replay two failed slices of one package using the actual outcome reducer.
            # The second invocation exits before test2json can emit even a start event.
            silent = subprocess.run(["bash", "-c", "echo 'go: creating work dir: denied' >&2; exit 1"],
                                    cwd=root, env=env, capture_output=True)
            self.assertEqual(silent.returncode, 1)
            self.assertEqual(silent.stdout, b"")
            self.assertIn(b"creating work dir", silent.stderr)
            env.update(GITHUB_EVENT_NAME="pull_request", GITHUB_RUN_ID="1",
                       TESTED_MERGE=revision)
            (outcomes / "expected").write_text((PKG + "\n") * 2)
            (outcomes / "whole").write_text("")
            (outcomes / "failed-slices").write_text(
                f"-run ^(TestKnown)$ {PKG}\n-run ^(TestOther)$ {PKG}\n")
            for data, want in ((failure() + silent.stdout, 1),
                               (failure() + failure(), 0)):
                with self.subTest(missing_json=want == 1):
                    stream.write_bytes(data)
                    first = subprocess.run(["sh", str(HERE / "outcomes.sh"), "first",
                                            str(outcomes), "1", str(stream)],
                                           cwd=root, env=env, capture_output=True, text=True)
                    self.assertEqual(first.returncode, 0, first.stdout + first.stderr)
                    self.assertEqual(json.loads((outcomes / "first.json").read_text()), {PKG: "fail"})
                    self.assertEqual(len((outcomes / "rerun-lines").read_text().splitlines()), 2)
                    rerun_stream = outcomes / "rerun.stream"
                    rerun_stream.write_bytes((event("start") + event("pass")) * 2)
                    rerun = subprocess.run(["sh", str(HERE / "outcomes.sh"), "rerun",
                                            str(outcomes), str(rerun_stream)],
                                           cwd=root, env=env, capture_output=True, text=True)
                    self.assertEqual(rerun.returncode, 0, rerun.stdout + rerun.stderr)
                    self.assertEqual(json.loads((outcomes / "rerun.json").read_text()), {PKG: "pass"})
                    diagnostic = subprocess.run(["bash", "-e", "-c", decision + '\nexit "$status"'],
                                                cwd=root, env=env, capture_output=True, text=True)
                    self.assertEqual(diagnostic.returncode, want, diagnostic.stdout + diagnostic.stderr)
                    report = json.loads((outcomes / "flaky.json").read_text())
                    self.assertEqual(report["verdict"], "FLAKY")
                    self.assertEqual(report["rerunInvocations"], 2)
                    self.assertIn("flaky=1", (root / "outputs").read_text())
            # A missing/unreachable/malformed revision and missing policy all refuse.
            for bad in ("", "main", "0" * 40, revision + ":" + m.LIST_PATH):
                env["KNOWN_FLAKES_REVISION"] = bad
                result = subprocess.run(["python3", "-B", str(HERE / "known-flakes.py"), str(stream),
                                         str(outcomes / "flaky-failed-packages"), str(outcomes / "expected")], cwd=root, env=env,
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            # An event base with the list but without the authorizer forgives nothing through the
            # workflow block, even though the tested tree holds a permissive authorizer.
            git("rm", "-q", "--cached", str(authorizer.relative_to(root)))
            git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                "commit", "-qm", "no authorizer")
            env["KNOWN_FLAKES_REVISION"] = git("rev-parse", "HEAD")
            (outcomes / "expected").write_text(PKG + "\n")
            for name, data in {"first.json": json.dumps({PKG: "fail"}), "rerun.json": json.dumps({PKG: "pass"}),
                               "rerun-lines": PKG + "\n", "rerun-status": "0\n"}.items():
                (outcomes / name).write_text(data)
            stream.write_bytes(failure("TestKnown"))
            diagnostic = subprocess.run(["bash", "-e", "-c", decision + '\nexit "$status"'],
                                        cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(diagnostic.returncode, 1, diagnostic.stdout + diagnostic.stderr)
            self.assertIn("CIN-V0-8720", diagnostic.stdout)
            self.assertEqual(json.loads((outcomes / "flaky.json").read_text())["verdict"], "FLAKY")
            git("rm", "-q", "--cached", m.LIST_PATH)
            git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                "commit", "-qm", "missing policy")
            env["KNOWN_FLAKES_REVISION"] = git("rev-parse", "HEAD")
            result = subprocess.run(["python3", "-B", str(HERE / "known-flakes.py"), str(stream),
                                     str(outcomes / "flaky-failed-packages"), str(outcomes / "expected")], cwd=root, env=env,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_CINV08720_workflow_trust_and_registration(self):
        ci = (HERE.parent / "workflows" / "ci.yml").read_text()
        tests = ci.split("      - name: Go tests (full until matching shadow qualification)\n", 1)[1]
        tests = tests.split("      - name:", 1)[0]
        self.assertIn("KNOWN_FLAKES_REVISION: ${{ github.event.pull_request.base.sha || "
                      "github.event.merge_group.base_sha || github.sha }}", tests)
        call = ('if git show "$KNOWN_FLAKES_REVISION:.github/forward-rows/known-flakes.py" > "$RUNNER_TEMP/known-flakes.py" 2> /dev/null &&\n'
                '                  python3 -I -B "$RUNNER_TEMP/known-flakes.py" "$capture" "$outcomes_dir/flaky-failed-packages" "$outcomes_dir/expected"; then\n'
                '                  status=0\n')
        self.assertEqual(tests.count(call), 1)
        # The authorization sits inside the flaky.sh branch and is the only status clear.
        flaky = tests.index('sh .github/forward-rows/flaky.sh "$outcomes_dir" "$capture" &&')
        self.assertLess(flaky, tests.index(call))
        self.assertLess(tests.index(call), tests.index('[ "$status" = 0 ] || exit "$status"'))
        self.assertEqual(len(re.findall(r"(?m)^\s+status=0$", tests)), 1)
        # No other code line of ci.yml runs or reads the authorizer (comments may name it).
        code = [line for line in ci.replace(call, "").splitlines() if not line.lstrip().startswith("#")]
        self.assertEqual([line for line in code if "known-flakes.py" in line], [])
        self.assertIn("python3 -B .github/forward-rows/known-flakes_test.py", ci)


if __name__ == "__main__":
    unittest.main()
