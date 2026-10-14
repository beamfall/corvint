"""usage: python3 -I stats.py TSV...; median = mean of the two middle values when n is even; p90 = nearest rank ceil(0.9 n)."""
import sys, math, csv
cols = ["pr_ci_wall_s", "queue_ci_wall_s", "pr_ci_runner_s", "queue_ci_runner_s", "ci_yml_runner_s", "all_workflows_runner_s", "open_to_merge_s"]
for path in sys.argv[1:]:
    rows = list(csv.DictReader(open(path), delimiter="\t"))
    print(path, "n=%d" % len(rows), "jobs_not_completed=%d" % sum(int(r["jobs_not_completed"]) for r in rows))
    for c in cols:
        v = sorted(float(r[c]) for r in rows if r[c] not in ("NONE", "NOT_OBSERVED"))
        n = len(v)
        if not n: print(" ", c, "n=0"); continue
        med = v[n // 2] if n % 2 else (v[n // 2 - 1] + v[n // 2]) / 2
        p90 = v[math.ceil(0.9 * n) - 1]
        print("  %-24s n=%2d median=%8.1fs (%6.1f min) p90=%8.1fs (%6.1f min) mean_min=%6.1f" % (c, n, med, med / 60, p90, p90 / 60, sum(v) / n / 60))
