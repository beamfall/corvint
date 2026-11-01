# SPDX-License-Identifier: Apache-2.0
"""V1-2755 AC4: offline replay of the AHI-215 OpenCode prompt frame over the ContextBench SWE-bench-Pro rows.

Per row: materialize the base commit as a fresh one-commit repository, index it, send the prompt
(AHI-016 derived query plus AHI-215 junk stripping when over the bound) to the real
`corvint harness event --host opencode --event user-prompt`, then apply frame-policy.js
(namedExistingPaths, frameCandidates, corroborated, 5 rows). Writes one JSON line per row; resumable.
"""
import concurrent.futures, json, os, re, shutil, subprocess, sys
BIN, OUT, LIMIT = sys.argv[1], sys.argv[2], int(sys.argv[3])
B = os.path.expanduser("~/bench/v0415/work")
CB = os.path.expanduser("~/lanes/indexcli-w5-scratch/cb")
rows = {}
for line in open(os.path.join(B, "full", "cb-full.jsonl")):
    d = json.loads(line)
    if d["instance_id"].startswith("SWE-Bench-Pro"):
        rows[d["instance_id"]] = d
merged = {}
for line in open(os.path.join(B, "full", "rows-merged.jsonl")):
    d = json.loads(line)
    if d["id"] in rows:
        merged[d["id"]] = d
gitdirs = {l.split("\t")[0]: l.rstrip("\n").split("\t")[3] for l in open(os.path.join(B, "rows.tsv"))}
done = set()
if os.path.exists(OUT):
    done = {json.loads(l)["id"] for l in open(OUT)}

# AHI-016 (cmd/corvint/prompt_bound.go) port.
BACKTICK = re.compile(r"`[ -_a-~]{1,128}`")
TOKEN = re.compile(r"[A-Za-z0-9_][A-Za-z0-9_./-]*[A-Za-z0-9_]")
PATHA = re.compile(r"^(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+$|^[A-Za-z0-9_-]{2,}\.[A-Za-z][A-Za-z0-9]{0,7}$")
IDENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")
IDHOW = re.compile(r"[A-Za-z0-9]_[A-Za-z0-9]|[a-z][A-Z]")
REQ = re.compile(r"^[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]+$")
def over(p): return len(p) > 2000 or len(p.encode()) > 16384
def derive(prompt):
    spans = [m.span() for m in BACKTICK.finditer(prompt)]
    anchors = [(s, prompt[s + 1:e - 1]) for s, e in spans]
    masked = list(prompt)
    for s, e in spans:
        masked[s:e] = " " * (e - s)
    masked = "".join(masked)
    for m in TOKEN.finditer(masked):
        t = m.group(0)
        if (IDENT.match(t) and IDHOW.search(t)) or PATHA.match(t) or REQ.match(t):
            anchors.append((m.start(), t))
    anchors.sort(key=lambda a: a[0])
    seen, texts = set(), []
    for _, t in anchors:
        if t not in seen:
            seen.add(t); texts.append(t)
    q = " ".join(texts)
    return "" if q == "" or over(q) else q
def strip_junk(task, prompt):  # frame-policy.js stripJunkAnchors
    words = task.split()
    tags = set(re.findall(r"</([A-Za-z_][A-Za-z0-9_.-]*)>", prompt))
    outside = re.sub(r"</?[A-Za-z_][A-Za-z0-9_.-]*[^<>]{0,256}>", " ", prompt)
    def junk(w):
        return (not re.search(r"[A-Za-z0-9]", w)) or w.startswith("/") or (w in tags and not re.search(r"(?:^|[^A-Za-z0-9_])" + re.escape(w) + r"(?:$|[^A-Za-z0-9_])", outside))
    kept = [w for w in words if not junk(w)]
    return task if not kept else " ".join(kept)
PATH_TOKEN = re.compile(r"`[^`\n]{1,256}`|[A-Za-z0-9_.][A-Za-z0-9_./-]*[A-Za-z0-9_]")
PATH_SHAPE = re.compile(r"^(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+/?$|^[A-Za-z0-9_.-]*[A-Za-z0-9_-]\.[A-Za-z][A-Za-z0-9]{0,7}$")
def named_existing(prompt, root):
    seen, found = set(), []
    for m in PATH_TOKEN.finditer(prompt):
        t = m.group(0)
        t = t[1:-1].strip() if t.startswith("`") else t
        if t.startswith("./"): t = t[2:]
        if len(t) > 512 or not PATH_SHAPE.match(t) or t in seen: continue
        if any(p in ("..", ".") for p in t.split("/")): continue
        seen.add(t)
        if len(seen) > 256: break
        p = os.path.join(root, t)
        if os.path.isfile(p) and not os.path.islink(p) or os.path.isdir(p) and not os.path.islink(p): found.append(t)
    return found
