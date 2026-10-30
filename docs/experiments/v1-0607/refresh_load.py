# SPDX-License-Identifier: Apache-2.0
"""Times one cold `index --if-stale` build under a CPU load, at a chosen worker and load priority."""
import json, os, shutil, signal, subprocess, sys, time

S, BIN, BURNERS, LOAD_NICE, WORKER_NICE, LIMIT = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5]), int(sys.argv[6])
WT = os.path.join(S, "fx", "linked")
shutil.rmtree(os.path.join(S, "fx", "main", ".git", "corvint"), ignore_errors=True)
burners = [subprocess.Popen(["nice", "-n", str(LOAD_NICE), "sh", "-c", "while :; do :; done"], start_new_session=True) for _ in range(BURNERS)]
try:
    time.sleep(15)
    load_before = open("/proc/loadavg").read().split()[:3]
    started = time.monotonic()
    proc = subprocess.run(["timeout", "-s", "KILL", str(LIMIT), "nice", "-n", str(WORKER_NICE), "env", "GOMAXPROCS=1", BIN, "--root", WT, "index", "--if-stale"],
                          capture_output=True, text=True)
    wall = round((time.monotonic() - started) * 1000)
    usage = os.times()
finally:
    for burner in burners:
        os.killpg(burner.pid, signal.SIGKILL)
    for burner in burners:
        burner.wait()
print(json.dumps({"burners": BURNERS, "loadNice": LOAD_NICE, "workerNice": WORKER_NICE, "gomaxprocs": 1, "limitS": LIMIT, "rc": proc.returncode,
                  "wallMs": wall, "childCpuS": round(usage.children_user + usage.children_system, 1), "loadBefore": load_before,
                  "loadAfter": open("/proc/loadavg").read().split()[:3], "stdoutTail": proc.stdout[-200:]}))
