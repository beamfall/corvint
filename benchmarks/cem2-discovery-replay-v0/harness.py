# SPDX-License-Identifier: AGPL-3.0-or-later
"""V1-2600 discovery replay harness (VRP-V0-030..049). Pre-registered in PROTOCOL.md.

  python3 -I harness.py check                     manifest digest + threshold status (exit 3 = OWNER-BLOCKER)
  python3 -I harness.py plan  --repo R --work W --out O [--corvint BIN]   print the exact per-change commands
  python3 -I harness.py run   --repo R --work W --out O --corvint BIN [--only VRP-Cnnn]
  python3 -I harness.py score --out O             fold labels and apply the decision rule
  python3 -I harness.py selftest                  synthetic fold/decision checks; reads no replay output

run and score refuse while thresholds.json holds OWNER-BLOCKER, while PROTOCOL.md, thresholds.json or
the manifest differ from the committed HEAD of --repo, and (run) for a change whose output directory
already exists: the first observation is kept (benchmarks/README.md).
"""
import hashlib, json, os, re, shlex, subprocess, sys, time

HERE = os.path.dirname(os.path.abspath(__file__))
MANIFEST, DIGEST = os.path.join(HERE, "selection-manifest.json"), os.path.join(HERE, "selection-manifest.sha256")
THRESHOLDS = os.path.join(HERE, "thresholds.json")
REL = "benchmarks/cem2-discovery-replay-v0/"
MAX_MUTANTS, WALL = 8, "1800s"           # VRP-V0-025: per-hunk mutants, per-change discriminate budget
COVER_TIMEOUT = 1800                       # seconds, the baseline go test run
MIN_PAIRED = 0.8                           # VRP-V0-052: run validity floor
# cite, mark, cover and discriminate write only a repository-relative --output (CEM-V1-647), so the
# maps live in the disposable worktree and are copied to the output directory after each step.
MAP, COVER_MAP, DISC_MAP = ".corvint/change.cem.json", ".corvint/vrp-cover.cem.json", ".corvint/vrp-discriminate.cem.json"


def keep(w, rel, o, name):
    try:
        with open(os.path.join(w, rel), "rb") as src:
            open(os.path.join(o, name), "wb").write(src.read())
    except OSError:
        pass


def die(code, message):
    print(message, file=sys.stderr)
    sys.exit(code)


def sha256(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()


def check(strict=False):
    recorded = open(DIGEST).read().split()[0]
    if sha256(MANIFEST) != recorded:
        die(2, "manifest-digest-mismatch: selection-manifest.json does not match selection-manifest.sha256")
    thresholds = json.load(open(THRESHOLDS))
    blocked = [k for k in ("X", "Y") if not isinstance(thresholds.get(k), (int, float))]
    print(json.dumps({"manifestSha256": recorded, "thresholds": thresholds, "blocked": blocked}, sort_keys=True))
    if blocked and strict:
        die(3, "OWNER-BLOCKER: thresholds " + ",".join(blocked) + " are not set in thresholds.json")
    return thresholds


def committed(repo):
    """VRP-V0-003: the protocol, thresholds and manifest must be committed and unmodified."""
    for name in ("PROTOCOL.md", "thresholds.json", "selection-manifest.json", "selection-manifest.sha256"):
        path = REL + name
        if subprocess.run(["git", "-C", repo, "ls-files", "--error-unmatch", path], capture_output=True).returncode:
            die(4, f"protocol-not-committed: {path}")
        if subprocess.run(["git", "-C", repo, "diff", "--quiet", "HEAD", "--", path]).returncode:
            die(4, f"protocol-modified: {path} differs from HEAD")


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], check=True, capture_output=True, text=True).stdout


def tests_for(repo, change):
    """Oracle test files (VRP-V0-021): every _test.go at the change in its source packages."""
    dirs = sorted({os.path.dirname(p) for p in change["changeSourceFiles"]})
    oracle, base_cited, added = {}, {}, {}
    for d in dirs:
        listed = git(repo, "ls-tree", "--name-only", change["change"], (d + "/") if d else "./").split()
        files = sorted(p for p in listed if p.endswith("_test.go"))
        oracle[d] = files
        base_cited[d], added[d] = [], []
        for p in files:
            at_base = subprocess.run(["git", "-C", repo, "cat-file", "-e", f'{change["base"]}:{p}'],
                                     capture_output=True).returncode == 0
            (base_cited if at_base else added)[d].append(p)
    return dirs, oracle, base_cited, added


