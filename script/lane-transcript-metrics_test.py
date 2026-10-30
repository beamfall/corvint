#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Exercise Git-write classification through the transcript reader and public CLI."""
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


if __name__ == "__main__":
    unittest.main()