TEST = re.compile(r"(?:^|/)(?:tests?|__tests__|testdata|spec)/|_test\.[A-Za-z]+$|\.(?:test|spec)\.[A-Za-z]+$|(?:^|/)test_[^/]*\.py$")
CORROB = {"project-instructions", "instruction-reference", "repository-spec", "git-history"}
def frame_candidates(results):
    rows = []
    for i, r in enumerate(results or []):
        ev = next((e for e in (r.get("evidence") or []) if isinstance(e.get("path"), str) and e["path"]), None)
        if not ev: continue
        rows.append({"path": ev["path"], "symbol": r.get("name") or None, "authority": ev.get("authority") or None, "i": i, "score": r.get("score") if isinstance(r.get("score"), (int, float)) else None})
    rows.sort(key=lambda x: (x["score"] is None, -(x["score"] or 0), x["i"]))
    decl = {}
    for x in rows:
        if x["symbol"] and not TEST.search(x["path"]): decl.setdefault(x["symbol"], set()).add(x["path"])
    amb = lambda x: x["symbol"] is not None and len(decl.get(x["symbol"], ())) >= 2
    ordered = [x for x in rows if not amb(x)] + [x for x in rows if amb(x)]
    files, out = set(), []
    for x in ordered:
        if x["path"] not in files: files.add(x["path"]); out.append(x)
    return out
def git(*a, **k): return subprocess.run(["git", *a], capture_output=True, text=True, **k)
def materialize(iid):
    d = rows[iid]; c = d["base_commit"]; g = gitdirs[iid]
    root = os.path.join(CB, d["repo"].replace("/", "__") + "-" + c[:12])
    marker = os.path.join(root, ".git", "corvint-replay-ready")
    if os.path.exists(marker): return root
    shutil.rmtree(root, ignore_errors=True); os.makedirs(root)
    env = dict(os.environ, GIT_INDEX_FILE=root + ".idx")
    subprocess.run(["git", "--git-dir=" + g, "read-tree", c], env=env, check=True, capture_output=True)
    subprocess.run(["git", "--git-dir=" + g, "--work-tree=" + root, "checkout-index", "-a", "-f"], env=env, check=True, capture_output=True)
    os.remove(root + ".idx")
    git("init", "-q", cwd=root, check=True)
    git("add", "-A", "-f", cwd=root, check=True)
    git("-c", "user.name=replay", "-c", "user.email=replay@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--no-verify", "-m", "base " + c, cwd=root, check=True)
    subprocess.run([BIN, "--root", root, "index"], capture_output=True, text=True, timeout=300)
    open(marker, "w").close()
    return root
def replay(iid):
    d = rows[iid]; prompt = d["problem_statement"]; m = merged.get(iid, {})
    gold = m.get("gold") or []
    rgidf5 = ((m.get("arms") or {}).get("grep-ident") or {}).get("ranked", [])[:5]
    root = materialize(iid)
    rec = {"id": iid, "repo": d["repo"], "gold": gold, "rg_idf_top5": rgidf5, "prompt_chars": len(prompt)}
    task = prompt
    if over(prompt):
        task = derive(prompt)
        if task: task = strip_junk(task, prompt)
    named = named_existing(prompt, root)
    rec["named_existing_paths"] = named[:5]
    if not task:
        rec.update(state=None, withheld="prompt-over-query-bound", candidates=[]); return rec
    p = subprocess.run([BIN, "--root", root, "harness", "event", "--host", "opencode", "--host-version", "2.1.3", "--surface", "plugin", "--adapter-version", "0.7.33", "--event", "user-prompt", "--input", "-", "--budget-bytes", "8000"],
                       input=json.dumps({"task": task}), capture_output=True, text=True, timeout=120)
    try: resp = json.loads(p.stdout)
    except ValueError: resp = {}
    ctx = resp.get("context") or {}
    cands = frame_candidates(ctx.get("results"))
    shown = cands[:LIMIT]
    rec.update(state=ctx.get("state"), abstention_active=bool((ctx.get("abstention") or {}).get("active")), rc=p.returncode, derived_query=task != prompt,
               candidates=[{"path": x["path"], "authority": x["authority"]} for x in shown])
    if named: rec["withheld"] = "prompt-names-path"
    elif ctx.get("state") != "READY" or rec["abstention_active"] or not cands: rec["withheld"] = "prompt-context-withheld"
    elif not any(x["authority"] in CORROB for x in shown): rec["withheld"] = "prompt-context-syntax-only"
    else: rec["withheld"] = None
    return rec
todo = [i for i in sorted(rows) if i not in done]
with open(OUT, "a") as out, concurrent.futures.ThreadPoolExecutor(4) as pool:
    futures = {pool.submit(replay, i): i for i in todo}
    for f in concurrent.futures.as_completed(futures, timeout=int(os.environ.get("REPLAY_BUDGET_S", "540"))):
        try: out.write(json.dumps(f.result()) + "\n"); out.flush()
        except Exception as e: print("ERR", futures[f], repr(e)[:200], file=sys.stderr)
