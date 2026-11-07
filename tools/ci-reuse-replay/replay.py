#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Replay CI history through `corvint test reuse` and `corvint affected jobs`.

usage: python3 -I tools/ci-reuse-replay/replay.py tests DATA CORVINT WORKTREE OUT
       python3 -I tools/ci-reuse-replay/replay.py jobs  DATA CORVINT WORKTREE OUT DECLARATION

DATA is a fetch.py directory, CORVINT the binary under test, WORKTREE a
detached linked worktree of this repository the replay checks out each
historical commit in (it must hold no work), OUT a directory for the records,
decisions, plans and summary.json. Existing output files are removed before
each recomputation and never read; only records computed during this
invocation may be shared between pairs.

tests (AFP-V0-2333): for each merged pull request with a re-push, every pull
request run with first-run outcomes is decided against a record of the same
pull request's previous run and a record of the newest successful merge-queue
run on main before it, each made by `corvint test reuse record` at that run's
own tested merge commit. A violation is a package decided REUSE whose outcome
in the later run was fail. Savings use the committed cost table.

jobs (AFP-V0-2346): for the last 100 completed pull_request and push runs, the
job plan of the run's change is compared with the conclusions of the declared
jobs in that run. A violation is a SKIP of a job that failed.
"""
import json
import os
import subprocess
import sys
from collections import defaultdict

ARGV = "-race -count=1 -timeout 50m"
HARNESS = [".github/cishards/test-commands.json", ".github/testconfine", ".github/workflows/ci.yml"]
ENV = dict(os.environ, GOFLAGS="", GOTOOLCHAIN="local", GOWORK="off", GOPROXY="off")


def load(path):
    with open(path) as handle:
        return json.load(handle)


def discard(path):
    """Remove an earlier invocation's output so it can never stand for this one's."""
    try:
        os.remove(path)
    except FileNotFoundError:
        pass


def git(worktree, *arguments, check=True):
    return subprocess.run(["git", "-C", worktree, *arguments], check=check, capture_output=True, text=True, env=ENV)


def checkout(worktree, commit):
    git(worktree, "checkout", "-q", "--force", "--detach", commit)
    git(worktree, "clean", "-q", "-fdx")


def fetch(worktree, commits):
    missing = [c for c in sorted(set(commits)) if git(worktree, "cat-file", "-e", c + "^{commit}", check=False).returncode != 0]
    for start in range(0, len(missing), 40):
        git(worktree, "fetch", "-q", "--no-tags", "origin", *missing[start:start + 40], check=False)
    return [c for c in missing if git(worktree, "cat-file", "-e", c + "^{commit}", check=False).returncode != 0]


def outcomes_of(data, run_id):
    directory = os.path.join(data, "outcomes", str(run_id))
    documents = []
    if os.path.isdir(directory):
        for root, _, files in os.walk(directory):
            if "outcomes.json" in files:
                documents.append(os.path.join(root, "outcomes.json"))
    return sorted(documents)


def merged_outcomes(paths):
    merged, target = {}, None
    for path in paths:
        document = load(path)
        target = target or document.get("target")
        if document.get("target") != target:
            return None, {}
        for name, outcome in (document.get("outcomes") or {}).items():
            merged[name] = outcome if merged.get(name, outcome) == outcome else "conflict"
    return target, merged


def lpt(costs, shards=16):
    """Longest-processing-time placement: the modelled wall of the slowest shard."""
    loads = [0] * shards
    for cost in sorted(costs, reverse=True):
        loads[loads.index(min(loads))] += cost
    return max(loads) if costs else 0


def run_tests(data, corvint, worktree, out):
    runs = load(os.path.join(data, "runs-ci.yml.json"))
    prs = {p["headRefName"]: p for p in load(os.path.join(data, "prs.json"))}
    costs = load(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", ".github", "cishards", "package-costs.json"))["milliseconds"]
    median = sorted(costs.values())[len(costs) // 2]
    split = load(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", ".github", "cishards", "test-split-allow.json"))["packages"]

    def pieces(names):
        # A split package (AFP-V0-041) runs as slices spread over the shards, modelled as 16
        # equal parts; every other package is one piece.
        out = []
        for name in names:
            cost = costs.get(name, median)
            out += [cost / 16] * 16 if name in split else [cost]
        return out
    by_pr = defaultdict(list)
    queue = []
    for run in runs:
        if run.get("status") != "completed":
            continue
        documents = outcomes_of(data, run["id"])
        if not documents:
            continue
        target, outcomes = merged_outcomes(documents)
        if not target:
            continue
        entry = {"id": run["id"], "created": run["created_at"], "head": run["head_sha"], "target": target, "outcomes": outcomes,
                 "documents": documents, "conclusion": run.get("conclusion"), "attempt": run.get("run_attempt", 1)}
        if run["event"] == "merge_group" and run.get("conclusion") == "success":
            queue.append(entry)
        elif run["event"] == "pull_request":
            pr = prs.get(run["head_branch"])
            if pr and pr["state"] == "MERGED":
                entry["pr"] = pr["number"]
                by_pr[pr["number"]].append(entry)
    queue.sort(key=lambda e: e["created"])
    pairs = []
    for number, entries in sorted(by_pr.items()):
        entries.sort(key=lambda e: e["created"])
        for previous, current in zip(entries, entries[1:]):
            if previous["head"] == current["head"] or previous["target"] == current["target"]:
                continue
            main = [q for q in queue if q["created"] < current["created"]]
            pairs.append({"pr": number, "previous": previous, "current": current, "main": main[-1] if main else None})
    needed = [p["previous"]["target"] for p in pairs] + [p["current"]["target"] for p in pairs] + [p["main"]["target"] for p in pairs if p["main"]]
    unavailable = set(fetch(worktree, needed))
    discard(os.path.join(out, "summary.json"))
    os.makedirs(os.path.join(out, "records"), exist_ok=True)
    os.makedirs(os.path.join(out, "decisions"), exist_ok=True)
    records = {}

    def record(entry, kind, number=None):
        path = os.path.join(out, "records", f"{kind}-{entry['id']}.json")
        if path in records:
            return records[path]
        discard(path)
        if entry["target"] in unavailable:
            records[path] = None
            return None
        checkout(worktree, entry["target"])
        command = [corvint, "--root", worktree, "test", "reuse", "record", "--argv", ARGV, "--out", path,
                   "--source-kind", kind, "--run-id", str(entry["id"]), "--run-attempt", str(entry["attempt"])]
        for harness in HARNESS:
            command += ["--harness", harness]
        if number:
            command += ["--pull-request", str(number)]
        for document in entry["documents"]:
            command += ["--outcomes", document]
        result = subprocess.run(command, capture_output=True, text=True, env=ENV)
        if result.returncode != 0:
            print(f"record {kind} {entry['id']}: {result.stderr.strip()}", file=sys.stderr)
            records[path] = None
            return None
        records[path] = path
        return path

    summary = {"pairs": [], "violations": [], "unavailable": sorted(unavailable)}
    for pair in pairs:
        current = pair["current"]
        decision_path = os.path.join(out, "decisions", f"{current['id']}.json")
        discard(decision_path)
        sources = [s for s in (record(pair["previous"], "pull_request", pair["pr"]), pair["main"] and record(pair["main"], "main")) if s]
        row = {"pr": pair["pr"], "run": current["id"], "previousRun": pair["previous"]["id"], "mainRun": pair["main"] and pair["main"]["id"],
               "ran": len(current["outcomes"]), "failed": sorted(n for n, o in current["outcomes"].items() if o == "fail")}
        if current["target"] in unavailable or not sources:
            row["state"] = "NOT_PRODUCED"
            summary["pairs"].append(row)
            continue
        checkout(worktree, current["target"])
        packages = os.path.join(out, "decisions", f"{current['id']}.packages")
        with open(packages, "w") as handle:
            handle.write("\n".join(sorted(current["outcomes"])) + "\n")
        command = [corvint, "--root", worktree, "test", "reuse", "decide", "--argv", ARGV, "--trust", "main,pull_request",
                   "--pull-request", str(pair["pr"]), "--packages", packages, "--out", decision_path]
        for harness in HARNESS:
            command += ["--harness", harness]
        for source in sources:
            command += ["--records", source]
        result = subprocess.run(command, capture_output=True, text=True, env=ENV)
        if result.returncode != 0:
            row["state"] = "NOT_PRODUCED"
            row["error"] = result.stderr.strip()[-400:]
            summary["pairs"].append(row)
            continue
        decision = load(decision_path)
        reused = set(decision["reused"])
        reasons = defaultdict(int)
        for unit in decision["units"]:
            reasons[unit["reason"]] += 1
        violations = sorted(n for n in reused if current["outcomes"].get(n) == "fail")
        ran = list(current["outcomes"])
        row.update({"state": "DECIDED", "reused": len(reused), "reasons": dict(reasons), "violations": violations,
                    "reusedMilliseconds": sum(costs.get(n, median) for n in reused),
                    "ranMilliseconds": sum(costs.get(n, median) for n in ran),
                    "modeledWallMilliseconds": lpt(pieces(ran)),
                    "modeledReuseWallMilliseconds": lpt(pieces([n for n in ran if n not in reused]))})
        summary["pairs"].append(row)
        summary["violations"] += [{"run": current["id"], "package": n} for n in violations]
    decided = [r for r in summary["pairs"] if r["state"] == "DECIDED"]
    summary["totals"] = {
        "pullRequests": len({r["pr"] for r in summary["pairs"]}),
        "decidedPullRequests": len({r["pr"] for r in decided}),
        "pairs": len(summary["pairs"]), "decided": len(decided),
        "reusedPackages": sum(r["reused"] for r in decided), "ranPackages": sum(r["ran"] for r in decided),
        "failedPackagesInDecidedRuns": sum(len(r["failed"]) for r in decided),
        "violations": len(summary["violations"]),
        "reusedRunnerMinutes": round(sum(r["reusedMilliseconds"] for r in decided) / 60000, 1),
        "ranRunnerMinutes": round(sum(r["ranMilliseconds"] for r in decided) / 60000, 1),
        "modeledWallMinutes": round(sum(r["modeledWallMilliseconds"] for r in decided) / 60000, 1),
        "modeledReuseWallMinutes": round(sum(r["modeledReuseWallMilliseconds"] for r in decided) / 60000, 1),
    }
    with open(os.path.join(out, "summary.json"), "w") as handle:
        json.dump(summary, handle, indent=1, sort_keys=True)
    print(json.dumps(summary["totals"], indent=1, sort_keys=True))


def duration(job):
    from datetime import datetime
    if not job.get("started_at") or not job.get("completed_at"):
        return 0
    parse = lambda value: datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ")
    return (parse(job["completed_at"]) - parse(job["started_at"])).total_seconds()


def run_jobs(data, corvint, worktree, out, declaration):
    runs = [r for r in load(os.path.join(data, "runs-ci.yml.json")) if r.get("status") == "completed" and r["event"] in ("pull_request", "push")]
    runs.sort(key=lambda r: r["created_at"], reverse=True)
    runs = runs[:100]
    pushes = sorted((r for r in load(os.path.join(data, "runs-ci.yml.json")) if r["event"] == "push"), key=lambda r: r["created_at"])
    intake = defaultdict(list)
    for run in load(os.path.join(data, "runs-intake-containment.yml.json")):
        intake[(run["event"], run["head_sha"])].append(run)
    jobs_of = {}

    def conclusions(run):
        found = {}
        path = os.path.join(data, "jobs", f"{run['id']}.json")
        if os.path.exists(path):
            for job in load(path).get("jobs", []):
                if job["name"] in ("artifact-integrity", "go-interop"):
                    found[job["name"]] = (job.get("conclusion"), duration(job))
        for other in intake.get((run["event"], run["head_sha"]), []):
            path = os.path.join(data, "jobs", f"{other['id']}.json")
            if os.path.exists(path):
                for job in load(path).get("jobs", []):
                    if job["name"] == "containment":
                        found["containment"] = (job.get("conclusion"), duration(job))
        return found

    heads = [r["head_sha"] for r in runs] + [p["head_sha"] for p in pushes]
    unavailable = set(fetch(worktree, heads))
    discard(os.path.join(out, "summary.json"))
    os.makedirs(os.path.join(out, "plans"), exist_ok=True)
    summary = {"runs": [], "violations": []}
    for run in runs:
        row = {"run": run["id"], "event": run["event"], "head": run["head_sha"], "jobs": {}}
        if run["head_sha"] in unavailable:
            row["state"] = "NOT_PRODUCED"
            summary["runs"].append(row)
            continue
        if run["event"] == "push":
            base = git(worktree, "rev-parse", run["head_sha"] + "^1", check=False).stdout.strip()
        else:
            main = [p for p in pushes if p["created_at"] < run["created_at"] and p["head_sha"] not in unavailable]
            base = main and git(worktree, "merge-base", run["head_sha"], main[-1]["head_sha"], check=False).stdout.strip()
        if not base:
            row["state"] = "NOT_PRODUCED"
            summary["runs"].append(row)
            continue
        plan_path = os.path.join(out, "plans", f"{run['id']}.json")
        discard(plan_path)
        checkout(worktree, run["head_sha"])
        result = subprocess.run([corvint, "--root", worktree, "affected", "jobs", "--base", base, "--declaration", declaration],
                                capture_output=True, text=True, env=ENV)
        if result.returncode != 0:
            row["state"] = "NOT_PRODUCED"
            row["error"] = result.stderr.strip()[-400:]
            summary["runs"].append(row)
            continue
        with open(plan_path, "w") as handle:
            handle.write(result.stdout)
        plan = load(plan_path)
        row.update({"state": "PLANNED", "base": base, "changed": plan["changed"], "planScope": plan.get("planScope")})
        found = conclusions(run)
        for job in plan["jobs"]:
            conclusion, seconds = found.get(job["job"], (None, 0))
            row["jobs"][job["job"]] = {"decision": job["decision"], "reason": job["reason"], "conclusion": conclusion, "seconds": seconds}
            if job["decision"] == "SKIP" and conclusion == "failure":
                summary["violations"].append({"run": run["id"], "job": job["job"]})
        summary["runs"].append(row)
    planned = [r for r in summary["runs"] if r["state"] == "PLANNED"]
    totals = {"runs": len(summary["runs"]), "planned": len(planned), "violations": len(summary["violations"]), "jobs": {}}
    for name in ("artifact-integrity", "go-interop", "containment"):
        rows = [r["jobs"].get(name) for r in planned if name in r["jobs"]]
        observed = [j for j in rows if j["conclusion"] in ("success", "failure")]
        totals["jobs"][name] = {
            "planned": len(rows), "skip": sum(1 for j in rows if j["decision"] == "SKIP"),
            "observedRuns": len(observed), "observedFailures": sum(1 for j in observed if j["conclusion"] == "failure"),
            "skipOnObserved": sum(1 for j in observed if j["decision"] == "SKIP"),
            "skipOnFailure": sum(1 for j in observed if j["decision"] == "SKIP" and j["conclusion"] == "failure"),
            "observedRunnerMinutes": round(sum(j["seconds"] for j in observed) / 60, 1),
            "savedRunnerMinutes": round(sum(j["seconds"] for j in observed if j["decision"] == "SKIP") / 60, 1),
        }
    summary["totals"] = totals
    with open(os.path.join(out, "summary.json"), "w") as handle:
        json.dump(summary, handle, indent=1, sort_keys=True)
    print(json.dumps(totals, indent=1, sort_keys=True))


def main():
    if len(sys.argv) == 6 and sys.argv[1] == "tests":
        run_tests(*sys.argv[2:6])
    elif len(sys.argv) == 7 and sys.argv[1] == "jobs":
        run_jobs(*sys.argv[2:7])
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main()
