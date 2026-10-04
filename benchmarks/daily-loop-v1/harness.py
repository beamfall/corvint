#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Daily change-evidence loop sealed benchmark V1 (V1-0012, PCCO-V0-015..017).

`corpus` derives the immutable corpus from `git log --first-parent` under the
preregistered rules. `run` verifies the sealed preregistration, builds the
candidate from its commit, runs every treatment and baseline, scores it, and
writes one result plus three corvint-use-case-evidence/0 receipts. Nothing here
estimates a value it cannot observe: live-agent dimensions stay NOT_OBSERVED
unless an observation file is supplied.

V1 revises the daily-loop-v0 harness, which stays frozen with its runs. The
committed preregistration.sha256 pins the preregistration (V1-0162); no host Git
configuration or inherited DOGFOOD_* or CORVINT_* input reaches a step, and a
bind or mutation that did not happen is HARNESS-FAILURE (V1-0382); L3c re-encodes
the CEM to different bytes and each designated case must carry its expected
refusal (V1-0161); a savings claim needs matched cases with no treatment human
failure (V1-0369); docs/build-log/ is excluded like docs/BUILD-LOG.md (decision
0423). The directory is unsealed until preregistration.json and
preregistration.sha256 are committed; until then `run` refuses.

Consequence scoring rule (preregistration amendment 1, V1-0202): the treatment
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
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REL = "benchmarks/daily-loop-v1"
EXCLUDED_PREFIXES = (".taskman/", ".corvint/", "docs/build-log/")
EXCLUDED_FILES = {"docs/BUILD-LOG.md", "docs/specs/INDEX.json", "docs/specs/REQUIREMENTS.tsv",
                  "docs/specs/README.md", "docs/decisions/README.md"}
BIND_SUBJECT = re.compile(r"^chore: (bind|seal|rebind)\b")
CONVENTIONAL = re.compile(r"^[a-z]+(\([^)]*\))?!?: ")
STOPWORDS = {"the", "and", "for", "with", "from", "into", "onto", "that", "this", "their", "them"}
ORIENTATION_MAX_FILES = 100
PACKET_LIMIT = 20
MAX_OBSERVATION_BYTES = 1 << 20
MAX_OBSERVATION_ROWS = 10000
COST_FIELDS = ("arm", "caseId", "completeTaskTokens", "humanFailure")
USE_CASES = {"orientation": "UC-TASK-ORIENTATION", "consequence": "UC-CHANGE-CONSEQUENCE",
             "completion": "UC-EVIDENCE-CARRYING-COMPLETION"}
CITATION_SPAN = "26:28"
RUN_ID = re.compile(r"[a-z0-9][a-z0-9-]{0,63}")
SEALED_LINE = re.compile(r"[0-9a-f]{64}\n")
ARMS = ("baseline", "treatment")
HARNESS_FAILURE = "HARNESS-FAILURE"
# dogfood-report-drift has two causes; each is told apart by the fix line that follows it.
NOT_COMPLETE = "dogfood-check: FAIL dogfood-report-drift\n  fix: the report is not complete"
# The refusal each designated case must carry: a cem status reason code at the CEM level, a
# dogfood check refusal line at the loop level.
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
# No host Git configuration or identity reaches a Git or Corvint subprocess.
ENV = dict({k: v for k, v in os.environ.items() if not k.startswith("GIT_")},
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


def git_status(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], capture_output=True, env=ENV).returncode


def git_ok(repo, *args):
    return git_status(repo, *args) == 0


def execute(argv, cwd, env=ENV, timeout=1800):
    start = time.monotonic()
    p = subprocess.run(argv, cwd=cwd, env=env, capture_output=True, timeout=timeout)
    wall = round((time.monotonic() - start) * 1000)
    return {"exit": p.returncode, "wallMs": wall, "stdout": p.stdout, "stderr": p.stderr}


def observation(r):
    return {"exit": r["exit"], "wallMs": r["wallMs"], "stdoutBytes": len(r["stdout"]),
            "stdoutSha256": sha256_bytes(r["stdout"]), "stdoutTail": r["stdout"].decode(errors="replace")[-400:],
            "stderrTail": r["stderr"].decode(errors="replace")[-400:]}


