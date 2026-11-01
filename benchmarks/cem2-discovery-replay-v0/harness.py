# SPDX-License-Identifier: AGPL-3.0-or-later
"""V1-2600 discovery replay harness (VRP-V0-030..049, 055..061). Pre-registered in PROTOCOL.md.

  python3 -I harness.py check                     manifest + registration digests, threshold status (exit 3 = OWNER-BLOCKER)
  python3 -I harness.py plan  --repo R --work W --out O [--corvint BIN]   print the exact per-change commands
  python3 -I harness.py run   --repo R --work W --out O --corvint BIN [--only VRP-Cnnn]
  python3 -I harness.py score --repo R --out O    fold labels and apply the decision rule
  python3 -I harness.py selftest                  synthetic checks in a temporary repository; reads no replay output

run and score refuse while thresholds.json holds OWNER-BLOCKER, while any registered file differs
from registration.sha256 (exit 5), while a registered file is untracked or differs from the HEAD of
--repo or this harness is not the one inside --repo (exit 4), and (run) for a change whose output
directory already exists: the first observation is kept (benchmarks/README.md). Every capture records
the registration digest; score refuses captures from another registration, change, tree or tool
build (exit 6).
"""
import hashlib, json, os, re, shlex, shutil, signal, subprocess, sys, tempfile, time

HERE = os.path.dirname(os.path.abspath(__file__))
MANIFEST, DIGEST = os.path.join(HERE, "selection-manifest.json"), os.path.join(HERE, "selection-manifest.sha256")
THRESHOLDS = os.path.join(HERE, "thresholds.json")
REGISTRATION = os.path.join(HERE, "registration.sha256")
# VRP-V0-055: the registered bytes, in registration.sha256 order (sha256sum format).
REGISTERED = ("PROTOCOL.md", "thresholds.json", "selection-manifest.json", "selection-manifest.sha256",
              "select.py", "harness.py")
REL = "benchmarks/cem2-discovery-replay-v0/"
MAX_MUTANTS, WALL = 8, "1800s"           # VRP-V0-025: per-hunk mutants, per-change discriminate budget
COVER_TIMEOUT = 1800                       # seconds, the baseline go test run
MIN_PAIRED = 0.8                           # VRP-V0-052: run validity floor
RETIRE_SECONDS = 10                        # VRP-V0-058: bound on waiting for a killed process group
GO = "go"                                  # the selftest substitutes a stub
# cite, mark, cover and discriminate write only a repository-relative --output (CEM-V1-647), so the
# maps live in the disposable worktree and are copied to the output directory after each step.
MAP, COVER_MAP, DISC_MAP = ".corvint/change.cem.json", ".corvint/vrp-cover.cem.json", ".corvint/vrp-discriminate.cem.json"
# VRP-V0-059: every artifact a capture may leave, digested into run.json.
ARTIFACTS = ("change.patch", "map.json", "cover.out", "coverage-states.json", "map.cover.json",
             "hunk-revert.json", "map.discriminate.json")


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


def registration():
    """VRP-V0-055: every registered file matches registration.sha256; returns that file's sha256."""
    try:
        rows = [line.split("  ", 1) for line in open(REGISTRATION).read().splitlines()]
    except OSError:
        die(5, "registration-missing: registration.sha256")
    if [r[1] if len(r) == 2 else None for r in rows] != list(REGISTERED):
        die(5, "registration-malformed: registration.sha256 must list " + ", ".join(REGISTERED) + " in order")
    for digest, name in rows:
        if sha256(os.path.join(HERE, name)) != digest:
            die(5, f"registration-mismatch: {name} differs from registration.sha256")
    return sha256(REGISTRATION)


