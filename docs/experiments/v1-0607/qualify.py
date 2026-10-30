# SPDX-License-Identifier: Apache-2.0
"""V1-0607 hook context-delivery qualification driver (lane hooks-w7).

Drives `corvint adapter claude-code user-prompt` exactly as the shipped hooks.json does
(stdin JSON, CLAUDE_PROJECT_DIR, 2 s host timeout) on a clean linked worktree, and records
per-step wall time, load, outcome, refresh state and the matching raw-event revision.
"""
import json, os, signal, subprocess, sys, time, glob, platform

S, BIN_A, BIN_B, OUT = sys.argv[1:5]
WT = os.path.join(S, "fx", "linked")
MAIN = os.path.join(S, "fx", "main")
STORE = os.path.join(MAIN, ".git", "corvint", "index")
LOCK = os.path.join(STORE, "refresh.lock")
PROMPT = "where does startSnapshotRefresh acquire the refresh lock"
steps = []


def git(*args, cwd=WT):
    return subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True, check=True).stdout.strip()


def load():
    return open("/proc/loadavg").read().split()[:3]


def workers():
    out = subprocess.run(["ps", "-eo", "pid,ni,args"], capture_output=True, text=True).stdout
    return [line.strip() for line in out.splitlines() if "index --if-stale" in line and "ps -eo" not in line]


def store_files():
    return sorted(os.path.basename(p) for p in glob.glob(os.path.join(STORE, "*")))


def hook(binary, label, prompt=PROMPT):
    payload = json.dumps({"session_id": "v1-0607-qual", "hook_event_name": "UserPromptSubmit", "cwd": WT, "prompt": prompt})
    env = dict(os.environ, CLAUDE_PROJECT_DIR=WT)
    started = time.monotonic()
    proc = subprocess.run(["timeout", "2", binary, "adapter", "claude-code", "user-prompt"], input=payload, cwd=WT, env=env, capture_output=True, text=True)
    elapsed = round((time.monotonic() - started) * 1000)
    out = proc.stdout.strip()
    try:
        parsed = json.loads(out) if out else None
    except ValueError:
        parsed = None
    ctx = ((parsed or {}).get("hookSpecificOutput") or {}).get("additionalContext", "") or (parsed or {}).get("systemMessage", "")
    if proc.returncode == 124:
        outcome = "HOST_TIMEOUT_KILLED"
    elif "FALLBACK degraded" in ctx or "fallback" in ctx.lower() and "corvint-event-rejected" in ctx:
        outcome = "FALLBACK:" + ctx.split("\n")[0].split("degraded: ")[-1].split(";")[0]
    elif "task_evidence" in ctx:
        outcome = "CONTEXT"
    elif parsed == {}:
        outcome = "SILENT_NO_ACTIONABLE"
    else:
        outcome = "OTHER"
    refresh = None
    for marker in ("Background index refresh started", "Background index refresh already running", "Background index refresh failed to start"):
        if marker in ctx:
            refresh = marker.split("refresh ")[1]
    row = {"step": label, "binary": os.path.basename(binary), "ms": elapsed, "rc": proc.returncode, "outcome": outcome,
           "refresh": refresh, "load": load(), "head": git("rev-parse", "HEAD"), "tree": git("rev-parse", "HEAD^{tree}"),
           "dirty": len(git("status", "--porcelain").splitlines()), "outputBytes": len(out)}
    steps.append(row)
    print(json.dumps(row), flush=True)
    return row


def raw_revision(binary, label):
    """Raw dogfood event: proves which tree the context packet was built for."""
    event = json.dumps({"sessionIdSha256": "0" * 64, "task": PROMPT})
    started = time.monotonic()
    proc = subprocess.run([binary, "--root", WT, "dogfood", "event", "--host", "claude-code", "--host-version", "unreported-by-hook-api",
                           "--surface", "plugin", "--adapter-version", "0.1.0", "--event", "user-prompt", "--input", "-", "--budget-bytes", "7000"],
                          input=event, capture_output=True, text=True)
    row = {"step": label, "ms": round((time.monotonic() - started) * 1000), "rc": proc.returncode}
    if proc.returncode == 0:
        context = json.loads(proc.stdout)["context"]
        row.update(packetRevision=context["revision"], freshness=context["freshness"], taskEvidence=len(context.get("task_evidence", [])),
                   matchesTree=context["revision"] == git("rev-parse", "HEAD^{tree}"))
    else:
        row["stderr"] = proc.stderr[:300]
    steps.append(row)
    print(json.dumps(row), flush=True)
    return row


def wait_refresh(label):
    started = time.monotonic()
    subprocess.run(["flock", LOCK, "true"], check=True)  # blocks until the single-flight owner releases
    row = {"step": label, "refreshWaitMs": round((time.monotonic() - started) * 1000), "workersAfter": workers(), "store": store_files()}
    steps.append(row)
    print(json.dumps(row), flush=True)


