# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-2140..2142: replay the operator alarm through its GitHub read boundary."""
import base64
import contextlib
import importlib.util
import io
import json
import pathlib
import subprocess
import sys
import unittest
import zipfile
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("absence_alarm", pathlib.Path(__file__).with_name("absence-alarm.py"))
alarm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(alarm)


class Replay:
    def __init__(self, count=3):
        self.runs = [self.run(n) for n in range(count, 0, -1)]
        self.rows = []
        self.epochs = {}
        self.names = {}
        self.changed = []
        self.calls = []
        self.fail = None
        self.bad_doc = False
        self.row_body = None  # Replaces every downloaded row.json body when set.
        self.size = 512
        self.archives = {}  # Artifact ID -> raw archive bytes overriding the generated zip.

    @staticmethod
    def run(n):
        repo = {"id": 1}
        return {"id": n, "run_attempt": 1, "status": "completed", "conclusion": "success", "event": "pull_request",
                "head_sha": f"{n:040x}", "updated_at": f"2020-01-01T00:{n % 60:02d}:00Z",
                "pull_requests": [{"number": n, "base": {"ref": "main", "sha": "a" * 40, "repo": repo},
                                   "head": {"repo": repo}}]}

    @staticmethod
    def target(n):
        return f"{n + 1000:040x}"

    def row(self, n, epoch="2", created="2020-01-02T00:00:00Z", branch="main", owner=1, kind="CLEAN"):
        self.rows.append({"id": 50000 + len(self.rows), "size_in_bytes": self.size,
                          "name": f"ci-forward-{kind}-pr{n}-{self.target(n)}-e{epoch}-f0",
                          "expired": False, "created_at": created,
                          "workflow_run": {"head_branch": branch, "repository_id": 1, "head_repository_id": owner, "id": n + 2000}})

    @staticmethod
    def archive(entries):
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", zipfile.ZIP_DEFLATED) as archive:
            for name, data in entries:
                archive.writestr(name, data)
        return buffer.getvalue()

    def outcome_id(self, n, index):
        return 100000 + n * 100 + index

    def download(self, artifact_id):
        if artifact_id in self.archives:
            return self.archives[artifact_id]
        if artifact_id >= 100000:
            n = (artifact_id - 100000) // 100
            filename = "outcomes.json"
            doc = {"profile": "corvint-ci-outcomes/0", "event": "pull_request", "runId": n,
                   "runAttempt": 1, "target": self.target(n), "head": f"{n:040x}"}
        else:
            artifact = next(a for a in self.rows if a["id"] == artifact_id)
            filename = "row.json"
            n = artifact["workflow_run"]["id"] - 2000
            name = artifact["name"]
            doc = {"profile": "corvint-pr-forward-row/0", "kind": "pr", "class": name.split("-")[2],
                   "runId": n, "selectionRunId": n, "outcomesRunId": n,
                   "pr": int(name.split("-")[3].removeprefix("pr")), "sha": name.split("-")[4], "epoch": 2}
            if self.row_body is not None:
                doc = self.row_body
        return self.archive([(filename, json.dumps([] if self.bad_doc else doc))])

    def gh(self, *args, limit=None):
        self.calls.append(args)
        if self.fail and self.fail in " ".join(args):
            raise alarm.Unavailable("fixture unreadable API")
        if args[0] == "run":
            raise AssertionError("gh run download extracts before bounds apply")
        if args[1].endswith("/zip"):
            assert limit is not None, "artifact transfer must be bounded"
            blob = self.download(int(args[1].split("/")[-2]))
            if len(blob) > limit:
                raise alarm.Unavailable("fixture transfer limit")
            return blob
        path = args[1].removeprefix("repos/owner/repo/")
        if path.startswith("actions/workflows/"):
            result = {"workflow_runs": self.runs}
        elif path.startswith("actions/artifacts?"):
            result = {"artifacts": self.rows}
        elif path.startswith("actions/runs/"):
            n = int(path.split("/")[2])
            result = {"artifacts": [{"id": self.outcome_id(n, i), "name": name, "expired": False,
                                     "size_in_bytes": self.size} for i, name in
                      enumerate(self.names.get(n, ["ci-outcomes-pull_request-0"]))]}
        elif path.startswith("commits/"):
            n = int(path.split("/")[1], 16) - 1000
            result = {"parents": [{"sha": "b" * 40}, {"sha": f"{n:040x}"}]}
        elif path.startswith("compare/"):
            result = {"status": "ahead", "files": self.changed}
        elif path.startswith("contents/"):
            n = int(path.split("ref=")[1], 16) - 1000
            workflow = f'    CORVINT_PR_FORWARD_EPOCH: "{self.epochs.get(n, "2")}"\n'
            result = {"content": base64.b64encode(workflow.encode()).decode()}
        else:
            raise AssertionError(path)
        return json.dumps(result).encode()


