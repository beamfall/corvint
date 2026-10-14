"""V1-1257 timing rows. usage: python3 -I timing.py DIR SLICE3_PR
DIR holds prs.json (merged PRs in merge order), runs.jsonl, attr.tsv and jobs/<run>.json;
writes timing-before.tsv, timing-after.tsv and timing-runs.tsv into DIR. PRs merged before
SLICE3_PR are the before window, PRs merged after it the after window."""
import json, sys, os
from datetime import datetime
d = sys.argv[1]; slice3 = sys.argv[2]
def ts(s): return datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ").timestamp()
prs = json.load(open(os.path.join(d, "prs.json")))
runs = {}
for line in open(os.path.join(d, "runs.jsonl")):
    r = json.loads(line); runs[str(r["id"])] = r
attr = [l.rstrip("\n").split("\t") for l in open(os.path.join(d, "attr.tsv"))]
def jobs(rid):
    return [json.loads(l) for l in open(os.path.join(d, "jobs", rid + ".json")) if l.strip()]
def run_stats(rid):
    js = jobs(rid); r = runs[rid]
    sec = sum(max(0, ts(j["completed_at"]) - ts(j["started_at"])) for j in js if j.get("started_at") and j.get("completed_at"))
    ends = [ts(j["completed_at"]) for j in js if j.get("completed_at")]
    wall = (max(ends) - ts(r["created_at"])) if ends else None
    incomplete = sum(1 for j in js if j["status"] != "completed")
    return int(sec), (int(wall) if wall is not None else None), len(js), incomplete
order = [p["n"] for p in prs]
i3 = order.index(slice3)
runrows = []
out = {"before": [], "after": []}
for idx, p in enumerate(prs):
    if idx == i3: continue
    win = "before" if idx < i3 else "after"
    mine = [a for a in attr if a[0] == p["n"]]
    tot = ci_tot = 0; inc = 0
    for a in mine:
        sec, wall, nj, ni = run_stats(a[1]); inc += ni
        tot += sec
        if a[2].endswith("/ci.yml"): ci_tot += sec
        runrows.append([win, p["n"], a[1], a[2].split("/")[-1], a[3], a[4], a[5][:12], a[6], a[7], a[8], str(nj), str(sec), "" if wall is None else str(wall)])
    def final(pred):
        c = [a for a in mine if a[2].endswith("/ci.yml") and pred(a)]
        c.sort(key=lambda a: a[6])
        return c[-1][1] if c else None
    prr = final(lambda a: a[3] == "pull_request" and a[5] == p["head"])
    qr = final(lambda a: a[3] == "merge_group" and a[5] == p["merge"])
    def f(rid, k):
        if rid is None: return "NOT_OBSERVED"
        v = run_stats(rid)[k]; return "NOT_OBSERVED" if v is None else str(v)
    out[win].append([p["n"], p["mergedAt"], p["merge"][:12], p["head"][:12],
        prr or "NOT_OBSERVED", f(prr, 1), f(prr, 0),
        qr or "NONE", f(qr, 1) if qr else "NONE", f(qr, 0) if qr else "0",
        str(len(mine)), str(ci_tot), str(tot), str(int(ts(p["mergedAt"]) - ts(p["created"]))), str(inc)])
hdr = ["pr","merged_at","merge_sha","head_sha","final_pr_ci_run","pr_ci_wall_s","pr_ci_runner_s","queue_ci_run","queue_ci_wall_s","queue_ci_runner_s","attributed_runs","ci_yml_runner_s","all_workflows_runner_s","open_to_merge_s","jobs_not_completed"]
for win in out:
    with open(os.path.join(d, "timing-%s.tsv" % win), "w") as fh:
        fh.write("\t".join(hdr) + "\n")
        for r in out[win]: fh.write("\t".join(r) + "\n")
with open(os.path.join(d, "timing-runs.tsv"), "w") as fh:
    fh.write("\t".join(["window","pr","run_id","workflow","event","head_branch","head_sha","created_at","conclusion","run_attempt","jobs","runner_s","wall_s"]) + "\n")
    for r in runrows: fh.write("\t".join(r) + "\n")