def plan(repo, work, out, corvint, change):
    """Return [(step, argv, cwd, timeout)] for one change. Hunk-dependent steps are built by run."""
    cid, w, o = change["id"], os.path.join(work, change["id"]), os.path.join(out, change["id"])
    dirs, _, _, _ = tests_for(repo, change)
    pkgs = ["./" + d if d else "." for d in dirs]
    return [
        ("worktree", ["git", "-C", repo, "worktree", "add", "--detach", w, change["change"]], None, 300),
        ("prepare", [corvint, "--root", w, "cem", "prepare", "--base", change["base"], "--target", change["change"],
                     "--patch", os.path.join(o, "change.patch")], None, 600),
        ("cite", ["<cem cite / cem mark per hunk, built from the prepared map; see PROTOCOL.md 4.2>"], None, 600),
        ("coverage", ["go", "test", "-count=1", "-coverprofile=" + os.path.join(o, "cover.out"),
                      "-coverpkg=" + ",".join(pkgs), *pkgs], w, COVER_TIMEOUT),
        ("cover-map", [corvint, "--root", w, "cem", "cover", "--map", MAP,
                       "--coverprofile", os.path.join(o, "cover.out"), "--test-run", "vrp-baseline-" + cid,
                       "--output", COVER_MAP,
                       "--states-output", os.path.join(o, "coverage-states.json")], None, 600),
        ("hunk-revert", [corvint, "--root", w, "cem", "verify", "--map", MAP,
                         "--expected-base", change["base"], "--target", change["change"], "--artifacts", w,
                         "--hunk-revert"], None, 1500),
        ("discriminate", [corvint, "--root", w, "cem", "discriminate", "--map", MAP,
                          "--target", change["change"], "--max-hunks", str(change["changeSourceHunks"] + 64),
                          "--max-mutants", str(MAX_MUTANTS), "--wall-time", WALL,
                          "<--same-change-test SELECTOR=PATH,... for added oracle files; see PROTOCOL.md 4.2>",
                          "--output", DISC_MAP], None, 2100),
        ("cleanup", ["git", "-C", repo, "worktree", "remove", "--force", w], None, 300),
    ]


def run_step(o, name, argv, cwd, timeout, log):
    start = time.monotonic()
    try:
        done = subprocess.run(argv, cwd=cwd, capture_output=True, timeout=timeout)
        code, stdout, stderr = done.returncode, done.stdout, done.stderr
    except subprocess.TimeoutExpired as e:
        code, stdout, stderr = "timeout", e.stdout or b"", e.stderr or b""
    elapsed = time.monotonic() - start
    open(os.path.join(o, name + ".stdout"), "ab").write(stdout)
    open(os.path.join(o, name + ".stderr"), "ab").write(stderr)
    log.append({"step": name, "argv": argv, "exit": code, "seconds": round(elapsed, 3)})
    return code, stdout


def cite_steps(repo, corvint, change, w, o, dirs, base_cited, added):
    """VRP-V0-022: cite every base oracle file in the hunk's package as a whole-file test-claim."""
    doc = json.load(open(os.path.join(w, ".corvint", "change.cem.json")))
    steps, same = [], []
    mapped = MAP
    for ordinal, hunk in enumerate(doc["hunks"], 1):
        path = hunk["path"]
        d = os.path.dirname(path)
        source = path.endswith(".go") and not path.endswith("_test.go") and d in base_cited
        cites = base_cited.get(d, []) if source else []
        for t in cites:
            lines = git(repo, "show", f'{change["base"]}:{t}').count("\n")
            steps.append([corvint, "--root", w, "cem", "cite", "--map", mapped, "--hunk", str(ordinal),
                          "--evidence-path", t, "--lines", f"1:{lines}", "--relation", "test-claim",
                          "--output", mapped])
        if not cites:
            steps.append([corvint, "--root", w, "cem", "mark", "--map", mapped, "--hunk", str(ordinal),
                          "--disposition", "unknown", "--reason", "VRP replay: no base oracle test cited",
                          "--output", mapped])
        if source:
            same += [f"{ordinal}={t}" for t in added.get(d, [])]
    return steps, same


