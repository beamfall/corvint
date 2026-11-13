# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-8721: execute the actual go-product aggregation step with controlled job outcomes.

Run: python3 -B .github/forward-rows/go-product_test.py
No hosted calls; this is a workflow extract check, not evidence of a hosted run.
"""
import os
import pathlib
import re
import subprocess
import tempfile
import textwrap
import unittest

CI = pathlib.Path(__file__).resolve().parents[1] / "workflows" / "ci.yml"


class GoProduct(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        ci = CI.read_text()
        cls.job = re.search(r"(?ms)^  go-product:\n(.*?)(?=^  [\w-]+:)", ci)[1]
        cls.script = textwrap.dedent(cls.job.split("        run: |\n", 1)[1])

    def aggregate(self, **changes):
        env = {"PATH": os.environ["PATH"], "INTEGRITY_RESULT": "success",
               "STATIC_REUSE_RESULT": "skipped", "STATIC_REUSE": "",
               "CADENCE_RESULT": "success", "CADENCE": "FULL",
               "GITHUB_EVENT_NAME": "pull_request", "EVENT_NAME": "pull_request",
               "GITHUB_REF": "refs/pull/1/merge", "BATCHED": "false",
               "HOST_RESULT": "success", "LIVE_RESULT": "skipped",
               "WEB_PLAN_RESULT": "success", "WEB_REQUIRED": "false", "WEB_RESULT": "skipped",
               "SHARDS_RESULT": "skipped", "STATIC_RESULT": "success", "SHELL_RESULT": "success",
               "DOCS_RESULT": "success", "DOCS_MODE": "DOCS", "SHARDS": "1"}
        env.update(changes)
        if env["GITHUB_EVENT_NAME"] == "merge_group":
            env["LIVE_RESULT"] = "success"
        with tempfile.TemporaryDirectory() as tmp:
            env.update(GITHUB_STEP_SUMMARY=str(pathlib.Path(tmp) / "summary"), RUNNER_TEMP=tmp)
            # GitHub's bash step uses -e and pipefail. Include -u to expose missing fixture inputs.
            return subprocess.run(["bash", "-euo", "pipefail", "-c", self.script],
                                  env=env, capture_output=True, text=True)

    def assert_passes(self, **changes):
        result = self.aggregate(**changes)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def assert_refuses(self, **changes):
        result = self.aggregate(**changes)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("::error::artifact-integrity", result.stdout)

    def test_CINV08721_success_and_failure_before_early_returns(self):
        for event in ("pull_request", "merge_group", "push"):
            for mode in ("DOCS", "REUSE"):
                with self.subTest(event=event, mode=mode):
                    self.assert_passes(GITHUB_EVENT_NAME=event, DOCS_MODE=mode)
                    for result in ("failure", "cancelled", "", "unknown"):
                        self.assert_refuses(GITHUB_EVENT_NAME=event, DOCS_MODE=mode,
                                            INTEGRITY_RESULT=result)
        self.assert_passes(BATCHED="true", STATIC_RESULT="skipped", INTEGRITY_RESULT="skipped")
        for result in ("failure", "cancelled", ""):
            self.assert_refuses(BATCHED="true", STATIC_RESULT="skipped", INTEGRITY_RESULT=result)
        self.assert_refuses(GITHUB_EVENT_NAME="merge_group", BATCHED="true", INTEGRITY_RESULT="skipped")

    def test_CINV08721_verified_reuse_only_on_main_push(self):
        valid = dict(INTEGRITY_RESULT="skipped", GITHUB_EVENT_NAME="push",
                     GITHUB_REF="refs/heads/main", STATIC_REUSE_RESULT="success", STATIC_REUSE="REUSE")
        self.assert_passes(**valid)
        for changes in ({"STATIC_REUSE_RESULT": "failure"}, {"STATIC_REUSE_RESULT": "skipped"},
                        {"STATIC_REUSE_RESULT": "cancelled"}, {"STATIC_REUSE_RESULT": ""},
                        {"STATIC_REUSE": ""}, {"STATIC_REUSE": "garbage"},
                        {"GITHUB_EVENT_NAME": "merge_group"}, {"GITHUB_EVENT_NAME": "pull_request"},
                        {"GITHUB_REF": "refs/heads/other"}):
            with self.subTest(changes=changes):
                self.assert_refuses(**(valid | changes))

    def test_CINV08721_verified_defer_only_on_pull_request(self):
        valid = dict(INTEGRITY_RESULT="skipped", CADENCE_RESULT="success", CADENCE="DEFER")
        self.assert_passes(**valid)
        for changes in ({"CADENCE_RESULT": "failure"}, {"CADENCE_RESULT": "skipped"},
                        {"CADENCE_RESULT": "cancelled"}, {"CADENCE_RESULT": ""},
                        {"CADENCE": "FULL"}, {"CADENCE": ""}, {"CADENCE": "garbage"},
                        {"GITHUB_EVENT_NAME": "merge_group"}, {"GITHUB_EVENT_NAME": "push"}):
            with self.subTest(changes=changes):
                self.assert_refuses(**(valid | changes))
        # Even a valid DEFER cannot erase an actual archive job failure.
        self.assert_refuses(**(valid | {"INTEGRITY_RESULT": "failure"}))

    def test_CINV08721_result_matrix_before_every_early_return(self):
        # Every integrity result across the three events and the DOCS, REUSE and batched early
        # returns: only success, or a skip of a ci:batched pull request, passes here (the REUSE and
        # DEFER skips need their producers, covered below). Unknown results refuse.
        for event in ("pull_request", "merge_group", "push"):
            for mode in ("DOCS", "REUSE", "BATCHED"):
                for result in ("success", "failure", "cancelled", "skipped", "", "future-result"):
                    with self.subTest(event=event, mode=mode, result=result):
                        changes = dict(GITHUB_EVENT_NAME=event, INTEGRITY_RESULT=result, DOCS_MODE=mode,
                                       CADENCE_RESULT="", CADENCE="", GITHUB_REF="refs/heads/main")
                        if mode == "BATCHED":
                            changes.update(BATCHED="true", STATIC_RESULT="skipped", DOCS_MODE="FULL")
                        if result == "success" or (result == "skipped" and event == "pull_request"
                                                   and mode == "BATCHED"):
                            self.assert_passes(**changes)
                        else:
                            self.assert_refuses(**changes)

    def test_CINV08721_check_precedes_early_returns_and_mirrors_the_job_condition(self):
        integrity = self.script.index('case "$INTEGRITY_RESULT" in')
        for later in ("# CIN-V0-2320: check before any DOCS, REUSE or batched early return.",
                      'if [ "$BATCHED" = true ] && [ "$SHARDS_RESULT" = skipped ]',
                      "# CIN-V0-007: shards skipped at job level only"):
            self.assertLess(integrity, self.script.index(later), later)
        # The three admitted skips are exactly the three skip clauses of the job's own if:.
        ci = CI.read_text()
        job = re.search(r"(?ms)^  artifact-integrity:\n(.*?)(?=^  [\w-]+:)", ci)[1]
        self.assertIn("    if: ${{ !cancelled() && !contains(github.event.pull_request.labels.*.name, 'ci:batched')"
                      " && needs.static-reuse.outputs.mode != 'REUSE' && !(github.event_name == 'pull_request'"
                      " && needs.cadence-plan.result == 'success' && needs.cadence-plan.outputs.cadence == 'DEFER') }}\n", job)
        self.assertIn("    needs: [static-reuse, cadence-plan]\n", job)
        static_reuse = re.search(r"(?ms)^  static-reuse:\n(.*?)(?=^  [\w-]+:)", ci)[1]
        self.assertIn("    if: github.event_name == 'push' && github.ref == 'refs/heads/main'\n", static_reuse)

    def test_CINV08721_workflow_dependencies_and_bindings(self):
        needs = re.search(r"(?m)^    needs: \[(.*)\]$", self.job)[1].split(", ")
        self.assertEqual(needs, ["go-product-shard", "go-static", "docs-plan", "static-reuse",
                                 "shell-test-selected", "web-flows-plan", "web-flows", "go-live",
                                 "shard-plan", "artifact-integrity", "cadence-plan"])
        for binding in ("INTEGRITY_RESULT: ${{ needs.artifact-integrity.result }}",
                        "STATIC_REUSE_RESULT: ${{ needs.static-reuse.result }}",
                        "STATIC_REUSE: ${{ needs.static-reuse.outputs.mode }}",
                        "CADENCE_RESULT: ${{ needs.cadence-plan.result }}",
                        "CADENCE: ${{ needs.cadence-plan.outputs.cadence }}"):
            self.assertIn(binding, self.job)
        self.assertIn("    if: always()", self.job)
        self.assertIn("python3 -B .github/forward-rows/go-product_test.py", CI.read_text())


if __name__ == "__main__":
    unittest.main()
