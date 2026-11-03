#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""TCP-V0-2223: paired frozen retrieval and history-backed RRF readings.

No downloads, model calls, commits or input writes. Missing inputs are NOT_RUN;
failed readings stay in the report. Existing retrieval-bench scoring is reused.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile


BUDGETS = (4096, 8192, 16384, 32768, 65536)
RELEASES = ("code2test", "comment2context", "edit2ripple", "trace2code")


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def invoke(argv, env=None):
    if env is None:
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    return subprocess.run([str(x) for x in argv], env=env, check=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=1800).stdout


def environment(spans="", rrf=False):
    # Isolate the arms from caller experiments, possession and trace weights.
    result = {k: v for k, v in os.environ.items() if not k.startswith(("CORVINT_", "GIT_"))}
    result["CORVINT_CONTEXT_SPANS"] = spans
    result["CORVINT_CONTEXT_RRF"] = "on" if rrf else ""
    return result


def paths(packet):
    return [row["id"] for row in packet["results"]]


def buckets(ranked, gold):
    counts = {"1-5": 0, "6-10": 0, "11-20": 0, "top20": 0, "absent": 0}
    for path in sorted(set(gold)):
        rank = ranked.index(path) + 1 if path in ranked else 21
        key = "1-5" if rank <= 5 else "6-10" if rank <= 10 else "11-20" if rank <= 20 else "absent"
        counts[key] += 1
        counts["top20"] += int(rank <= 20)
    return counts


def history_reading(binary, history, tasks, subset):
    manifests = [json.loads(p.read_text()) for p in (tasks, subset)]
    all_tasks, v1 = manifests[0]["tasks"], manifests[1]["tasks"]
    if len(all_tasks) != 63 or len(v1) != 34:
        raise ValueError("frozen history manifests must contain 63 and 34 tasks")
    by_id = {task["id"]: task for task in all_tasks}
    # Different query text or gold would invalidate a subset comparison.
    for task in v1:
        if by_id.get(task["id"]) != task:
            raise ValueError("v1 is not an identical subset of v2: " + task["id"])
    missing = []
    for commit in sorted({task["base_commit"] for task in all_tasks}):
        try:
            invoke(["git", "-C", history, "cat-file", "-e", commit + "^{commit}"])
        except subprocess.CalledProcessError:
            missing.append(commit)
    if missing:
        return {"state": "NOT_RUN", "reason": "history base commits unavailable",
                "missing_base_commits": missing, "samples_scored": 0,
                "tasks_sha256": digest(tasks), "subset_sha256": digest(subset)}
    records = []
    with tempfile.TemporaryDirectory(prefix="ctx-w58-history-") as scratch:
        # Same bounded materialization as cw-trial --history: the source
        # receives no worktree registration, checkout or object writes.
        root = Path(scratch) / "repo"
        invoke(["git", "clone", "--shared", "--no-checkout", "-q", history, root])
        for task in all_tasks:
            invoke(["git", "-C", root, "checkout", "-q", "--detach", task["base_commit"]])
            count = int(invoke(["git", "-C", root, "rev-list", "--count", "HEAD"]))
            if (root / ".git/shallow").exists():
                raise ValueError("history-backed reading refuses shallow materialization")
            gold = sorted({p for group in task["gold"].values() for p in group})
            present, absent = [], []
            for path in gold:
                try:
                    invoke(["git", "-C", root, "cat-file", "-e", task["base_commit"] + ":" + path])
                    present.append(path)
                except subprocess.CalledProcessError:
                    absent.append(path)
            record = {"id": task["id"], "base_commit": task["base_commit"],
                      "history_commits": count, "gold_at_base": present, "gold_absent_at_base": absent,
                      "v1": task["id"] in {t["id"] for t in v1}, "arms": {}}
            for rrf in (False, True):
                raw, ranked, error = b"", [], ""
                try:
                    raw = invoke([binary, "--root", root, "context", "--task", task["text"],
                                  "--subject", task["changed_file"], "--limit", "20"], environment(rrf=rrf))
                    ranked = paths(json.loads(raw))
                except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as failure:
                    # An unavailable subject is a failed invocation, never an
                    # abstention, renamed path, skipped task or invented rank.
                    # It fails this arm only; completed tasks stay recorded.
                    raw = failure.stdout or b""
                    error = (failure.stderr or b"")[-65536:].decode(errors="replace") or type(failure).__name__
                except (ValueError, KeyError, TypeError) as failure:
                    # An unreadable packet keeps its bytes and scores no rank.
                    ranked, error = [], type(failure).__name__ + ": " + str(failure)
                record["arms"]["rrf" if rrf else "corroboration"] = {
                    "ranked": ranked, "gold_buckets": buckets(ranked, gold),
                    "gold_at_base_buckets": buckets(ranked, present), "packet_bytes": len(raw),
                    "packet_sha256": hashlib.sha256(raw).hexdigest(), "error": error}
            a, b = (record["arms"][arm]["ranked"] for arm in ("corroboration", "rrf"))
            record["membership_equal"] = sorted(a) == sorted(b)
            records.append(record)
    totals = {}
    for name in ("v2", "v1"):
        selected = [r for r in records if name == "v2" or r["v1"]]
        totals[name] = {"samples": len(selected)}
        for arm in ("corroboration", "rrf"):
            totals[name][arm] = {k: sum(r["arms"][arm]["gold_buckets"][k] for r in selected)
                                 for k in ("1-5", "6-10", "11-20", "top20", "absent")}
            totals[name][arm]["errors"] = sum(bool(r["arms"][arm]["error"]) for r in selected)
    return {"state": "MEASURED", "tasks_sha256": digest(tasks), "subset_sha256": digest(subset),
            "membership_failures": sum(not r["membership_equal"] for r in records),
            "totals": totals, "records": records}