def cmd_run(args):
    thresholds = check(strict=True)
    committed(args.repo)
    manifest = json.load(open(MANIFEST))
    for change in manifest["changes"]:
        if args.only and change["id"] != args.only:
            continue
        o = os.path.join(args.out, change["id"])
        if os.path.exists(o):
            print(f"{change['id']}: output exists, first observation kept; skipped", file=sys.stderr)
            continue
        os.makedirs(o)
        w = os.path.join(args.work, change["id"])
        dirs, oracle, base_cited, added = tests_for(args.repo, change)
        log, steps = [], plan(args.repo, args.work, args.out, args.corvint, change)
        for name, argv, cwd, timeout in steps:
            if name == "cite":
                cmds, same = cite_steps(args.repo, args.corvint, change, w, o, dirs, base_cited, added)
                for argv2 in cmds:
                    run_step(o, "cite", argv2, None, timeout, log)
                keep(w, MAP, o, "map.json")
                continue
            if name == "discriminate":
                argv = [a for a in argv if not a.startswith("<")]
                if same:
                    argv[-2:-2] = ["--same-change-test", ",".join(same)]
            code, stdout = run_step(o, name, argv, cwd, timeout, log)
            if name == "hunk-revert":
                open(os.path.join(o, "hunk-revert.json"), "wb").write(stdout)
            if name == "cover-map":
                keep(w, COVER_MAP, o, "map.cover.json")
            if name == "discriminate":
                keep(w, DISC_MAP, o, "map.discriminate.json")
            if name in ("worktree", "prepare") and code != 0:
                break
        try:
            module = re.search(r"^module (\S+)", open(os.path.join(w, "go.mod")).read(), re.M)[1]
        except (OSError, TypeError):
            module = None
        version = subprocess.run([args.corvint, "--version"], capture_output=True, text=True).stdout.strip()
        json.dump({"change": change["id"], "module": module, "corvintVersion": version,
                   "goVersion": subprocess.run(["go", "env", "GOVERSION"], capture_output=True, text=True).stdout.strip(), "oracle": oracle, "baseCited": base_cited, "sameChange": added,
                   "thresholds": thresholds, "steps": log,
                   "replaySeconds": round(sum(s["seconds"] for s in log if s["step"] not in ("worktree", "cleanup")), 3)},
                  open(os.path.join(o, "run.json"), "w"), indent=2, sort_keys=True)
        if os.path.exists(w):
            run_step(o, "cleanup", ["git", "-C", args.repo, "worktree", "remove", "--force", w], None, 300, [])


# ---- scoring (VRP-V0-040..059) -------------------------------------------------------------------

HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")


def added_lines(patch):
    """{(path, newStart): [added new-side line numbers]} from the canonical patch."""
    rows, path, key, line = {}, None, None, 0
    for text in patch.splitlines():
        if text.startswith("+++ "):
            path = text[6:] if text.startswith("+++ b/") else None
        elif HUNK.match(text):
            m = HUNK.match(text)
            line, key = int(m[1]), (path, int(m[1]))
            rows[key] = []
        elif key and text.startswith("+"):
            rows[key].append(line); line += 1
        elif key and text.startswith(" "):
            line += 1
    return rows


def coverage(profile, module):
    """{path: (executable lines, executed lines)} from a Go coverprofile."""
    out = {}
    for text in profile.splitlines()[1:]:
        m = re.match(r"^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$", text)
        if not m:
            continue
        path = m[1][len(module) + 1:] if m[1].startswith(module + "/") else m[1]
        execu, done = out.setdefault(path, (set(), set()))
        span = range(int(m[2]), int(m[3]) + 1)
        execu.update(span)
        if int(m[4]) > 0:
            done.update(span)
    return out


def baseline(hunk_added, cov, path):
    """VRP-V0-030: primary flag = an executable added line not executed; secondary = none executed."""
    if cov is None:
        return "NOT_OBSERVED(no-coverprofile)", None, None
    if path not in cov:
        return "NOT_OBSERVED(not-instrumented)", None, None
    execu, done = cov[path]
    lines = [x for x in hunk_added if x in execu]
    if not lines:
        return "no-executable-line", False, False
    missed = [x for x in lines if x not in done]
    return f"{len(lines) - len(missed)}/{len(lines)}", bool(missed), len(missed) == len(lines)


def typed(revert, disc, base_state, secondary):
    """VRP-V0-031: fold the existing witnesses into one V1-2602 prototype label, first match wins."""
    if revert and revert.get("result") == "held":
        return "discriminated(hunk-revert)"
    if disc and disc.get("state") == "discriminates":
        return "discriminated(mutant-kill)"
    if revert and revert.get("reason") == "revert-breaks-build":
        return "type-discriminated"
    if secondary is True:
        return "uncovered(no-executed-line)"
    if base_state == "no-executable-line":
        return "uncovered(no-executable-line)"
    if base_state and base_state.startswith("NOT_OBSERVED(not-instrumented"):
        return "uncovered(not-instrumented)"
    if (revert and revert.get("result") == "survived") or (disc and disc.get("state") == "survived" and disc.get("mutants", 0) > 0):
        return "weak-test"
    reasons = [r for r in ((revert or {}).get("reason"), (disc or {}).get("detail")) if r]
    return "NOT_OBSERVED(" + ("; ".join(reasons)[:120] or "no-witness") + ")"


