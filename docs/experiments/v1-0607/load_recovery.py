# SPDX-License-Identifier: Apache-2.0
"""Loaded stale-index sequence, then load removal and recovery, through the real hook command."""
import json, os, shutil, signal, subprocess, sys, time

S, BIN, BURNERS, LOAD_NICE, OUT = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), sys.argv[5]
WT = os.path.join(S, "fx", "linked")
STORE = os.path.join(S, "fx", "main", ".git", "corvint", "index")
LOCK = os.path.join(STORE, "refresh.lock")
PROMPT = "where does startSnapshotRefresh acquire the refresh lock"
steps = []


def emit(row):
    steps.append(row)
    print(json.dumps(row)[:700], flush=True)


def hook(label):
    payload = json.dumps({"session_id": "v1-0607-load", "hook_event_name": "UserPromptSubmit", "cwd": WT, "prompt": PROMPT})
    started = time.monotonic()
    proc = subprocess.run(["timeout", "2", BIN, "adapter", "claude-code", "user-prompt"], input=payload, cwd=WT,
                          env=dict(os.environ, CLAUDE_PROJECT_DIR=WT), capture_output=True, text=True)
    text = proc.stdout
    outcome = "CONTEXT" if "task_evidence" in text else ("SILENT" if text.strip() == "{}" else text.split("degraded: ")[-1].split(";")[0][:80])
    previous = text.split("The previous background refresh")[1].split("Until a refresh")[0] if "The previous background refresh" in text else None
    emit({"step": label, "ms": round((time.monotonic() - started) * 1000), "rc": proc.returncode, "outcome": outcome,
          "refresh": next((m for m in ("started", "already running", "failed to start") if "Background index refresh " + m in text), None),
          "previous": previous, "load": open("/proc/loadavg").read().split()[:3]})


def wait(label):
    started = time.monotonic()
    subprocess.run(["flock", LOCK, "true"], check=True)
    emit({"step": label, "waitMs": round((time.monotonic() - started) * 1000), "metadata": open(LOCK).read().strip(),
          "snapshots": sorted(f for f in os.listdir(STORE) if f.endswith(".gob"))})


shutil.rmtree(os.path.join(S, "fx", "main", ".git", "corvint"), ignore_errors=True)
emit({"step": "tuple", "core": subprocess.run([BIN, "--version"], capture_output=True, text=True).stdout.strip(), "burners": BURNERS, "loadNice": LOAD_NICE,
      "head": subprocess.run(["git", "-C", WT, "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip()})
burners = [subprocess.Popen(["nice", "-n", str(LOAD_NICE), "sh", "-c", "while :; do :; done"], start_new_session=True) for _ in range(BURNERS)]
try:
    time.sleep(15)
    hook("L1-cold-loaded")
    wait("L2-refresh-ended")
    hook("L3-next-loaded")
    wait("L4-refresh-ended")
    hook("L5-next-loaded")
finally:
    for burner in burners:
        os.killpg(burner.pid, signal.SIGKILL)
    for burner in burners:
        burner.wait()
emit({"step": "L6-load-removed", "load": open("/proc/loadavg").read().split()[:3]})
wait("L7-refresh-ended")
hook("L8-unloaded")
wait("L9-refresh-ended")
hook("L10-next-unloaded")
hook("L11-next-unloaded")
json.dump(steps, open(OUT, "w"), indent=1)
