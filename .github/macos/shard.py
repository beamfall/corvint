# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-4625 / V1-4732: cost-place whole Darwin packages and run one shard.

Usage: python3 -B .github/macos/shard.py SELECTION_JSON COSTS_JSON SHARD OUT_DIR
Four shards use the existing Linux cost history only for placement, never selection.
OUT_DIR retains the exact package list, estimates and per-module test logs.
"""
from collections import deque
import json
import os
import pathlib
import re
import subprocess
import sys

SHARDS = 4
PARALLEL = 3
PACKAGE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._~/-]*")


def costs(path):
    try:
        record = json.loads(pathlib.Path(path).read_text())
        weights = record["milliseconds"]
        if (record["profile"] != "corvint-ci-package-costs/0" or not weights
                or not isinstance(weights, dict)
                or any(type(n) is not int or n <= 0 for n in weights.values())):
            return {}
        return weights
    except (OSError, ValueError, KeyError, TypeError):
        return {}


def partition(packages, weights):
    """Same whole-package LPT rule as cishards.Partition; unknowns cost the median.

    Invalid/missing cost history falls back to round robin. Neither path may omit
    a package. Keep expensive packages first for Go's concurrent package runner.
    """
    if len({p["package"] for p in packages}) != len(packages):
        raise ValueError("duplicate selected package")
    ordered = sorted(packages, key=lambda p: p["package"])
    bins, loads = [[] for _ in range(SHARDS)], [0] * SHARDS
    if not weights:
        for i, pkg in enumerate(ordered):
            bins[i % SHARDS].append(pkg)
        return bins, loads
    median = sorted(weights.values())[len(weights) // 2]
    weight = lambda p: weights.get(p["package"], median)
    for pkg in sorted(ordered, key=lambda p: (-weight(p), p["package"])):
        index = min(range(SHARDS), key=lambda i: (loads[i], i))
        bins[index].append(pkg)
        loads[index] += weight(pkg)
    return bins, loads


def module_groups(packages):
    groups = {}
    for pkg in packages:
        name, module = pkg["package"], pkg["module"]
        if (not PACKAGE.fullmatch(name) or ".." in name or "//" in name
                or not module or not (name == module or name.startswith(module + "/"))):
            raise ValueError("unsafe Go package")
        directory = pathlib.Path(pkg["dir"])
        suffix = name[len(module):].strip("/")
        if suffix:
            parts = pathlib.PurePosixPath(suffix).parts
            if directory.parts[-len(parts):] != parts:
                raise ValueError("package directory mismatch")
            directory = directory.parents[len(parts) - 1]
        groups.setdefault(directory, []).append(name)
    return groups


def run(packages, out, root, runner=subprocess.run):
    failed = False
    for i, (directory, names) in enumerate(module_groups(packages).items()):
        command = ["bash", str(root / "script/heavy.sh"), "go", "test", "-json",
                   "-p", str(PARALLEL), "-parallel", "2", "-count=1", "-timeout", "30m", *names]
        env = dict(os.environ, HEAVY_WHOLE_REASON="CIN-V0-4625 native Darwin shard",
                   HEAVY_REMOTE="0", HEAVY_LOCAL_REASON="native Darwin qualification")
        print("Darwin shard module:", directory, "packages:", len(names), flush=True)
        with (out / ("module-%d.jsonl" % i)).open("w") as log:
            result = runner(command, cwd=directory, env=env, stdout=log, stderr=subprocess.STDOUT)
        if result.returncode:
            failed = True
            print("Module failed; full output retained in", log.name, flush=True)
            with pathlib.Path(log.name).open() as retained:
                print("".join(deque(retained, maxlen=40)), flush=True)
    return int(failed)


def main(args):
    selection, cost_file, shard, output = args
    index = int(shard)
    if not 0 <= index < SHARDS:
        raise ValueError("invalid shard index")
    report = json.loads(pathlib.Path(selection).read_text())
    weights = costs(cost_file)
    # Own packages before narrowing: a leg's FULL fallback must not move another
    # leg's affected packages into its shard, where they could be filtered out.
    bins, loads = partition(report["inventory"], weights)
    selected = {p["package"] for p in report["selected"]}
    bins = [[p for p in assigned if p["package"] in selected] for assigned in bins]
    if weights:
        median = sorted(weights.values())[len(weights) // 2]
        loads = [sum(weights.get(p["package"], median) for p in assigned) for assigned in bins]
    out = pathlib.Path(output)
    out.mkdir(parents=True, exist_ok=True)
    (out / "packages.txt").write_text("".join(p["package"] + "\n" for p in bins[index]))
    (out / "estimates.txt").write_text(
        "Linux placement estimates only; Darwin wall time NOT_OBSERVED.\n"
        + "\n".join("shard %d: %d packages, %d ms total" % (i, len(bins[i]), loads[i])
                    for i in range(SHARDS)) + "\n")
    return run(bins[index], out, pathlib.Path(__file__).resolve().parents[2])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
