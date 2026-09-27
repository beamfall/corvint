#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Untouched-repository sealed benchmark V1 on go-chi/chi, Corvint and beamfall/core (V1-0019, PRS-V1-008).

`--benchmark-dir` selects the directory that holds one repository's
preregistration, seal, corpus, runs and receipts (default: this directory,
go-chi/chi); every repository runs these exact harness bytes. `corpus` derives
the immutable corpus from the most recent first-parent commits ending at the
preregistered pin; a preregistration may restrict them to a pathspec
(corpusPathspec) and drop paths that are never critical (pathExclusions).
`run` verifies the preregistration against preregistration.sha256, re-derives
the corpus from the supplied clone, extracts the verified release candidate
binary, writes a started-run marker, runs every treatment and baseline, scores
them, and writes one result plus three corvint-use-case-evidence/0 receipts. It is adapted from
benchmarks/daily-loop-v0/harness.py and preregistration.json lists every
difference. Nothing here estimates a value it cannot observe: live-agent
dimensions stay NOT_OBSERVED unless an observation file is supplied.

Consequence scoring rule (daily-loop amendment 1, V1-0202): the treatment
covers a critical package when `plan.selected` names a `go:` unit of that
package, or when `plan.unknown` carries a `NO_SELECTABLE_TEST` entry whose
detail names it (AFP-V0-020). A changed package with no selectable test is then
named for verification rather than silently omitted; it is still not selected.
"""
import argparse
import hashlib
import json
import math
import os
import platform
import re
import shutil
import statistics
import subprocess
import sys
import tarfile
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
WINDOW = 20
BIND_SUBJECT = re.compile(r"^chore: (bind|seal|rebind)\b")
CONVENTIONAL = re.compile(r"^[a-z]+(\([^)]*\))?!?: ")
PULL_SUFFIX = re.compile(r" \(#\d+\)$")
STOPWORDS = {"the", "and", "for", "with", "from", "into", "onto", "that", "this", "their", "them"}
ORIENTATION_MAX_FILES = 100
PACKET_LIMIT = 20
USE_CASES = {"orientation": "UC-TASK-ORIENTATION", "consequence": "UC-CHANGE-CONSEQUENCE",
             "completion": "UC-EVIDENCE-CARRYING-COMPLETION"}
CITATION = "README.md\t1:3\tspecification"
INTENTS = "#no-intent-declared\n"
VERIFY = "go test -count=1 ./...\n"
PASS_LINE = re.compile(rb"^dogfood-check: PASS$", re.M)
# The adopter ignore entries docs/DOGFOOD.md gives under DCW-V0-023; the subverbs write no ignore rules.
ADOPTER_EXCLUDES = (".corvint/dogfood-report.json\n.corvint/change.ocm-intents\n.corvint/change.ocm-status.json\n"
                    ".corvint/change.ocm.*.json\n.corvint/self-observations.jsonl\n.context-corvint/\n")
VERSION_LINE = re.compile(r"^Corvint (\S+) ")
GOARCH = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}
RUN_ID = re.compile(r"[a-z0-9][a-z0-9-]{0,63}")
SEALED_LINE = re.compile(r"[0-9a-f]{64}\n")
NOT_PRODUCED = re.compile(r"^dogfood-[a-z]+: .*\bNOT_PRODUCED\b.*$", re.M)
IMPACT_ABSTENTION = re.compile(r"^dogfood-[a-z]+: NOTE coordination-time-impact NOT_PRODUCED (\S+)$", re.M)
HARNESS_FAILURE = "HARNESS-FAILURE"
ARMS = ("baseline", "treatment")
# dogfood-report-drift has two causes; each is told apart by the fix line that follows it.
NOT_COMPLETE = "dogfood-check: FAIL dogfood-report-drift\n  fix: the report is not complete"
# The refusal each designated case must carry (jobs.completion expectedRefusal): a cem status reason
# code at the CEM level, a dogfood check refusal line at the loop level.
EXPECTED_REFUSAL = {"M1-stale-target": "patch-digest-mismatch", "M2-hunk-removed": "uncited-hunk",
                    "M3-basis-removed": "unsupported-without-basis",
                    "M4-evidence-digest-staled": "fabricated-evidence-id", "M5-wrong-base": "base-revision-mismatch",
                    "M6-supported-to-unknown": "max-unknown-exceeded",
                    "loop-L1-stale-after-bind": "dogfood-check: REFUSE dirty-worktree",
                    "loop-L2-cem-removed": "dogfood-check: REFUSE dirty-worktree",
                    "loop-L3-cem-tampered": NOT_COMPLETE,
                    "loop-L4-local-outcome-removed": "dogfood-check: FAIL local-outcome-evidence-drift",
                    "loop-L5-verify-missing": NOT_COMPLETE,
                    "loop-L6-citations-missing": NOT_COMPLETE}
# No host Git configuration or identity, and no inherited Corvint setting, reaches a subprocess.
ENV = dict({k: v for k, v in os.environ.items() if not k.startswith(("GIT_", "CORVINT_"))},
           GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_SYSTEM=os.devnull,
           GIT_AUTHOR_NAME="bench", GIT_AUTHOR_EMAIL="bench@invalid",
           GIT_COMMITTER_NAME="bench", GIT_COMMITTER_EMAIL="bench@invalid")


def sha256_bytes(raw):
    return hashlib.sha256(raw).hexdigest()


def sha256_file(path):
    with open(path, "rb") as f:
        return sha256_bytes(f.read())


def canonical(value):
    return (json.dumps(value, sort_keys=True, indent=2, ensure_ascii=False) + "\n").encode()


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True, check=True, env=ENV).stdout


def git_ok(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], capture_output=True, env=ENV).returncode == 0


def execute(argv, cwd, env=ENV, timeout=1800):
    start = time.monotonic()
    p = subprocess.run(argv, cwd=cwd, env=env, capture_output=True, timeout=timeout)
    wall = round((time.monotonic() - start) * 1000)
    return {"exit": p.returncode, "wallMs": wall, "stdout": p.stdout, "stderr": p.stderr}


def observation(r):
    text = (r["stdout"] + b"\n" + r["stderr"]).decode(errors="replace")
    abstention = IMPACT_ABSTENTION.search(text)
    return {"exit": r["exit"], "wallMs": r["wallMs"], "stdoutBytes": len(r["stdout"]),
            "stdoutSha256": sha256_bytes(r["stdout"]), "stdoutTail": r["stdout"].decode(errors="replace")[-400:],
            "stderrTail": r["stderr"].decode(errors="replace")[-400:], "notProduced": NOT_PRODUCED.findall(text),
            "impactAbstention": abstention.group(1) if abstention else None}


def timing(walls):
    s = sorted(walls)
    return {"runsMs": walls, "medianMs": statistics.median(s), "maxMs": s[-1]}


def refuse(reason):
    sys.exit(f"untouched-repository: REFUSE {reason}")


# ---------------------------------------------------------------- corpus derivation

def task_text(repo, p1, p2):
    subjects = git(repo, "log", "--no-merges", "--reverse", "--format=%s", f"{p1}..{p2}").splitlines()
    for s in subjects:
        if not BIND_SUBJECT.match(s):
            return CONVENTIONAL.sub("", s)
    return None


def subject_task(repo, commit):
    subject = git(repo, "log", "-1", "--format=%s", commit).strip()
    return CONVENTIONAL.sub("", PULL_SUFFIX.sub("", subject))


def module_dirs(repo, rev):
    return sorted(os.path.dirname(p) for p in git(repo, "ls-tree", "-r", "--name-only", rev).splitlines()
                  if p == "go.mod" or p.endswith("/go.mod"))


def module_path_at(repo, rev):
    return git(repo, "show", f"{rev}:go.mod").splitlines()[0].split()[1]


def root_package(path, modules, module_path):
    d = os.path.dirname(path)
    parts = d.split("/") if d else []
    if any(p == "testdata" or p.startswith(("_", ".")) for p in parts):
        return None
    owners = [m for m in modules if m == "" or d == m or d.startswith(m + "/")]
    if max(owners, key=len) != "":
        return None
    return module_path + ("/" + d if d else "")


def go_packages(repo, rev, paths):
    modules = module_dirs(repo, rev)
    module_path = module_path_at(repo, rev)
    pkgs = {root_package(p, modules, module_path) for p in paths if p.endswith(".go")}
    return sorted(p for p in pkgs if p)


def excluded(path, exclusions):
    """True when the preregistration's pathExclusions make path never critical, a baseline hit or the L1 probe."""
    if not exclusions:
        return False
    return path.startswith(tuple(exclusions["prefixes"])) or path in exclusions["files"]


