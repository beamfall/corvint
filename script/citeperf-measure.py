#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Measure V1-4851/4852 on one committed fixture with two prebuilt binaries.

Usage: python3 script/citeperf-measure.py --before /abs/base-corvint
  --after /abs/tree-corvint --root /abs/fixture --base FULL_SHA
  --citations /abs/cites.tsv --intents /abs/intents.txt --verify /abs/verify.txt
  --out /abs/external-evidence --rounds 3

Runs draft, refresh and inputs-check (prepare, no bind) sequentially, alternating
arm order. Raw output, argv, wall time, host load, return code and digests are
retained. Every invocation uses the same root, HEAD and input files. Review the
results before claiming savings: failed or unequal draft/refresh arms refuse.
Prepare output may name executable identities; its byte parity is unit-tested.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import time


def file_digest(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(65536), b""):
            digest.update(block)
    return digest.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("before", "after", "root", "base", "citations", "intents", "verify", "out"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--rounds", type=int, default=3)
    args = parser.parse_args()
    root, out = Path(args.root).resolve(), Path(args.out).resolve()
    if root == out or root in out.parents or not 1 <= args.rounds <= 20:
        parser.error("--out must be outside --root; --rounds must be 1..20")
    # Refuse an occupied output directory rather than overwrite prior evidence.
    out.mkdir(mode=0o700, parents=True, exist_ok=False)
    inputs = {name: Path(getattr(args, name)).resolve() for name in ("before", "after", "citations", "intents", "verify")}
    pins = {name: file_digest(path) for name, path in inputs.items()}

    def revision():
        return subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()

    head = revision()
    commands = {
        "draft": ["propose-citations", args.base],
        "refresh": ["propose-citations", args.base, "--refresh", str(inputs["citations"])],
        "prepare": ["inputs-check", "--base", args.base, "--citations", str(inputs["citations"]), "--intents", str(inputs["intents"]), "--verify", str(inputs["verify"])],
    }
    report = {"host": platform.node(), "platform": platform.platform(), "head": head, "base": args.base, "input_sha256": pins, "runs": []}
    failed = False
    for round_index in range(args.rounds):
        arms = ("before", "after") if round_index % 2 == 0 else ("after", "before")
        for mode, command in commands.items():
            equality = []
            for arm in arms:
                if revision() != head or any(file_digest(path) != pins[name] for name, path in inputs.items()):
                    raise SystemExit("fixture HEAD or pinned input changed; results are not comparable")
                argv = [str(inputs[arm]), "--root", str(root), "dogfood", *command]
                stem = out / f"{round_index + 1}-{mode}-{arm}"
                load = list(os.getloadavg()) if hasattr(os, "getloadavg") else "NOT_OBSERVED"
                start = time.perf_counter()
                with stem.with_suffix(".stdout").open("wb") as stdout, stem.with_suffix(".stderr").open("wb") as stderr:
                    result = subprocess.run(argv, stdout=stdout, stderr=stderr, check=False, env={**os.environ, "LC_ALL": "C"})
                elapsed = time.perf_counter() - start
                output_pins = [file_digest(stem.with_suffix(suffix)) for suffix in (".stdout", ".stderr")]
                report["runs"].append({"round": round_index + 1, "mode": mode, "arm": arm, "argv": argv, "load_before": load, "wall_seconds": elapsed, "exit": result.returncode, "stdout_sha256": output_pins[0], "stderr_sha256": output_pins[1]})
                failed |= result.returncode != 0
                equality.append(output_pins)
                (out / "measurements.json").write_text(json.dumps(report, indent=2) + "\n")
            if mode != "prepare" and equality[0] != equality[1]:
                failed = True
                print(f"{mode} round {round_index + 1}: unequal stdout/stderr; no parity claim")
    print(out / "measurements.json")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