TYPED_FLAGGED = ("weak-test", "uncovered(no-executed-line)")


def touched(hunks, lines):
    out = set()
    for index, h in enumerate(hunks):
        start, count = h["newRange"]["start"], h["newRange"]["count"]
        if any(p == h["path"] and start <= n < start + count for p, n in lines):
            out.add(index)
    return out


def score_change(change, o):
    """One results row per change (VRP-V0-040). Every unavailable value is NOT_OBSERVED."""
    row = {"id": change["id"], "change": change["change"], "fix": change["fix"]}
    try:
        doc = json.load(open(os.path.join(o, "map.json")))
        patch = open(os.path.join(o, "change.patch")).read()
        run = json.load(open(os.path.join(o, "run.json")))
    except (OSError, ValueError) as e:
        return {**row, "status": f"NOT_OBSERVED({type(e).__name__})"}
    module = run.get("module") or "NOT_OBSERVED"
    try:
        cov = coverage(open(os.path.join(o, "cover.out")).read(), module)
    except OSError:
        cov = None
    try:
        reverts = {r["hunkId"]: r for r in json.load(open(os.path.join(o, "hunk-revert.json")))["hunkRevert"]["hunks"]}
    except (OSError, ValueError, KeyError, TypeError):
        reverts = None
    try:
        discs = [h.get("discriminates") for h in json.load(open(os.path.join(o, "map.discriminate.json")))["hunks"]]
    except (OSError, ValueError, KeyError):
        discs = None
    added = added_lines(patch)
    units = [i for i, h in enumerate(doc["hunks"]) if h["path"].endswith(".go") and not h["path"].endswith("_test.go")]
    primary = touched(doc["hunks"], change["touchedLines"]) & set(units)
    later = set()
    for f in change["laterFixes"]:
        later |= touched(doc["hunks"], f["touchedLines"])
    rows = []
    for i in units:
        h = doc["hunks"][i]
        state, flag, secondary = baseline(added.get((h["path"], h["newRange"]["start"]), []), cov, h["path"])
        revert = reverts.get(h["id"]) if reverts is not None else None
        disc = discs[i] if discs is not None and i < len(discs) else None
        label = typed(revert, disc, state, secondary)
        rows.append({"hunk": i + 1, "path": h["path"], "baseline": state, "baselineFlag": flag,
                     "baselineSecondaryFlag": secondary, "label": label,
                     "typedFlag": None if label.startswith("NOT_OBSERVED") else label in TYPED_FLAGGED,
                     "touchedByFix": i in primary, "touchedByLaterFix": i in later})

    def capture(key):
        hit = [r for r in rows if r["touchedByFix"]]
        if not hit:
            return "NOT_OBSERVED(touch-unmapped)"
        if any(r[key] is True for r in hit):
            return True
        return False if all(r[key] is False for r in hit) else "NOT_OBSERVED(touched-unit-unobserved)"

    def alarms(key):
        flagged = [r for r in rows if r[key] is True]
        return len(flagged), sum(1 for r in flagged if not r["touchedByFix"] and not r["touchedByLaterFix"])

    return {**row, "status": "OBSERVED", "units": len(units), "touchedUnits": len(primary),
            "baselineCapture": capture("baselineFlag"), "baselineSecondaryCapture": capture("baselineSecondaryFlag"),
            "typedCapture": capture("typedFlag"),
            "baselineFlagged": alarms("baselineFlag")[0], "baselineFalseAlarms": alarms("baselineFlag")[1],
            "typedFlagged": alarms("typedFlag")[0], "typedFalseAlarms": alarms("typedFlag")[1],
            "replaySeconds": run.get("replaySeconds", "NOT_OBSERVED"),
            "dogfoodSeconds": "NOT_OBSERVED", "hunks": rows}