def derive_corpus(repo, head, pathspec=None, exclusions=None):
    cases, excluded_cases = [], []
    # A pathspec keeps only first-parent commits whose diff against the first parent touches it.
    limit = ["--", pathspec] if pathspec else []
    for line in git(repo, "log", "--first-parent", "-n", str(WINDOW), "--format=%H %P", head, *limit).splitlines():
        parts = line.split()
        if len(parts) < 2:
            excluded_cases.append({"commit": parts[0], "rule": "E1-no-parent"})
            continue
        commit, p1, p2 = parts[0], parts[1], (parts[2] if len(parts) > 2 else None)
        rows = [r.split("\t") for r in git(repo, "diff", "--name-status", "--no-renames", p1, commit).splitlines()]
        rows = [(st, f) for st, f in rows if not excluded(f, exclusions)]
        if not rows:
            excluded_cases.append({"commit": commit, "rule": "E2-empty-diff"})
            continue
        task = task_text(repo, p1, p2) if p2 else subject_task(repo, commit)
        if not task:
            excluded_cases.append({"commit": commit, "rule": "E3-no-task"})
            continue
        critical = sorted(f for s, f in rows if s in ("M", "D"))
        live = [f for s, f in rows if s != "D"]
        cases.append({
            "commit": commit, "parent": p1, "branchTip": p2, "task": task,
            "changedFiles": len(rows),
            "orientation": {"eligible": len(rows) <= ORIENTATION_MAX_FILES and bool(critical), "critical": critical},
            "consequence": {"eligible": bool(go_packages(repo, commit, live)),
                            "critical": go_packages(repo, commit, live)},
        })
    head = git(repo, "rev-parse", head).strip()
    corpus = {"profile": "corvint-untouched-repository-corpus/0", "head": head, "module": module_path_at(repo, head),
              "window": WINDOW, "cases": cases, "excluded": excluded_cases}
    if pathspec:
        corpus["pathspec"] = pathspec
    if exclusions:
        corpus["pathExclusions"] = exclusions
    return corpus


# ---------------------------------------------------------------- baselines

def lexical_topk(wt, rev, task, exclusions=None):
    tokens = sorted({t for t in re.split(r"[^a-z0-9_-]+", task.lower()) if len(t) >= 4 and t not in STOPWORDS})
    hits = {}
    for t in tokens:
        out = subprocess.run(["git", "-C", wt, "grep", "-l", "-i", "-F", "-e", t, rev, "--"],
                             capture_output=True, text=True, env=ENV).stdout
        for line in out.splitlines():
            path = line.split(":", 1)[1]
            if not excluded(path, exclusions):
                hits[path] = hits.get(path, 0) + 1
    ranked = sorted(hits.items(), key=lambda kv: (-kv[1], kv[0]))
    return [p for p, _ in ranked[:PACKET_LIMIT]], tokens