CAPTURE_PROFILE = "corvint-retrieval-context-capture-v0"


def captures(path, registration):
    """Score only a sealed capture bound to its report (RBD-V0-003..007).

    The header must carry the report's exact registration (binary, engine,
    arm environment and samples) and ordered invocation identities; every
    record must match them in order with COMPLETE stdout; and the footer must
    be COMPLETE with the true count, byte total and prior-bytes digest.
    """
    data = path.read_bytes()
    lines = data.splitlines(keepends=True)
    if len(lines) < 2 or not data.endswith(b"\n"):
        raise ValueError("capture lacks header or footer")
    header, footer = json.loads(lines[0]), json.loads(lines[-1])
    if header.get("type") != "header" or header.get("profile") != CAPTURE_PROFILE:
        raise ValueError("capture header missing or wrong profile")
    if header.get("registration") != registration:
        raise ValueError("capture registration differs from its report")
    expected = header.get("invocations")
    if not isinstance(expected, list) or [d.get("sample_id") for d in expected] != registration.get("sample_ids") \
            or any(d.get("phase") != "ordinary" for d in expected):
        raise ValueError("capture invocations differ from registered samples")
    prior = data[:len(data) - len(lines[-1])]
    if footer.get("type") != "footer" or footer.get("status") != "COMPLETE" \
            or footer.get("invocations") != len(expected) or len(lines) != len(expected) + 2 \
            or footer.get("total_bytes") != len(data) \
            or footer.get("prior_sha256") != hashlib.sha256(prior).hexdigest():
        raise ValueError("capture footer is not a complete seal")
    found = {}
    for descriptor, line in zip(expected, lines[1:-1]):
        record = json.loads(line)
        if record.get("type") != "invocation" or record.get("invocation") != descriptor:
            raise ValueError("capture invocation identity mismatch")
        if record["stdout"].get("status") != "COMPLETE":
            raise ValueError("capture stdout not complete: " + descriptor["sample_id"])
        raw = base64.b64decode(record["stdout"]["base64"], validate=True)
        if hashlib.sha256(raw).hexdigest() != record["stdout"]["sha256"] or len(raw) != record["stdout"]["bytes"]:
            raise ValueError("capture stdout digest mismatch")
        key = descriptor["sample_id"]
        if key in found:
            raise ValueError("duplicate capture sample: " + key)
        found[key] = (raw, json.loads(raw))
    return found