def timing(walls):
    s = sorted(walls)
    return {"runsMs": walls, "medianMs": statistics.median(s), "maxMs": s[-1]}


def excluded(path):
    return path.startswith(EXCLUDED_PREFIXES) or path in EXCLUDED_FILES


# ---------------------------------------------------------------- corpus derivation

def task_text(repo, p1, p2):
    subjects = git(repo, "log", "--no-merges", "--reverse", "--format=%s", f"{p1}..{p2}").splitlines()
    for s in subjects:
        if not BIND_SUBJECT.match(s):
            return CONVENTIONAL.sub("", s)
    return None


def module_dirs(repo, rev):
    return sorted(os.path.dirname(p) for p in git(repo, "ls-tree", "-r", "--name-only", rev).splitlines()
                  if p == "go.mod" or p.endswith("/go.mod"))


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
    module_path = git(repo, "show", f"{rev}:go.mod").splitlines()[0].split()[1]
    pkgs = {root_package(p, modules, module_path) for p in paths if p.endswith(".go")}
    return sorted(p for p in pkgs if p)


def derive_corpus(repo, head):
    cases, excluded_cases = [], []
    for line in git(repo, "log", "--first-parent", "--format=%H %P", head).splitlines():
        parts = line.split()
        if len(parts) < 3:
            excluded_cases.append({"commit": parts[0], "rule": "E1-no-first-parent-diff"})
            continue
        merge, p1, p2 = parts[0], parts[1], parts[2]
        rows = [r.split("\t") for r in git(repo, "diff", "--name-status", "--no-renames", p1, merge).splitlines()]
        kept = [(s, f) for s, f in rows if not excluded(f)]
        if not kept:
            excluded_cases.append({"commit": merge, "rule": "E2-empty-after-path-exclusions"})
            continue
        cems = sorted(f for s, f in rows if s == "A" and f.startswith(".corvint/changes/"))
        cem_cases = []
        for f in cems:
            cem = json.loads(git(repo, "show", f"{merge}:{f}"))
            cem_cases.append({"bind": os.path.basename(f).split(".")[0], "base": cem["baseRevision"],
                              "sealedPath": f, "sealedSha256": sha256_bytes(git(repo, "show", f"{merge}:{f}").encode()),
                              "citedEvidencePaths": sorted({e["path"] for e in cem["evidence"]})})
        modified = sorted(f for s, f in kept if s in ("M", "D"))
        at_parent = set(git(repo, "ls-tree", "-r", "--name-only", p1).splitlines())
        cited = sorted({p for c in cem_cases for p in c["citedEvidencePaths"]} & at_parent)
        live = [f for s, f in kept if s != "D"]
        cases.append({
            "commit": merge, "parent": p1, "branchTip": p2,
            "task": task_text(repo, p1, p2),
            "changedFiles": len(kept),
            "orientation": {"eligible": len(kept) <= ORIENTATION_MAX_FILES,
                            "critical": sorted(set(modified) | set(cited))},
            "consequence": {"eligible": bool(go_packages(repo, merge, live)),
                            "critical": go_packages(repo, merge, live)},
            "completion": cem_cases,
        })
    return {"profile": "corvint-daily-loop-corpus/0", "head": git(repo, "rev-parse", head).strip(),
            "cases": cases, "excluded": excluded_cases}


# ---------------------------------------------------------------- baselines

def lexical_topk(wt, rev, task):
    tokens = sorted({t for t in re.split(r"[^a-z0-9_-]+", task.lower()) if len(t) >= 4 and t not in STOPWORDS})
    hits = {}
    for t in tokens:
        out = subprocess.run(["git", "-C", wt, "grep", "-l", "-i", "-F", "-e", t, rev, "--"],
                             capture_output=True, text=True, env=ENV).stdout
        for line in out.splitlines():
            path = line.split(":", 1)[1]
            if not excluded(path):
                hits[path] = hits.get(path, 0) + 1
    ranked = sorted(hits.items(), key=lambda kv: (-kv[1], kv[0]))
    return [p for p, _ in ranked[:PACKET_LIMIT]], tokens