def lexical_importers(wt, rev, critical):
    modules = module_dirs(wt, rev)
    module_path = module_path_at(wt, rev)
    selected = set(critical)
    for ip in critical:
        out = subprocess.run(["git", "-C", wt, "grep", "-l", "-F", "-e", f'"{ip}"', rev, "--", "*.go"],
                             capture_output=True, text=True, env=ENV).stdout
        for line in out.splitlines():
            pkg = root_package(line.split(":", 1)[1], modules, module_path)
            if pkg:
                selected.add(pkg)
    return sorted(selected)


# ---------------------------------------------------------------- harness plumbing

class Bench:
    def __init__(self, repo, work, runs, corvint, wt="wt", exclusions=None):
        self.repo, self.work, self.runs, self.corvint = repo, work, runs, corvint
        self.exclusions = exclusions
        self.wt = os.path.join(work, wt)

    def checkout(self, rev):
        subprocess.run(["git", "-C", self.wt, "checkout", "-q", "--detach", "-f", rev], check=True, env=ENV)
        subprocess.run(["git", "-C", self.wt, "clean", "-qfdx"], check=True, env=ENV)

    def repeat(self, argv):
        return [execute(argv, self.wt) for _ in range(self.runs)]

    def timed(self, fn):
        values, walls = None, []
        for _ in range(self.runs):
            start = time.monotonic()
            values = fn()
            walls.append(round((time.monotonic() - start) * 1000))
        return values, walls


def stability(results):
    return len({sha256_bytes(r["stdout"]) for r in results}) == 1


def retries(results):
    return 0 if results[0]["exit"] == 0 else next((i for i, r in enumerate(results) if r["exit"] == 0), len(results))


def parse_json(raw):
    try:
        return json.loads(raw)
    except ValueError:
        return None


def unit_packages(unit_ids):
    return sorted({u[3:].split("::")[0] for u in unit_ids if u.startswith("go:")})


def score(critical, treatment, baseline):
    miss_t = sorted(set(critical) - set(treatment))
    miss_b = sorted(set(critical) - set(baseline))
    return {"criticalMissesTreatment": miss_t, "criticalMissesBaseline": miss_b,
            "treatmentOnlyCriticalMisses": sorted(set(miss_t) - set(miss_b))}


def orientation_case(b, case):
    c = case["orientation"]
    b.checkout(case["parent"])
    runs = b.repeat([b.corvint, "--root", b.wt, "context", "--task", case["task"], "--limit", str(PACKET_LIMIT)])
    doc = parse_json(runs[0]["stdout"])
    abstained = runs[0]["exit"] != 0 or not doc or doc.get("ok") is not True
    packet = [] if abstained else sorted({e["path"] for r in doc.get("results", []) for e in r.get("evidence", [])})
    (baseline, tokens), walls = b.timed(lambda: lexical_topk(b.wt, case["parent"], case["task"], b.exclusions))
    return {"commit": case["commit"], "task": case["task"], "critical": c["critical"],
            "treatment": {"argv": ["corvint", "context", "--task", case["task"], "--limit", str(PACKET_LIMIT)],
                          "runs": [observation(r) for r in runs], "latency": timing([r["wallMs"] for r in runs]),
                          "deterministic": stability(runs), "retries": retries(runs), "abstained": abstained,
                          "state": doc.get("state") if doc else None, "packet": packet},
            "baseline": {"procedure": "lexical-git-grep-top-k", "tokens": tokens, "packet": baseline,
                         "packetBytes": len("\n".join(baseline).encode()), "latency": timing(walls)},
            "score": score(c["critical"], packet, baseline)}


def consequence_case(b, case):
    c = case["consequence"]
    b.checkout(case["commit"])
    runs = b.repeat([b.corvint, "--root", b.wt, "affected", "--base", case["parent"]])
    doc = parse_json(runs[0]["stdout"])
    abstained = runs[0]["exit"] != 0 or not doc or doc.get("ok") is not True
    selected = [] if abstained else unit_packages(u["unitId"] for u in doc["plan"]["selected"])
    named = [] if abstained else unit_packages(u["detail"] for u in doc["plan"]["unknown"]
                                             if u["reason"] == "NO_SELECTABLE_TEST")
    covered = sorted(set(selected) | set(named))
    impact = b.repeat([b.corvint, "--root", b.wt, "impact", "--base", case["parent"],
                       "--range-profile", "expanded-256", "--limit", str(PACKET_LIMIT)])
    idoc = parse_json(impact[0]["stdout"]) or {}
    coverage = (idoc.get("context") or {}).get("coverage") or {}
    baseline, walls = b.timed(lambda: lexical_importers(b.wt, case["commit"], c["critical"]))
    return {"commit": case["commit"], "scored": c["eligible"], "critical": c["critical"],
            "treatment": {"argv": ["corvint", "affected", "--base", case["parent"]],
                          "runs": [observation(r) for r in runs], "latency": timing([r["wallMs"] for r in runs]),
                          "deterministic": stability(runs), "retries": retries(runs), "abstained": abstained,
                          "scope": doc["plan"]["scope"] if not abstained else None,
                          "unknownCount": len(doc["plan"]["unknown"]) if not abstained else None,
                          "selectedGoPackages": len(selected), "selected": selected,
                          "namedNoSelectableTest": named, "covered": covered},
            "impactRange": {"argv": ["corvint", "impact", "--base", case["parent"], "--range-profile", "expanded-256",
                                     "--limit", str(PACKET_LIMIT)],
                            "runs": [observation(r) for r in impact], "latency": timing([r["wallMs"] for r in impact]),
                            "ok": idoc.get("ok"), "criticalMissingReported": len(coverage.get("critical_missing", [])),
                            "uncertainty": coverage.get("uncertainty")},
            "baseline": {"procedure": "changed-packages-plus-direct-importers", "selectedGoPackages": len(baseline),
                         "latency": timing(walls)},
            "score": score(c["critical"], covered, baseline)}


