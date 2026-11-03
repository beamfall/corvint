# SPDX-License-Identifier: Apache-2.0
"""Scorer controls; synthetic rows never count as frozen evaluation evidence."""
import base64
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("measure", Path(__file__).with_name("measure-context-packet.py"))
measure = importlib.util.module_from_spec(spec)
spec.loader.exec_module(measure)


class PairedReadingControls(unittest.TestCase):
    def test_rank_buckets_count_gold_once_at_boundaries(self):
        ranked = [str(i) for i in range(1, 21)]
        self.assertEqual(measure.buckets(ranked, ["1", "5", "6", "10", "11", "20", "21", "1"]),
                         {"1-5": 2, "6-10": 2, "11-20": 2, "top20": 6, "absent": 1})

    def test_span_score_deduplicates_gold_and_prediction(self):
        packet = {"spans": {"rows": [{"path": "f", "start_line": 3, "end_line": 5},
                                      {"path": "f", "start_line": 4, "end_line": 7},
                                      {"path": "g", "start_line": 1, "end_line": 99}]}}
        gold = [{"path": "f", "start_line": 1, "end_line": 5},
                {"path": "f", "start_line": 4, "end_line": 8}]
        self.assertEqual(measure.spanscore(packet, gold), (5, 8))

    def test_byte_budget_counts_complete_message(self):
        packet = {"results": [{"id": "gold"}], "spans": {"rows": []}}
        admitted = measure.fixed_budget(b"x" * 4096, packet, ["gold"], [])
        refused = measure.fixed_budget(b"x" * 4097, packet, ["gold"], [])
        self.assertEqual(admitted["4096"]["file_hits"], 1)
        self.assertEqual(refused["4096"]["file_hits"], 0)
        self.assertFalse(refused["4096"]["delivered"])
        self.assertEqual(refused["8192"]["file_hits"], 1)

    def test_no_inherited_experiments(self):
        with patch.dict(os.environ, {"CORVINT_CONTEXT_ADDITIVE": "on", "CORVINT_CONTEXT_SPANS": "merged", "GIT_INDEX_FILE": "/wrong-index"}):
            env = measure.environment(rrf=True)
        self.assertNotIn("CORVINT_CONTEXT_ADDITIVE", env)
        self.assertNotIn("GIT_INDEX_FILE", env)
        self.assertEqual(env["CORVINT_CONTEXT_SPANS"], "")
        self.assertEqual(env["CORVINT_CONTEXT_RRF"], "on")

    def test_missing_history_is_not_a_zero_recall_measurement(self):
        root = Path(__file__).resolve().parents[1]
        with patch.object(measure, "invoke", side_effect=subprocess.CalledProcessError(128, ["git"])) as call:
            result = measure.history_reading(Path("/not-invoked"), Path("/missing"),
                root / "tools/cw-trial/testdata/unseen-corvint-v2/tasks.json",
                root / "tools/cw-trial/testdata/unseen-corvint-v1/tasks.json")
        self.assertEqual(result["state"], "NOT_RUN")
        self.assertEqual(result["samples_scored"], 0)
        self.assertTrue(result["missing_base_commits"])
        self.assertTrue(all(c.args[0][0] == "git" for c in call.call_args_list))

    def test_capture_digest_is_checked(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "capture.jsonl"
            path.write_text(json.dumps({"invocation": {"sample_id": "x"}, "stdout": {
                "base64": base64.b64encode(b"{}").decode(), "sha256": "bad"}}) + "\n")
            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                measure.captures(path)

    def test_missing_retrieval_data_keeps_metrics_absent(self):
        result = measure.retrieval_reading(None, None, Path("/absent-corpus"), Path("/unused"))
        self.assertEqual(result["state"], "NOT_RUN")
        self.assertEqual(len(result["missing_samples"]), 4)
        self.assertNotIn("readings", result)

    def test_rescore_reads_release_top_level_gold_spans_and_checks_arm(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            samples = root / "benchmark/v2_trace2code/trace2code.jsonl"
            samples.parent.mkdir(parents=True)
            samples.write_text(json.dumps({"id": "s", "gold": {"files": ["f"]},
                "gold_spans": [{"path": "f", "start_line": 3, "end_line": 4}]}) + "\n")
            binary, bench = root / "binary", root / "bench"
            binary.write_bytes(b"binary")
            bench.write_bytes(b"bench")
            packet = json.dumps({"results": [{"id": "f"}], "spans": {"rows": [
                {"path": "f", "start_line": 3, "end_line": 3}]}}).encode()
            for mode in ("corroboration", "on", "hits", "merged", "rrf"):
                stem = root / ("trace2code-" + mode)
                Path(str(stem) + ".json").write_text(json.dumps({"samples_sha256": measure.digest(samples),
                    "corvint": {"sha256": measure.digest(binary)}, "limit": 20, "arms": {},
                    "registration": {"bench_sha256": measure.digest(bench), "environment": [
                        "CORVINT_CONTEXT_RRF=" + ("on" if mode == "rrf" else ""),
                        "CORVINT_CONTEXT_SPANS=" + ("" if mode in ("rrf", "corroboration") else mode)]},
                    "details": [{"id": "s", "gold": ["f"]}]}))
                Path(str(stem) + ".jsonl").write_text(json.dumps({"invocation": {"sample_id": "s"},
                    "stdout": {"base64": base64.b64encode(packet).decode(),
                               "sha256": measure.hashlib.sha256(packet).hexdigest()}}) + "\n")
            with patch.object(measure, "invoke", side_effect=AssertionError("rescore invoked an engine")):
                result = measure.retrieval_reading(binary, bench, root, root, reuse=True)
            score = result["readings"]["trace2code"]["arms"]["hits"]["s"]["budget"]["4096"]
            self.assertEqual((score["span_hits"], score["span_gold"]), (1, 2))
            wrong = root / "trace2code-hits.json"
            contents = json.loads(wrong.read_text())
            contents["registration"]["environment"] = []
            wrong.write_text(json.dumps(contents))
            with self.assertRaisesRegex(ValueError, "arm identity mismatch"):
                measure.retrieval_reading(binary, bench, root, root, reuse=True)


if __name__ == "__main__":
    unittest.main()
