# SPDX-License-Identifier: Apache-2.0
"""AFP-V0-2333 / AFP-V0-2346: existing output files are never replay evidence."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("reuse_replay", Path(__file__).with_name("replay.py"))
replay = importlib.util.module_from_spec(spec)
spec.loader.exec_module(replay)


class ReplayFreshnessTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.data = self.root / "data"
        self.out = self.root / "out"
        self.binary = self.root / "binary"
        self.declaration = self.root / "declaration.json"
        self.write(self.binary, "baseline")
        self.write(self.declaration, "baseline")
        self.calls = []
        self.record_calls = []
        self.generation = "baseline"
        self.fault = False
        self.failure = False
        for name, replacement in (
            ("fetch", lambda *_: []), ("checkout", lambda *_: None),
            ("git", lambda *_args, **_kwargs: SimpleNamespace(stdout="base\n")),
        ):
            patcher = patch.object(replay, name, replacement)
            patcher.start()
            self.addCleanup(patcher.stop)
        patcher = patch.object(replay.subprocess, "run", self.command)
        patcher.start()
        self.addCleanup(patcher.stop)

    def write(self, path, value):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(value)

    def document(self, path, value):
        self.write(path, json.dumps(value))

    def command(self, argv, **_kwargs):
        self.calls.append(argv)
        if self.failure:
            return SimpleNamespace(returncode=1, stderr="injected changed-binary failure")
        if "record" in argv:
            self.record_calls.append(argv)
            self.document(Path(argv[argv.index("--out") + 1]), {"generation": self.generation})
        elif "decide" in argv:
            for i, arg in enumerate(argv):
                if arg == "--records":
                    self.assertEqual(replay.load(argv[i + 1])["generation"], self.generation)
            self.document(Path(argv[argv.index("--out") + 1]), {
                "reused": ["example.com/fault"] if self.fault else [], "units": [],
            })
        else:
            return SimpleNamespace(returncode=0, stdout=json.dumps({
                "changed": 1, "jobs": [{"job": "artifact-integrity", "decision": "SKIP" if self.fault else "RUN", "reason": "fixture"}],
            }))
        return SimpleNamespace(returncode=0)

    def test_records_and_decisions_regenerated(self):
        """AFP-V0-2333: changed binary/env/source must expose the injected violation."""
        runs = [{"id": n, "status": "completed", "event": "pull_request", "head_branch": "feature",
                 "created_at": str(n), "head_sha": str(n), "conclusion": "failure"} for n in (1, 2)]
        self.document(self.data / "runs-ci.yml.json", runs)
        self.document(self.data / "prs.json", [{"headRefName": "feature", "state": "MERGED", "number": 1}])
        for n in (1, 2):
            self.document(self.data / "outcomes" / str(n) / "outcomes.json", {
                "target": str(n), "outcomes": {"example.com/fault": "fail" if n == 2 else "pass"},
            })

        def run():
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                replay.run_tests(str(self.data), str(self.binary), "fixture-worktree", str(self.out))
            return replay.load(self.out / "summary.json")

        self.assertEqual(run()["totals"]["violations"], 0)
        self.assertEqual(len(self.calls), 2)
        for change in ("binary", "environment", "source"):
            self.generation = change
            self.fault = True
            if change == "binary":
                self.write(self.binary, "changed")
            elif change == "environment":
                replay.ENV["REPLAY_QUALIFICATION"] = "changed"
                self.addCleanup(replay.ENV.pop, "REPLAY_QUALIFICATION")
            else:
                runs[0]["run_attempt"] = 2
                self.document(self.data / "runs-ci.yml.json", runs)
            before = len(self.calls)
            result = run()
            self.assertEqual(len(self.calls) - before, 2)
            self.assertEqual(result["totals"]["violations"], 1)
            self.assertEqual(self.record_calls[-1][self.record_calls[-1].index("--run-attempt") + 1], "2" if change == "source" else "1")
        self.failure = True
        result = run()
        self.assertEqual(result["pairs"][0]["state"], "NOT_PRODUCED")
        self.assertEqual(result["totals"]["decided"], 0)
        # A failed recomputation leaves no earlier output that could be read as its result.
        self.assertEqual(sorted(p.name for p in (self.out / "records").iterdir()), [])
        self.assertFalse((self.out / "decisions" / "2.json").exists())

    def test_job_plans_regenerated(self):
        """AFP-V0-2346: changing the declaration/binary must rerun job planning."""
        self.document(self.data / "runs-ci.yml.json", [{
            "id": 1, "status": "completed", "event": "push", "head_sha": "head", "created_at": "1",
        }])
        self.document(self.data / "runs-intake-containment.yml.json", [])
        self.document(self.data / "jobs" / "1.json", {"jobs": [{"name": "artifact-integrity", "conclusion": "failure"}]})

        def run():
            with contextlib.redirect_stdout(io.StringIO()):
                replay.run_jobs(str(self.data), str(self.binary), "fixture-worktree", str(self.out), str(self.declaration))
            return replay.load(self.out / "summary.json")

        self.assertEqual(run()["totals"]["violations"], 0)
        self.fault = True
        self.write(self.declaration, "changed")
        self.write(self.binary, "changed")
        self.assertEqual(run()["totals"]["violations"], 1)
        self.assertEqual(len(self.calls), 2)
        self.failure = True
        result = run()
        self.assertEqual(result["runs"][0]["state"], "NOT_PRODUCED")
        self.assertEqual(result["totals"]["planned"], 0)
        self.assertFalse((self.out / "plans" / "1.json").exists())


if __name__ == "__main__":
    unittest.main()