# ---------------------------------------------------------------- completion: CEM level

def cem_mutants(cem):
    def mutate(fn):
        m = json.loads(json.dumps(cem))
        fn(m)
        cited_ids = {x["evidenceId"] for hunk in m["hunks"] for x in hunk.get("basis", [])}
        m["evidence"] = [e for e in m["evidence"] if e["id"] in cited_ids]
        return m
    supported = next(i for i, h in enumerate(cem["hunks"]) if h["disposition"] == "supported")
    cited = next(i for i, e in enumerate(cem["evidence"])
                 if e["id"] == cem["hunks"][supported]["basis"][0]["evidenceId"])

    def drop_hunk(m): del m["hunks"][supported]

    def drop_basis(m): m["hunks"][supported]["basis"] = []

    def stale_span(m): m["evidence"][cited]["spanSha256"] = sha256_bytes(b"stale-evidence-probe")

    def to_unknown(m):
        m["hunks"][supported].update({"disposition": "unknown", "reason": "no-evidence", "basis": []})
    return {"M0-reencoded-control": mutate(lambda m: None), "M2-hunk-removed": mutate(drop_hunk), "M3-basis-removed": mutate(drop_basis),
            "M4-evidence-digest-staled": mutate(stale_span), "M6-supported-to-unknown": mutate(to_unknown)}


def cem_status(b, base, target, ceiling):
    return execute([b.corvint, "--root", b.wt, "cem", "status", "--map", ".corvint/change.cem.json",
                    "--expected-base", base, "--target", target, "--max-unknown", str(ceiling),
                    "--max-mechanical", "0"], b.wt)


def bench_commit(wt, message):
    subprocess.run(["git", "-C", wt, "-c", "user.name=bench", "-c", "user.email=bench@invalid",
                    "commit", "-q", "--allow-empty", "-am", message], check=True, env=ENV)
    return git(wt, "rev-parse", "HEAD").strip()


def reasons(r):
    doc = parse_json(r["stdout"]) or parse_json(r["stderr"]) or {}
    found = [doc["code"]] if "code" in doc else []
    found += [i.get("code") for i in (doc.get("verification") or {}).get("issues", [])]
    found += [i.get("code") if isinstance(i, dict) else i for i in doc.get("policyIssues", [])]
    return found


# A designated case is informative only when its gate holds and it is COMPLETE (a false complete) or
# REFUSED with its expected refusal among those found; HARNESS-FAILURE is neither.
def expect(case, name, found, gate):
    case["expectedRefusal"] = EXPECTED_REFUSAL[name]
    case["refusedAsExpected"] = case["verdict"] == "REFUSED" and EXPECTED_REFUSAL[name] in found
    case["informative"] = gate and (case["verdict"] == "COMPLETE" or case["refusedAsExpected"])
    return case


def cem_case(b, bind, base):
    b.checkout(bind)
    map_path = os.path.join(b.wt, ".corvint", "change.cem.json")
    with open(map_path, "rb") as f:
        raw = f.read()
    cem = json.loads(raw)
    ceiling = sum(1 for h in cem["hunks"] if h["disposition"] == "unknown")
    control = [cem_status(b, base, bind, ceiling) for _ in range(b.runs)]
    cases = {}
    for name, mutant in cem_mutants(cem).items():
        b.checkout(bind)
        with open(map_path, "wb") as f:
            f.write(canonical(mutant))
        cases[name] = cem_status(b, base, bench_commit(b.wt, name), ceiling)
    b.checkout(bind)
    cases["M5-wrong-base"] = cem_status(b, git(b.wt, "rev-parse", bind + "^").strip(), bind, ceiling)
    probe = next(h["path"] for h in cem["hunks"] if os.path.isfile(os.path.join(b.wt, h["path"])))
    with open(os.path.join(b.wt, probe), "ab") as f:
        f.write(b"\nstale-evidence-probe\n")
    cases["M1-stale-target"] = cem_status(b, base, bench_commit(b.wt, "stale probe"), ceiling)
    reencoded = cases.pop("M0-reencoded-control")
    control_complete = control[0]["exit"] == 0 and reencoded["exit"] == 0
    return {"bind": bind, "base": base, "mapSha256": sha256_bytes(raw), "unknownCeiling": ceiling,
            "positiveControl": {"runs": [observation(r) for r in control],
                                "latency": timing([r["wallMs"] for r in control]),
                                "verdict": "COMPLETE" if control[0]["exit"] == 0 else "REFUSED"},
            "reencodedControl": {"observation": observation(reencoded), "byteIdentical": canonical(cem) == raw,
                                 "verdict": "COMPLETE" if reencoded["exit"] == 0 else "REFUSED"},
            "missingEvidence": {k: expect({"observation": observation(v), "reasons": reasons(v),
                                           "verdict": "COMPLETE" if v["exit"] == 0 else "REFUSED"},
                                          k, reasons(v), control_complete)
                                for k, v in sorted(cases.items())}}


def cem_level(b, source, base, target):
    # Only the loop's own bind counts: a repository's committed CEM history is never an input.
    bind = git(source, "log", "-1", "--first-parent", "--diff-filter=A", "--format=%H", f"{target}..HEAD", "--",
               ".corvint/change.cem.json").strip()
    if not bind:
        return []
    cb = Bench(b.repo, b.work, b.runs, b.corvint, wt="cem-wt")
    shutil.copytree(source, cb.wt, symlinks=True)
    return [cem_case(cb, bind, base)]


# ---------------------------------------------------------------- completion: full loop rehearsal