def check(strict=False):
    recorded = open(DIGEST).read().split()[0]
    if sha256(MANIFEST) != recorded:
        die(2, "manifest-digest-mismatch: selection-manifest.json does not match selection-manifest.sha256")
    registered = registration()
    thresholds = json.load(open(THRESHOLDS))
    blocked = [k for k in ("X", "Y") if not isinstance(thresholds.get(k), (int, float))]
    print(json.dumps({"manifestSha256": recorded, "registrationSha256": registered, "thresholds": thresholds,
                      "blocked": blocked}, sort_keys=True))
    if blocked and strict:
        die(3, "OWNER-BLOCKER: thresholds " + ",".join(blocked) + " are not set in thresholds.json")
    return thresholds, registered


def git_env():
    """The caller's environment without GIT_* overrides, system or global Git configuration."""
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull, GIT_NO_REPLACE_OBJECTS="1")
    return env


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], check=True, capture_output=True, text=True,
                          env=git_env()).stdout


def committed(repo):
    """VRP-V0-003, VRP-V0-055: the executing harness is the one in --repo, and every registered file is
    committed and unmodified there. Returns (toplevel, git common dir, HEAD)."""
    if not repo:
        die(4, "repo-required: pass --repo")
    try:
        top = os.path.realpath(git(repo, "rev-parse", "--show-toplevel").strip())
        common = os.path.realpath(git(repo, "rev-parse", "--path-format=absolute", "--git-common-dir").strip())
        head = git(repo, "rev-parse", "HEAD").strip()
    except subprocess.CalledProcessError:
        die(4, f"repo-invalid: {repo} is not a Git work tree with a HEAD")
    if os.path.realpath(HERE) != os.path.realpath(os.path.join(top, REL)):
        die(4, f"harness-not-in-repo: {HERE} is not {os.path.join(top, REL)}")
    for name in (*REGISTERED, "registration.sha256"):
        path = REL + name
        if subprocess.run(["git", "-C", top, "ls-files", "--error-unmatch", path], capture_output=True,
                          env=git_env()).returncode:
            die(4, f"protocol-not-committed: {path}")
        if subprocess.run(["git", "-C", top, "diff", "--quiet", "HEAD", "--", path], env=git_env()).returncode:
            die(4, f"protocol-modified: {path} differs from HEAD")
    return top, common, head


def inside(child, parent):
    return child == parent or child.startswith(parent.rstrip(os.sep) + os.sep)


def validate_paths(top, common, work, out):
    """VRP-V0-057: canonical work and output directories, outside the repository, its Git directory and
    each other."""
    if not work or not out:
        die(4, "paths-required: pass --work and --out")
    w, o = os.path.realpath(work), os.path.realpath(out)
    for flag, p in (("--work", w), ("--out", o)):
        for label, q in (("repository", top), ("Git directory", common)):
            if inside(p, q) or inside(q, p):
                die(4, f"path-overlap: {flag} {p} overlaps the {label} {q}")
    if inside(w, o) or inside(o, w):
        die(4, f"path-overlap: --work {w} and --out {o} overlap")
    return w, o


def module_at(repo, commit):
    """The Go module path from go.mod at the immutable commit, read before any step runs (VRP-V0-061)."""
    try:
        text = git(repo, "cat-file", "blob", f"{commit}:go.mod")
    except subprocess.CalledProcessError:
        return None
    m = re.search(r"^module\s+(\S+)", text, re.M)
    return m[1].strip('"') if m else None


