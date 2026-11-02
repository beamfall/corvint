#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Read-only developer alarm for missing forward rows (CIN-V0-2140..2142).

Usage: python3 tools/ci-forward-row/absence-alarm.py OWNER/REPO EPOCH
Requires gh authentication with actions:read and contents:read. Checks the latest
20 eligible completed PR CI runs among at most 100 completed PR runs. Allows
one hour for the asynchronous collector. Prints JSON and a GitHub error annotation
on alarm; exits 1 on absence, 2 on unavailable evidence, 0 on clear. No writes to
GitHub, repository state, or narrowing configuration. Artifacts are bounded before
transfer and read in memory, never extracted to disk.
Example: python3 tools/ci-forward-row/absence-alarm.py beamfall/corvint 2
"""

import base64
import datetime as dt
import io
import json
import re
import subprocess
import sys
import threading
import zipfile
import zlib

WINDOW = 20
THRESHOLD = 3
GRACE = dt.timedelta(hours=1)
GH = ("gh",)
TIMEOUT = 60
MAX_DOCS = 32
DOC_LIMIT = 4 * 1024 * 1024
ZIP_OVERHEAD = 64 * 1024
# ci-control-plane.yml forward-rows collects only these CI conclusions.
COLLECTED = ("success", "failure")
ROW_PROFILE = "corvint-pr-forward-row/0"
SHA = re.compile(r"[0-9a-f]{40}\Z")
ROW = re.compile(r"ci-forward-(CLEAN|FLAKY|MISS|INCOMPLETE|INFRA|NARROWED)-pr([0-9]+)-([0-9a-f]{40})-e([0-9]+)-f[01]\Z")


class Unavailable(Exception):
    pass


def gh(*args, limit=None):
    """Run one read; with limit, stop the transfer once it exceeds limit bytes."""
    if limit is None:
        try:
            return subprocess.run([*GH, *args], check=True, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, timeout=TIMEOUT).stdout
        except (OSError, subprocess.SubprocessError) as exc:
            # Do not echo CLI stderr, which may contain credentials or downloaded bytes.
            raise Unavailable("GitHub read failed: " + args[0]) from exc
    try:
        proc = subprocess.Popen([*GH, *args], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except OSError as exc:
        raise Unavailable("GitHub read failed: " + args[0]) from exc
    timer = threading.Timer(TIMEOUT, proc.kill)
    timer.start()
    try:
        data = proc.stdout.read(limit + 1)
        if len(data) > limit:
            raise Unavailable("artifact transfer exceeds its limit")
        code = proc.wait()
    finally:
        timer.cancel()
        if proc.poll() is None:
            proc.kill()
        proc.stdout.close()
        proc.wait()
    if code != 0:  # Includes a transfer the timer killed.
        raise Unavailable("GitHub read failed: " + args[0])
    return data


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
                    rows.append((match.groups(), timestamp(artifact["created_at"]), artifact))
        # GitHub lists newest first. Older artifacts cannot cover any sampled
        # run, because a row must be created after that run completed.
        if len(artifacts) < 100 or any(timestamp(a["created_at"]) < since for a in artifacts):
            return rows
    raise Unavailable("artifact listing exceeds 20 pages")


def is_int(value):
    return isinstance(value, int) and not isinstance(value, bool)


def download(repo, artifacts, filename):
    """Bound count and archive size from listing metadata before any transfer."""
    if not artifacts or len(artifacts) > MAX_DOCS:
        raise Unavailable("missing or excessive matching artifacts")
    limit = DOC_LIMIT + ZIP_OVERHEAD
    for artifact in artifacts:
        size = artifact["size_in_bytes"]
        if not is_int(artifact["id"]) or not is_int(size) or not 0 < size <= limit:
            raise Unavailable("artifact exceeds its transfer limit")
    return [document(gh("api", f"repos/{repo}/actions/artifacts/{a['id']}/zip", limit=limit),
                     filename) for a in artifacts]


def document(blob, filename):
    """Read the archive's single expected entry in memory, bounded before decompression."""
    try:
        with zipfile.ZipFile(io.BytesIO(blob)) as archive:
            entries = archive.infolist()
            if len(entries) != 1 or entries[0].filename != filename or entries[0].is_dir():
                raise Unavailable("unexpected artifact archive entries")
            if entries[0].file_size > DOC_LIMIT:
                raise Unavailable("unbounded downloaded document")
            with archive.open(entries[0]) as handle:
                data = handle.read(DOC_LIMIT + 1)
    except (zipfile.BadZipFile, zipfile.LargeZipFile, RuntimeError, NotImplementedError,
            EOFError, OSError, ValueError, zlib.error) as exc:
        raise Unavailable("unreadable artifact archive") from exc
    if len(data) > DOC_LIMIT:
        raise Unavailable("unbounded downloaded document")
    try:
        doc = json.loads(data)
    except ValueError as exc:
        raise Unavailable("downloaded document is not JSON") from exc
    if not isinstance(doc, dict):
        raise Unavailable("downloaded document is not an object")
    return doc


