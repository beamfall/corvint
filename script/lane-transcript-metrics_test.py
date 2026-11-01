#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Exercise lane-transcript-metrics.py through its transcript reader and public CLI.

The ASD-V0-834 and ASD-V0-836 groups moved here from script/lane-scripts_test.sh, which as a
script/*.sh test may not name a Python interpreter (GOC-V0-008).
"""
import importlib.util
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest


SCRIPT = pathlib.Path(__file__).with_name("lane-transcript-metrics.py")
SPEC = importlib.util.spec_from_file_location("lane_metrics", SCRIPT)
METRICS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(METRICS)


def transcript(command):
    return json.dumps({"timestamp": "2026-10-25T00:00:00Z", "message": {"content": [
        {"type": "tool_use", "id": "test", "name": "Bash", "input": {"command": command}}
    ]}}) + "\n"


class GitWriteTests(unittest.TestCase):
    def test_classification_through_transcript_reader(self):
        cases = {
            "git commit -m fixture": True,
            "git push origin HEAD": True,
            " A= B=value git -C /repo -c user.name=Fixture --no-pager commit": True,
            "git --git-dir=/repo/.git push": True,
            "git -c user.name=Fixture commit": True,
            "echo git commit; git push": True,
            "git status && git commit": True,
            "git log --grep=commit": False,
            "echo git commit": False,
            "printf 'git push'": False,
            "git -C": False,
            "git --no-pager status": False,
            "git --git-dir= push": False,
            "git --foo-bar !": False,
            "A=git commit": False,
            "git committed": False,
            "git pushy": False,
        }
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "agent-fixture.jsonl"
            for command, expected in cases.items():
                with self.subTest(command=command):
                    path.write_text(transcript(command))
                    self.assertEqual(METRICS.scan(path)[2]["lane"], expected)

    def test_adversarial_options_finish_through_cli(self):
        # The old nested repetition takes exponential time for either repeated
        # options or ambiguous -c operands, even though neither invokes Git.
        for command in ["git " + "--foo-bar " * 32 + "!", "git " + "-c " * 48 + "!"]:
            with self.subTest(command=command), tempfile.TemporaryDirectory() as directory:
                path = pathlib.Path(directory) / "session" / "subagents" / "agent-fixture.jsonl"
                path.parent.mkdir(parents=True)
                path.write_text(transcript(command))
                result = subprocess.run([sys.executable, str(SCRIPT), directory,
                    "--since", "2026-10-25", "--until", "2026-10-26"],
                    capture_output=True, text=True, timeout=3, check=True)
                report = json.loads(result.stdout)
                self.assertEqual(report["subagents"], 1)
                self.assertEqual(report["lanes"], 0)
                self.assertNotIn(command, result.stdout)


def record(timestamp, *content):
    return json.dumps({"timestamp": timestamp, "message": {"content": list(content)}}) + "\n"


def bash(identifier, command):
    return {"type": "tool_use", "id": identifier, "name": "Bash", "input": {"command": command}}


def result(identifier, content):
    return {"type": "tool_result", "tool_use_id": identifier, "content": content}


def cli(*arguments):
    completed = subprocess.run([sys.executable, str(SCRIPT), *arguments],
                               capture_output=True, text=True, timeout=10, check=True)
    return json.loads(completed.stdout)


WINDOW = ("--since", "2026-10-30", "--until", "2026-11-01")


class SessionTests(unittest.TestCase):
    def test_asd_v0_834_project_directories_and_sessions(self):
        # ASD-V0-834 (V1-1630): several project directories aggregate into one window and, with
        # --sessions, top-level session transcripts count with the subagent rules and a kind.
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "p1" / "s1" / "subagents").mkdir(parents=True)
            (root / "p2").mkdir()
            (root / "p1" / "s1" / "subagents" / "agent-sub1.jsonl").write_text(
                record("2026-10-31T01:00:00Z", bash("a", "ls")))
            (root / "p2" / "sess1.jsonl").write_text(
                record("2026-10-31T02:00:00Z", bash("b", "git commit -m x"))
                + record("2026-10-31T02:10:00Z", result("b", "Command timed out after 10m 0s")))
            (root / "p2" / "late.jsonl").write_text(record("2026-11-02T00:00:00Z"))
            dirs = (str(root / "p1"), str(root / "p2"))
            d = cli(*dirs, *WINDOW)
            self.assertEqual((d["subagents"], d["sessions"], d["lanes"]), (1, 0, 0))
            d = cli(*dirs, *WINDOW, "--sessions")
            self.assertEqual((d["subagents"], d["sessions"], d["lanes"]), (1, 1, 1))
            self.assertEqual(sorted(r["kind"] for r in d["perSubagent"]), ["session", "subagent"])
            self.assertEqual([(r["harnessTimeouts600s"], r["lane"]) for r in d["perSubagent"]
                              if r["kind"] == "session"], [(1, True)])

    def test_asd_v0_836_harness_timeouts_and_orchestrator(self):
        # ASD-V0-836 (V1-3549): only the harness's own timeout result counts, never the phrase
        # quoted in output, and a session that starts sessions (or one named by --orchestrator)
        # is reported apart.
        with tempfile.TemporaryDirectory() as directory:
            project = pathlib.Path(directory) / "p"
            project.mkdir()
            lane = (record("2026-10-31T03:00:00Z", bash("c", "cat x.sh"), bash("d", "git commit -m y"),
                           bash("e", "make t"))
                    + record("2026-10-31T03:01:00Z",
                             result("c", "echo timed out at 1800s; agent-x did not complete within its 600s "
                                         "timeout\nCommand timed out after 10m 0s"),
                             result("d", "Exit code 143\nCommand timed out after 3m 0s"),
                             result("e", "Command did not complete within its 600s timeout and was moved to "
                                         "the background")))
            (project / "lane1.jsonl").write_text(lane)
            (project / "named.jsonl").write_text(lane)
            (project / "orch.jsonl").write_text(
                record("2026-10-31T04:00:00Z",
                       {"type": "tool_use", "id": "f", "name": "mcp__ccd_session__start_session", "input": {}},
                       bash("g", "git commit -m z"))
                + record("2026-10-31T04:01:00Z", result("g", "Command timed out after 10m 0s")))
            d = cli(str(project), *WINDOW, "--sessions", "--orchestrator", "named")
            self.assertEqual([(r["agent"], r["harnessTimeouts"], r["harnessTimeouts600s"])
                              for r in d["perSubagent"]], [("lane1", 2, 1)])
            self.assertEqual((d["sessions"], d["lanes"], d["harnessTimeoutsAllSubagents"],
                              d["harnessTimeouts600sLanes"]), (1, 1, 2, 1))
            self.assertEqual(sorted((r["agent"], r["kind"], r["lane"]) for r in d["orchestratorSessions"]),
                             [("named", "orchestrator", False), ("orch", "orchestrator", False)])


if __name__ == "__main__":
    unittest.main()
