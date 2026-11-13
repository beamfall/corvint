# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-8722 (V1-5258): the line-level selection step keeps main's stamped planner and accepts
only its expected self-binary-stale degradation.

Run: python3 -B .github/forward-rows/linecov-stale_test.py (requires bash and jq).
No hosted calls; this extracts and runs the workflow's own lines, it is not a hosted run.
"""
import json
import os
import pathlib
import re
import subprocess
import tempfile
import textwrap
import unittest

CI = pathlib.Path(__file__).resolve().parents[1] / "workflows" / "ci.yml"
STEP = "      - name: Line-level selection (GLS-V0-040..043; narrows only while CORVINT_LINECOV_NARROWING is on)\n"


class LinecovStale(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        ci = CI.read_text()
        assert ci.count(STEP) == 1, "the line-level selection step is missing or repeated"
        step = ci.split(STEP, 1)[1].split("\n      - name: ", 1)[0]
        cls.script = textwrap.dedent(step.split("        run: |\n", 1)[1])

    def test_CINV08722_bindings(self):
        s = self.script
        # The planner keeps its Git stamp: -buildvcs=false would report no degradation at all.
        self.assertEqual(s.count('go build -trimpath -buildvcs=true -o "$RUNNER_TEMP/linecov-corvint" ./cmd/corvint'), 1)
        self.assertNotIn('-buildvcs=false -o "$RUNNER_TEMP/linecov-corvint"', s)
        call = '"$RUNNER_TEMP/linecov-corvint" --root "$PWD" affected --accept-stale-binary --base "$base" \\\n'
        self.assertEqual(s.count(call), 1)
        self.assertEqual(s.count("--accept-stale-binary"), 2)  # the call and its comment
        check = s.index("degraded=$(jq -r ")
        self.assertLess(s.index(call), check)
        self.assertLess(check, s.index("jq -e '.lineSelection'"))
        self.assertLess(check, s.index("state=$(jq -r '.state'"))
        # The proof directory retained for a NARROWED shard is the one the marker is written to.
        self.assertIn('d="$RUNNER_TEMP/linecov-narrow"', s)
        self.assertIn("path: ${{ runner.temp }}/linecov-narrow/\n", CI.read_text())
        self.assertIn("python3 -B .github/forward-rows/linecov-stale_test.py", CI.read_text())

    def check(self, receipt):
        """Run the step's degradation check on RECEIPT (bytes, or None for no file)."""
        start = self.script.index("degraded=$(jq -r ")
        end = self.script.index("\nesac\n", start) + len("\nesac\n")
        block = self.script[start:end]
        with tempfile.TemporaryDirectory() as tmp:
            d = pathlib.Path(tmp) / "linecov-narrow"
            d.mkdir()
            if receipt is not None:
                (d / "affected.json").write_bytes(receipt)
            summary, output = pathlib.Path(tmp) / "summary", pathlib.Path(tmp) / "output"
            prelude = textwrap.dedent('''\
                d="$D"
                stop() {
                  printf '%s\\n' "$1" > "$d/reason"
                  echo "mode=FALLBACK" >> "$GITHUB_OUTPUT"
                  exit 0
                }
                ''')
            env = {"PATH": os.environ["PATH"], "D": str(d), "SHARD": "3", "tools": "a" * 40,
                   "GITHUB_STEP_SUMMARY": str(summary), "GITHUB_OUTPUT": str(output)}
            result = subprocess.run(["bash", "--noprofile", "--norc", "-eo", "pipefail", "-c",
                                     prelude + block + 'echo "mode=CONTINUED" >> "$GITHUB_OUTPUT"\n'],
                                    env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            read = lambda p: p.read_text() if p.exists() else None
            return read(output), read(d / "reason"), read(d / "degraded"), read(summary)

    def test_CINV08722_only_the_stale_degradation_continues(self):
        for receipt in (json.dumps({"lineSelection": {}}), json.dumps({"degradations": []}),
                        json.dumps({"degradations": None})):
            with self.subTest(receipt=receipt):
                mode, reason, degraded, summary = self.check(receipt.encode())
                self.assertEqual(mode, "mode=CONTINUED\n")
                self.assertIsNone(reason)
                self.assertIsNone(degraded)
                self.assertIsNone(summary)
        mode, reason, degraded, summary = self.check(json.dumps(
            {"degradations": ["self-binary-stale"], "degradationDetails": {"self-binary-stale": "x"}}).encode())
        self.assertEqual(mode, "mode=CONTINUED\n")
        self.assertIsNone(reason)
        self.assertEqual(degraded, "self-binary-stale\n")
        self.assertEqual(summary, "CIN-V0-8722 shard 3: planned by the trusted selector of main commit "
                         + "a" * 40 + ", not the tested tree; degraded self-binary-stale (AFP-V0-9730)\n")

    def test_CINV08722_any_other_degradation_falls_back(self):
        for receipt, why in (
                (json.dumps({"degradations": ["self-binary-unverified"]}).encode(), "self-binary-unverified"),
                (json.dumps({"degradations": ["self-binary-stale", "future-code"]}).encode(), "self-binary-stale,future-code"),
                (json.dumps({"degradations": ["self-binary-stale", "self-binary-stale"]}).encode(), "self-binary-stale,self-binary-stale"),
                (json.dumps({"degradations": "self-binary-stale"}).encode(), "malformed"),
                (json.dumps({"degradations": {"self-binary-stale": True}}).encode(), "malformed"),
                (json.dumps({"degradations": [1]}).encode(), "1"),
                (b"not json", "unreadable"),
                (None, "unreadable")):
            with self.subTest(receipt=receipt):
                mode, reason, degraded, _ = self.check(receipt)
                self.assertEqual(mode, "mode=FALLBACK\n")
                self.assertIn(why, reason)
                self.assertIsNone(degraded)


if __name__ == "__main__":
    unittest.main()