class Loop:
    def __init__(self, b, base, target):
        self.b, self.base, self.target = b, base, target
        self.task = git(b.repo, "log", "-1", "--format=%s", target).strip()
        self.inputs = os.path.join(b.work, "loop-inputs")
        os.makedirs(self.inputs, exist_ok=True)
        self.files = {"DOGFOOD_INTENTS_FILE": self.write("intents", INTENTS),
                      "DOGFOOD_VERIFY_FILE": self.write("verify", VERIFY),
                      "DOGFOOD_CITATIONS": os.path.join(self.inputs, "citations.tsv")}

    def write(self, name, text):
        path = os.path.join(self.inputs, name)
        with open(path, "w") as f:
            f.write(text)
        return path

    def env(self, omit):
        env = {k: v for k, v in ENV.items() if not k.startswith("DOGFOOD_")}
        env.update(DOGFOOD_OUTCOME="passed", DOGFOOD_TASK=self.task)
        env.update({k: v for k, v in self.files.items() if k not in omit})
        return env

    def clone(self, name):
        d = os.path.join(self.b.work, name)
        subprocess.run(["git", "clone", "-q", self.b.repo, d], check=True, env=ENV)
        subprocess.run(["git", "-C", d, "checkout", "-q", "-B", "rehearsal", self.target], check=True, env=ENV)
        subprocess.run(["git", "-C", d, "config", "user.name", "bench"], check=True, env=ENV)
        subprocess.run(["git", "-C", d, "config", "user.email", "bench@invalid"], check=True, env=ENV)
        os.makedirs(os.path.join(d, ".git", "info"), exist_ok=True)
        with open(os.path.join(d, ".git", "info", "exclude"), "a") as f:
            f.write(ADOPTER_EXCLUDES)
        return d

    def step(self, d, verb, env, log):
        r = execute([self.b.corvint, "dogfood", verb, self.base], d, env=env)
        log.append(dict(observation(r), step="dogfood-" + verb))
        return r

    def run(self, name, omit=()):
        d, env, log = self.clone(name), self.env(omit), []
        self.step(d, "change", env, log)
        cem_path = os.path.join(d, ".corvint", "change.cem.json")
        hunks = len(json.load(open(cem_path))["hunks"]) if os.path.isfile(cem_path) else 0
        with open(self.files["DOGFOOD_CITATIONS"], "w") as f:
            f.write("".join(f"{i}\t{CITATION}\n" for i in range(1, hunks + 1)))
        self.step(d, "change", env, log)
        added = git_ok(d, "add", "-f", ".corvint/change.cem.json")
        bound = added and git_ok(d, "commit", "-q", "-m", "chore: bind change evidence")
        self.step(d, "change", env, log)
        check = self.step(d, "check", env, log)
        return d, env, {"variant": name, "omittedInputs": sorted(omit), "hunks": hunks, "steps": log,
                        "invocations": len(log), "failedInvocations": sum(1 for s in log if s["exit"] != 0),
                        "unexpectedFailures": sum(1 for s in log[2:] if s["exit"] != 0), "bindCommitted": bound,
                        "wallMs": sum(s["wallMs"] for s in log), "verdict": verdict(check) if bound else HARNESS_FAILURE,
                        "refusal": refusal(check)}

    # Every variant runs at the L0 run 1 path, restored from a pristine copy: when impact abstains,
    # dogfood check requires the recorded --root to be the worktree it checks.
    def restore(self, source, pristine):
        shutil.rmtree(source)
        shutil.copytree(pristine, source, symlinks=True)

    # A variant that commits past the bind reruns dogfood change before check, as the adopter loop
    # does (docs/DOGFOOD.md section 4); the change exit is data, and the verdict comes from check.
    def mutate_and_check(self, source, pristine, name, env, mutate, rebind):
        self.restore(source, pristine)
        if not mutate(source):
            return {"variant": name, "rebind": rebind, "steps": [], "verdict": HARNESS_FAILURE, "refusal": []}
        log = []
        verbs = ("change", "check") if rebind else ("check",)
        results = [self.step(source, verb, env, log) for verb in verbs]
        return {"variant": name, "rebind": rebind, "steps": log, "verdict": verdict(results[-1]),
                "refusal": [line for r in results for line in refusal(r)]}


def verdict(r):
    return "COMPLETE" if r["exit"] == 0 and PASS_LINE.search(r["stdout"]) else "REFUSED"


def refusal(r):
    text = (r["stdout"] + r["stderr"]).decode(errors="replace")
    return re.findall(r"^dogfood-(?:change|check): (?:FAIL|REFUSE) \S+(?:\n  fix: the report (?:binds another BASE or HEAD|is not complete))?", text, re.M)


def commit_all(d, message):
    return git_ok(d, "commit", "-q", "-am", message)


def noncanonical(value):
    return (json.dumps(value, separators=(",", ":"), ensure_ascii=False) + "\n").encode()


