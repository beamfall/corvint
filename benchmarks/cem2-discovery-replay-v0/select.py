# SPDX-License-Identifier: AGPL-3.0-or-later
"""V1-2600 change-set selector (VRP-V0-010..019): reproduce selection-manifest.json from Git alone.

Usage: python3 -I select.py REPO REF [--repository-label LABEL] > selection-manifest.json

Reads only Git history (log, diff, blame); runs no test, no Corvint command and reads no replay
result. The rule it implements is frozen verbatim in the manifest's selectionRule and in PROTOCOL.md.
Deterministic: the same REPO at the same REF yields byte-identical output.
"""
import json, re, subprocess, sys

ID = re.compile(r"\b(?:V1-\d{4}|[A-Z][A-Z0-9]{1,7}-V\d+-\d{3})\b")
FIX = re.compile(r"^fix(\([^)]*\))?!?:")
HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")
DIFF = ["-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "-U0",
        "--diff-algorithm=myers", "--no-indent-heuristic", "--no-renames"]
TARGET, LOW, HIGH = 40, 30, 50
MAX_SOURCE_HUNKS = 60
RULE = (
    "Population: non-merge commits F reachable from REF, excluding the root snapshot, whose subject "
    "matches ^fix(\\(scope\\))?!?: and names at least one ticket or requirement ID "
    "(V1-NNNN or PREFIX-VN-NNN). Touched lines: the old-side lines of F's -U0 myers diff over non-test "
    ".go files that are code (not blank, not starting with //, /* or *); a pure insertion of at least "
    "one code line touches the old-side lines immediately before and after it; a hunk whose removed "
    "and added lines are equal once every ID is masked (an ID renumbering) touches nothing. Each "
    "touched line is blamed at F^ (git blame --porcelain, no -M/-C); blame to the root snapshot is "
    "dropped. Linked change P: among blamed commits that share at least one ID with F's subject, the "
    "one owning the most touched lines (ties: earlier committer date, then SHA). P qualifies when it "
    "has between 1 and 60 non-test .go source hunks in its -U0 myers diff against its first parent. "
    "One row per P: the earliest qualifying F is the fix, later ones are listed. Rows are ordered by "
    "P committer date then SHA; when more than 50 qualify, 40 are taken at index floor(i*n/40). "
    "touchedHunks names P's source hunks (changeHunk: 1-based over P's non-test Go hunks in path order) containing a touched line by blame's original line number; touchedLines lists the same lines as [path, line in P] so any hunking (the CEM canonical diff included) can be mapped; laterFixes carry their own touchedHunks and touchedLines. "
    "escape is post-merge when P reached the first-parent line of REF in an earlier first-parent "
    "commit than F, else same-merge (found by review or integration before P reached main).")


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], check=True, capture_output=True, text=True,
                          errors="surrogateescape").stdout


def is_source(path):
    return path.endswith(".go") and not path.endswith("_test.go")


def hunks(repo, a, b):
    """Return {path: [(oldStart, oldCount, newStart, newCount)]} for non-test .go files."""
    out, path, rows = git(repo, *DIFF, a, b, "--", "*.go"), None, {}
    for line in out.splitlines():
        if line.startswith("+++ "):
            path = line[6:] if line.startswith("+++ b/") else None
        elif line.startswith("--- ") and path is None:
            pass
        elif line.startswith("diff --git"):
            path = None
        m = HUNK.match(line)
        if m and path is not None and is_source(path):
            os_, oc = int(m[1]), 1 if m[2] is None else int(m[2])
            ns, nc = int(m[3]), 1 if m[4] is None else int(m[4])
            rows.setdefault(path, []).append((os_, oc, ns, nc))
    return rows


def code(text):
    t = text.strip()
    return bool(t) and not t.startswith(("//", "/*", "*"))


def touched_lines(repo, f):
    """Old-side code lines F's non-test Go hunks touch, skipping comment-only and ID-only rewrites."""
    out, path, lines, current = git(repo, *DIFF, f + "^", f, "--", "*.go"), None, {}, None

    def flush():
        if current is None or path is None or not is_source(path):
            return
        os_, oc, removed, added = current
        if sorted(ID.sub("ID", r) for r in removed) == sorted(ID.sub("ID", a) for a in added):
            return  # pure requirement/ticket ID renumbering
        picked = lines.setdefault(path, set())
        if oc > 0:
            picked.update(os_ + i for i, r in enumerate(removed) if code(r))
        elif any(code(a) for a in added):  # pure insertion after old line os_
            picked.update(x for x in (os_, os_ + 1) if x >= 1)

    for line in out.splitlines():
        if line.startswith("diff --git"):
            flush(); current, path = None, None
        elif line.startswith("+++ "):
            path = line[6:] if line.startswith("+++ b/") else None
        elif HUNK.match(line):
            flush()
            m = HUNK.match(line)
            current = (int(m[1]), 1 if m[2] is None else int(m[2]), [], [])
        elif current is not None and line.startswith("-"):
            current[2].append(line[1:])
        elif current is not None and line.startswith("+"):
            current[3].append(line[1:])
    flush()
    return {p: sorted(v) for p, v in lines.items() if v}


def blame(repo, f, path, wanted):
    """Map touched old-side lines at F^ to (commit, original path, original line)."""
    try:
        total = int(git(repo, "show", f"{f}^:{path}").count("\n"))
    except subprocess.CalledProcessError:
        return []
    args, owners = [], []
    for x in wanted:
        if 1 <= x <= total:
            args += ["-L", f"{x},{x}"]
    if not args:
        return []
    out = git(repo, "blame", "--porcelain", *args, f + "^", "--", path)
    meta, current = {}, None
    for line in out.splitlines():
        head = line.split(" ")
        if len(head) >= 3 and re.fullmatch(r"[0-9a-f]{40}", head[0]):
            current = [head[0], None, int(head[1])]
            owners.append(current)
            if head[0] in meta:
                current[1] = meta[head[0]]
        elif line.startswith("filename ") and current is not None:
            meta[current[0]] = line[9:]
            current[1] = line[9:]
    return [tuple(o) for o in owners]