def lexical_importers(wt, rev, critical):
    modules = module_dirs(wt, rev)
    module_path = git(wt, "show", f"{rev}:go.mod").splitlines()[0].split()[1]
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
    def __init__(self, repo, work, runs, corvint):
        self.repo, self.work, self.runs, self.corvint = repo, work, runs, corvint
        self.wt = os.path.join(work, "wt")

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
    (baseline, tokens), walls = b.timed(lambda: lexical_topk(b.wt, case["parent"], case["task"]))
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


def cem_case(b, commit, c):
    bind, base = c["bind"], c["base"]
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
    return {"commit": commit, "bind": bind, "base": base, "mapSha256": sha256_bytes(raw),
            "sealedSha256": c["sealedSha256"], "unknownCeiling": ceiling,
            "positiveControl": {"runs": [observation(r) for r in control],
                                "latency": timing([r["wallMs"] for r in control]),
                                "verdict": "COMPLETE" if control[0]["exit"] == 0 else "REFUSED"},
            "reencodedControl": {"observation": observation(reencoded), "byteIdentical": canonical(cem) == raw,
                                 "verdict": "COMPLETE" if reencoded["exit"] == 0 else "REFUSED"},
            "missingEvidence": {k: expect({"observation": observation(v), "reasons": reasons(v),
                                           "verdict": "COMPLETE" if v["exit"] == 0 else "REFUSED"},
                                          k, reasons(v), control_complete)
                                for k, v in sorted(cases.items())}}


# ---------------------------------------------------------------- completion: full loop rehearsal

class Loop:
    def __init__(self, b, base, target, spec):
        self.b, self.base, self.target = b, base, target
        self.inputs = os.path.join(b.work, "loop-inputs")
        os.makedirs(self.inputs, exist_ok=True)
        self.files = {"DOGFOOD_INTENTS_FILE": self.write("intents", spec + "\n"),
                      "DOGFOOD_VERIFY_FILE": self.write("verify", f"python3 -m py_compile {REL}/harness.py\n"),
                      "DOGFOOD_CITATIONS": os.path.join(self.inputs, "citations.tsv")}

    def write(self, name, text):
        path = os.path.join(self.inputs, name)
        with open(path, "w") as f:
            f.write(text)
        return path

    # An input a variant omits, or a Corvint binary the operator selected, never arrives from the shell.
    def env(self, omit):
        env = {k: v for k, v in ENV.items() if not k.startswith(("DOGFOOD_", "CORVINT_"))}
        env.update(DOGFOOD_OUTCOME="passed",
                   DOGFOOD_TASK="Preregister the daily-loop sealed benchmark corpus and harness.")
        env.update({k: v for k, v in self.files.items() if k not in omit})
        return env

    def clone(self, name):
        d = os.path.join(self.b.work, name)
        subprocess.run(["git", "clone", "-q", self.b.repo, d], check=True, env=ENV)
        subprocess.run(["git", "-C", d, "checkout", "-q", "-B", "rehearsal", self.target], check=True, env=ENV)
        subprocess.run(["git", "-C", d, "config", "user.name", "bench"], check=True, env=ENV)
        subprocess.run(["git", "-C", d, "config", "user.email", "bench@invalid"], check=True, env=ENV)
        return d

    def step(self, d, target, env, log):
        r = execute(["make", target, f"BASE={self.base}"], d, env=env)
        log.append(dict(observation(r), step=target))
        return r

    def run(self, name, omit=()):
        d, env, log = self.clone(name), self.env(omit), []
        self.step(d, "dogfood-change", env, log)
        cem_path = os.path.join(d, ".corvint", "change.cem.json")
        hunks = len(json.load(open(cem_path))["hunks"]) if os.path.isfile(cem_path) else 0
        with open(self.files["DOGFOOD_CITATIONS"], "w") as f:
            f.write("".join(f"{i}\tAGENTS.md\t{CITATION_SPAN}\tspecification\n" for i in range(1, hunks + 1)))
        self.step(d, "dogfood-change", env, log)
        bind_exit = (git_status(d, "add", "-f", ".corvint/change.cem.json")
                     or git_status(d, "commit", "-q", "-m", "chore: bind change evidence"))
        bound = bind_exit == 0
        self.step(d, "dogfood-change", env, log)
        check = self.step(d, "dogfood-check", env, log)
        return d, env, {"variant": name, "omittedInputs": sorted(omit), "hunks": hunks, "steps": log,
                        "invocations": len(log), "failedInvocations": sum(1 for s in log if s["exit"] != 0),
                        "unexpectedFailures": sum(1 for s in log[2:] if s["exit"] != 0), "bindExit": bind_exit,
                        "wallMs": sum(s["wallMs"] for s in log), "verdict": verdict(check) if bound else HARNESS_FAILURE,
                        "refusal": refusal(check)}

    # A variant that commits past the bind reruns dogfood-change before check, as the adopter loop
    # does (docs/DOGFOOD.md section 4); the change exit is data, and the verdict comes from check.
    def mutate_and_check(self, source, name, env, mutate, rebind):
        d = os.path.join(self.b.work, name)
        shutil.copytree(source, d, symlinks=True)
        if not mutate(d):
            return {"variant": name, "rebind": rebind, "steps": [], "verdict": HARNESS_FAILURE, "refusal": []}
        log = []
        targets = ("dogfood-change", "dogfood-check") if rebind else ("dogfood-check",)
        results = [self.step(d, target, env, log) for target in targets]
        return {"variant": name, "rebind": rebind, "steps": log, "verdict": verdict(results[-1]),
                "refusal": [line for r in results for line in refusal(r)]}