def loop_rehearsal(b, base, target):
    loop = Loop(b, base, target)
    positives = [loop.run(f"loop-L0-{i}") for i in range(1, b.runs + 1)]
    source, env, _ = positives[0]
    pristine = os.path.join(b.work, "loop-L0-1-pristine")
    shutil.copytree(source, pristine, symlinks=True)
    probe = next(f for f in git(b.repo, "diff", "--name-only", "--diff-filter=AM", base, target).splitlines()
                 if not excluded(f, b.exclusions))

    # Each mutation returns whether it happened; one that did not makes its variant HARNESS-FAILURE.
    def stale(d):
        with open(os.path.join(d, probe), "ab") as f:
            f.write(b"\n")
        return commit_all(d, "stale probe")

    def cem_removed(d):
        return git_ok(d, "rm", "-q", ".corvint/change.cem.json") and commit_all(d, "remove change evidence")

    def rewrite_cem(d, fn, encode, message):
        p = os.path.join(d, ".corvint", "change.cem.json")
        with open(p, "rb") as f:
            raw = f.read()
        cem = json.loads(raw)
        fn(cem)
        new = encode(cem)
        with open(p, "wb") as f:
            f.write(new)
        return new != raw and commit_all(d, message)

    def reencoded(d):
        return rewrite_cem(d, lambda cem: None, noncanonical, "re-encode change evidence")

    def cem_tampered(d):
        return rewrite_cem(d, lambda cem: cem["hunks"][0].update(basis=[]), canonical, "tamper change evidence")

    def outcome_removed(d):
        os.remove(os.path.join(d, ".git", "corvint", "local-outcome.json"))
        return True
    controls = [loop.mutate_and_check(source, pristine, "loop-L0c-restored-in-place-control", env, lambda d: True, False),
                loop.mutate_and_check(source, pristine, "loop-L3c-reencoded-control", env, reencoded, True)]
    copies = [loop.mutate_and_check(source, pristine, n, env, fn, rebind) for n, fn, rebind in
              [("loop-L1-stale-after-bind", stale, True), ("loop-L2-cem-removed", cem_removed, True),
               ("loop-L3-cem-tampered", cem_tampered, True), ("loop-L4-local-outcome-removed", outcome_removed, False)]]
    loop.restore(source, pristine)
    fresh = [loop.run("loop-L5-verify-missing", omit=("DOGFOOD_VERIFY_FILE",))[2],
             loop.run("loop-L6-citations-missing", omit=("DOGFOOD_CITATIONS",))[2]]
    restored_ok = controls[0]["verdict"] == "COMPLETE"
    past_bind_ok = restored_ok and controls[1]["verdict"] == "COMPLETE"
    # L3c controls for a commit past the bind and the change rerun, which L1-L3 make and L4 does not.
    gates = {"loop-L4-local-outcome-removed": restored_ok}
    for m in copies:
        expect(m, m["variant"], m["refusal"], gates.get(m["variant"], past_bind_ok))
    for m in fresh:
        expect(m, m["variant"], m["refusal"], positives[0][2]["verdict"] == "COMPLETE")
    return source, {"base": base, "target": target, "task": loop.task, "positiveRuns": [p[2] for p in positives],
                    "controls": controls, "latency": timing([p[2]["wallMs"] for p in positives]),
                    "unexpectedFailures": [p[2]["unexpectedFailures"] for p in positives],
                    "positiveVerdicts": [p[2]["verdict"] for p in positives] + [c["verdict"] for c in controls],
                    "missingEvidence": copies + fresh}


def plain_git_completion(b, base, target):
    def fn():
        git(b.repo, "diff", "--stat", base, target)
        git(b.repo, "log", "--format=%H %s", f"{base}..{target}")
    _, walls = b.timed(fn)
    return {"procedure": "git diff --stat + git log over the range", "verdict": "NOT_APPLICABLE",
            "reason": "plain Git emits no evidence verdict, so it cannot give a complete-evidence answer",
            "latency": timing(walls)}


# ---------------------------------------------------------------- gates, cost, receipts

def agent_cost(path, outcomes):
    if not path:
        return {"completeTaskCost": "NOT_OBSERVED", "humanFailureRate": "NOT_OBSERVED",
                "reason": "no live model-driven agent or human reviewer ran this benchmark",
                "savingsClaim": "measured, no savings claim"}
    rows = [json.loads(l) for l in open(path) if l.strip()]
    problems = cost_problems(rows)
    if problems:
        return {"observations": rows, "invalidObservations": problems, "savingsClaim": "measured, no savings claim"}
    arms = {a: [r for r in rows if r["arm"] == a] for a in ARMS}
    tokens = {a: [r["completeTaskTokens"] for r in v] for a, v in arms.items()}
    med = {a: statistics.median(v) for a, v in tokens.items()}
    p75 = {a: statistics.quantiles(v, n=4)[2] if len(v) > 1 else v[0] for a, v in tokens.items()}
    failures = {a: sum(1 for r in v if r["humanFailure"]) / len(v) for a, v in arms.items()}
    met = (med["treatment"] <= 0.75 * med["baseline"] and p75["treatment"] <= 0.80 * p75["baseline"]
           and failures["treatment"] == 0 and all(o["outcome"] == "PASS" for o in outcomes.values()))
    return {"observations": rows, "medianCompleteTaskTokens": med, "p75CompleteTaskTokens": p75,
            "humanFailureRate": failures, "savingsClaim": "threshold met" if met else "measured, no savings claim"}


def cost_row_problem(r):
    if not isinstance(r, dict) or r.get("arm") not in ARMS:
        return "unknown-arm"
    if not isinstance(r.get("caseId"), str) or not r["caseId"]:
        return "case-id-missing"
    t = r.get("completeTaskTokens")
    if isinstance(t, bool) or not isinstance(t, (int, float)) or not math.isfinite(t) or t <= 0:
        return "tokens-invalid"
    if not isinstance(r.get("humanFailure"), bool):
        return "outcome-missing"
    return None


# A savings comparison needs every row well formed and the same unique cases observed in both arms.
def cost_problems(rows):
    problems = sorted({p for p in map(cost_row_problem, rows) if p})
    if problems:
        return problems
    cases = {a: [r["caseId"] for r in rows if r["arm"] == a] for a in ARMS}
    if any(len(set(v)) != len(v) for v in cases.values()):
        problems.append("duplicate-case")
    if set(cases["baseline"]) != set(cases["treatment"]):
        problems.append("unmatched-cases")
    if not cases["treatment"]:
        problems.append("no-observations")
    return problems