class AbsenceAlarmTest(unittest.TestCase):
    def execute(self, replay, epoch="2"):
        out, err = io.StringIO(), io.StringIO()
        with patch.object(alarm, "gh", replay.gh), contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = alarm.main(["owner/repo", epoch])
        return code, json.loads(out.getvalue()), err.getvalue()

    def test_CIN_V0_2140_missing_selection_trips_alarm_end_to_end(self):
        # No selection document exists anywhere in this replay. Full outcomes,
        # immutable workflow bytes and completed run metadata still prove eligibility.
        replay = Replay()
        code, result, stderr = self.execute(replay)
        self.assertEqual((code, result["alarm"], result["absentRunIds"]), (1, "TRIPPED", [3, 2, 1]))
        self.assertIn("::error::Forward-row absence", stderr)
        self.assertFalse(any("shadow" in " ".join(c) for c in replay.calls))
        replay.row(1)
        code, result, _ = self.execute(replay)
        self.assertEqual((code, result["alarm"], len(result["absentRunIds"])), (0, "clear", 2))

    def test_CIN_V0_2140_outage_while_narrowing_is_on(self):
        replay = Replay()
        replay.names = {n: ["ci-outcomes-pull_request-0", "ci-narrowing-0"] for n in range(1, 4)}
        self.assertEqual(self.execute(replay)[0], 1)
        for n in range(1, 4):
            replay.row(n, kind="NARROWED")
        self.assertEqual(self.execute(replay)[0], 0)

    def test_CIN_V0_2141_epoch_one_runs_without_selection_are_excluded(self):
        replay = Replay(5)
        replay.epochs = {1: "1", 2: "1", 3: "1"}
        code, result, _ = self.execute(replay)
        self.assertEqual((code, result["runIds"], result["absentRunIds"]), (0, [5, 4], [5, 4]))
        replay.row(5, epoch="1")  # Old epoch row cannot cover epoch 2.
        self.assertEqual(self.execute(replay)[1]["absentRunIds"], [5, 4])

    def test_CIN_V0_2140_window_and_old_rerun_row(self):
        replay = Replay(23)
        for n in range(4, 24):
            replay.row(n)
        result = self.execute(replay)[1]
        self.assertEqual((result["window"], result["absentRunIds"]), (20, []))
        replay = Replay()
        for n in range(1, 4):
            replay.row(n, created="2019-01-01T00:00:00Z")
        self.assertEqual(self.execute(replay)[0], 1)
        replay.runs.append(replay.runs[0])
        self.assertEqual(self.execute(replay)[1]["window"], 3)

    def test_CIN_V0_2142_forward_body_binds_run_and_rerun_is_unknown(self):
        replay = Replay()
        for n in range(1, 4):
            replay.row(n)
            # Valid name, but a delayed artifact from a different CI run.
            replay.rows[-1]["workflow_run"]["id"] += 10
        self.assertEqual(self.execute(replay)[0], 1)
        replay = Replay()
        replay.runs[0]["run_attempt"] = 2
        self.assertEqual(self.execute(replay)[0], 2)

    def test_CIN_V0_2141_intentional_non_rows(self):
        for kind in ("docs", "github", "rename", "large", "fork", "incomplete", "grace"):
            with self.subTest(kind=kind):
                replay = Replay()
                if kind == "docs":
                    replay.names = {n: [] for n in range(1, 4)}
                elif kind == "github":
                    replay.changed = [{"filename": ".github/workflows/ci.yml"}]
                elif kind == "rename":
                    replay.changed = [{"filename": "foo", "previous_filename": ".github/old"}]
                elif kind == "large":
                    replay.changed = [{"filename": "foo"}] * 300
                elif kind == "fork":
                    for run in replay.runs:
                        run["pull_requests"][0]["head"]["repo"] = {"id": 2}
                elif kind == "incomplete":
                    for run in replay.runs:
                        run["status"] = "in_progress"
                elif kind == "grace":
                    for run in replay.runs:
                        run["updated_at"] = alarm.dt.datetime.now(alarm.dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
                code, result, _ = self.execute(replay)
                self.assertEqual((code, result["window"]), (0, 0))

    def test_CIN_V0_2142_untrusted_rows_and_unavailable_evidence(self):
        for branch, owner in (("pr", 1), ("main", 2)):
            replay = Replay()
            for n in range(1, 4):
                replay.row(n, branch=branch, owner=owner)
            self.assertEqual(self.execute(replay)[0], 1)
        for failure in ("actions/artifacts", "contents/", "/zip", "commits/", "compare/"):
            with self.subTest(failure=failure):
                replay = Replay()
                replay.fail = failure
                code, result, stderr = self.execute(replay)
                self.assertEqual((code, result["alarm"]), (2, "UNKNOWN"))
                self.assertIn("::error::", stderr)
        replay = Replay()
        replay.epochs[1] = "bad"
        self.assertEqual(self.execute(replay)[0], 2)
        self.assertEqual(self.execute(Replay(), epoch="bad")[0], 2)

    def test_CIN_V0_2142_malformed_api_and_document_are_unknown(self):
        for bad in (None, [], "bad", {"expired": False, "workflow_run": None}):
            replay = Replay()
            replay.rows = [bad]
            self.assertEqual(self.execute(replay)[0], 2)
        replay = Replay()
        replay.bad_doc = True
        self.assertEqual(self.execute(replay)[0], 2)

    def test_CIN_V0_2142_artifact_lower_bound_avoids_old_pages(self):
        replay = Replay()
        replay.rows = [{"name": "unrelated", "expired": False, "created_at": "2019-01-01T00:00:00Z"}] * 100
        self.assertEqual(self.execute(replay)[0], 1)
        self.assertEqual(len([c for c in replay.calls if "actions/artifacts?" in " ".join(c)]), 1)

    def test_CIN_V0_2142_bounds_and_command_failure(self):
        replay = Replay(100)
        replay.names = {n: [] for n in range(1, 101)}
        self.assertEqual(self.execute(replay)[0], 2)
        replay = Replay()
        replay.rows = [{"name": "unrelated", "expired": False, "created_at": "2030-01-01T00:00:00Z"}] * 100
        code, result, _ = self.execute(replay)
        self.assertEqual((code, result["alarm"]), (2, "UNKNOWN"))
        self.assertIn("20 pages", result["limit"])
        with patch.object(alarm.subprocess, "run", side_effect=subprocess.TimeoutExpired("gh", 60)):
            with self.assertRaises(alarm.Unavailable):
                alarm.gh("api", "x")

    def test_CIN_V0_2141_uncollected_conclusions_are_not_candidates(self):
        # Review P2: outcomes upload under always(), but the collector runs only on success or
        # failure, so a cancelled or timed-out run with outcomes is an intentional non-row.
        for conclusion in ("cancelled", "timed_out", "skipped", "neutral", "action_required", "stale", None):
            with self.subTest(conclusion=conclusion):
                replay = Replay()
                for run in replay.runs:
                    run["conclusion"] = conclusion
                code, result, _ = self.execute(replay)
                self.assertEqual((code, result["alarm"], result["window"]), (0, "clear", 0))
                self.assertFalse(any(c[1].endswith("/zip") for c in replay.calls))
        replay = Replay()
        replay.runs[0]["conclusion"] = "failure"
        self.assertEqual(self.execute(replay)[1]["absentRunIds"], [3, 2, 1])

    def test_CIN_V0_2142_malformed_row_bodies_are_unknown(self):
        # Review P2: a matching artifact whose body cannot be validated is uncertainty, not absence.
        for body in ({}, {"profile": "corvint-pr-forward-row/0", "class": "INCOMPLETE",
                          "incomplete": ["row classification failed"], "truncated": False},
                     {"profile": "corvint-pr-forward-row/0", "kind": "pr", "class": "CLEAN", "pr": 1,
                      "sha": Replay.target(1), "epoch": "2", "runId": 1, "selectionRunId": 1,
                      "outcomesRunId": 1},
                     {"profile": "corvint-pr-forward-row/0", "kind": "pr", "class": "FLAKY", "pr": 1,
                      "sha": Replay.target(1), "epoch": 2, "runId": 1, "selectionRunId": 1,
                      "outcomesRunId": True}):
            with self.subTest(body=body):
                replay = Replay()
                for n in range(1, 4):
                    replay.row(n)
                replay.row_body = body
                code, result, stderr = self.execute(replay)
                self.assertEqual((code, result["alarm"]), (2, "UNKNOWN"))
                self.assertIn("malformed", result["limit"])
                self.assertIn("::error::", stderr)
        # A well-formed row naming another CI run still does not discharge absence.
        replay = Replay()
        for n in range(1, 4):
            replay.row(n)
            replay.rows[-1]["workflow_run"]["id"] += 10
        code, result, _ = self.execute(replay)
        self.assertEqual((code, result["alarm"], result["absentRunIds"]), (1, "TRIPPED", [3, 2, 1]))

    def test_CIN_V0_2142_download_bounds_apply_before_transfer_and_extraction(self):
        # Review P2: count and size come from listing metadata before any transfer.
        replay = Replay()
        replay.size = alarm.DOC_LIMIT + alarm.ZIP_OVERHEAD + 1
        code, result, _ = self.execute(replay)
        self.assertEqual((code, result["limit"]), (2, "artifact exceeds its transfer limit"))
        self.assertFalse(any(c[1].endswith("/zip") for c in replay.calls))
        replay = Replay()
        replay.names = {n: [f"ci-outcomes-pull_request-{i}" for i in range(33)] for n in range(1, 4)}
        code, result, _ = self.execute(replay)
        self.assertEqual((code, result["limit"]), (2, "missing or excessive matching artifacts"))
        self.assertFalse(any(c[1].endswith("/zip") for c in replay.calls))
        replay = Replay()
        for n in range(1, 4):
            for _ in range(33):
                replay.row(n)
        self.assertEqual(self.execute(replay)[1]["limit"], "missing or excessive matching artifacts")
        self.assertFalse(any("/artifacts/5" in c[1] for c in replay.calls))
        # Archive entries are bounded before decompression; nothing is extracted to disk.
        doc = json.dumps({"profile": "corvint-ci-outcomes/0"})
        for name, archive in (
                ("extra entry", Replay.archive([("outcomes.json", doc), ("unrelated.bin", "x")])),
                ("wrong entry", Replay.archive([("nested/outcomes.json", doc)])),
                ("empty archive", Replay.archive([])),
                ("not a zip", b"not a zip"),
                ("oversized entry", Replay.archive([("outcomes.json", " " * (alarm.DOC_LIMIT + 1))]))):
            with self.subTest(archive=name):
                replay = Replay()
                replay.archives = {replay.outcome_id(n, 0): archive for n in range(1, 4)}
                code, result, _ = self.execute(replay)
                self.assertEqual((code, result["alarm"]), (2, "UNKNOWN"))

    def test_CIN_V0_2142_bounded_transfer_stops_the_reader(self):
        write = "import sys; sys.stdout.buffer.write(b'x' * int(sys.argv[1]))"
        with patch.object(alarm, "GH", (sys.executable, "-I", "-c", write)):
            self.assertEqual(alarm.gh("64", limit=64), b"x" * 64)
            with self.assertRaisesRegex(alarm.Unavailable, "transfer exceeds"):
                alarm.gh(str(64 * 1024 * 1024), limit=1024)
        fail = "import sys; sys.exit(3)"
        with patch.object(alarm, "GH", (sys.executable, "-I", "-c", fail)):
            with self.assertRaisesRegex(alarm.Unavailable, "read failed"):
                alarm.gh("api", limit=1024)
        hang = "import time; time.sleep(30)"
        with patch.object(alarm, "GH", (sys.executable, "-I", "-c", hang)), patch.object(alarm, "TIMEOUT", 0.5):
            with self.assertRaisesRegex(alarm.Unavailable, "read failed"):
                alarm.gh("api", limit=1024)


if __name__ == "__main__":
    unittest.main()