def spanscore(packet, gold):
    """Distinct gold lines covered, without materializing arbitrary line sets."""
    wanted = {}
    for row in gold:
        path = row.get("path", row.get("file"))
        start, end = row.get("start_line", row.get("start")), row.get("end_line", row.get("end"))
        if path and isinstance(start, int) and isinstance(end, int) and 1 <= start <= end <= 1000000:
            wanted.setdefault(path, []).append((start, end))
    total = shared = 0
    for path, intervals in wanted.items():
        # Gold ranges in release rows are small; interval iteration is bounded
        # by the admitted million-line source ceiling, independently per path.
        lines = {n for start, end in intervals for n in range(start, end + 1)}
        total += len(lines)
        rows = [r for r in packet.get("spans", {}).get("rows", []) if r["path"] == path]
        shared += sum(any(r["start_line"] <= n <= r["end_line"] for r in rows) for n in lines)
    return shared, total


def fixed_budget(raw, packet, gold, gold_spans):
    # Whole-message admission: a result is delivered only if the ENTIRE
    # actual CLI response fits. Never truncate JSON or ignore envelope bytes.
    # These offline measurements do not add a product budget flag.
    shared, total = spanscore(packet, gold_spans)
    hits = len(set(paths(packet)) & set(gold))
    return {str(b): {"packet_bytes": len(raw) if len(raw) <= b else 0,
                     "file_hits": hits if len(raw) <= b else 0, "file_gold": len(set(gold)),
                     "span_hits": shared if len(raw) <= b else 0, "span_gold": total,
                     "delivered": len(raw) <= b} for b in BUDGETS}