def job_outcomes(orientation, consequence, cems, loop):
    def gate(cases):
        scored = [c for c in cases if c.get("scored", True)]
        misses = sum(len(c["score"]["treatmentOnlyCriticalMisses"]) for c in scored)
        abstained = sum(1 for c in scored if c["treatment"]["abstained"])
        return {"scoredCases": len(scored), "treatmentOnlyCriticalMisses": misses, "abstentions": abstained,
                "outcome": "PASS" if scored and misses == 0 and abstained == 0 else "FAIL"}
    designated = [m for c in cems for m in c["missingEvidence"].values()] + loop["missingEvidence"]
    informative = [m for m in designated if m["informative"]]
    false_complete = sum(1 for m in designated if m["verdict"] == "COMPLETE")
    controls = ([c["positiveControl"]["verdict"] for c in cems] + [c["reencodedControl"]["verdict"] for c in cems]
                + loop["positiveVerdicts"])
    completion = {"designatedCases": len(designated), "informativeCases": len(informative),
                  "falseCompleteVerdicts": false_complete,
                  "positiveControlRefusals": sum(1 for v in controls if v != "COMPLETE"),
                  "outcome": "PASS" if false_complete == 0 and len(informative) == len(designated) else "FAIL"}
    return {"orientation": gate(orientation), "consequence": gate(consequence), "completion": completion}


def write_receipts(bench, run_id, candidate, digests, outcomes, paths):
    out = os.path.join(bench, "receipts", run_id)
    os.makedirs(out, exist_ok=True)
    written = []
    for job, uc in USE_CASES.items():
        outcome = outcomes[job]["outcome"]
        receipt = {"attestation": {"corpusSha256": digests["corpus"], "outcome": outcome,
                                   "preregistrationSha256": digests["preregistration"],
                                   "resultSha256": digests["result"]},
                   "evidenceClass": "sealed-benchmark", "repositoryRevision": candidate, "result": outcome,
                   "spec": "corvint-use-case-evidence/0",
                   "subjects": [{"path": paths[k], "sha256": digests[k]} for k in ("preregistration", "corpus", "result")],
                   "useCaseId": uc}
        path = os.path.join(out, uc + ".json")
        with open(path, "wb") as f:
            f.write(canonical(receipt))
        written.append({"path": os.path.relpath(path, ROOT), "sha256": sha256_file(path)})
    return written


# ---------------------------------------------------------------- commands

def extract_candidate(candidate_dir, work, target_version):
    name = f"corvint_{platform.system().lower()}_{GOARCH.get(platform.machine(), platform.machine())}.tar.gz"
    archive = os.path.join(candidate_dir, name)
    if not os.path.isfile(archive):
        refuse("candidate-archive-missing")
    digest = sha256_file(archive)
    sums = os.path.join(candidate_dir, "SHA256SUMS")
    if not os.path.isfile(sums):
        refuse("candidate-checksums-missing")
    with open(sums) as f:
        if f"{digest}  {name}" not in f.read().splitlines():
            refuse("candidate-archive-checksum-mismatch")
    report_path = os.path.join(candidate_dir, "verification-report.json")
    if not os.path.isfile(report_path):
        refuse("candidate-report-missing")
    with open(report_path, "rb") as f:
        report = parse_json(f.read())
    if not isinstance(report, dict):
        refuse("candidate-report-malformed")
    target = next((t for t in report.get("targets") or [] if t.get("archiveName") == name), {})
    retained = target.get("retainedArchive", {}).get("sha256")
    if report.get("verdict") != "PASS" or not report.get("revision") or retained != digest:
        refuse("candidate-report-mismatch")
    with tarfile.open(archive, "r:gz") as tar:
        members = [m for m in tar.getmembers() if os.path.basename(m.name) == "corvint"]
        if len(members) != 1 or not members[0].isreg():
            refuse("candidate-binary-member")
        raw = tar.extractfile(members[0]).read()
    binary = os.path.join(work, "corvint")
    with open(binary, "wb") as f:
        f.write(raw)
    os.chmod(binary, 0o755)
    try:
        ran = subprocess.run([binary, "--version"], capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.TimeoutExpired):
        refuse("candidate-binary-not-executable")
    if ran.returncode != 0:
        refuse("candidate-binary-not-executable")
    version = ran.stdout.strip()
    match = VERSION_LINE.match(version)
    if not match or match.group(1) != target_version:
        refuse("candidate-version-mismatch")
    return binary, {"archive": name, "archiveSha256": digest, "binarySha256": sha256_bytes(raw),
                    "commit": report["revision"], "version": version}


def cmd_corpus(args):
    prereg = json.load(open(os.path.join(args.benchmark_dir, "preregistration.json")))
    corpus = derive_corpus(args.repo, args.head, prereg.get("corpusPathspec"), prereg.get("pathExclusions"))
    with open(os.path.join(args.benchmark_dir, "corpus.json"), "wb") as f:
        f.write(canonical(corpus))
    print(sha256_file(os.path.join(args.benchmark_dir, "corpus.json")))


def sealed_preregistration_digest(bench):
    path = os.path.join(bench, "preregistration.sha256")
    if not os.path.isfile(path):
        refuse("preregistration-seal-missing")
    with open(path) as f:
        text = f.read()
    if not SEALED_LINE.fullmatch(text):
        refuse("preregistration-seal-malformed")
    return text.strip()