def commit(message, path):
    with open(os.path.join(WT, path), "a") as handle:
        handle.write("\n// " + message + "\n")
    git("-c", "user.name=qual", "-c", "user.email=qual@example.invalid", "commit", "-qam", message)


def burst(binary, label, count=4):
    procs = []
    payload = json.dumps({"session_id": "v1-0607-burst", "hook_event_name": "UserPromptSubmit", "cwd": WT, "prompt": PROMPT})
    env = dict(os.environ, CLAUDE_PROJECT_DIR=WT)
    started = time.monotonic()
    for _ in range(count):
        procs.append(subprocess.Popen(["timeout", "2", binary, "adapter", "claude-code", "user-prompt"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, cwd=WT, env=env, text=True))
    for proc in procs:
        proc.stdin.write(payload)
        proc.stdin.close()
    results = []
    for proc in procs:
        text = proc.stdout.read()
        proc.wait()
        status = next((m for m in ("started", "already running", "failed to start") if "Background index refresh " + m in text), None)
        results.append({"rc": proc.returncode, "refresh": status, "context": "task_evidence" in text})
    row = {"step": label, "ms": round((time.monotonic() - started) * 1000), "results": results, "workers": len(workers())}
    steps.append(row)
    print(json.dumps(row), flush=True)


LOAD = int(os.environ.get("QUAL_LOAD", "0"))
burners = [subprocess.Popen(["timeout", "400", "sh", "-c", "while :; do :; done"]) for _ in range(LOAD)]
time.sleep(20 if LOAD else 0)  # let the 1-minute load average climb
tuple_ = {"cpuBurners": LOAD,
    "core": subprocess.run([BIN_A, "--version"], capture_output=True, text=True).stdout.strip(),
    "coreB": subprocess.run([BIN_B, "--version"], capture_output=True, text=True).stdout.strip(),
    "binarySha256": {os.path.basename(b): subprocess.run(["sha256sum", b], capture_output=True, text=True).stdout.split()[0] for b in (BIN_A, BIN_B)},
    "adapterWire": "0.1.0", "plugin": "corvint 0.3.6 (integrations/claude-code/plugins/corvint, hooks.json timeout 2 s)",
    "hostEmulated": "Claude Code UserPromptSubmit command hook payload; host binary on PATH: " + subprocess.run(["claude", "--version"], capture_output=True, text=True).stdout.strip(),
    "os": platform.platform(), "cpus": os.cpu_count(), "trackedFiles": len(git("ls-files").splitlines()),
    "worktree": "clean linked worktree (git worktree add) of corvint at " + git("rev-parse", "HEAD"),
}
print(json.dumps(tuple_), flush=True)

# A. cold start: no snapshot for this engine in the shared store.
hook(BIN_A, "A1-cold")
burst(BIN_A, "A2-concurrent-during-refresh")
wait_refresh("A3-refresh-done")
hook(BIN_A, "A4-next-hook")
raw_revision(BIN_A, "A5-packet-tree")

# B. a commit in the linked worktree moves the tree.
commit("qualification commit B", "cmd/corvint/index_refresh_other.go")
hook(BIN_A, "B1-after-commit")
wait_refresh("B2-refresh-done")
hook(BIN_A, "B3-next-hook")
raw_revision(BIN_A, "B4-packet-tree")

# C. a different binary (index engine key = executable digest) on the same tree.
hook(BIN_B, "C1-new-binary")
wait_refresh("C2-refresh-done")
hook(BIN_B, "C3-next-hook")
raw_revision(BIN_B, "C4-packet-tree")
hook(BIN_A, "C5-old-binary-still-served")

# D. cancellation: kill the refresh group mid-build; no partial snapshot, no orphan, next hook retries.
commit("qualification commit D", "cmd/corvint/index_refresh_other.go")
before = store_files()
row = hook(BIN_A, "D1-after-commit")
time.sleep(0.3)
victims = workers()
for line in victims:
    os.killpg(int(line.split()[0]), signal.SIGTERM)
time.sleep(0.5)
steps.append({"step": "D2-killed", "killed": victims, "workersAfter": workers(), "storeBefore": before, "storeAfter": store_files(),
              "lockFree": subprocess.run(["flock", "-n", LOCK, "true"]).returncode == 0})
print(json.dumps(steps[-1]), flush=True)
hook(BIN_A, "D3-retry")
wait_refresh("D4-refresh-done")
hook(BIN_A, "D5-next-hook")
raw_revision(BIN_A, "D6-packet-tree")

# E. warm latency on an unchanged tree.
warm = [hook(BIN_A, "E-warm-%02d" % i)["ms"] for i in range(20)]
warm.sort()
summary = {"step": "E-summary", "warmMs": {"min": warm[0], "p50": warm[10], "p95": warm[18], "max": warm[-1]},
           "warmContext": sum(1 for s in steps if s["step"].startswith("E-warm") and s["outcome"] == "CONTEXT")}
steps.append(summary)
print(json.dumps(summary), flush=True)

for burner in burners:
    burner.kill()
json.dump({"tuple": tuple_, "steps": steps}, open(OUT, "w"), indent=1)
