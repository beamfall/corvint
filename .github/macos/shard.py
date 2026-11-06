# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-4625 / V1-4732: cost-place Darwin packages and run one shard.

Usage: python3 -B .github/macos/shard.py SELECTION_JSON COSTS_JSON SHARD OUT_DIR
Four shards use the existing Linux cost history only for placement, never selection.
CIN-V0-4627 / V1-4760: a package too slow for the 30 m per-package hang detector
(decision 0082) is split into deterministic `go test -run` partitions of its listed
top-level tests; it and other heavy packages run in their own invocation.
OUT_DIR retains the exact package list, estimates, partitions and per-invocation test logs.
"""
from collections import deque
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys

SHARDS = 4
PARALLEL = 3
PACKAGE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._~/-]*")
# CIN-V0-4627: Linux history underestimates the 3-core macOS runner (cmd/corvint took
# 1127 s on Linux and exceeded 1800 s on Darwin), so placement scales Linux cost by two.
# Any package whose scaled estimate exceeds half the 30 m detector is split into parts
# that each run alone. The 30 m `-timeout` itself never changes.
DARWIN_SCALE = 2
BUDGET_MS = 15 * 60 * 1000
LISTED = re.compile(r"(Test|Fuzz|Example|Benchmark)\S*")
TEST_NAME = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


def costs(path, *, required=False):
    """Optional callers may omit history; matrix legs require a readable file."""
    try:
        record = json.loads(pathlib.Path(path).read_text())
        weights = record["milliseconds"]
        if (record["profile"] != "corvint-ci-package-costs/0" or not weights
                or not isinstance(weights, dict)
                or any(type(n) is not int or n <= 0 for n in weights.values())):
            return {}
        return weights
    except OSError:
        if required:
            raise
        return {}
    except (ValueError, KeyError, TypeError):
        return {}


def item_cost(item, weights):
    """Linux estimate of one placement item: a whole package or one of its partitions."""
    median = sorted(weights.values())[len(weights) // 2]
    return -(-weights.get(item["package"], median) // item.get("parts", 1))


def items(packages, weights):
    """CIN-V0-4627: split a package whose scaled cost exceeds BUDGET_MS into parts.

    Each part is a copy of the package carrying part/parts and runs in its own
    invocation. Without cost history every package stays whole and grouped.
    """
    result = []
    for pkg in packages:
        weight = item_cost(pkg, weights) if weights else 0
        parts = -(-weight * DARWIN_SCALE // BUDGET_MS)
        if parts > 1:
            result.extend(dict(pkg, part=k, parts=parts) for k in range(parts))
        else:
            result.append(pkg)
    return result


def partition(packages, weights):
    """Same LPT rule as cishards.Partition over items(); unknowns cost the median.

    Invalid/missing cost history falls back to round robin. Neither path may omit
    a package or partition. Keep expensive items first for Go's concurrent package runner.
    """
    if len({p["package"] for p in packages}) != len(packages):
        raise ValueError("duplicate selected package")
    ordered = items(sorted(packages, key=lambda p: p["package"]), weights)
    bins, loads = [[] for _ in range(SHARDS)], [0] * SHARDS
    if not weights:
        for i, pkg in enumerate(ordered):
            bins[i % SHARDS].append(pkg)
        return bins, loads
    weight = lambda p: item_cost(p, weights)
    for pkg in sorted(ordered, key=lambda p: (-weight(p), p["package"], p.get("part", 0))):
        index = min(range(SHARDS), key=lambda i: (loads[i], i))
        bins[index].append(pkg)
        loads[index] += weight(pkg)
    return bins, loads


def assigned_tests(names, part, parts):
    """CIN-V0-4627: every listed name lands in exactly one part, by a stable name hash."""
    return [name for name in sorted(names)
            if int(hashlib.sha256(name.encode()).hexdigest(), 16) % parts == part]


def listed_tests(lines):
    """Top-level names from `go test -list .`; Benchmarks are not run without -bench.

    A name-shaped line that is not a plain identifier or repeats fails closed.
    """
    names, seen = [], set()
    for line in lines:
        text = line.strip()
        if not LISTED.fullmatch(text):
            continue
        if not TEST_NAME.fullmatch(text) or text in seen:
            raise ValueError("unsafe or duplicate listed test %r" % text)
        seen.add(text)
        if not text.startswith("Benchmark"):
            names.append(text)
    return names


def ran_tests(lines, package):
    """Top-level tests of package that reached a pass/fail/skip event in a -json log."""
    ran = set()
    for line in lines:
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if (isinstance(event, dict) and event.get("Package") == package
                and event.get("Action") in ("pass", "fail", "skip")
                and event.get("Test") and "/" not in event["Test"]):
            ran.add(event["Test"])
    return ran


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
    """Each partition alone in its own invocation first, then one invocation per module."""
    heavy = [p for p in packages if "parts" in p]
    grouped = [p for p in packages if "parts" not in p]
    invocations = [(directory, names, item) for item in heavy
                   for directory, names in module_groups([item]).items()]
    invocations += [(d, n, None) for d, n in module_groups(grouped).items()]
    heavy_sh = str(root / "script/heavy.sh")
    # A hosted runner is dedicated to this job, so heavy.sh's host-load queue only adds
    # its 120 s wait to every shard (CIN-V0-4625).
    env = dict(os.environ, HEAVY_WHOLE_REASON="CIN-V0-4625 native Darwin shard",
               HEAVY_REMOTE="0", HEAVY_LOCAL_REASON="native Darwin qualification",
               HEAVY_LOAD_WAIT="0")
    failed = False
    for i, (directory, names, item) in enumerate(invocations):
        selector, assigned = [], None
        if item is not None:
            assigned = partition_tests(item, directory, out / ("module-%d" % i), heavy_sh, env, runner)
            if assigned is None:
                failed = True
                continue
            if not assigned:
                print("Darwin shard partition is empty:", names[0], flush=True)
                continue
            selector = ["-run", "^(%s)$" % "|".join(assigned)]
        command = ["bash", heavy_sh, "go", "test", "-json", "-p", str(PARALLEL), "-parallel", "2",
                   "-count=1", "-timeout", "30m", *selector, *names]
        print("Darwin shard module:", directory, "packages:", len(names),
              "" if item is None else describe(item), flush=True)
        with (out / ("module-%d.jsonl" % i)).open("w") as log:
            result = runner(command, cwd=directory, env=env, stdout=log, stderr=subprocess.STDOUT)
        with pathlib.Path(log.name).open() as retained:
            missing = set()
            if assigned is not None:
                ran = ran_tests(retained, names[0])
                missing = set(assigned) ^ ran
                retained.seek(0)
            if result.returncode or missing:
                failed = True
                print("Module failed; full output retained in", log.name, flush=True)
                if missing:
                    print("Partition tests missing or unassigned:", " ".join(sorted(missing)), flush=True)
                print(failure_summary(retained), flush=True)
    return int(failed)


def describe(item):
    return "partition %d/%d" % (item["part"] + 1, item["parts"]) if "parts" in item else ""


def partition_tests(item, directory, prefix, heavy_sh, env, runner):
    """CIN-V0-4627: list the package's tests and keep this item's part, or None on failure.

    The listing and assignment are retained so every leg's share can be audited.
    """
    package = item["package"]
    listing = prefix.with_suffix(".list.txt")
    with listing.open("w") as log:
        result = runner(["bash", heavy_sh, "go", "test", "-list", ".", package],
                        cwd=directory, env=env, stdout=log, stderr=subprocess.STDOUT)
    try:
        names = listed_tests(listing.read_text().splitlines()) if not result.returncode else []
    except ValueError as error:
        print("Test listing rejected for", package, ":", error, flush=True)
        names = []
    if not names:
        print("Test listing failed or empty for", package, "; output retained in", listing, flush=True)
        with listing.open() as retained:
            print(failure_summary(retained), flush=True)
        return None
    assigned = assigned_tests(names, item["part"], item["parts"])
    prefix.with_suffix(".partition.json").write_text(json.dumps({
        "package": package, "part": item["part"], "parts": item["parts"], "listed": len(names),
        "listedSha256": hashlib.sha256("".join(n + "\n" for n in sorted(names)).encode()).hexdigest(),
        "assigned": assigned}, indent=1) + "\n")
    return assigned


BUILD = "(build)"
# Framing lines that repeat what the FAIL row already says.
NOISE = ("=== RUN", "--- FAIL", "FAIL\t")


def failure_summary(lines, *, per_test=6, limit=60):
    """CIN-V0-4625: name each failed test with its last output lines, not a raw JSON tail."""
    output, failures, other = {}, [], deque(maxlen=20)
    for line in lines:
        try:
            event = json.loads(line)
        except ValueError:
            other.append(line.rstrip("\n"))
            continue
        if not isinstance(event, dict):
            continue
        action = event.get("Action")
        if action in ("build-output", "build-fail"):
            key = (event.get("ImportPath", ""), BUILD)
        else:
            key = (event.get("Package", ""), event.get("Test", ""))
        if action in ("output", "build-output"):
            output.setdefault(key, deque(maxlen=per_test)).append(str(event.get("Output", "")).rstrip("\n"))
        elif action in ("fail", "build-fail") and key not in failures:
            failures.append(key)
    if not failures:
        return "\n".join(other)
    rows = []
    for package, test in failures[:limit]:
        rows.append("FAIL %s %s" % (package, test or "(package)"))
        rows.extend("    " + text.strip() for text in output.get((package, test), ())
                    if text.strip() not in ("", "FAIL") and not text.startswith(NOISE))
    if len(failures) > limit:
        rows.append("... %d more failures in the retained log" % (len(failures) - limit))
    return "\n".join(rows)


def main(args):
    selection, cost_file, shard, output = args
    index = int(shard)
    if not 0 <= index < SHARDS:
        raise ValueError("invalid shard index")
    report = json.loads(pathlib.Path(selection).read_text())
    # A runner-local I/O failure cannot choose a different ownership algorithm
    # from the other legs. Fail this leg instead of silently skipping packages.
    weights = costs(cost_file, required=True)
    # Own packages before narrowing: a leg's FULL fallback must not move another
    # leg's affected packages into its shard, where they could be filtered out.
    bins, loads = partition(report["inventory"], weights)
    selected = {p["package"] for p in report["selected"]}
    bins = [[p for p in assigned if p["package"] in selected] for assigned in bins]
    if weights:
        loads = [sum(item_cost(p, weights) for p in assigned) for assigned in bins]
    out = pathlib.Path(output)
    out.mkdir(parents=True, exist_ok=True)
    (out / "packages.txt").write_text("".join(
        " ".join(filter(None, (p["package"], describe(p)))) + "\n" for p in bins[index]))
    (out / "estimates.txt").write_text(
        "Linux placement estimates only; Darwin wall time NOT_OBSERVED.\n"
        + "\n".join("shard %d: %d items, %d ms total" % (i, len(bins[i]), loads[i])
                    for i in range(SHARDS)) + "\n")
    return run(bins[index], out, pathlib.Path(__file__).resolve().parents[2])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
