#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Read-only developer alarm for missing forward rows (CIN-V0-2140..2142).

Usage: python3 tools/ci-forward-row/absence-alarm.py OWNER/REPO EPOCH
Requires gh authentication with actions:read and contents:read. Checks the latest
20 eligible completed PR CI runs among at most 100 completed PR runs. Allows
one hour for the asynchronous collector. Prints JSON and a GitHub error annotation
on alarm; exits 1 on absence, 2 on unavailable evidence, 0 on clear. No writes to
GitHub, repository state, or narrowing configuration. Temporary downloads only.
Example: python3 tools/ci-forward-row/absence-alarm.py beamfall/corvint 2
"""

import base64
import datetime as dt
import json
import pathlib
import re
import subprocess
import sys
import tempfile

WINDOW = 20
THRESHOLD = 3
GRACE = dt.timedelta(hours=1)
SHA = re.compile(r"[0-9a-f]{40}\Z")
ROW = re.compile(r"ci-forward-(CLEAN|FLAKY|MISS|INCOMPLETE|INFRA|NARROWED)-pr([0-9]+)-([0-9a-f]{40})-e([0-9]+)-f[01]\Z")


class Unavailable(Exception):
    pass


def gh(*args):
    try:
        return subprocess.run(["gh", *args], check=True, stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE, timeout=60).stdout
    except (OSError, subprocess.SubprocessError) as exc:
        # Do not echo CLI stderr, which may contain credentials or downloaded bytes.
        raise Unavailable("GitHub read failed: " + args[0]) from exc


def api(repo, path):
    try:
        return json.loads(gh("api", f"repos/{repo}/{path}"))
    except (ValueError, TypeError) as exc:
        raise Unavailable("unreadable GitHub response") from exc


def timestamp(value):
    return dt.datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=dt.timezone.utc)


def trusted_rows(repo, since):
    """Bounded listing; an incomplete listing cannot establish absence."""
    rows = []
    for page in range(1, 21):
        artifacts = api(repo, f"actions/artifacts?per_page=100&page={page}")["artifacts"]
        for artifact in artifacts:
            run = artifact.get("workflow_run", {})
            if (not artifact["expired"] and run.get("head_branch") == "main"
                    and run.get("repository_id") is not None
                    and run.get("head_repository_id") == run["repository_id"]):
                match = ROW.fullmatch(artifact["name"])
                if match:
                    rows.append((match.groups(), timestamp(artifact["created_at"]),
                                 run["id"], artifact["name"]))
        # GitHub lists newest first. Older artifacts cannot cover any sampled
        # run, because a row must be created after that run completed.
        if len(artifacts) < 100 or any(timestamp(a["created_at"]) < since for a in artifacts):
            return rows
    raise Unavailable("artifact listing exceeds 20 pages")


def download(repo, run_id, pattern, directory, filename):
    gh("run", "download", str(run_id), "--repo", repo, "--pattern", pattern,
       "--dir", str(directory))
    paths = list(directory.rglob(filename))
    if not paths or len(paths) > 32:
        raise Unavailable("missing or excessive downloaded documents")
    docs = []
    for path in paths:
        if path.is_symlink() or path.stat().st_size > 4 * 1024 * 1024:
            raise Unavailable("unbounded downloaded document")
        doc = json.loads(path.read_text())
        if not isinstance(doc, dict):
            raise Unavailable("downloaded document is not an object")
        docs.append(doc)
    return docs


def candidate(repo, run, scratch):
    """Return (PR, tested commit, epoch), or None for a run outside the alarm.

    Eligibility uses outcome artifacts, never the shadow selection. Narrowed PR
    runs are included: a shadow outage must still alarm while narrowing is on. GitHub
    proves the target's parents and ancestry before its workflow supplies epoch.
    Only PR runs with outcomes are covered; queue and push runs remain outside this alarm.
    """
    prs = run["pull_requests"]
    if (run["status"] != "completed" or run["event"] != "pull_request"
            or len(prs) != 1 or prs[0]["base"]["ref"] != "main"):
        return None
    pr = prs[0]
    # Forks and PRs with an unprovable repository binding are outside this alarm.
    if pr["base"]["repo"]["id"] != pr["head"]["repo"]["id"]:
        return None
    artifacts = api(repo, f'actions/runs/{run["id"]}/artifacts?per_page=100')["artifacts"]
    if len(artifacts) >= 100:
        raise Unavailable("run artifact listing truncated")
    names = [a["name"] for a in artifacts if not a["expired"]]
    if not any(n.startswith("ci-outcomes-pull_request-") for n in names):
        return None  # DOCS, REUSE, skipped or infrastructure-only run.
    run_dir = scratch / str(run["id"])
    docs = download(repo, run["id"], "ci-outcomes-*", run_dir / "outcomes", "outcomes.json")
    targets = {d.get("target") for d in docs}
    if (len(targets) != 1 or any(d.get("profile") != "corvint-ci-outcomes/0"
            or d.get("event") != "pull_request" or d.get("runId") != run["id"]
            or d.get("runAttempt") != run["run_attempt"] or d.get("head") != run["head_sha"]
            for d in docs)):
        raise Unavailable("outcome run binding is unprovable")
    target = targets.pop()
    if not isinstance(target, str) or not SHA.fullmatch(target):
        raise Unavailable("invalid outcome target")
    parents = api(repo, f"commits/{target}")["parents"]
    if len(parents) != 2 or parents[1]["sha"] != run["head_sha"]:
        raise Unavailable("tested merge parents mismatch")
    base = parents[0]["sha"]
    for left, right in ((pr["base"]["sha"], base), (base, "main")):
        if api(repo, f"compare/{left}...{right}")["status"] not in ("ahead", "identical"):
            raise Unavailable("tested base ancestry unprovable")
    for right in (run["head_sha"], target):
        change = api(repo, f"compare/{base}...{right}")
        files = change["files"]
        if len(files) >= 300:
            return None
        if any(f.get(k, "").startswith(".github/") for f in files
               for k in ("filename", "previous_filename")):
            return None
    content = api(repo, f"contents/.github/workflows/ci.yml?ref={target}")
    workflow = base64.b64decode(content["content"], validate=False).decode("utf-8")
    epochs = re.findall(r'^\s*CORVINT_PR_FORWARD_EPOCH: *[\'\"]?([0-9]+)[\'\"]?\s*(?:#.*)?$', workflow, re.M)
    if len(epochs) != 1:
        raise Unavailable("producing workflow has no single epoch")
    return pr["number"], target, epochs[0]


def evaluate(repo, epoch, now):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo) or not re.fullmatch(r"0|[1-9][0-9]*", epoch):
        raise Unavailable("expected OWNER/REPO and a non-negative epoch")
    runs = api(repo, "actions/workflows/ci.yml/runs?event=pull_request&status=completed&per_page=100")["workflow_runs"]
    # GitHub lists by creation. Order this bounded sample by completion; this
    # does not claim globally latest completion order outside the sample.
    runs.sort(key=lambda r: (timestamp(r["updated_at"]), r["id"]), reverse=True)
    since = min((timestamp(r["updated_at"]) for r in runs), default=now)
    rows = trusted_rows(repo, since)
    absent, checked = [], []
    seen = set()
    with tempfile.TemporaryDirectory(prefix="corvint-row-absence-") as tmp:
        for run in runs:
            if now - timestamp(run["updated_at"]) < GRACE:
                continue
            key = (run["id"], run["run_attempt"])
            if key in seen:
                continue
            seen.add(key)
            entry = candidate(repo, run, pathlib.Path(tmp))
            if entry is None or entry[2] != epoch:
                continue
            pr, target, _ = entry
            # The current row wire has no runAttempt. Do not pretend its body
            # distinguishes two attempts of the same CI run.
            if run["run_attempt"] != 1:
                raise Unavailable("forward row wire cannot bind a rerun attempt")
            matching = [(r, collector, name) for r, created, collector, name in rows
                        if r[1:] == (str(pr), target, epoch)
                        and created >= timestamp(run["updated_at"])]
            checked.append(run["id"])
            present = False
            if len(matching) > 32:
                raise Unavailable("too many matching forward artifacts")
            for r, collector, name in matching:
                docs = download(repo, collector, name,
                                pathlib.Path(tmp) / "rows" / str(run["id"]) / name, "row.json")
                if any(d.get("profile") == "corvint-pr-forward-row/0" and d.get("kind") == "pr"
                       and d.get("runId") == run["id"] and d.get("selectionRunId") == run["id"]
                       and d.get("outcomesRunId") == run["id"] and d.get("pr") == pr
                       and d.get("sha") == target and str(d.get("epoch")) == epoch
                       and d.get("class") == r[0] for d in docs):
                    present = True
                    break
            if not present:
                absent.append(run["id"])
            if len(checked) == WINDOW:
                break
    if len(runs) == 100 and len(checked) < WINDOW:
        raise Unavailable("run listing exceeds 100 candidates before covering window")
    return {"alarm": "TRIPPED" if len(absent) >= THRESHOLD else "clear", "epoch": epoch,
            "threshold": THRESHOLD, "window": len(checked), "runIds": checked, "absentRunIds": absent}


def main(argv):
    if argv == ["--help"]:
        print(__doc__)
        return 0
    try:
        if len(argv) != 2:
            raise Unavailable("usage: absence-alarm.py OWNER/REPO EPOCH")
        result = evaluate(*argv, dt.datetime.now(dt.timezone.utc))
    except (Unavailable, KeyError, ValueError, TypeError, AttributeError, UnicodeError, OSError) as exc:
        print(json.dumps({"alarm": "UNKNOWN", "limit": str(exc)}))
        print("::error::Forward-row absence evidence unavailable", file=sys.stderr)
        return 2
    print(json.dumps(result, sort_keys=True))
    if result["alarm"] == "TRIPPED":
        print("::error::Forward-row absence: 3 or more eligible completed CI runs have no forward row", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