def cmd_run(args):
    if not RUN_ID.fullmatch(args.run_id):
        refuse("run-id-invalid")
    bench = args.benchmark_dir
    prereg_path = os.path.join(bench, "preregistration.json")
    prereg_digest = sha256_file(prereg_path)
    if prereg_digest != sealed_preregistration_digest(bench):
        refuse("preregistration-digest-mismatch")
    if args.prereg_sha256 != prereg_digest:
        refuse("preregistration-digest-argument-mismatch")
    prereg = json.load(open(prereg_path))
    corpus_path = os.path.join(bench, "corpus.json")
    for name, path in (("corpusSha256", corpus_path), ("harnessSha256", os.path.abspath(__file__))):
        if sha256_file(path) != prereg["sealed"][name]:
            refuse(f"{name}-mismatch")
    corpus = json.load(open(corpus_path))
    if corpus["head"] != prereg["pin"] or corpus["module"] != prereg["module"]:
        refuse("corpus-pin-mismatch")
    if canonical(derive_corpus(args.repo, corpus["head"], prereg.get("corpusPathspec"), prereg.get("pathExclusions"))) != canonical(corpus):
        refuse("corpus-rederivation-mismatch")
    out = os.path.join(bench, "runs", args.run_id + ".json")
    marker = os.path.join(bench, "runs", args.run_id + ".started.json")
    if os.path.exists(out) or os.path.exists(marker):
        refuse("run-id-already-used")
    # Corvint runs with each worktree as cwd, so a relative --work would not reach the extracted binary.
    work = tempfile.mkdtemp(prefix="untouched-repository-", dir=os.path.abspath(args.work))
    corvint, candidate = extract_candidate(args.candidate_dir, work, prereg["targetVersion"])
    b = Bench(args.repo, work, prereg["runsPerMeasurement"], corvint, exclusions=prereg.get("pathExclusions"))
    started = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    # The marker is written before any execution and never removed, so an aborted run leaves a record.
    os.makedirs(os.path.dirname(marker), exist_ok=True)
    with open(marker, "wb") as f:
        f.write(canonical({"profile": "corvint-untouched-repository-started/0", "runId": args.run_id,
                           "startedAt": started, "preregistrationSha256": prereg_digest,
                           "corpusSha256": prereg["sealed"]["corpusSha256"],
                           "harnessSha256": prereg["sealed"]["harnessSha256"],
                           "candidateArchiveSha256": candidate["archiveSha256"],
                           "candidateBinarySha256": candidate["binarySha256"], "candidateVersion": candidate["version"]}))
    subprocess.run(["git", "clone", "-q", "--no-checkout", args.repo, b.wt], check=True, env=ENV)
    load_start = os.getloadavg()
    cases = corpus["cases"]
    orientation = [orientation_case(b, c) for c in cases if c["orientation"]["eligible"]]
    consequence = [consequence_case(b, c) for c in cases]
    target = corpus["head"]
    base = git(args.repo, "rev-parse", target + "^").strip()
    source, loop = loop_rehearsal(b, base, target)
    cems = cem_level(b, source, base, target)
    outcomes = job_outcomes(orientation, consequence, cems, loop)
    result = {"profile": "corvint-untouched-repository-result/0", "runId": args.run_id, "startedAt": started,
              "finishedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "preregistrationSha256": prereg_digest, "corpusSha256": prereg["sealed"]["corpusSha256"],
              "harnessSha256": prereg["sealed"]["harnessSha256"], "candidateCommit": candidate["commit"],
              "candidateArchive": candidate["archive"], "candidateArchiveSha256": candidate["archiveSha256"],
              "candidateBinarySha256": candidate["binarySha256"], "candidateVersion": candidate["version"],
              "externalRepository": prereg["repository"], "externalRepositoryRevision": corpus["head"],
              "host": {"system": platform.system(), "machine": platform.machine(), "python": platform.python_version(),
                       "cpus": os.cpu_count(), "loadAverageStart": load_start, "loadAverageEnd": os.getloadavg(),
                       "go": subprocess.run(["go", "env", "GOVERSION"], capture_output=True, text=True,
                                            env=dict(os.environ, GOTOOLCHAIN="local")).stdout.strip()},
              "runsPerMeasurement": b.runs,
              "orientation": orientation, "consequence": consequence,
              "completion": {"cem": cems, "loop": loop,
                             "plainGitBaseline": plain_git_completion(b, base, target)},
              "outcomes": outcomes, "cost": agent_cost(args.agent_observations, outcomes)}
    with open(out, "wb") as f:
        f.write(canonical(result))
    rel = {"preregistration": os.path.relpath(prereg_path, ROOT), "corpus": os.path.relpath(corpus_path, ROOT),
           "result": os.path.relpath(out, ROOT)}
    digests = {"preregistration": prereg_digest, "corpus": prereg["sealed"]["corpusSha256"],
               "result": sha256_file(out)}
    receipts = write_receipts(bench, args.run_id, candidate["commit"], digests, outcomes, rel)
    print(json.dumps({"result": rel["result"], "resultSha256": digests["result"], "outcomes": outcomes,
                      "receipts": receipts}, indent=1))
    if not args.keep_work:
        shutil.rmtree(work, ignore_errors=True)


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = p.add_subparsers(dest="command", required=True)
    c = sub.add_parser("corpus", help="derive corpus.json from first-parent history ending at the pin")
    c.add_argument("--repo", required=True)
    c.add_argument("--head", required=True)
    r = sub.add_parser("run", help="run the sealed benchmark")
    r.add_argument("--repo", required=True)
    r.add_argument("--prereg-sha256", required=True)
    r.add_argument("--run-id", required=True)
    r.add_argument("--candidate-dir", required=True)
    r.add_argument("--work", required=True)
    r.add_argument("--agent-observations")
    r.add_argument("--keep-work", action="store_true")
    for q in (c, r):
        q.add_argument("--benchmark-dir", default=HERE,
                       help="directory holding preregistration.json, its seal, corpus.json, runs and receipts")
    args = p.parse_args()
    args.benchmark_dir = os.path.abspath(args.benchmark_dir)
    args.repo = os.path.abspath(getattr(args, "repo", "."))
    {"corpus": cmd_corpus, "run": cmd_run}[args.command](args)


if __name__ == "__main__":
    main()