def row_binds(doc, groups, run_id):
    """Validate a row body against its own name, then report whether it binds run_id.

    A malformed body, or one contradicting its artifact name, is uncertainty; a well-formed
    row from another CI run is not this run's row.
    """
    kind, pr, sha, epoch = groups
    ids = ("runId", "selectionRunId", "outcomesRunId")
    if (doc.get("profile") != ROW_PROFILE or doc.get("kind") != "pr"
            or not all(is_int(doc.get(k)) for k in ("pr", "epoch", *ids))
            or (doc.get("class"), doc["pr"], doc.get("sha"), doc["epoch"]) != (kind, int(pr), sha, int(epoch))):
        raise Unavailable("forward row body is malformed or contradicts its name")
    return all(doc[k] == run_id for k in ids)


def candidate(repo, run):
    """Return (PR, tested commit, epoch), or None for a run outside the alarm.

    Eligibility uses outcome artifacts, never the shadow selection. Narrowed PR
    runs are included: a shadow outage must still alarm while narrowing is on. GitHub
    proves the target's parents and ancestry before its workflow supplies epoch.
    Only PR runs with outcomes are covered; queue and push runs remain outside this alarm.
    A cancelled or timed-out run may retain outcomes, but the collector never runs for it.
    """
    prs = run["pull_requests"]
    if (run["status"] != "completed" or run.get("conclusion") not in COLLECTED
            or run["event"] != "pull_request" or len(prs) != 1 or prs[0]["base"]["ref"] != "main"):
        return None
    pr = prs[0]
    # Forks and PRs with an unprovable repository binding are outside this alarm.
    if pr["base"]["repo"]["id"] != pr["head"]["repo"]["id"]:
        return None
    artifacts = api(repo, f'actions/runs/{run["id"]}/artifacts?per_page=100')["artifacts"]
    if len(artifacts) >= 100:
        raise Unavailable("run artifact listing truncated")
    live = [a for a in artifacts if not a["expired"]]
    if not any(a["name"].startswith("ci-outcomes-pull_request-") for a in live):
        return None  # DOCS, REUSE, skipped or infrastructure-only run.
    docs = download(repo, [a for a in live if a["name"].startswith("ci-outcomes-")], "outcomes.json")
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
    for run in runs:
        if now - timestamp(run["updated_at"]) < GRACE:
            continue
        key = (run["id"], run["run_attempt"])
        if key in seen:
            continue
        seen.add(key)
        entry = candidate(repo, run)
        if entry is None or entry[2] != epoch:
            continue
        pr, target, _ = entry
        # The current row wire has no runAttempt. Do not pretend its body
        # distinguishes two attempts of the same CI run.
        if run["run_attempt"] != 1:
            raise Unavailable("forward row wire cannot bind a rerun attempt")
        matching = [(r, artifact) for r, created, artifact in rows
                    if r[1:] == (str(pr), target, epoch)
                    and created >= timestamp(run["updated_at"])]
        checked.append(run["id"])
        present = False
        if matching:
            docs = download(repo, [artifact for _, artifact in matching], "row.json")
            # Validate every body before any binding counts.
            bound = [row_binds(doc, r, run["id"]) for (r, _), doc in zip(matching, docs)]
            present = any(bound)
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
