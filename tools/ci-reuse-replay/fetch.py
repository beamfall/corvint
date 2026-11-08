#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Fetch the CI history the AFP-V0-2333 and AFP-V0-2346 replays read.

usage: python3 -I tools/ci-reuse-replay/fetch.py OUT_DIR [REPOSITORY]

Writes OUT_DIR/runs-<workflow>.json (every run of ci.yml and
intake-containment.yml), OUT_DIR/prs.json, OUT_DIR/jobs/<run>.json (the jobs of
each run) and OUT_DIR/outcomes/<run>/<artifact>/outcomes.json (every
ci-outcomes-* artifact of each pull_request and merge_group run). It reads
only through the GitHub CLI (gh), read-only, and skips what it already holds,
so a second run resumes. Downloaded artifacts are untrusted data: the replay
reads them with json only.
"""
import json
import os
import shutil
import subprocess
import sys


def gh(*arguments):
    return subprocess.run(["gh", *arguments], check=True, capture_output=True, text=True).stdout


def runs(repository, workflow):
    collected, page = [], 1
    while True:
        body = json.loads(gh("api", f"repos/{repository}/actions/workflows/{workflow}/runs?per_page=100&page={page}"))
        batch = body.get("workflow_runs", [])
        collected.extend(batch)
        if len(batch) < 100:
            return collected
        page += 1


def main():
    if len(sys.argv) not in (2, 3):
        sys.exit(__doc__)
    out = sys.argv[1]
    repository = sys.argv[2] if len(sys.argv) == 3 else "beamfall/corvint"
    os.makedirs(os.path.join(out, "jobs"), exist_ok=True)
    os.makedirs(os.path.join(out, "outcomes"), exist_ok=True)
    for workflow in ("ci.yml", "intake-containment.yml"):
        listing = os.path.join(out, f"runs-{workflow}.json")
        if not os.path.exists(listing):
            with open(listing, "w") as handle:
                json.dump(runs(repository, workflow), handle)
    with open(os.path.join(out, "prs.json"), "w") as handle:
        handle.write(gh("pr", "list", "-R", repository, "--state", "all", "--limit", "1000",
                        "--json", "number,headRefName,state,mergedAt,createdAt,baseRefName"))
    for workflow in ("ci.yml", "intake-containment.yml"):
        with open(os.path.join(out, f"runs-{workflow}.json")) as handle:
            listed = json.load(handle)
        for run in listed:
            if run.get("status") != "completed":
                continue
            run_id = str(run["id"])
            jobs = os.path.join(out, "jobs", run_id + ".json")
            if not os.path.exists(jobs) or os.path.getsize(jobs) == 0:
                try:
                    body = gh("api", f"repos/{repository}/actions/runs/{run_id}/jobs?filter=latest&per_page=100")
                except subprocess.CalledProcessError as error:
                    print(f"jobs of run {run_id}: {error.stderr.strip()}", file=sys.stderr)
                    body = None
                if body is not None:
                    with open(jobs + ".tmp", "w") as handle:
                        handle.write(body)
                    os.replace(jobs + ".tmp", jobs)
            if workflow != "ci.yml" or run["event"] not in ("pull_request", "merge_group"):
                continue
            target = os.path.join(out, "outcomes", run_id)
            done = os.path.join(target, ".done")
            if os.path.exists(done):
                continue
            try:
                artifacts = json.loads(gh("api", f"repos/{repository}/actions/runs/{run_id}/artifacts?per_page=100"))
            except subprocess.CalledProcessError as error:
                print(f"artifacts of run {run_id}: {error.stderr.strip()}", file=sys.stderr)
                continue
            names = [a["name"] for a in artifacts.get("artifacts", []) if a["name"].startswith("ci-outcomes-") and not a.get("expired")]
            # A directory an interrupted fetch left is complete only when it holds every artifact.
            held = sum(1 for name in names if os.path.exists(os.path.join(target, name, "outcomes.json")))
            if names and held != len(names):
                shutil.rmtree(target, ignore_errors=True)
                os.makedirs(target, exist_ok=True)
                subprocess.run(["gh", "run", "download", run_id, "-R", repository, "-p", "ci-outcomes-*", "-D", target],
                               check=False, capture_output=True)
                held = sum(1 for name in names if os.path.exists(os.path.join(target, name, "outcomes.json")))
            os.makedirs(target, exist_ok=True)
            if held == len(names):
                open(done, "w").close()


if __name__ == "__main__":
    main()
