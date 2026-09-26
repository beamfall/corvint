#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Freeze the V1-0319 Beamfall roadmap-ticket-to-diff task set (git only; runs no Corvint)."""
import json, re, subprocess, sys

repo, tip, out = sys.argv[1], sys.argv[2], sys.argv[3]
N_TASKS, MIN_GOLD, MAX_GOLD = 50, 1, 40
EXCLUDED = {"LCRES-15", "COREAPI-AUDIT-0822-3"}  # observed by the 2026-09-25 panel: development


def git(*args):
    return subprocess.run(["git", "-C", repo, "-c", "core.quotepath=false", *args],
                          capture_output=True, text=True, check=True).stdout


def is_test(p):  # tools/cw-trial/generate.go isTestPath
    name, wrapped = p.rsplit("/", 1)[-1].lower(), "/" + p.lower() + "/"
    return (name.startswith("test_") or name.endswith("_test.go") or name.endswith("_test.py")
            or ".test." in name or ".spec." in name or "/test/" in wrapped or "/tests/" in wrapped)


tip = git("rev-parse", tip).strip()
checkoffs = [l.split(" ", 1) for l in git("log", "--first-parent", "--reverse", "--format=%H %s", tip).splitlines()]
checkoffs = [(h, s[len("roadmap: check off "):].strip()) for h, s in checkoffs if s.startswith("roadmap: check off ")]
topo = git("log", "--topo-order", "--reverse", "--no-merges", "--format=%H\t%s", tip).splitlines()
by_ticket = {}
for line in topo:
    h, s = line.split("\t", 1)
    tid = s.split(" ", 1)[0]
    if " " in s and not s.startswith("roadmap:"):
        by_ticket.setdefault(tid, []).append(h)

candidates, reasons = [], {}
def skip(tid, why):
    reasons[why] = reasons.get(why, 0) + 1

for check, tid in checkoffs:
    entry = re.search(r"^-- \[ \] \*\*" + re.escape(tid) + r" .*$", git("show", "--format=", "-U0", check), re.M)
    if not entry:
        skip(tid, "no-entry"); continue
    line = entry.group(0)[len("-- [ ] "):]
    primary = re.search(r"\*\*Primary Repo\*\* ([^ ·]+)", line)
    if not primary or primary.group(1) != "beamfall":
        skip(tid, "not-core"); continue
    if tid in EXCLUDED:
        skip(tid, "observed"); continue
    commits = by_ticket.get(tid, [])
    if not commits:
        skip(tid, "no-resolving-commit"); continue
    base = git("rev-parse", commits[0] + "^").strip()
    if any(subprocess.run(["git", "-C", repo, "merge-base", "--is-ancestor", base, c]).returncode for c in commits):
        skip(tid, "base-not-ancestor"); continue
    gold = set()
    for c in commits:
        for row in git("diff-tree", "--no-commit-id", "-r", "--numstat", c).splitlines():
            added, _, path = row.split("\t", 2)
            if added != "-" and not path.startswith("docs/plans/roadmap/"):
                gold.add(path)
    at_base = set(git("ls-tree", "-r", "--name-only", base).splitlines())
    gold = sorted(p for p in gold if p in at_base)
    if not (MIN_GOLD <= len(gold) <= MAX_GOLD):
        skip(tid, "gold-out-of-bounds"); continue
    kinds = {}
    for p in gold:
        kinds.setdefault("test-file" if is_test(p) else "source-file", []).append(p)
    candidates.append({
        "id": "roadmap:" + tid, "kind": "retrieval", "repo": "Beamfall/core", "base_commit": base,
        "text": "The roadmap ticket below was resolved in this repository. Which existing file(s) must change to resolve it?\n\nTicket:\n"
                + line.strip() + "\n\nReport each source path as a claim of kind source-file and each test path as a claim of kind test-file.",
        "gold": kinds,
        "source": {"ticket": tid, "checkoff_commit": check, "resolving_commits": commits},
    })

n = len(candidates)
chosen = [candidates[(i * n) // N_TASKS] for i in range(min(N_TASKS, n))]
doc = {
    "name": "beamfall-roadmap-v1", "partition": "unseen", "repository": "Beamfall/core", "tip": tip,
    "source": "tools/cw-trial/testdata/beamfall-roadmap-v1/build_roadmap_set.py over Beamfall/core first-parent history at tip (V1-0319)",
    "population": n, "excluded": dict(sorted(reasons.items())),
    "sampling_rule": "first-parent 'roadmap: check off ID' commits oldest first whose checked-off entry names Primary Repo beamfall; "
                     "resolving commits = non-merge commits whose subject starts with 'ID ' (not 'roadmap:'); base = first parent of the "
                     "earliest in topological order, an ancestor of every resolving commit; gold = non-binary paths those commits touched that "
                     "exist at base, outside docs/plans/roadmap/, 1..40 paths, kind by cw-trial isTestPath; text = the checked-off entry's "
                     "one checkbox line as written before check-off (its sub-bullets, such as Allowed Paths, are withheld); LCRES-15 and "
                     "COREAPI-AUDIT-0822-3 excluded as panel-observed; select index floor(i*N/n) for i in 0..n-1",
    "gold_paths": sum(len(v) for t in chosen for v in t["gold"].values()),
    "tasks": chosen,
}
with open(out, "w") as f:
    json.dump(doc, f, indent=2, ensure_ascii=False)
    f.write("\n")
print(n, len(chosen), reasons, file=sys.stderr)