def touched_set(owned):
    """Touched lines as [path, line] in P's new-side numbering, sorted and unique."""
    return [list(x) for x in sorted(set(owned))]


def touched_hunks(source, owned):
    """P's source hunks (1-based ordinal over its non-test Go hunks in path order) holding a touched line."""
    ordinal, found = {}, {}
    for path in sorted(source):
        for index, row in enumerate(source[path]):
            ordinal[(path, index)] = len(ordinal) + 1
    for opath, oline in set(owned):
        for index, (_, _, ns, nc) in enumerate(source.get(opath, [])):
            if nc > 0 and ns <= oline < ns + nc:
                found[(opath, index)] = {"path": opath, "fileHunk": index + 1,
                                         "changeHunk": ordinal[(opath, index)], "newStart": ns, "newCount": nc}
    return [found[k] for k in sorted(found)]


def main():
    repo, ref = sys.argv[1], sys.argv[2]
    label = sys.argv[4] if len(sys.argv) > 4 and sys.argv[3] == "--repository-label" else repo
    head = git(repo, "rev-parse", ref + "^{commit}").strip()
    root = git(repo, "rev-list", "--max-parents=0", head).split()
    log = git(repo, "log", "--no-merges", "--reverse", "--format=%H%x00%ct%x00%s%x00%b%x01", head)
    commits = {}
    for chunk in log.split("\x01"):
        chunk = chunk.lstrip("\n")
        if not chunk:
            continue
        sha, ct, subject, body = chunk.split("\x00", 3)
        commits[sha] = {"ct": int(ct), "subject": subject, "ids": set(ID.findall(subject + "\n" + body))}
    landing, merges = {}, git(repo, "log", "--first-parent", "--reverse", "--format=%H%x00%P%x00%s", head)
    for index, line in enumerate(merges.splitlines()):
        sha, parents, subject = line.split("\x00", 2)
        parents = parents.split()
        span = [sha] if len(parents) < 2 else git(repo, "rev-list", f"{parents[0]}..{sha}").split()
        for c in span:
            landing.setdefault(c, (index, sha, subject))
    population, by_change = 0, {}
    for f, info in commits.items():
        if f in root or not FIX.match(info["subject"]):
            continue
        fids = set(ID.findall(info["subject"]))
        if not fids:
            continue
        population += 1
        owners = {}
        for path, wanted in touched_lines(repo, f).items():
            for sha, opath, oline in blame(repo, f, path, wanted):
                if sha in root or sha not in commits or sha == f:
                    continue
                owners.setdefault(sha, []).append((opath, oline))
        linked = [s for s in owners if commits[s]["ids"] & fids]
        if not linked:
            continue
        p = sorted(linked, key=lambda s: (-len(owners[s]), commits[s]["ct"], s))[0]
        source = hunks(repo, p + "^", p)
        count = sum(len(v) for v in source.values())
        if not 1 <= count <= MAX_SOURCE_HUNKS:
            continue
        if p in by_change:
            by_change[p]["laterFixes"].append({"fix": f, "fixSubject": info["subject"],
                                               "touchedHunks": touched_hunks(source, owners[p]),
                                               "touchedLines": touched_set(owners[p])})
            continue
        touched = touched_hunks(source, owners[p])
        by_change[p] = {
            "change": p, "base": git(repo, "rev-parse", p + "^").strip(), "fix": f,
            "changeSubject": commits[p]["subject"], "fixSubject": info["subject"],
            "sharedIds": sorted(commits[p]["ids"] & fids),
            "touchedLinesBlamedToChange": len(owners[p]),
            "touchedLinesBlamedElsewhere": sum(len(v) for s, v in owners.items() if s != p),
            "changeSourceHunks": count,
            "changeSourceFiles": sorted(source),
            "touchedHunks": touched,
            "touchedLines": touched_set(owners[p]),
            "laterFixes": [],
            "changeLandedVia": landing[p][1], "changeLandedSubject": landing[p][2],
            "fixLandedVia": landing[f][1],
            "escape": "post-merge" if landing[p][0] < landing[f][0] else "same-merge",
            "qualifies": "fix subject names " + ", ".join(sorted(commits[p]["ids"] & fids)) +
                         f" shared with the change; {len(owners[p])} touched non-test Go line(s) blame to the change",
        }
    rows = sorted(by_change.values(), key=lambda r: (commits[r["change"]]["ct"], r["change"]))
    n = len(rows)
    chosen = rows if n <= HIGH else [rows[(i * n) // TARGET] for i in range(TARGET)]
    for i, r in enumerate(chosen):
        r["id"] = f"VRP-C{i + 1:03d}"
    doc = {
        "profile": "corvint-vrp-selection/0", "ticket": "V1-2600",
        "requirements": ["VRP-V0-010", "VRP-V0-011", "VRP-V0-012", "VRP-V0-013"],
        "repository": label, "ref": head, "rootExcluded": root, "selectionRule": RULE,
        "fixCommitsWithIds": population, "qualifying": n, "selected": len(chosen),
        "status": "SELECTED" if len(chosen) >= LOW else "BELOW_MINIMUM",
        "changes": [dict(sorted(r.items())) for r in chosen],
    }
    sys.stdout.write(json.dumps(doc, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
