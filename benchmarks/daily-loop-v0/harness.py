#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Daily change-evidence loop sealed benchmark V0 (V1-0012, PCCO-V0-015..017).

`corpus` derives the immutable corpus from `git log --first-parent` under the
preregistered rules. `run` verifies the sealed preregistration, builds the
candidate from its commit, runs every treatment and baseline, scores it, and
writes one result plus three corvint-use-case-evidence/0 receipts. Nothing here
estimates a value it cannot observe: live-agent dimensions stay NOT_OBSERVED
unless an observation file is supplied.
"""
import argparse
import hashlib
import json
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
REL = "benchmarks/daily-loop-v0"
EXCLUDED_PREFIXES = (".taskman/", ".corvint/")
EXCLUDED_FILES = {"docs/BUILD-LOG.md", "docs/specs/INDEX.json", "docs/specs/REQUIREMENTS.tsv",
                  "docs/specs/README.md", "docs/decisions/README.md"}
BIND_SUBJECT = re.compile(r"^chore: (bind|seal|rebind)\b")
CONVENTIONAL = re.compile(r"^[a-z]+(\([^)]*\))?!?: ")
STOPWORDS = {"the", "and", "for", "with", "from", "into", "onto", "that", "this", "their", "them"}
ORIENTATION_MAX_FILES = 100
PACKET_LIMIT = 20
USE_CASES = {"orientation": "UC-TASK-ORIENTATION", "consequence": "UC-CHANGE-CONSEQUENCE",
             "completion": "UC-EVIDENCE-CARRYING-COMPLETION"}
CITATION_SPAN = "26:28"


def sha256_bytes(raw):
    return hashlib.sha256(raw).hexdigest()


def sha256_file(path):
    with open(path, "rb") as f:
        return sha256_bytes(f.read())


def canonical(value):
    return (json.dumps(value, sort_keys=True, indent=2, ensure_ascii=False) + "\n").encode()


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True, check=True).stdout


def execute(argv, cwd, env=None, timeout=1800):
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
                             capture_output=True, text=True).stdout
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
                             capture_output=True, text=True).stdout
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
        subprocess.run(["git", "-C", self.wt, "checkout", "-q", "--detach", "-f", rev], check=True)
        subprocess.run(["git", "-C", self.wt, "clean", "-qfdx"], check=True)

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
    selected = [] if abstained else sorted({u["unitId"][3:].split("::")[0] for u in doc["plan"]["selected"]
                                            if u["unitId"].startswith("go:")})
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
                          "selectedGoPackages": len(selected), "selected": selected},
            "impactRange": {"argv": ["corvint", "impact", "--base", case["parent"], "--range-profile", "expanded-256",
                                     "--limit", str(PACKET_LIMIT)],
                            "runs": [observation(r) for r in impact], "latency": timing([r["wallMs"] for r in impact]),
                            "ok": idoc.get("ok"), "criticalMissingReported": len(coverage.get("critical_missing", [])),
                            "uncertainty": coverage.get("uncertainty")},
            "baseline": {"procedure": "changed-packages-plus-direct-importers", "selectedGoPackages": len(baseline),
                         "latency": timing(walls)},
            "score": score(c["critical"], selected, baseline)}


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
                    "commit", "-q", "--allow-empty", "-am", message], check=True)
    return git(wt, "rev-parse", "HEAD").strip()


def reasons(r):
    doc = parse_json(r["stdout"]) or parse_json(r["stderr"]) or {}
    found = [doc["code"]] if "code" in doc else []
    found += [i.get("code") for i in (doc.get("verification") or {}).get("issues", [])]
    found += [i.get("code") if isinstance(i, dict) else i for i in doc.get("policyIssues", [])]
    return found


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
            "missingEvidence": {k: {"observation": observation(v), "reasons": reasons(v),
                                    "verdict": "COMPLETE" if v["exit"] == 0 else "REFUSED",
                                    "informative": control_complete}
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

    def env(self, omit):
        env = dict(os.environ, DOGFOOD_OUTCOME="passed",
                   DOGFOOD_TASK="Preregister the daily-loop sealed benchmark corpus and harness.")
        env.update({k: v for k, v in self.files.items() if k not in omit})
        return env

    def clone(self, name):
        d = os.path.join(self.b.work, name)
        subprocess.run(["git", "clone", "-q", self.b.repo, d], check=True)
        subprocess.run(["git", "-C", d, "checkout", "-q", "-B", "rehearsal", self.target], check=True)
        subprocess.run(["git", "-C", d, "config", "user.name", "bench"], check=True)
        subprocess.run(["git", "-C", d, "config", "user.email", "bench@invalid"], check=True)
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
        subprocess.run(["git", "-C", d, "add", "-f", ".corvint/change.cem.json"], check=False)
        subprocess.run(["git", "-C", d, "commit", "-q", "-m", "chore: bind change evidence"], check=False)
        self.step(d, "dogfood-change", env, log)
        check = self.step(d, "dogfood-check", env, log)
        return d, env, {"variant": name, "omittedInputs": sorted(omit), "hunks": hunks, "steps": log,
                        "invocations": len(log), "failedInvocations": sum(1 for s in log if s["exit"] != 0),
                        "unexpectedFailures": sum(1 for s in log[2:] if s["exit"] != 0),
                        "wallMs": sum(s["wallMs"] for s in log), "verdict": verdict(check), "refusal": refusal(check)}

    def mutate_and_check(self, source, name, env, mutate):
        d = os.path.join(self.b.work, name)
        shutil.copytree(source, d, symlinks=True)
        mutate(d)
        log = []
        check = self.step(d, "dogfood-check", env, log)
        return {"variant": name, "steps": log, "verdict": verdict(check), "refusal": refusal(check)}


def verdict(r):
    return "COMPLETE" if r["exit"] == 0 and b"dogfood-check: PASS" in r["stdout"] + r["stderr"] else "REFUSED"


def refusal(r):
    text = (r["stdout"] + r["stderr"]).decode(errors="replace")
    return re.findall(r"^dogfood-(?:change|check): (?:FAIL|REFUSE) \S+", text, re.M)


def commit_all(d, message):
    subprocess.run(["git", "-C", d, "commit", "-q", "-am", message], check=True)


def loop_rehearsal(b, base, target, spec):
    loop = Loop(b, base, target, spec)
    positives = [loop.run(f"loop-L0-{i}") for i in range(1, b.runs + 1)]
    source, env, _ = positives[0]
    probe = next(f for f in git(b.repo, "diff", "--name-only", "--diff-filter=AM", base, target).splitlines()
                 if not excluded(f))

    def stale(d):
        with open(os.path.join(d, probe), "ab") as f:
            f.write(b"\n")
        commit_all(d, "stale probe")

    def cem_removed(d):
        subprocess.run(["git", "-C", d, "rm", "-q", ".corvint/change.cem.json"], check=True)
        commit_all(d, "remove change evidence")

    def rewrite_cem(d, fn, message):
        p = os.path.join(d, ".corvint", "change.cem.json")
        cem = json.load(open(p))
        fn(cem)
        with open(p, "wb") as f:
            f.write(canonical(cem))
        subprocess.run(["git", "-C", d, "commit", "-q", "-am", message], capture_output=True)

    def reencoded(d):
        rewrite_cem(d, lambda cem: None, "re-encode change evidence")

    def cem_tampered(d):
        rewrite_cem(d, lambda cem: cem["hunks"][0].update(basis=[]), "tamper change evidence")

    def outcome_removed(d):
        os.remove(os.path.join(d, ".git", "corvint", "local-outcome.json"))
    controls = [loop.mutate_and_check(source, "loop-L0c-copy-control", env, lambda d: None),
                loop.mutate_and_check(source, "loop-L3c-reencoded-control", env, reencoded)]
    copies = [loop.mutate_and_check(source, n, env, fn) for n, fn in
               [("loop-L1-stale-after-bind", stale), ("loop-L2-cem-removed", cem_removed),
                ("loop-L3-cem-tampered", cem_tampered), ("loop-L4-local-outcome-removed", outcome_removed)]]
    fresh = [loop.run("loop-L5-verify-missing", omit=("DOGFOOD_VERIFY_FILE",))[2],
             loop.run("loop-L6-citations-missing", omit=("DOGFOOD_CITATIONS",))[2]]
    copy_ok = all(c["verdict"] == "COMPLETE" for c in controls)
    for m in copies:
        m["informative"] = copy_ok
    for m in fresh:
        m["informative"] = positives[0][2]["verdict"] == "COMPLETE"
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
    rows = [json.loads(l) for l in open(path) if l.strip()]
    arms = {}
    for r in rows:
        arms.setdefault(r["arm"], []).append(r["completeTaskTokens"])
    med = {a: statistics.median(v) for a, v in arms.items()}
    p75 = {a: statistics.quantiles(v, n=4)[2] if len(v) > 1 else v[0] for a, v in arms.items()}
    met = ("treatment" in med and "baseline" in med and med["treatment"] <= 0.75 * med["baseline"]
           and p75["treatment"] <= 0.80 * p75["baseline"] and all(o["outcome"] == "PASS" for o in outcomes.values()))
    return {"observations": rows, "medianCompleteTaskTokens": med, "p75CompleteTaskTokens": p75,
            "humanFailureRate": {a: sum(1 for r in rows if r["arm"] == a and r.get("humanFailure")) / len(v)
                                 for a, v in arms.items()},
            "savingsClaim": "threshold met" if met else "measured, no savings claim"}


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
    archive = subprocess.run(["git", "-C", repo, "archive", "--format=tar", candidate], capture_output=True, check=True)
    subprocess.run(["tar", "-x", "-C", src], input=archive.stdout, check=True)
    binary = os.path.join(work, "corvint")
    env = dict(os.environ, GOTOOLCHAIN="local")
    subprocess.run(["go", "build", "-trimpath", "-o", binary, "./cmd/corvint"], cwd=src, env=env, check=True)
    version = subprocess.run([binary, "--version"], capture_output=True, text=True, check=True).stdout.strip()
    return binary, version


def cmd_corpus(args):
    corpus = derive_corpus(args.repo, args.head)
    with open(os.path.join(HERE, "corpus.json"), "wb") as f:
        f.write(canonical(corpus))
    print(sha256_file(os.path.join(HERE, "corpus.json")))


def cmd_run(args):
    prereg_path = os.path.join(HERE, "preregistration.json")
    if sha256_file(prereg_path) != args.prereg_sha256:
        sys.exit("daily-loop: REFUSE preregistration-digest-mismatch")
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
    subprocess.run(["git", "clone", "-q", "--no-checkout", args.repo, b.wt], check=True)
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
              "preregistrationSha256": args.prereg_sha256, "corpusSha256": prereg["sealed"]["corpusSha256"],
              "harnessSha256": prereg["sealed"]["harnessSha256"], "candidateCommit": candidate,
              "candidateBinarySha256": sha256_file(corvint), "candidateVersion": version,
              "rehearsalTarget": target,
              "host": {"system": platform.system(), "machine": platform.machine(), "python": platform.python_version(),
                       "cpus": os.cpu_count(), "loadAverageStart": load_start, "loadAverageEnd": os.getloadavg(),
                       "go": subprocess.run(["go", "env", "GOVERSION"], capture_output=True, text=True,
                                            env=dict(os.environ, GOTOOLCHAIN="local")).stdout.strip()},
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
    digests = {"preregistration": args.prereg_sha256, "corpus": prereg["sealed"]["corpusSha256"],
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