def decide(results, X, Y, selected):
    """VRP-V0-050..054: the pre-registered decision rule."""
    paired = [r for r in results if isinstance(r.get("typedCapture"), bool) and isinstance(r.get("baselineCapture"), bool)]
    b = sum(1 for r in paired if r["typedCapture"] and not r["baselineCapture"])
    c = sum(1 for r in paired if r["baselineCapture"] and not r["typedCapture"])
    flagged = sum(r["typedFlagged"] for r in results if r.get("status") == "OBSERVED")
    false = sum(r["typedFalseAlarms"] for r in results if r.get("status") == "OBSERVED")
    bflag = sum(r["baselineFlagged"] for r in results if r.get("status") == "OBSERVED")
    bfalse = sum(r["baselineFalseAlarms"] for r in results if r.get("status") == "OBSERVED")
    summary = {"selected": selected, "paired": len(paired), "typedOnly": b, "baselineOnly": c,
               "typedCapture": sum(r["typedCapture"] for r in paired) / len(paired) if paired else "NOT_OBSERVED",
               "baselineCapture": sum(r["baselineCapture"] for r in paired) / len(paired) if paired else "NOT_OBSERVED",
               "delta": (b - c) / len(paired) if paired else "NOT_OBSERVED",
               "typedFalseAlarmRate": false / flagged if flagged else "NOT_OBSERVED",
               "baselineFalseAlarmRate": bfalse / bflag if bflag else "NOT_OBSERVED",
               "X": X, "Y": Y}
    if len(paired) < MIN_PAIRED * selected:
        summary["verdict"] = "INVALID_RUN"
    elif (isinstance(summary["typedFalseAlarmRate"], float) and summary["delta"] >= X
          and summary["typedFalseAlarmRate"] <= Y and b > c):
        summary["verdict"] = "CONTINUE"
    else:
        summary["verdict"] = "KILL"
    return summary


def cmd_score(args):
    thresholds = check(strict=True)
    committed(args.repo)
    manifest = json.load(open(MANIFEST))
    results = [score_change(c, os.path.join(args.out, c["id"])) for c in manifest["changes"]]
    summary = decide(results, thresholds["X"], thresholds["Y"], manifest["selected"])
    json.dump({"profile": "corvint-vrp-results/0", "manifestSha256": sha256(MANIFEST), "summary": summary,
               "changes": results}, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")


def cmd_selftest():
    """Synthetic witnesses only: proves the fold and the rule are executable as registered."""
    assert typed({"result": "held"}, None, "1/1", False) == "discriminated(hunk-revert)"
    assert typed({"result": "not-run", "reason": "revert-breaks-build"}, None, "1/1", False) == "type-discriminated"
    assert typed(None, {"state": "survived", "mutants": 3}, "2/2", False) == "weak-test"
    assert typed(None, {"state": "survived", "mutants": 3}, "0/2", True) == "uncovered(no-executed-line)"
    assert typed(None, {"state": "not-run", "detail": "budget"}, "2/2", False).startswith("NOT_OBSERVED")
    assert baseline([3, 4, 5], {"a.go": ({3, 4}, {3})}, "a.go") == ("1/2", True, False)
    assert baseline([9], {"a.go": ({3}, {3})}, "a.go") == ("no-executable-line", False, False)
    mk = lambda t, bc, f, fa: {"status": "OBSERVED", "typedCapture": t, "baselineCapture": bc,
                               "typedFlagged": f, "typedFalseAlarms": fa, "baselineFlagged": f, "baselineFalseAlarms": fa}
    rows = [mk(True, False, 2, 1)] * 8 + [mk(False, True, 2, 1)] * 2 + [mk(True, True, 2, 1)] * 30
    assert decide(rows, 0.15, 0.6, 40)["verdict"] == "CONTINUE"
    assert decide(rows, 0.20, 0.6, 40)["verdict"] == "KILL"
    assert decide(rows, 0.15, 0.4, 40)["verdict"] == "KILL"
    assert decide(rows[:20], 0.15, 0.6, 40)["verdict"] == "INVALID_RUN"
    print("selftest: ok")


def main():
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["check", "plan", "run", "score", "selftest"])
    parser.add_argument("--repo"); parser.add_argument("--work"); parser.add_argument("--out")
    parser.add_argument("--corvint", default="corvint"); parser.add_argument("--only")
    args = parser.parse_args()
    if args.command == "check":
        check()
    elif args.command == "selftest":
        cmd_selftest()
    elif args.command == "plan":
        for change in json.load(open(MANIFEST))["changes"]:
            print(f"# {change['id']} {change['change']} (fix {change['fix']})")
            for name, argv, cwd, timeout in plan(args.repo, args.work, args.out, args.corvint, change):
                print(("(cd " + shlex.quote(cwd) + " && " if cwd else "") + shlex.join(argv) + (")" if cwd else "")
                      + f"  # {name}, timeout {timeout}s")
    elif args.command == "run":
        cmd_run(args)
    else:
        cmd_score(args)


if __name__ == "__main__":
    main()