def retrieval_reading(binary, bench, data, output, reuse=False):
    missing = [str(data / "benchmark" / ("v2_" + r) / (r + ".jsonl")) for r in RELEASES
               if not (data / "benchmark" / ("v2_" + r) / (r + ".jsonl")).is_file()]
    if len(missing) == len(RELEASES):
        return {"state": "NOT_RUN", "reason": "frozen retrieval corpus unavailable",
                "missing_samples": missing, "samples_scored": 0}
    readings = {}
    for release in RELEASES:
        samples = data / "benchmark" / ("v2_" + release) / (release + ".jsonl")
        if not samples.is_file():
            continue
        corpus = data / "corpus" / ("v2_" + release)
        sample_rows = {r["id"]: r for r in map(json.loads, samples.read_text().splitlines())}
        readings[release] = {"samples_sha256": digest(samples), "arms": {}, "runner_metrics": {}}
        for mode in ("corroboration", "on", "hits", "merged", "rrf"):
            name = release + "-" + mode
            report, capture = output / (name + ".json"), output / (name + ".jsonl")
            argv = [bench, "--samples", samples, "--corpus", corpus, "--corvint", binary,
                    "--arms", "context", "--limit", "20", "--jobs", "1",
                    "--context-packets", capture, "--output", report]
            if not reuse:
                invoke(argv, environment("" if mode in ("rrf", "corroboration") else mode, mode == "rrf"))
            parsed = json.loads(report.read_text())
            if parsed["samples_sha256"] != digest(samples) or parsed["corvint"]["sha256"] != digest(binary) or parsed["limit"] != 20:
                raise ValueError("report input identity mismatch: " + name)
            expected_env = ["CORVINT_CONTEXT_RRF=" + ("on" if mode == "rrf" else ""),
                            "CORVINT_CONTEXT_SPANS=" + ("" if mode in ("rrf", "corroboration") else mode)]
            registration = parsed["registration"]
            if registration["bench_sha256"] != digest(bench) or sorted(registration["environment"]) != sorted(expected_env) \
                    or registration.get("corvint_sha256") != digest(binary) or registration.get("limit") != 20:
                raise ValueError("report engine/arm identity mismatch: " + name)
            readings[release]["runner_metrics"][mode] = parsed["arms"]
            # Keep the runner's entire scored report and registration beside
            # this derived reading; no metric replaces an existing gate.
            detail = {}
            for sample in parsed["details"]:
                detail[sample["id"]] = sample
            packets = captures(capture, parsed["registration"])
            if set(packets) != set(detail):
                raise ValueError("capture/scorer sample mismatch: " + name)
            per_sample = {}
            for key, (raw, packet) in packets.items():
                per_sample[key] = {"bytes": len(raw), "ranked": paths(packet),
                                   "budget": fixed_budget(raw, packet, detail[key]["gold"],
                                                          sample_rows[key].get("gold_spans", sample_rows[key]["gold"].get("gold_spans", [])))}
            readings[release]["arms"][mode] = per_sample
        arms = readings[release]["arms"]
        if any(set(arm) != set(arms["on"]) for arm in arms.values()):
            raise ValueError("paired arms scored different samples")
        readings[release]["comparisons"] = {}
        for base, candidate in (("on", "hits"), ("hits", "merged"), ("corroboration", "rrf")):
            growth = sum(arms[candidate][k]["bytes"] > arms[base][k]["bytes"] for k in arms[base])
            losses = {str(b): {metric: sum(arms[candidate][k]["budget"][str(b)][metric] <
                                             arms[base][k]["budget"][str(b)][metric] for k in arms[base])
                               for metric in ("file_hits", "span_hits")} for b in BUDGETS}
            readings[release]["comparisons"][base + "-vs-" + candidate] = {
                "packet_growth_samples": growth, "losses_at_budget": losses}
    return {"state": "PARTIAL_MEASURED" if missing else "MEASURED", "budgets": BUDGETS,
            "missing_samples": missing, "budget_policy": "whole actual CLI message or abstain",
            "readings": readings}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corvint", type=Path, required=True, help="built lane binary")
    parser.add_argument("--bench", type=Path, required=True, help="built tools/retrieval-bench")
    parser.add_argument("--data-root", type=Path, required=True, help="frozen Agent Retrieval Bench v2 data root")
    parser.add_argument("--history", type=Path, required=True, help="Git repository holding decision-0027 bases")
    parser.add_argument("--output", type=Path, required=True, help="new evidence directory, outside corpus/history")
    parser.add_argument("--only", choices=("history", "retrieval", "all"), default="all",
                        help="run just one independently available frozen corpus")
    parser.add_argument("--reuse-retrieval", action="store_true",
                        help="rescore existing captured reports after checking binary/sample identities")
    args = parser.parse_args()
    args.corvint, args.bench, args.history = [p.resolve() for p in (args.corvint, args.bench, args.history)]
    destination = args.output.resolve()
    for source in (args.data_root.resolve(), args.history):
        if destination.is_relative_to(source):
            parser.error("--output must be outside corpus and history inputs")
    args.output.mkdir(parents=True, exist_ok=args.reuse_retrieval)
    root = Path(__file__).resolve().parents[1]
    result = {"profile": "ctx-w58-paired-retrieval/0", "corvint_sha256": digest(args.corvint),
              "bench_sha256": digest(args.bench), "runner_sha256": digest(__file__),
              "promotion": "KEEP_OPT_IN", "independent_outcome": "NOT_OBSERVED"}
    for name, run in (
        ("history", lambda: history_reading(args.corvint, args.history,
                                          root / "tools/cw-trial/testdata/unseen-corvint-v2/tasks.json",
                                          root / "tools/cw-trial/testdata/unseen-corvint-v1/tasks.json")),
        ("retrieval", lambda: retrieval_reading(args.corvint, args.bench, args.data_root, args.output, args.reuse_retrieval))):
        if args.only not in ("all", name):
            result[name] = {"state": "NOT_RUN", "reason": "not selected"}
            continue
        try:
            result[name] = run()
        except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
            result[name] = {"state": "NOT_RUN", "reason": str(error)}
            if isinstance(error, subprocess.CalledProcessError):
                result[name]["stderr"] = error.stderr[-65536:].decode(errors="replace") if error.stderr else ""
    (args.output / "reading.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({k: result[k]["state"] for k in ("history", "retrieval")}))
    selected = ("history", "retrieval") if args.only == "all" else (args.only,)
    return 0 if all(result[k]["state"] == "MEASURED" for k in selected) else 2


if __name__ == "__main__":
    raise SystemExit(main())