def tool_identity(corvint):
    """VRP-V0-023: the one Corvint build and Go toolchain a run uses."""
    path = shutil.which(corvint)
    out = lambda argv: subprocess.run(argv, capture_output=True, text=True).stdout.strip()
    return {"corvintPath": path, "corvintSha256": sha256(path) if path else None,
            "corvintVersion": out([corvint, "--version"]), "goVersion": out([GO, "env", "GOVERSION"])}


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
                                     capture_output=True, env=git_env()).returncode == 0
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
        ("coverage", [GO, "test", "-count=1", "-coverprofile=" + os.path.join(o, "cover.out"),
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


def retire(proc):
    """VRP-V0-058: SIGKILL the step's whole process group and wait, bounded, until it is empty."""
    try:
        os.killpg(proc.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    proc.wait()
    deadline = time.monotonic() + RETIRE_SECONDS
    while True:
        try:
            os.killpg(proc.pid, 0)
        except ProcessLookupError:
            return True
        if time.monotonic() >= deadline:
            return False
        time.sleep(0.05)


def run_step(o, name, argv, cwd, timeout, log, env=None):
    """Run one step as its own process group; the group is retired on exit, timeout or interrupt."""
    start, code, stdout, stderr = time.monotonic(), None, b"", b""
    proc = subprocess.Popen(argv, cwd=cwd, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, start_new_session=True, env=env)
    try:
        stdout, stderr = proc.communicate(timeout=timeout)
        code = proc.returncode
    except subprocess.TimeoutExpired:
        code = "timeout"
    finally:
        retired = retire(proc)
        if code is None or code == "timeout":
            try:  # collect what the killed group wrote; a pipe held by an escaped process is abandoned
                more = proc.communicate(timeout=RETIRE_SECONDS)
                stdout, stderr = stdout + (more[0] or b""), stderr + (more[1] or b"")
            except (subprocess.TimeoutExpired, ValueError):
                pass
        elapsed = time.monotonic() - start
        open(os.path.join(o, name + ".stdout"), "ab").write(stdout)
        open(os.path.join(o, name + ".stderr"), "ab").write(stderr)
        log.append({"step": name, "argv": argv, "exit": "interrupted" if code is None else code,
                    "seconds": round(elapsed, 3), "groupRetired": retired})
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
            lines = git(repo, "cat-file", "blob", f'{change["base"]}:{t}').count("\n")
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


def capture(repo, work, o, corvint, change, identity):
    """Capture one change into the fresh output directory o (VRP-V0-026, 057..061). Only a worktree this
    call created is removed, in finally, so an interrupt or failure never touches another tree."""
    w = os.path.join(work, change["id"])
    module = module_at(repo, change["change"])
    dirs, oracle, base_cited, added = tests_for(repo, change)
    log, created, complete, refused, head = [], False, False, None, None
    try:
        if os.path.lexists(w):
            refused = "worktree-path-exists"
            return
        for name, argv, cwd, timeout in plan(repo, work, os.path.dirname(o), corvint, change):
            if name == "cleanup":
                continue
            if name == "cite":
                cmds, same = cite_steps(repo, corvint, change, w, o, dirs, base_cited, added)
                for argv2 in cmds:
                    run_step(o, "cite", argv2, None, timeout, log)
                keep(w, MAP, o, "map.json")
                continue
            if name == "discriminate":
                argv = [a for a in argv if not a.startswith("<")]
                if same:
                    argv[-2:-2] = ["--same-change-test", ",".join(same)]
            code, stdout = run_step(o, name, argv, cwd, timeout, log, env=git_env() if argv[0] == "git" else None)
            if name == "worktree":
                created = os.path.lexists(w)  # absent before this step, so anything there is ours
                if code != 0:
                    break
                head = subprocess.run(["git", "-C", w, "rev-parse", "HEAD"], capture_output=True, text=True,
                                      env=git_env()).stdout.strip()
                if head != change["change"]:
                    refused = "worktree-head-mismatch"
                    break
            if name == "hunk-revert":
                open(os.path.join(o, "hunk-revert.json"), "wb").write(stdout)
            if name == "cover-map":
                keep(w, COVER_MAP, o, "map.cover.json")
            if name == "discriminate":
                keep(w, DISC_MAP, o, "map.discriminate.json")
            if name == "prepare" and code != 0:
                break
        complete = True
    finally:
        if created:
            run_step(o, "cleanup", ["git", "-C", repo, "worktree", "remove", "--force", w], None, 300, log,
                     env=git_env())
        artifacts = {n: sha256(os.path.join(o, n)) for n in ARTIFACTS if os.path.isfile(os.path.join(o, n))}
        json.dump({"change": change["id"], "changeCommit": change["change"], "base": change["base"],
                   "fix": change["fix"], "module": module, "worktreeHead": head, "complete": complete,
                   "refused": refused, "artifacts": artifacts, **identity,
                   "oracle": oracle, "baseCited": base_cited, "sameChange": added, "steps": log,
                   "replaySeconds": round(sum(s["seconds"] for s in log if s["step"] not in ("worktree", "cleanup")), 3)},
                  open(os.path.join(o, "run.json"), "w"), indent=2, sort_keys=True)


def cmd_run(args):
    thresholds, registered = check(strict=True)
    top, common, head = committed(args.repo)
    work, out = validate_paths(top, common, args.work, args.out)
    identity = {"registrationSha256": registered, "repositoryHead": head, "thresholds": thresholds,
                **tool_identity(args.corvint)}
    manifest = json.load(open(MANIFEST))
    for change in manifest["changes"]:
        if args.only and change["id"] != args.only:
            continue
        o = os.path.join(out, change["id"])
        if os.path.lexists(o):
            print(f"{change['id']}: output exists, first observation kept; skipped", file=sys.stderr)
            continue
        os.makedirs(o)
        capture(top, work, o, args.corvint, change, identity)


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


class CaptureMismatch(Exception):
    """An artifact does not belong to the selected change, its tree, its run or the registration."""


def verify_capture(change, o, run, registered):
    """VRP-V0-059: bind every artifact to this change, base, tree, patch, run and registration."""
    def need(ok, what):
        if not ok:
            raise CaptureMismatch(f"{change['id']}: {what}")

    def load(name):
        try:
            return json.load(open(os.path.join(o, name)))
        except ValueError:
            return None  # unreadable output is NOT_OBSERVED at scoring, not an identity claim

    need(run.get("change") == change["id"], "run.json change id")
    need(run.get("changeCommit") == change["change"], "run.json changeCommit")
    need(run.get("base") == change["base"], "run.json base")
    need(run.get("registrationSha256") == registered, "run.json registrationSha256 is not this registration")
    recorded = run.get("artifacts")
    need(isinstance(recorded, dict), "run.json artifacts")
    present = {n for n in ARTIFACTS if os.path.isfile(os.path.join(o, n))}
    need(present == set(recorded), "artifact set differs from run.json")
    for name in sorted(present):
        need(sha256(os.path.join(o, name)) == recorded[name], f"{name} sha256 differs from run.json")
    if "map.json" in present:
        need(run.get("worktreeHead") == change["change"], "worktree head is not the change")
    patch = recorded.get("change.patch")
    ids = {}
    for name in ("map.json", "map.cover.json", "map.discriminate.json"):
        doc = load(name) if name in present else None
        if not isinstance(doc, dict):
            continue
        need(doc.get("baseRevision") == change["base"], f"{name} baseRevision")
        need(patch is not None and doc.get("patchSha256") == patch, f"{name} patchSha256")
        hunks = doc.get("hunks") or []
        ids[name] = [h.get("id") for h in hunks]
        for h in hunks:
            if name == "map.discriminate.json" and h.get("discriminates") is not None:
                need(h["discriminates"].get("treeRevision") == change["change"], f"{name} treeRevision")
            if name == "map.cover.json" and h.get("coverage") is not None:
                need(h["coverage"].get("profileSha256") in (None, recorded.get("cover.out")), f"{name} profileSha256")
    need(len({tuple(v) for v in ids.values()}) <= 1, "hunk ids differ across maps")
    states = load("coverage-states.json") if "coverage-states.json" in present else None
    if isinstance(states, dict):
        need(states.get("profileSha256") == recorded.get("cover.out"), "coverage-states.json profileSha256")
        need(states.get("testRun") == "vrp-baseline-" + change["id"], "coverage-states.json testRun")
    revert = load("hunk-revert.json") if "hunk-revert.json" in present else None
    if isinstance(revert, dict) and revert.get("hunkRevert") is not None:
        result = revert.get("stableVerification") or {}
        need(result.get("expectedBase") == change["base"], "hunk-revert.json expectedBase")
        need(result.get("targetRevision") == change["change"], "hunk-revert.json targetRevision")
        need(patch is not None and result.get("patchSha256") == patch, "hunk-revert.json patchSha256")


def score_change(change, o, registered):
    """One results row per change (VRP-V0-040). Every unavailable value is NOT_OBSERVED."""
    row = {"id": change["id"], "change": change["change"], "fix": change["fix"]}
    try:
        run = json.load(open(os.path.join(o, "run.json")))
    except (OSError, ValueError) as e:
        return {**row, "status": f"NOT_OBSERVED({type(e).__name__})"}
    verify_capture(change, o, run, registered)
    row["tool"] = {k: run.get(k) for k in ("corvintSha256", "corvintVersion", "goVersion")}
    if run.get("refused"):
        return {**row, "status": f"NOT_OBSERVED(refused:{run['refused']})"}
    if run.get("complete") is not True:
        return {**row, "status": "NOT_OBSERVED(incomplete-capture)"}
    try:
        doc = json.load(open(os.path.join(o, "map.json")))
        patch = open(os.path.join(o, "change.patch")).read()
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
    (thresholds, registered), _ = check(strict=True), committed(args.repo)
    if not args.out:
        die(4, "paths-required: pass --out")
    manifest = json.load(open(MANIFEST))
    try:
        results = [score_change(c, os.path.join(args.out, c["id"]), registered) for c in manifest["changes"]]
    except CaptureMismatch as e:
        die(6, f"capture-mismatch: {e}")
    tools = {json.dumps(r["tool"], sort_keys=True) for r in results if "tool" in r}
    if len(tools) > 1:
        die(6, "capture-mismatch: captures come from more than one Corvint build or Go toolchain: " + " | ".join(sorted(tools)))
    summary = decide(results, thresholds["X"], thresholds["Y"], manifest["selected"])
    json.dump({"profile": "corvint-vrp-results/0", "manifestSha256": sha256(MANIFEST),
               "registrationSha256": registered, "summary": summary, "changes": results},
              sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")


# ---- selftest ------------------------------------------------------------------------------------

STUB_CORVINT = r'''
import hashlib, json, os, sys
a = sys.argv[1:]
if a == ["--version"]:
    print("Corvint stub"); sys.exit(0)
opt = lambda k: a[a.index(k) + 1]
root, cmd = opt("--root"), a[a.index("cem") + 1]
path = lambda rel: os.path.join(root, rel)
if cmd == "prepare":
    patch = b"--- a/a.go\n+++ b/a.go\n@@ -3,0 +3,1 @@\n+func F() int { return 1 }\n"
    open(opt("--patch"), "wb").write(patch)
    os.makedirs(path(".corvint"), exist_ok=True)
    doc = {"baseRevision": opt("--base"), "patchSha256": hashlib.sha256(patch).hexdigest(),
           "hunks": [{"id": "hunk:sha256:" + "1" * 64, "path": "a.go", "newRange": {"start": 3, "count": 1}}]}
    json.dump(doc, open(path(".corvint/change.cem.json"), "w"))
elif cmd in ("cite", "mark"):
    pass
elif cmd == "cover":
    doc = json.load(open(path(opt("--map"))))
    digest = hashlib.sha256(open(opt("--coverprofile"), "rb").read()).hexdigest()
    for h in doc["hunks"]:
        h["coverage"] = {"profileSha256": digest, "testRun": opt("--test-run")}
    json.dump(doc, open(path(opt("--output")), "w"))
    json.dump({"profileSha256": digest, "testRun": opt("--test-run")}, open(opt("--states-output"), "w"))
elif cmd == "verify":
    doc = json.load(open(path(opt("--map"))))
    print(json.dumps({"stableVerification": {"expectedBase": opt("--expected-base"), "targetRevision": opt("--target"),
                                             "patchSha256": doc["patchSha256"]},
                      "hunkRevert": {"hunks": [{"hunkId": h["id"], "result": "survived"} for h in doc["hunks"]]}}))
elif cmd == "discriminate":
    doc = json.load(open(path(opt("--map"))))
    for h in doc["hunks"]:
        h["discriminates"] = {"treeRevision": opt("--target"), "state": "survived", "mutants": 2}
    json.dump(doc, open(path(opt("--output")), "w"))
'''

STUB_GO = r'''
import sys
a = sys.argv[1:]
if a[:2] == ["env", "GOVERSION"]:
    print("go-stub"); sys.exit(0)
out = [x for x in a if x.startswith("-coverprofile=")][0].split("=", 1)[1]
open(out, "w").write("mode: set\nexample.com/vrp/a.go:3.1,3.26 1 0\n")
'''


def selftest_repo(t):
    """A two-commit Go repository for the synthetic capture checks."""
    g = lambda *a: git(t, "-c", "user.name=vrp", "-c", "user.email=vrp@example.invalid", *a)
    g("init", "-q")
    open(os.path.join(t, "go.mod"), "w").write("module example.com/vrp\n\ngo 1.21\n")
    open(os.path.join(t, "a.go"), "w").write("package vrp\n\n")
    open(os.path.join(t, "a_test.go"), "w").write("package vrp\n")
    g("add", "."); g("commit", "-qm", "base")
    base = g("rev-parse", "HEAD").strip()
    open(os.path.join(t, "a.go"), "a").write("func F() int { return 1 }\n")
    g("commit", "-qam", "feat: F (V1-0001)")
    return g, base, g("rev-parse", "HEAD").strip()


def stub(directory, name, body):
    path = os.path.join(directory, name)
    open(path, "w").write(f"#!{sys.executable} -I\n" + body)
    os.chmod(path, 0o755)
    return path


def load_select():
    import importlib.util
    spec = importlib.util.spec_from_file_location("vrp_select", os.path.join(HERE, "select.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def selftest_selector(t):
    """Findings 5 and 6: hunking ignores ambient diff configuration; ID masking keeps line order."""
    select = load_select()
    g = lambda *a: git(t, "-c", "user.name=vrp", "-c", "user.email=vrp@example.invalid", *a)
    write = lambda lines: open(os.path.join(t, "a.go"), "w").write("package a\n\nfunc h() {\n%s}\n" % "".join(lines))
    call = lambda n, f: f'\tuse("VRP-V0-{n:03d}", {f}())\n'
    pad = ["\tx := 1\n", "\t_ = x\n", "\ty := 2\n", "\t_ = y\n"]
    g("init", "-q")
    write([call(1, "f"), call(2, "g"), *pad, call(3, "k")])
    g("add", "."); g("commit", "-qm", "feat: V1-0001")
    base = g("rev-parse", "HEAD").strip()
    write([call(11, "f"), call(12, "g"), *pad, call(13, "k")])
    g("commit", "-qam", "fix: renumber (V1-0001)")
    renumber = g("rev-parse", "HEAD").strip()
    write([call(21, "g"), call(22, "f"), *pad, call(13, "k")])
    g("commit", "-qam", "fix: call order (V1-0001)")
    order = g("rev-parse", "HEAD").strip()
    assert select.touched_lines(t, renumber) == {}, "an in-order ID renumbering touches nothing"
    assert select.touched_lines(t, order) == {"a.go": [4, 5]}, "finding 6: swapped calls with new IDs are behavioural"
    plain = select.hunks(t, base, renumber)
    assert plain == {"a.go": [(4, 2, 4, 2), (10, 1, 10, 1)]}, plain
    g("config", "diff.interHunkContext", "1000000")
    g("config", "diff.context", "9")
    assert select.hunks(t, base, renumber) == plain, "finding 5: repository config changed the hunks"


def selftest_capture(t):
    """Findings 2, 3 and 4 on a synthetic capture with stub corvint and go binaries."""
    global GO
    repo, work, out, bins = (os.path.join(t, d) for d in ("repo", "work", "out", "bin"))
    for d in (repo, bins):
        os.makedirs(d)
    g, base, sha = selftest_repo(repo)
    corvint, GO = stub(bins, "corvint", STUB_CORVINT), stub(bins, "go", STUB_GO)
    change = {"id": "VRP-C001", "change": sha, "base": base, "fix": "0" * 40, "changeSourceFiles": ["a.go"],
              "changeSourceHunks": 1, "touchedLines": [["a.go", 3]], "laterFixes": []}
    identity = {"registrationSha256": "r" * 64, **tool_identity(corvint)}
    o = os.path.join(out, change["id"])
    os.makedirs(o)
    capture(repo, work, o, corvint, change, identity)
    assert not os.path.exists(os.path.join(work, change["id"])), "the created worktree is removed"
    row = score_change(change, o, "r" * 64)
    assert row["status"] == "OBSERVED", row
    assert row["hunks"][0]["baseline"] == "0/1", "finding 2: module read before cleanup maps coverage"
    assert row["baselineCapture"] is True and row["typedCapture"] is True, row
    # Finding 4: another registration, another change's map or altered bytes are refused.
    for tamper in ("registration", "map", "patch"):
        scratch = os.path.join(t, "tamper-" + tamper)
        shutil.copytree(o, scratch)
        run = json.load(open(os.path.join(scratch, "run.json")))
        if tamper == "registration":
            run["registrationSha256"] = "s" * 64
        elif tamper == "map":
            doc = json.load(open(os.path.join(scratch, "map.json")))
            doc["baseRevision"] = sha
            json.dump(doc, open(os.path.join(scratch, "map.json"), "w"))
            run["artifacts"]["map.json"] = sha256(os.path.join(scratch, "map.json"))
        else:
            open(os.path.join(scratch, "change.patch"), "a").write("+other\n")
        json.dump(run, open(os.path.join(scratch, "run.json"), "w"))
        try:
            score_change(change, scratch, "r" * 64)
            raise AssertionError(f"finding 4: tampered {tamper} was scored")
        except CaptureMismatch:
            pass
    # Finding 3: a pre-existing, dirty, registered worktree at the change path is never removed.
    w = os.path.join(work, change["id"])
    g("worktree", "add", "-q", "--detach", w, base)
    open(os.path.join(w, "dirty.txt"), "w").write("keep")
    o2 = os.path.join(out, "again")
    os.makedirs(o2)
    capture(repo, work, o2, corvint, change, identity)
    assert open(os.path.join(w, "dirty.txt")).read() == "keep", "finding 3: pre-existing worktree removed"
    assert json.load(open(os.path.join(o2, "run.json")))["refused"] == "worktree-path-exists"
    assert score_change(change, o2, "r" * 64)["status"] == "NOT_OBSERVED(refused:worktree-path-exists)"
    common = os.path.realpath(os.path.join(repo, ".git"))
    for bad_work, bad_out in ((os.path.join(repo, "w"), out), (work, os.path.join(work, "o")), (t, out)):
        try:
            validate_paths(os.path.realpath(repo), common, bad_work, bad_out)
            raise AssertionError(f"finding 3: overlapping paths accepted: {bad_work} {bad_out}")
        except SystemExit as e:
            assert e.code == 4
    validate_paths(os.path.realpath(repo), common, work, out)


def selftest_timeout(t):
    """Finding 7: a timed-out step's descendants are retired with it."""
    code, _ = run_step(t, "probe", ["sh", "-c", f"sleep 60 & echo $! > {t}/pid; wait"], None, 1, [])
    pid = int(open(os.path.join(t, "pid")).read())
    try:
        alive = open(f"/proc/{pid}/stat").read().split()[2] != "Z"
    except OSError:
        alive = False
    if alive and not os.path.isdir("/proc"):
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            alive = False
    assert code == "timeout" and not alive, "finding 7: descendant survived the timeout"


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
    checks = (("selector", selftest_selector), ("capture", selftest_capture), ("timeout", selftest_timeout))
    for name, test in checks:
        with tempfile.TemporaryDirectory(prefix="vrp-selftest-") as t:
            test(os.path.realpath(t))
        print(f"selftest {name}: ok")
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