def verdict(r):
    return "COMPLETE" if r["exit"] == 0 and b"dogfood-check: PASS" in r["stdout"] + r["stderr"] else "REFUSED"


def refusal(r):
    text = (r["stdout"] + r["stderr"]).decode(errors="replace")
    return re.findall(r"^dogfood-(?:change|check): (?:FAIL|REFUSE) \S+(?:\n  fix: the report (?:binds another BASE or HEAD|is not complete))?", text, re.M)


def commit_all(d, message):
    return git_ok(d, "commit", "-q", "-am", message)


def noncanonical(value):
    return (json.dumps(value, separators=(",", ":"), ensure_ascii=False) + "\n").encode()


def loop_rehearsal(b, base, target, spec):
    loop = Loop(b, base, target, spec)
    positives = [loop.run(f"loop-L0-{i}") for i in range(1, b.runs + 1)]
    source, env, _ = positives[0]
    probe = next(f for f in git(b.repo, "diff", "--name-only", "--diff-filter=AM", base, target).splitlines()
                 if not excluded(f))

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
    controls = [loop.mutate_and_check(source, "loop-L0c-copy-control", env, lambda d: True, False),
                loop.mutate_and_check(source, "loop-L3c-reencoded-control", env, reencoded, True)]
    copies = [loop.mutate_and_check(source, n, env, fn, rebind) for n, fn, rebind in
              [("loop-L1-stale-after-bind", stale, True), ("loop-L2-cem-removed", cem_removed, True),
               ("loop-L3-cem-tampered", cem_tampered, True), ("loop-L4-local-outcome-removed", outcome_removed, False)]]
    fresh = [loop.run("loop-L5-verify-missing", omit=("DOGFOOD_VERIFY_FILE",))[2],
             loop.run("loop-L6-citations-missing", omit=("DOGFOOD_CITATIONS",))[2]]
    copy_ok = controls[0]["verdict"] == "COMPLETE"
    past_bind_ok = copy_ok and controls[1]["verdict"] == "COMPLETE"
    # L3c controls for a commit past the bind and the change rerun, which L1-L3 make and L4 does not.
    gates = {"loop-L4-local-outcome-removed": copy_ok}
    for m in copies:
        expect(m, m["variant"], m["refusal"], gates.get(m["variant"], past_bind_ok))
    for m in fresh:
        expect(m, m["variant"], m["refusal"], positives[0][2]["verdict"] == "COMPLETE")
    return {"base": base, "target": target, "positiveRuns": [p[2] for p in positives], "controls": controls,
            "latency": timing([p[2]["wallMs"] for p in positives]),
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
    rows, problem = read_observations(path)
    problems = [problem] if problem else cost_problems(rows)
    if problems:
        # Only the class names reach the result: an invalid row is never copied into a sealed file.
        return {"invalidObservations": problems, "savingsClaim": "measured, no savings claim"}
    rows = [{k: r[k] for k in COST_FIELDS} for r in rows]
    arms = {a: [r for r in rows if r["arm"] == a] for a in ARMS}
    tokens = {a: [r["completeTaskTokens"] for r in v] for a, v in arms.items()}
    med = {a: statistics.median(v) for a, v in tokens.items()}
    p75 = {a: statistics.quantiles(v, n=4)[2] if len(v) > 1 else v[0] for a, v in tokens.items()}
    failures = {a: sum(1 for r in v if r["humanFailure"]) / len(v) for a, v in arms.items()}
    met = (med["treatment"] <= 0.75 * med["baseline"] and p75["treatment"] <= 0.80 * p75["baseline"]
           and failures["treatment"] == 0 and all(o["outcome"] == "PASS" for o in outcomes.values()))
    return {"observations": rows, "medianCompleteTaskTokens": med, "p75CompleteTaskTokens": p75,
            "humanFailureRate": failures, "savingsClaim": "threshold met" if met else "measured, no savings claim"}


# The observations file is operator input: read it under a byte and row bound, and report prose,
# a non-object row or an over-bound file as a named class instead of raising.
def read_observations(path):
    with open(path, "rb") as f:
        data = f.read(MAX_OBSERVATION_BYTES + 1)
    if len(data) > MAX_OBSERVATION_BYTES:
        return None, "observations-over-bound"
    try:
        lines = [l for l in data.decode("utf-8").splitlines() if l.strip()]
    except UnicodeDecodeError:
        return None, "observations-unparseable"
    if len(lines) > MAX_OBSERVATION_ROWS:
        return None, "observations-over-bound"
    rows = []
    for line in lines:
        try:
            rows.append(json.loads(line))
        except (ValueError, RecursionError):
            return None, "observations-unparseable"
    if any(not isinstance(r, dict) for r in rows):
        return None, "observation-not-object"
    return rows, None


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


def write_receipts(repo, run_id, candidate, digests, outcomes, paths):
    out = os.path.join(HERE, "receipts", run_id)
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
        written.append({"path": os.path.relpath(path, repo), "sha256": sha256_file(path)})
    return written


# ---------------------------------------------------------------- commands

def build_candidate(repo, candidate, work):
    src = os.path.join(work, "candidate-src")
    os.makedirs(src)
    archive = subprocess.run(["git", "-C", repo, "archive", "--format=tar", candidate], capture_output=True, check=True, env=ENV)
    subprocess.run(["tar", "-x", "-C", src], input=archive.stdout, check=True)
    binary = os.path.join(work, "corvint")
    env = dict(ENV, GOTOOLCHAIN="local")
    subprocess.run(["go", "build", "-trimpath", "-o", binary, "./cmd/corvint"], cwd=src, env=env, check=True)
    version = subprocess.run([binary, "--version"], capture_output=True, text=True, check=True).stdout.strip()
    return binary, version


def cmd_corpus(args):
    corpus = derive_corpus(args.repo, args.head)
    with open(os.path.join(HERE, "corpus.json"), "wb") as f:
        f.write(canonical(corpus))
    print(sha256_file(os.path.join(HERE, "corpus.json")))


# The committed preregistration.sha256 is the seal; the --prereg-sha256 argument must agree with it.
def sealed_preregistration_digest():
    path = os.path.join(HERE, "preregistration.sha256")
    if not os.path.isfile(path):
        sys.exit("daily-loop: REFUSE preregistration-seal-missing")
    with open(path) as f:
        text = f.read()
    if not SEALED_LINE.fullmatch(text):
        sys.exit("daily-loop: REFUSE preregistration-seal-malformed")
    return text.strip()


def cmd_run(args):
    if not RUN_ID.fullmatch(args.run_id):
        sys.exit("daily-loop: REFUSE run-id-invalid")
    seal = sealed_preregistration_digest()
    prereg_path = os.path.join(HERE, "preregistration.json")
    prereg_digest = sha256_file(prereg_path)
    if prereg_digest != seal:
        sys.exit("daily-loop: REFUSE preregistration-digest-mismatch")
    if args.prereg_sha256 != prereg_digest:
        sys.exit("daily-loop: REFUSE preregistration-digest-argument-mismatch")
    if os.path.exists(os.path.join(HERE, "runs", args.run_id + ".json")):
        sys.exit("daily-loop: REFUSE run-id-already-used")
    prereg = json.load(open(prereg_path))
    corpus_path = os.path.join(HERE, "corpus.json")
    for name, path in (("corpusSha256", corpus_path), ("harnessSha256", os.path.abspath(__file__))):
        if sha256_file(path) != prereg["sealed"][name]:
            sys.exit(f"daily-loop: REFUSE {name}-mismatch")
    corpus = json.load(open(corpus_path))
    if canonical(derive_corpus(args.repo, corpus["head"])) != canonical(corpus):
        sys.exit("daily-loop: REFUSE corpus-rederivation-mismatch")
    candidate = git(args.repo, "rev-parse", prereg["candidateCommit"] + "^{commit}").strip()
    work = tempfile.mkdtemp(prefix="daily-loop-", dir=args.work)
    corvint, version = build_candidate(args.repo, candidate, work)
    b = Bench(args.repo, work, prereg["runsPerMeasurement"], corvint)
    subprocess.run(["git", "clone", "-q", "--no-checkout", args.repo, b.wt], check=True, env=ENV)
    started = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    load_start = os.getloadavg()
    cases = corpus["cases"]
    orientation = [orientation_case(b, c) for c in cases if c["orientation"]["eligible"]]
    consequence = [consequence_case(b, c) for c in cases]
    cems = [cem_case(b, c["commit"], x) for c in cases for x in c["completion"]]
    target = git(args.repo, "rev-parse", args.rehearsal_target + "^{commit}").strip()
    loop = loop_rehearsal(b, candidate, target, prereg["rehearsal"]["intent"])
    outcomes = job_outcomes(orientation, consequence, cems, loop)
    result = {"profile": "corvint-daily-loop-result/0", "runId": args.run_id, "startedAt": started,
              "finishedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "preregistrationSha256": prereg_digest, "corpusSha256": prereg["sealed"]["corpusSha256"],
              "harnessSha256": prereg["sealed"]["harnessSha256"], "candidateCommit": candidate,
              "candidateBinarySha256": sha256_file(corvint), "candidateVersion": version,
              "rehearsalTarget": target,
              "host": {"system": platform.system(), "machine": platform.machine(), "python": platform.python_version(),
                       "cpus": os.cpu_count(), "loadAverageStart": load_start, "loadAverageEnd": os.getloadavg(),
                       "go": subprocess.run(["go", "env", "GOVERSION"], capture_output=True, text=True,
                                            env=dict(ENV, GOTOOLCHAIN="local")).stdout.strip()},
              "runsPerMeasurement": b.runs,
              "orientation": orientation, "consequence": consequence,
              "completion": {"cem": cems, "loop": loop,
                             "plainGitBaseline": plain_git_completion(b, candidate, target)},
              "outcomes": outcomes, "cost": agent_cost(args.agent_observations, outcomes)}
    out = os.path.join(HERE, "runs", args.run_id + ".json")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "wb") as f:
        f.write(canonical(result))
    rel = {"preregistration": os.path.relpath(prereg_path, args.repo), "corpus": os.path.relpath(corpus_path, args.repo),
           "result": os.path.relpath(out, args.repo)}
    digests = {"preregistration": prereg_digest, "corpus": prereg["sealed"]["corpusSha256"],
               "result": sha256_file(out)}
    receipts = write_receipts(args.repo, args.run_id, candidate, digests, outcomes, rel)
    print(json.dumps({"result": rel["result"], "resultSha256": digests["result"], "outcomes": outcomes,
                      "receipts": receipts}, indent=1))
    if not args.keep_work:
        shutil.rmtree(work, ignore_errors=True)


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = p.add_subparsers(dest="command", required=True)
    c = sub.add_parser("corpus", help="derive corpus.json from first-parent history")
    c.add_argument("--repo", required=True)
    c.add_argument("--head", required=True)
    r = sub.add_parser("run", help="run the sealed benchmark")
    r.add_argument("--repo", required=True)
    r.add_argument("--prereg-sha256", required=True)
    r.add_argument("--run-id", required=True)
    r.add_argument("--rehearsal-target", required=True)
    r.add_argument("--work", required=True)
    r.add_argument("--agent-observations")
    r.add_argument("--keep-work", action="store_true")
    args = p.parse_args()
    args.repo = os.path.abspath(getattr(args, "repo", "."))
    {"corpus": cmd_corpus, "run": cmd_run}[args.command](args)


if __name__ == "__main__":
    main()
